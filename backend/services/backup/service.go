package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/crypto"
	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service manages IMBK2 encrypted backups, IMBK2/IMBK1 restore workflows, .imkey export/import, and schedules.
type Service struct {
	db          *sql.DB
	security    *security.Service
	audit       *audit.Service
	secrets     crypto.SecretStore
	restoreCtrl *connection.RestoreControl
	backupsDir  string
	tempDir     string
	dbKey       string
}

// NewService constructs a Backup Service instance.
func NewService(
	db *sql.DB,
	sec *security.Service,
	aud *audit.Service,
	secrets crypto.SecretStore,
	restoreCtrl *connection.RestoreControl,
	backupsDir string,
	tempDir string,
	dbKey string,
) *Service {
	return &Service{
		db:          db,
		security:    sec,
		audit:       aud,
		secrets:     secrets,
		restoreCtrl: restoreCtrl,
		backupsDir:  backupsDir,
		tempDir:     tempDir,
		dbKey:       dbKey,
	}
}

func (s *Service) resolvePassphrase(supplied string) (string, error) {
	if trimmed := strings.TrimSpace(supplied); trimmed != "" {
		return trimmed, nil
	}
	if s.secrets != nil {
		if saved, err := s.secrets.GetSecret(crypto.BackupPassphraseAccount); err == nil && strings.TrimSpace(saved) != "" {
			return saved, nil
		}
	}
	return "", errors.New("backup passphrase is required (supply one or save it in credential manager)")
}

// SaveBackupPassphrase stores the default backup passphrase in Windows Credential Manager.
func (s *Service) SaveBackupPassphrase(passphrase string) error {
	sess, err := s.security.RequirePermission(security.PermBackupSettings)
	if err != nil {
		return err
	}
	if strings.TrimSpace(passphrase) == "" {
		return errors.New("passphrase cannot be empty")
	}
	if s.secrets == nil {
		return errors.New("credential store unavailable")
	}
	if err := s.secrets.SetSecret(crypto.BackupPassphraseAccount, passphrase); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("backup_settings", "passphrase", "UPDATE", sess.UserID, nil, map[string]string{"saved": "true"}, nil)
	return nil
}

// HasSavedBackupPassphrase reports whether a default backup passphrase exists in Credential Manager.
func (s *Service) HasSavedBackupPassphrase() (bool, error) {
	if _, err := s.security.RequireAnyPermission(security.PermBackupCreate, security.PermBackupRestore, security.PermBackupSettings); err != nil {
		return false, err
	}
	if s.secrets == nil {
		return false, nil
	}
	val, err := s.secrets.GetSecret(crypto.BackupPassphraseAccount)
	return err == nil && strings.TrimSpace(val) != "", nil
}

// ClearBackupPassphrase removes any saved backup passphrase from Credential Manager.
func (s *Service) ClearBackupPassphrase() error {
	sess, err := s.security.RequirePermission(security.PermBackupSettings)
	if err != nil {
		return err
	}
	if s.secrets == nil {
		return nil
	}
	if err := s.secrets.DeleteSecret(crypto.BackupPassphraseAccount); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("backup_settings", "passphrase", "DELETE", sess.UserID, nil, nil, nil)
	return nil
}

// CreateBackup builds an atomic hot snapshot via VACUUM INTO, encrypts it as IMBK2 + .sha256, and logs it.
func (s *Service) CreateBackup(req models.CreateBackupRequest) (models.BackupRecord, error) {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermBackupCreate)
	if err != nil {
		return models.BackupRecord{}, err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return models.BackupRecord{}, err
	}
	passphrase, err := s.resolvePassphrase(req.Passphrase)
	if err != nil {
		return models.BackupRecord{}, err
	}
	dest := strings.TrimSpace(req.Destination)
	if dest == "" {
		dest = "local"
	}
	outDir := strings.TrimSpace(req.OutputDir)
	if outDir == "" {
		outDir = s.backupsDir
	}

	rec, err := s.createEncryptedBackupInternal(ctx, sess.UserID, passphrase, dest, outDir, "import_manager_backup", req.Notes, true)
	if err != nil {
		return models.BackupRecord{}, err
	}

	if req.SavePassToKey && s.secrets != nil && strings.TrimSpace(req.Passphrase) != "" {
		_ = s.secrets.SetSecret(crypto.BackupPassphraseAccount, req.Passphrase)
	}

	_ = s.audit.RecordTableAudit("backups", fmt.Sprintf("%d", rec.ID), "CREATE", sess.UserID, nil, rec, nil)
	return rec, nil
}

// CreateSafetySnapshot creates a mandatory pre-operation encrypted safety backup on disk (usable by Restore & Migration).
func (s *Service) CreateSafetySnapshot(ctx context.Context, actor, passphrase, notes string) (models.BackupRecord, error) {
	if strings.TrimSpace(passphrase) == "" {
		if s.secrets != nil {
			if saved, err := s.secrets.GetSecret(crypto.BackupPassphraseAccount); err == nil && saved != "" {
				passphrase = saved
			}
		}
	}
	if strings.TrimSpace(passphrase) == "" {
		passphrase = s.dbKey
	}
	return s.createEncryptedBackupInternal(ctx, actor, passphrase, "local", s.backupsDir, "safety_backup", notes, false)
}

func (s *Service) createEncryptedBackupInternal(
	ctx context.Context,
	actor string,
	passphrase string,
	destination string,
	outDir string,
	prefix string,
	notes string,
	recordInTable bool,
) (models.BackupRecord, error) {
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return models.BackupRecord{}, fmt.Errorf("prepare backup directory: %w", err)
	}
	if err := os.MkdirAll(s.tempDir, 0o700); err != nil {
		return models.BackupRecord{}, fmt.Errorf("prepare temp directory: %w", err)
	}

	stamp := time.Now().UTC().Format("20060102_150405.000000000")
	stamp = strings.ReplaceAll(stamp, ".", "_")
	snapPath := filepath.Join(s.tempDir, fmt.Sprintf("%s_snap_%s.db", prefix, stamp))
	_ = os.Remove(snapPath)
	defer os.Remove(snapPath)

	if err := s.exportConsistentSnapshot(ctx, snapPath); err != nil {
		return models.BackupRecord{}, err
	}

	snapBytes, err := os.ReadFile(snapPath)
	if err != nil {
		return models.BackupRecord{}, fmt.Errorf("read snapshot payload: %w", err)
	}

	encBytes, err := crypto.EncryptBackupIMBK2(snapBytes, passphrase)
	if err != nil {
		return models.BackupRecord{}, err
	}

	filename := fmt.Sprintf("%s_%s.imbk2", prefix, stamp)
	targetPath := filepath.Join(outDir, filename)
	if err := os.WriteFile(targetPath, encBytes, 0o600); err != nil {
		return models.BackupRecord{}, fmt.Errorf("write encrypted backup file: %w", err)
	}
	sumHex, err := crypto.WriteSidecarSHA256(targetPath, encBytes)
	if err != nil {
		return models.BackupRecord{}, err
	}

	sizeBytes := int64(len(encBytes))
	createdAt := time.Now().UTC().Format(time.RFC3339)

	rec := models.BackupRecord{
		Filename:      filename,
		Path:          targetPath,
		Destination:   destination,
		SizeBytes:     sizeBytes,
		SHA256:        sumHex,
		CreatedBy:     actor,
		CreatedAt:     createdAt,
		Notes:         notes,
		Status:        "completed",
		SidecarExists: true,
	}

	if recordInTable {
		id, err := s.insertBackupRow(ctx, rec)
		if err != nil {
			return models.BackupRecord{}, err
		}
		rec.ID = id
	}
	return rec, nil
}

func (s *Service) exportConsistentSnapshot(ctx context.Context, snapPath string) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire database connection: %w", err)
	}
	defer conn.Close()

	_, _ = conn.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE);`)
	escapedSnap := strings.ReplaceAll(snapPath, "'", "''")

	attachSQL := fmt.Sprintf(`ATTACH DATABASE '%s' AS backup_snap KEY '';`, escapedSnap)
	if _, err := conn.ExecContext(ctx, attachSQL); err == nil {
		defer func() {
			_, _ = conn.ExecContext(context.Background(), `DETACH DATABASE backup_snap;`)
		}()
		if _, expErr := conn.ExecContext(ctx, `SELECT sqlcipher_export('backup_snap');`); expErr != nil {
			return fmt.Errorf("export consistent database snapshot: %w", expErr)
		}
		_, _ = conn.ExecContext(ctx, `DETACH DATABASE backup_snap;`)
		return nil
	}

	// Fallback to VACUUM INTO if ATTACH fails (e.g. non-sqlcipher environment)
	if _, vacErr := conn.ExecContext(ctx, fmt.Sprintf(`VACUUM INTO '%s';`, escapedSnap)); vacErr != nil {
		return fmt.Errorf("create consistent database snapshot: %w", vacErr)
	}
	return nil
}

func (s *Service) insertBackupRow(ctx context.Context, rec models.BackupRecord) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO backups (filename, path, destination, size_bytes, sha256, created_by, notes, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Filename, rec.Path, rec.Destination, rec.SizeBytes, rec.SHA256, rec.CreatedBy, rec.Notes, rec.Status,
	)
	if err != nil {
		return 0, fmt.Errorf("record backup metadata: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// ListBackups lists local and synced backup metadata ordered newest-first.
func (s *Service) ListBackups() ([]models.BackupRecord, error) {
	if _, err := s.security.RequireAnyPermission(security.PermBackupCreate, security.PermBackupRestore); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, filename, path, COALESCE(destination, 'local'), COALESCE(size_bytes, 0),
		        COALESCE(sha256, ''), COALESCE(created_by, ''), COALESCE(created_at, ''),
		        COALESCE(notes, ''), COALESCE(status, 'completed'), COALESCE(error_message, '')
		 FROM backups ORDER BY id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	defer rows.Close()

	var out []models.BackupRecord
	for rows.Next() {
		var r models.BackupRecord
		if err := rows.Scan(&r.ID, &r.Filename, &r.Path, &r.Destination, &r.SizeBytes, &r.SHA256, &r.CreatedBy, &r.CreatedAt, &r.Notes, &r.Status, &r.ErrorMessage); err != nil {
			return nil, err
		}
		if r.Path != "" {
			if _, statErr := os.Stat(r.Path + ".sha256"); statErr == nil {
				r.SidecarExists = true
			}
		}
		out = append(out, r)
	}
	if out == nil {
		out = []models.BackupRecord{}
	}
	return out, rows.Err()
}

// DeleteBackupRecord deletes a backup metadata entry and optionally removes disk artifacts.
func (s *Service) DeleteBackupRecord(id int64, deleteFile bool) error {
	sess, err := s.security.RequirePermission(security.PermBackupCreate)
	if err != nil {
		return err
	}
	var path string
	if err := s.db.QueryRow(`SELECT path FROM backups WHERE id = ?`, id).Scan(&path); err != nil {
		return fmt.Errorf("backup %d not found: %w", id, err)
	}
	if deleteFile && path != "" {
		_ = os.Remove(path)
		_ = os.Remove(path + ".sha256")
	}
	if _, err := s.db.Exec(`DELETE FROM backups WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete backup record: %w", err)
	}
	_ = s.audit.RecordTableAudit("backups", fmt.Sprintf("%d", id), "DELETE", sess.UserID, map[string]string{"path": path}, nil, nil)
	return nil
}

// RestoreBackup validates .sha256 sidecar, decrypts IMBK2 or legacy IMBK1, checks integrity & FKs,
// and performs atomic table replacement under a global restore_control lock (or dry-run simulation).
func (s *Service) RestoreBackup(req models.RestoreBackupRequest) (models.RestoreVerificationSummary, error) {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermBackupRestore)
	if err != nil {
		return models.RestoreVerificationSummary{}, err
	}
	if strings.TrimSpace(req.BackupPath) == "" {
		return models.RestoreVerificationSummary{}, errors.New("backup path is required")
	}
	passphrase, err := s.resolvePassphrase(req.Passphrase)
	if err != nil {
		return models.RestoreVerificationSummary{}, err
	}

	sidecarPath := req.BackupPath + ".sha256"
	sidecarVerified := false
	if _, statErr := os.Stat(sidecarPath); statErr == nil || req.VerifySidecar {
		if _, err := crypto.VerifyFileSidecarSHA256(req.BackupPath); err != nil {
			return models.RestoreVerificationSummary{}, fmt.Errorf("backup integrity check failed: %w", err)
		}
		sidecarVerified = true
	}

	encBytes, err := os.ReadFile(req.BackupPath)
	if err != nil {
		return models.RestoreVerificationSummary{}, fmt.Errorf("read backup file: %w", err)
	}

	plainDB, format, err := crypto.DecryptBackup(encBytes, passphrase)
	if err != nil {
		return models.RestoreVerificationSummary{}, err
	}

	if err := os.MkdirAll(s.tempDir, 0o700); err != nil {
		return models.RestoreVerificationSummary{}, err
	}
	tempPath := filepath.Join(s.tempDir, fmt.Sprintf("restore_stage_%d.db", time.Now().UnixNano()))
	defer os.Remove(tempPath)
	if err := os.WriteFile(tempPath, plainDB, 0o600); err != nil {
		return models.RestoreVerificationSummary{}, fmt.Errorf("stage decrypted backup: %w", err)
	}

	isPlain, _, err := transactions.DetectSQLitePlaintext(tempPath)
	if err != nil {
		return models.RestoreVerificationSummary{}, err
	}
	attachKey := s.dbKey
	if isPlain {
		attachKey = ""
	}
	inspect, err := transactions.InspectExternalDB(tempPath, attachKey)
	if err != nil {
		return models.RestoreVerificationSummary{}, err
	}
	if !inspect.KeyValid && !isPlain {
		altInspect, altErr := transactions.InspectExternalDB(tempPath, passphrase)
		if altErr == nil && altInspect.KeyValid {
			inspect = altInspect
			attachKey = passphrase
		} else if envSrcKey := strings.TrimSpace(os.Getenv("IMPORT_MANAGER_SOURCE_KEY")); envSrcKey != "" {
			envInspect, envErr := transactions.InspectExternalDB(tempPath, envSrcKey)
			if envErr == nil && envInspect.KeyValid {
				inspect = envInspect
				attachKey = envSrcKey
			}
		}
	}
	if !inspect.KeyValid {
		return models.RestoreVerificationSummary{}, errors.New("decrypted payload could not be opened as a valid SQLite/SQLCipher database (the payload may be an encrypted snapshot from another machine requiring the original database key)")
	}
	if strings.ToLower(inspect.IntegrityCheck) != "ok" {
		return models.RestoreVerificationSummary{}, fmt.Errorf("staged backup failed integrity_check: %s", inspect.IntegrityCheck)
	}
	if inspect.ForeignKeyErrors > 0 {
		return models.RestoreVerificationSummary{}, fmt.Errorf("staged backup failed foreign_key_check (%d violations)", inspect.ForeignKeyErrors)
	}
	for _, coreTbl := range []string{"suppliers", "items", "shipments", "invoices", "app_settings"} {
		if _, ok := inspect.TableCounts[coreTbl]; !ok {
			return models.RestoreVerificationSummary{}, fmt.Errorf("backup archive is missing required ERP table %q", coreTbl)
		}
	}

	var totalRows int64
	for _, c := range inspect.TableCounts {
		totalRows += c
	}

	summary := models.RestoreVerificationSummary{
		DryRun:           req.DryRun,
		SourcePath:       req.BackupPath,
		BackupFormat:     format,
		PayloadEncrypted: !isPlain,
		SidecarVerified:  sidecarVerified,
		IntegrityCheck:   inspect.IntegrityCheck,
		ForeignKeysOK:    inspect.ForeignKeyErrors == 0,
		TableCounts:      inspect.TableCounts,
		TotalRows:        totalRows,
	}

	if req.DryRun {
		return summary, nil
	}

	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return models.RestoreVerificationSummary{}, err
	}
	s.restoreCtrl.SetRestoring(true, "Restoring encrypted backup "+filepath.Base(req.BackupPath))
	defer s.restoreCtrl.SetRestoring(false, "")

	safetyRec, err := s.CreateSafetySnapshot(ctx, sess.UserID, passphrase, "Pre-restore safety backup before restoring "+filepath.Base(req.BackupPath))
	if err != nil {
		return models.RestoreVerificationSummary{}, fmt.Errorf("create pre-restore safety backup: %w", err)
	}
	summary.SafetyBackupPath = safetyRec.Path

	copyRes, err := transactions.CopyAttachedTables(ctx, s.db, tempPath, attachKey, isPlain, transactions.CopyAttachedOptions{
		ReplaceExisting: true,
		ExcludeTables:   []string{"next_schema_migrations", "schema_migrations", "backups"},
	})
	if err != nil {
		return models.RestoreVerificationSummary{}, fmt.Errorf("apply restored database state: %w", err)
	}
	if copyRes.ForeignKeyErrors > 0 || strings.ToLower(copyRes.IntegrityCheck) != "ok" {
		return models.RestoreVerificationSummary{}, fmt.Errorf("post-restore validation failed (integrity=%s, fkErrors=%d)", copyRes.IntegrityCheck, copyRes.ForeignKeyErrors)
	}

	_, _ = s.insertBackupRow(ctx, safetyRec)
	summary.RestoredAt = time.Now().UTC().Format(time.RFC3339)

	_ = s.audit.RecordTableAudit("backups", "restore", "RESTORE", sess.UserID, nil, summary, nil)
	return summary, nil
}

// ExportIMKey exports the active backup passphrase encrypted in IMKEY1 format wrapped with transportPassphrase.
func (s *Service) ExportIMKey(outputPath, backupPassphrase, transportPassphrase string) (models.IMKeyOperationResult, error) {
	sess, err := s.security.RequirePermission(security.PermBackupSettings)
	if err != nil {
		return models.IMKeyOperationResult{}, err
	}
	resolvedPass, err := s.resolvePassphrase(backupPassphrase)
	if err != nil {
		return models.IMKeyOperationResult{}, err
	}
	if strings.TrimSpace(outputPath) == "" {
		outputPath = filepath.Join(s.backupsDir, fmt.Sprintf("import_manager_key_%s.imkey", time.Now().UTC().Format("20060102_150405")))
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return models.IMKeyOperationResult{}, err
	}

	hint := ""
	if len(s.dbKey) >= 8 {
		hint = s.dbKey[:8] + "..."
	}
	now := time.Now().UTC().Format(time.RFC3339)
	blob, err := crypto.EncryptIMKey(transportPassphrase, crypto.IMKeyPayload{
		Version:          1,
		ExportedAt:       now,
		BackupPassphrase: resolvedPass,
		DatabaseKeyHint:  hint,
	})
	if err != nil {
		return models.IMKeyOperationResult{}, err
	}
	if err := os.WriteFile(outputPath, blob, 0o600); err != nil {
		return models.IMKeyOperationResult{}, fmt.Errorf("write .imkey file: %w", err)
	}
	_, _ = crypto.WriteSidecarSHA256(outputPath, blob)
	_ = s.audit.RecordTableAudit("backup_settings", "imkey", "EXPORT", sess.UserID, nil, map[string]string{"file": outputPath}, nil)
	return models.IMKeyOperationResult{
		FilePath:      outputPath,
		HasBackupPass: true,
		CompletedAt:   now,
	}, nil
}

// ImportIMKey decrypts an IMKEY1 file using transportPassphrase and persists the backup passphrase in Credential Manager.
func (s *Service) ImportIMKey(filePath, transportPassphrase string) (models.IMKeyOperationResult, error) {
	sess, err := s.security.RequirePermission(security.PermBackupSettings)
	if err != nil {
		return models.IMKeyOperationResult{}, err
	}
	if s.secrets == nil {
		return models.IMKeyOperationResult{}, errors.New("credential store unavailable")
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return models.IMKeyOperationResult{}, fmt.Errorf("read .imkey file: %w", err)
	}
	payload, err := crypto.DecryptIMKey(transportPassphrase, raw)
	if err != nil {
		return models.IMKeyOperationResult{}, err
	}
	if err := s.secrets.SetSecret(crypto.BackupPassphraseAccount, payload.BackupPassphrase); err != nil {
		return models.IMKeyOperationResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_ = s.audit.RecordTableAudit("backup_settings", "imkey", "IMPORT", sess.UserID, nil, map[string]string{"file": filePath}, nil)
	return models.IMKeyOperationResult{
		FilePath:        filePath,
		HasBackupPass:   true,
		StoredInKeyring: true,
		CompletedAt:     now,
	}, nil
}

// ListBackupSchedules returns configured automated backup schedules.
func (s *Service) ListBackupSchedules() ([]models.BackupSchedule, error) {
	if _, err := s.security.RequirePermission(security.PermBackupSchedule); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, name, COALESCE(cron_expr, ''), COALESCE(time_zone, 'UTC'), COALESCE(destination, 'local'),
		        COALESCE(retention_count, 5), COALESCE(retention_days, 30), COALESCE(enabled, 1),
		        COALESCE(last_run, ''), COALESCE(next_run, ''), COALESCE(created_by, ''),
		        COALESCE(created_at, ''), COALESCE(notes, '')
		 FROM backup_schedules ORDER BY id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list backup schedules: %w", err)
	}
	defer rows.Close()

	var out []models.BackupSchedule
	for rows.Next() {
		var sc models.BackupSchedule
		var enabledInt int
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.CronExpr, &sc.TimeZone, &sc.Destination, &sc.RetentionCount, &sc.RetentionDays, &enabledInt, &sc.LastRun, &sc.NextRun, &sc.CreatedBy, &sc.CreatedAt, &sc.Notes); err != nil {
			return nil, err
		}
		sc.Enabled = enabledInt == 1
		out = append(out, sc)
	}
	if out == nil {
		out = []models.BackupSchedule{}
	}
	return out, rows.Err()
}

// SaveBackupSchedule creates or updates a scheduled backup configuration.
func (s *Service) SaveBackupSchedule(input models.SaveBackupScheduleInput) (models.BackupSchedule, error) {
	sess, err := s.security.RequirePermission(security.PermBackupSchedule)
	if err != nil {
		return models.BackupSchedule{}, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return models.BackupSchedule{}, errors.New("schedule name is required")
	}
	if strings.TrimSpace(input.CronExpr) == "" {
		input.CronExpr = "0 2 * * *"
	}
	if strings.TrimSpace(input.TimeZone) == "" {
		input.TimeZone = "UTC"
	}
	if strings.TrimSpace(input.Destination) == "" {
		input.Destination = "local"
	}
	if input.RetentionCount <= 0 {
		input.RetentionCount = 5
	}
	if input.RetentionDays <= 0 {
		input.RetentionDays = 30
	}
	enabledInt := 0
	if input.Enabled {
		enabledInt = 1
	}

	id := input.ID
	if id == 0 {
		res, err := s.db.Exec(
			`INSERT INTO backup_schedules (name, cron_expr, time_zone, destination, retention_count, retention_days, enabled, created_by, notes)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			strings.TrimSpace(input.Name), strings.TrimSpace(input.CronExpr), strings.TrimSpace(input.TimeZone),
			strings.TrimSpace(input.Destination), input.RetentionCount, input.RetentionDays, enabledInt, sess.UserID, input.Notes,
		)
		if err != nil {
			return models.BackupSchedule{}, fmt.Errorf("create backup schedule: %w", err)
		}
		id, _ = res.LastInsertId()
	} else {
		if _, err := s.db.Exec(
			`UPDATE backup_schedules SET name = ?, cron_expr = ?, time_zone = ?, destination = ?,
			        retention_count = ?, retention_days = ?, enabled = ?, notes = ?
			 WHERE id = ?`,
			strings.TrimSpace(input.Name), strings.TrimSpace(input.CronExpr), strings.TrimSpace(input.TimeZone),
			strings.TrimSpace(input.Destination), input.RetentionCount, input.RetentionDays, enabledInt, input.Notes, id,
		); err != nil {
			return models.BackupSchedule{}, fmt.Errorf("update backup schedule: %w", err)
		}
	}

	list, err := s.ListBackupSchedules()
	if err != nil {
		return models.BackupSchedule{}, err
	}
	for _, sc := range list {
		if sc.ID == id {
			_ = s.audit.RecordTableAudit("backup_schedules", fmt.Sprintf("%d", id), "UPSERT", sess.UserID, nil, sc, nil)
			return sc, nil
		}
	}
	return models.BackupSchedule{}, fmt.Errorf("saved schedule %d not found", id)
}

// DeleteBackupSchedule removes a scheduled backup configuration.
func (s *Service) DeleteBackupSchedule(id int64) error {
	sess, err := s.security.RequirePermission(security.PermBackupSchedule)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM backup_schedules WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete backup schedule: %w", err)
	}
	_ = s.audit.RecordTableAudit("backup_schedules", fmt.Sprintf("%d", id), "DELETE", sess.UserID, nil, nil, nil)
	return nil
}

// MarkScheduleExecuted updates last_run/next_run timestamps when the scheduler triggers a backup schedule.
func (s *Service) MarkScheduleExecuted(id int64, lastRun, nextRun string) error {
	_, err := s.db.Exec(`UPDATE backup_schedules SET last_run = ?, next_run = ? WHERE id = ?`, lastRun, nextRun, id)
	return err
}
