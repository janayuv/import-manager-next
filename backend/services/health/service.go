package health

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/backup"
	"github.com/janayuv/import-manager-next/backend/services/google_drive"
	"github.com/janayuv/import-manager-next/backend/services/mobile_sync"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/workflow"
	"github.com/janayuv/import-manager-next/backend/windows"
)

// JobHealthSummary reports detailed state for an individual scheduled background job.
type JobHealthSummary struct {
	JobID              string `json:"jobId"`
	JobName            string `json:"jobName"`
	IsEnabled          bool   `json:"isEnabled"`
	LastStatus         string `json:"lastStatus"`
	LastStartedAt      string `json:"lastStartedAt"`
	LastCompletedAt    string `json:"lastCompletedAt"`
	ExpectedDurationMs int64  `json:"expectedDurationMs"`
	IsStuck            bool   `json:"isStuck"`
	ErrorMessage       string `json:"errorMessage,omitempty"`
}

// SystemHealthSummary aggregates all 12 system health and diagnostic indicators.
type SystemHealthSummary struct {
	OverallStatus           string `json:"overallStatus"` // "HEALTHY" | "WARNING" | "CRITICAL" | "UNKNOWN"
	EvaluatedAt             string `json:"evaluatedAt"`

	// 1. Database & SQLCipher
	DatabaseIntegrity       string `json:"databaseIntegrity"` // "OK" | "WARNING" | "CORRUPTED"
	LastIntegrityCheckAt    string `json:"lastIntegrityCheckAt"`
	ForeignKeysEnforced     bool   `json:"foreignKeysEnforced"`
	SQLCipherConnected      bool   `json:"sqlCipherConnected"`
	DiskHeaderEncrypted     bool   `json:"diskHeaderEncrypted"`
	CipherVersion           string `json:"cipherVersion"`
	DatabasePath            string `json:"databasePath"`
	DatabaseSizeBytes       int64  `json:"databaseSizeBytes"`
	DatabasePageCount       int64  `json:"databasePageCount"`
	SchemaVersion           int    `json:"schemaVersion"`

	// 2. Storage
	DatabaseDiskTotalBytes  uint64 `json:"databaseDiskTotalBytes"`
	DatabaseDiskFreeBytes   uint64 `json:"databaseDiskFreeBytes"`
	BackupDiskTotalBytes    uint64 `json:"backupDiskTotalBytes"`
	BackupDiskFreeBytes     uint64 `json:"backupDiskFreeBytes"`
	LowDiskWarning          bool   `json:"lowDiskWarning"` // < 2 GB free

	// 3. Application Process Memory
	MemoryWorkingSetBytes   uint64 `json:"memoryWorkingSetBytes"`
	MemoryPrivateUsageBytes uint64 `json:"memoryPrivateUsageBytes"`
	GoHeapAllocBytes        uint64 `json:"goHeapAllocBytes"`
	GoHeapSysBytes          uint64 `json:"goHeapSysBytes"`

	// 4. Background Job Engine
	ActiveJobsCount         int                `json:"activeJobsCount"`
	RunningJobsCount        int                `json:"runningJobsCount"`
	StuckJobsCount          int                `json:"stuckJobsCount"`
	FailedJobs24hCount      int                `json:"failedJobs24hCount"`
	JobSummaries            []JobHealthSummary `json:"jobSummaries"`

	// 5. Encrypted Backup Verification
	BackupReadinessStatus   string  `json:"backupReadinessStatus"` // "HEALTHY" | "WARNING" | "CRITICAL"
	TotalBackupsCount       int     `json:"totalBackupsCount"`
	VerifiedBackupsCount    int     `json:"verifiedBackupsCount"`
	LastSuccessfulBackupAt  string  `json:"lastSuccessfulBackupAt"`
	LastSuccessfulFilename  string  `json:"lastSuccessfulFilename"`
	LastTestedRestoreAt     string  `json:"lastTestedRestoreAt"`
	HoursSinceLastBackup    float64 `json:"hoursSinceLastBackup"`

	// 6. Companion & Remote Synchronizations
	MobileSyncRunning       bool   `json:"mobileSyncRunning"`
	MobileSyncPort          int    `json:"mobileSyncPort"`
	MobileConnectedDevices  int    `json:"mobileConnectedDevices"`
	GoogleDriveConnected    bool   `json:"googleDriveConnected"`
	GoogleDriveEmailMasked  string `json:"googleDriveEmailMasked"`
	GoogleDriveTokenExpiry  string `json:"googleDriveTokenExpiry"`
}

// Service provides unified, non-blocking system health inspection and safe maintenance actions.
type Service struct {
	mu                   sync.RWMutex
	db                   *sql.DB
	dbMgr                *connection.Manager
	dbPath               string
	sec                  *security.Service
	backupSvc            *backup.Service
	workflowSvc          *workflow.Service
	mobileSyncSvc        *mobile_sync.Service
	googleDriveSvc       *google_drive.Service
	paths                windows.AppPaths
	lastIntegrityCheckAt string
	lastIntegrityResult  string
}

// NewService constructs a centralized System Health Service.
func NewService(
	db *sql.DB,
	dbMgr *connection.Manager,
	dbPath string,
	sec *security.Service,
	backupSvc *backup.Service,
	workflowSvc *workflow.Service,
	mobileSyncSvc *mobile_sync.Service,
	googleDriveSvc *google_drive.Service,
	paths windows.AppPaths,
) *Service {
	return &Service{
		db:                  db,
		dbMgr:               dbMgr,
		dbPath:              dbPath,
		sec:                 sec,
		backupSvc:           backupSvc,
		workflowSvc:         workflowSvc,
		mobileSyncSvc:       mobileSyncSvc,
		googleDriveSvc:      googleDriveSvc,
		paths:               paths,
		lastIntegrityResult: "OK",
	}
}

// SetDB rebinds the service on company/database switch.
func (s *Service) SetDB(db *sql.DB, dbMgr *connection.Manager, dbPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = db
	s.dbMgr = dbMgr
	s.dbPath = dbPath
}

// maskEmail obscures an email for safe display (e.g. j***v@example.com).
func maskEmail(email string) string {
	parts := strings.Split(strings.TrimSpace(email), "@")
	if len(parts) != 2 || len(parts[0]) == 0 {
		return ""
	}
	name := parts[0]
	domain := parts[1]
	if len(name) <= 2 {
		return name[:1] + "***@" + domain
	}
	return name[:1] + "***" + name[len(name)-1:] + "@" + domain
}

// GetHealthSummary gathers a comprehensive diagnostic view without running heavy checks.
func (s *Service) GetHealthSummary(ctx context.Context) (SystemHealthSummary, error) {
	if s.sec != nil {
		if _, err := s.sec.RequireAnyPermission(security.PermAdminSystemHealth, security.PermAdminSystemTools, security.PermDataRead); err != nil {
			return SystemHealthSummary{}, err
		}
	}

	s.mu.RLock()
	db := s.db
	dbPath := s.dbPath
	lastIntegAt := s.lastIntegrityCheckAt
	lastIntegRes := s.lastIntegrityResult
	s.mu.RUnlock()

	nowStr := time.Now().UTC().Format(time.RFC3339)
	res := SystemHealthSummary{
		EvaluatedAt:          nowStr,
		DatabasePath:         dbPath,
		DatabaseIntegrity:    lastIntegRes,
		LastIntegrityCheckAt: lastIntegAt,
		OverallStatus:        "HEALTHY",
	}

	// 1. Database & SQLCipher Configuration
	if db != nil {
		var cipherVer string
		_ = db.QueryRowContext(ctx, "PRAGMA cipher_version;").Scan(&cipherVer)
		res.CipherVersion = cipherVer
		res.SQLCipherConnected = cipherVer != ""

		fkOn, _ := connection.VerifyForeignKeysActive(db)
		res.ForeignKeysEnforced = fkOn
		if !fkOn {
			res.OverallStatus = "WARNING"
		}

		var pageCount, pageSize int64
		_ = db.QueryRowContext(ctx, "PRAGMA page_count;").Scan(&pageCount)
		_ = db.QueryRowContext(ctx, "PRAGMA page_size;").Scan(&pageSize)
		res.DatabasePageCount = pageCount

		var schemaVer int
		_ = db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM next_schema_migrations WHERE status = 'APPLIED'").Scan(&schemaVer)
		res.SchemaVersion = schemaVer
	}

	// Disk header check
	if dbPath != "" {
		if info, err := os.Stat(dbPath); err == nil {
			res.DatabaseSizeBytes = info.Size()
		}
		enc, _ := connection.VerifyDiskHeaderEncrypted(dbPath)
		res.DiskHeaderEncrypted = enc
		if !enc {
			res.OverallStatus = "WARNING"
		}
	}

	// 2. Storage Diagnostics (Dual Drive Probes)
	if s.paths.DataDir != "" {
		if ds, err := windows.GetDiskSpace(s.paths.DataDir); err == nil {
			res.DatabaseDiskTotalBytes = ds.TotalNumberOfBytes
			res.DatabaseDiskFreeBytes = ds.FreeBytesAvailable
			if ds.FreeBytesAvailable < 2*1024*1024*1024 { // < 2 GB
				res.LowDiskWarning = true
				res.OverallStatus = "WARNING"
			}
		}
	}
	if s.paths.BackupsDir != "" {
		if ds, err := windows.GetDiskSpace(s.paths.BackupsDir); err == nil {
			res.BackupDiskTotalBytes = ds.TotalNumberOfBytes
			res.BackupDiskFreeBytes = ds.FreeBytesAvailable
			if ds.FreeBytesAvailable < 2*1024*1024*1024 {
				res.LowDiskWarning = true
				res.OverallStatus = "WARNING"
			}
		}
	}

	// 3. Memory Metrics
	if mem, err := windows.GetOSMemoryStats(); err == nil {
		res.MemoryWorkingSetBytes = mem.WorkingSetBytes
		res.MemoryPrivateUsageBytes = mem.PrivateUsageBytes
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	res.GoHeapAllocBytes = m.Alloc
	res.GoHeapSysBytes = m.Sys

	// 4. Background Job Engine & Stuck Job Heartbeat
	if s.workflowSvc != nil && db != nil {
		jobs, err := s.workflowSvc.ListBackgroundJobs()
		if err == nil {
			nowTime := time.Now().UTC()
			for _, j := range jobs {
				if j.IsEnabled {
					res.ActiveJobsCount++
				}
				if j.LastStatus == "RUNNING" {
					res.RunningJobsCount++
				}

				isStuck := false
				if j.LastStatus == "RUNNING" && j.LastStartedAt != "" {
					if t, parseErr := time.Parse("2006-01-02 15:04:05", j.LastStartedAt); parseErr == nil {
						timeout := time.Duration(j.ExpectedDurationMs*2) * time.Millisecond
						if timeout < 15*time.Minute {
							timeout = 15 * time.Minute
						}
						if nowTime.Sub(t) > timeout {
							isStuck = true
							res.StuckJobsCount++
							res.OverallStatus = "WARNING"
						}
					}
				}

				res.JobSummaries = append(res.JobSummaries, JobHealthSummary{
					JobID:              j.JobID,
					JobName:            j.JobName,
					IsEnabled:          j.IsEnabled,
					LastStatus:         j.LastStatus,
					LastStartedAt:      j.LastStartedAt,
					LastCompletedAt:    j.LastCompletedAt,
					ExpectedDurationMs: j.ExpectedDurationMs,
					IsStuck:            isStuck,
				})
			}
		}

		var failed24h int
		_ = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM workflow_job_execution_log
			WHERE status IN ('FAILED', 'TIMEOUT')
			  AND datetime(started_at) > datetime('now', '-24 hours')`).Scan(&failed24h)
		res.FailedJobs24hCount = failed24h
		if failed24h > 5 {
			res.OverallStatus = "WARNING"
		}
	}

	// 5. Encrypted Backup Readiness
	if s.backupSvc != nil {
		bHealth, err := s.backupSvc.GetBackupHealth(ctx)
		if err == nil {
			res.BackupReadinessStatus = bHealth.Status
			res.TotalBackupsCount = bHealth.TotalBackups
			res.VerifiedBackupsCount = bHealth.VerifiedBackups
			res.LastSuccessfulBackupAt = bHealth.LastBackupTime
			res.LastSuccessfulFilename = bHealth.LastBackupFilename
			res.HoursSinceLastBackup = bHealth.HoursSinceLast

			if bHealth.Status == "CRITICAL" {
				res.OverallStatus = "CRITICAL"
			} else if bHealth.Status == "WARNING" && res.OverallStatus != "CRITICAL" {
				res.OverallStatus = "WARNING"
			}
		}

		// Check last tested restore timestamp if any
		var lastRestore string
		_ = db.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(created_at), '') FROM audit_logs
			WHERE action = 'RESTORE_BACKUP' AND outcome = 'SUCCESS'`).Scan(&lastRestore)
		res.LastTestedRestoreAt = lastRestore
	}

	// 6. Companion & Remote Synchronizations
	if s.mobileSyncSvc != nil {
		if sInfo, err := s.mobileSyncSvc.GetServerInfo(); err == nil {
			res.MobileSyncRunning = sInfo.IsRunning
			res.MobileSyncPort = sInfo.Port
			res.MobileConnectedDevices = s.mobileSyncSvc.GetActiveDeviceCount()
		}
	}
	if s.googleDriveSvc != nil {
		if gdStatus, err := s.googleDriveSvc.GetStatus(); err == nil {
			res.GoogleDriveConnected = gdStatus.Connected
			res.GoogleDriveEmailMasked = maskEmail(gdStatus.AccountEmail)
			res.GoogleDriveTokenExpiry = gdStatus.TokenExpiry
		}
	}

	return res, nil
}

// RunManualIntegrityCheck performs an explicit PRAGMA integrity_check and stores result.
func (s *Service) RunManualIntegrityCheck(ctx context.Context) (string, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermAdminSystemTools); err != nil {
			return "", err
		}
	}

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return "", errors.New("database not open")
	}

	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check;").Scan(&result); err != nil {
		return "", fmt.Errorf("integrity check failed: %w", err)
	}

	s.mu.Lock()
	s.lastIntegrityCheckAt = time.Now().UTC().Format(time.RFC3339)
	s.lastIntegrityResult = result
	s.mu.Unlock()

	return result, nil
}

// RunVerifiedBackup triggers an immediate on-demand backup with verification.
func (s *Service) RunVerifiedBackup(ctx context.Context) (models.BackupRecord, error) {
	if s.backupSvc == nil {
		return models.BackupRecord{}, errors.New("backup service not available")
	}
	req := models.CreateBackupRequest{
		Notes: "Manual backup executed from System Health Dashboard",
	}
	return s.backupSvc.CreateBackup(req)
}

// RunIdempotentJobRecovery handles stuck background jobs cleanly.
func (s *Service) RunIdempotentJobRecovery(ctx context.Context, jobID string) error {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermAutomationOpsCenter); err != nil {
			return err
		}
	}

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return errors.New("database not open")
	}

	cleanJobID := strings.TrimSpace(jobID)
	if cleanJobID == "" {
		return errors.New("jobID cannot be empty")
	}

	// Mark older stuck executions as TIMEOUT so they are not left in RUNNING forever
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.ExecContext(ctx, `
		UPDATE workflow_job_execution_log
		SET status = 'TIMEOUT', completed_at = ?, error_message = 'Job execution marked as timed out by admin health recovery'
		WHERE job_id = ? AND status = 'RUNNING'`, nowStr, cleanJobID)
	if err != nil {
		return fmt.Errorf("timeout stuck job execution: %w", err)
	}

	// Trigger a clean execution
	if s.workflowSvc != nil {
		_, runErr := s.workflowSvc.RunBackgroundJobNow(cleanJobID)
		return runErr
	}
	return nil
}
