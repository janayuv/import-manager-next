package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/janayuv/import-manager-next/backend/app/config"
	"github.com/janayuv/import-manager-next/backend/crypto"
	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/logging"
	"github.com/janayuv/import-manager-next/backend/memory"
	"github.com/janayuv/import-manager-next/backend/scheduler"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/backup"
	"github.com/janayuv/import-manager-next/backend/services/boe"
	"github.com/janayuv/import-manager-next/backend/services/boe_entry"
	"github.com/janayuv/import-manager-next/backend/services/boe_summary"
	"github.com/janayuv/import-manager-next/backend/services/cbic_rates"
	"github.com/janayuv/import-manager-next/backend/services/dashboard"
	"github.com/janayuv/import-manager-next/backend/services/database_management"
	"github.com/janayuv/import-manager-next/backend/services/expense_reports"
	"github.com/janayuv/import-manager-next/backend/services/expenses"
	"github.com/janayuv/import-manager-next/backend/services/google_drive"
	"github.com/janayuv/import-manager-next/backend/services/invoice"
	"github.com/janayuv/import-manager-next/backend/services/invoice_wizard"
	"github.com/janayuv/import-manager-next/backend/services/item_master"
	memService "github.com/janayuv/import-manager-next/backend/services/memory"
	"github.com/janayuv/import-manager-next/backend/services/options"
	"github.com/janayuv/import-manager-next/backend/services/payment_po"
	"github.com/janayuv/import-manager-next/backend/services/recycle_bin"
	"github.com/janayuv/import-manager-next/backend/services/reports"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/settings"
	"github.com/janayuv/import-manager-next/backend/services/shipment"
	"github.com/janayuv/import-manager-next/backend/services/supplier"
	"github.com/janayuv/import-manager-next/backend/services/updater"
	"github.com/janayuv/import-manager-next/backend/services/workflow"
)

// Runtime bundles initialized core infrastructure and domain services for Wails application binding.
type Runtime struct {
	Config             config.Config
	Logger             *logging.Logger
	DB                 *connection.Manager
	RestoreControl     *connection.RestoreControl
	Health             connection.HealthInfo
	MigrationResult    migrations.RunResult
	Integrity          integrity.Report
	Audit              *audit.Service
	Security           *security.Service
	Options            *options.Service
	Suppliers          *supplier.Service
	Items              *item_master.Service
	Shipments          *shipment.Service
	Invoices           *invoice.Service
	InvoiceWizard      *invoice_wizard.Service
	PaymentPO          *payment_po.Service
	BOE                *boe.Service
	BOEEntry           *boe_entry.Service
	BOESummary         *boe_summary.Service
	CBICRates          *cbic_rates.Service
	Expenses           *expenses.Service
	ExpenseReports     *expense_reports.Service
	Reports            *reports.Service
	Dashboard          *dashboard.Service
	Settings           *settings.Service
	Backup             *backup.Service
	GoogleDrive        *google_drive.Service
	RecycleBin         *recycle_bin.Service
	DatabaseManagement *database_management.Service
	Workflow           *workflow.Service
	Updater            *updater.Service
	Scheduler          *scheduler.Scheduler
	MemoryMgr          *memory.Manager
	Memory             *memService.Service
}

// Initialize resolves configuration, opens logging, fetches/creates the SQLCipher database key
// via Windows Credential Manager, opens the encrypted database, runs versioned migrations,
// recovers interrupted writes, verifies integrity, and initializes Security & Audit services.
func Initialize(args []string, dataDirOverride string) (*Runtime, error) {
	cfg, err := config.Load(args, dataDirOverride)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logger, err := logging.New(cfg.Paths.LogsDir)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	keyStore := crypto.NewWinCredKeyStore()
	dbKey, err := keyStore.GetOrCreateDatabaseKey()
	if err != nil {
		_ = logger.Close()
		return nil, fmt.Errorf("resolve database encryption key: %w", err)
	}

	dbMgr, err := connection.OpenEncrypted(cfg.Paths.DatabasePath, dbKey)
	if err != nil {
		_ = logger.Close()
		return nil, fmt.Errorf("open encrypted database: %w", err)
	}

	cleanupOnErr := func(wrapErr error) (*Runtime, error) {
		_ = dbMgr.Close()
		_ = logger.Close()
		return nil, wrapErr
	}

	ctx := context.Background()
	migRes, err := migrations.Run(ctx, dbMgr.DB(), cfg.AppVersion, func(fromVer, toVer int) (string, error) {
		snapName := fmt.Sprintf("pre-migration-v%d-to-v%d-%s.db", fromVer, toVer, time.Now().Format("20060102-150405"))
		snapPath := filepath.Join(cfg.Paths.BackupsDir, snapName)
		if _, vacErr := dbMgr.DB().Exec(`VACUUM INTO ?`, snapPath); vacErr != nil {
			return "", fmt.Errorf("vacuum pre-migration backup: %w", vacErr)
		}
		return snapPath, nil
	})
	if err != nil {
		return cleanupOnErr(fmt.Errorf("run database migrations: %w", err))
	}

	recoveredWrites, err := integrity.MarkInterruptedWritesFailed(dbMgr.DB())
	if err != nil {
		return cleanupOnErr(fmt.Errorf("recover interrupted writes: %w", err))
	}

	integRep, err := integrity.Verify(dbMgr.DB())
	if err != nil {
		return cleanupOnErr(fmt.Errorf("verify database integrity: %w", err))
	}
	integRep.RecoveredWrites = recoveredWrites
	if !integRep.IntegrityOK {
		return cleanupOnErr(fmt.Errorf("database integrity check failed: %v", integRep.IntegrityMessages))
	}

	health, err := dbMgr.Health()
	if err != nil {
		return cleanupOnErr(fmt.Errorf("verify database health: %w", err))
	}

	auditSvc := audit.NewService(dbMgr.DB())
	securitySvc := security.NewService(dbMgr.DB(), cfg.RecoveryMode, auditSvc)
	auditSvc.SetPermissionGate(func(perm string) error {
		_, gErr := securitySvc.RequirePermission(perm)
		return gErr
	})
	optionsSvc := options.NewService(dbMgr.DB(), securitySvc, auditSvc)
	supplierSvc := supplier.NewService(dbMgr.DB(), securitySvc, auditSvc)
	itemSvc := item_master.NewService(dbMgr.DB(), securitySvc, auditSvc, cfg.Paths.AttachmentsDir)
	shipSvc := shipment.NewService(dbMgr.DB(), securitySvc, auditSvc)
	invSvc := invoice.NewService(dbMgr.DB(), securitySvc, auditSvc)
	wizSvc := invoice_wizard.NewService(dbMgr.DB(), securitySvc, auditSvc)
	paymentPOSvc := payment_po.NewService(dbMgr.DB(), securitySvc, auditSvc)
	boeSvc := boe.NewService(dbMgr.DB(), securitySvc, auditSvc, cfg.Paths.AttachmentsDir)
	boeEntrySvc := boe_entry.NewService(dbMgr.DB(), securitySvc, auditSvc)
	boeSummarySvc := boe_summary.NewService(dbMgr.DB(), securitySvc, auditSvc)
	expSvc := expenses.NewService(dbMgr.DB(), securitySvc, auditSvc, cfg.Paths.AttachmentsDir)
	expRepSvc := expense_reports.NewService(dbMgr.DB(), securitySvc)
	repSvc := reports.NewService(dbMgr.DB(), securitySvc)
	dashSvc := dashboard.NewService(dbMgr.DB(), securitySvc, auditSvc)
	settingsSvc := settings.NewService(dbMgr.DB(), securitySvc, auditSvc)
	restoreCtrl := connection.NewRestoreControl()
	backupSvc := backup.NewService(dbMgr.DB(), securitySvc, auditSvc, keyStore, restoreCtrl, cfg.Paths.BackupsDir, cfg.Paths.TempDir, dbKey)
	gdriveSvc := google_drive.NewService(dbMgr.DB(), securitySvc, auditSvc, keyStore, cfg.Paths.BackupsDir)
	recycleSvc := recycle_bin.NewService(dbMgr.DB(), securitySvc, auditSvc, restoreCtrl)
	dbAdminSvc := database_management.NewService(dbMgr.DB(), dbMgr, securitySvc, auditSvc, restoreCtrl, backupSvc, cfg.Paths.DatabasePath, dbKey, cfg.Paths.TempDir)
	cbicRatesSvc := cbic_rates.NewService(dbMgr.DB(), securitySvc, auditSvc)
	workflowSvc := workflow.NewService(dbMgr.DB(), securitySvc, auditSvc, restoreCtrl, cfg.Paths.ExportsDir, cfg.Paths.LogsDir, cfg.AppVersion)
	sched := scheduler.New(workflowSvc, backupSvc, restoreCtrl, 5*time.Minute)
	if !cfg.RecoveryMode {
		sched.Start()
	}

	updaterSvc := updater.NewService(cfg, securitySvc, auditSvc)

	memMgr := memory.NewManager(dbMgr, logger, memory.Config{
		AutoTrimEnabled: true,
	})
	memSvc := memService.NewService(memMgr, securitySvc, auditSvc)

	logger.Info("runtime initialized",
		"app", cfg.AppName,
		"version", cfg.AppVersion,
		"recoveryMode", cfg.RecoveryMode,
		"cipherVersion", health.CipherVersion,
		"journalMode", health.JournalMode,
		"foreignKeysOn", health.ForeignKeysOn,
		"schemaVersion", migRes.CurrentVersion,
		"migrationsApplied", migRes.AppliedCount,
		"tables", integRep.TableCount,
		"views", integRep.ViewCount,
		"recoveredWrites", recoveredWrites,
		"dataDir", cfg.Paths.DataDir,
	)

	return &Runtime{
		Config:             cfg,
		Logger:             logger,
		DB:                 dbMgr,
		RestoreControl:     restoreCtrl,
		Health:             health,
		MigrationResult:    migRes,
		Integrity:          integRep,
		Audit:              auditSvc,
		Security:           securitySvc,
		Options:            optionsSvc,
		Suppliers:          supplierSvc,
		Items:              itemSvc,
		Shipments:          shipSvc,
		Invoices:           invSvc,
		InvoiceWizard:      wizSvc,
		PaymentPO:          paymentPOSvc,
		BOE:                boeSvc,
		BOEEntry:           boeEntrySvc,
		BOESummary:         boeSummarySvc,
		CBICRates:          cbicRatesSvc,
		Expenses:           expSvc,
		ExpenseReports:     expRepSvc,
		Reports:            repSvc,
		Dashboard:          dashSvc,
		Settings:           settingsSvc,
		Backup:             backupSvc,
		GoogleDrive:        gdriveSvc,
		RecycleBin:         recycleSvc,
		DatabaseManagement: dbAdminSvc,
		Workflow:           workflowSvc,
		Updater:            updaterSvc,
		Scheduler:          sched,
		MemoryMgr:          memMgr,
		Memory:             memSvc,
	}, nil
}

// Shutdown flushes and closes open database and log handles cleanly.
func (r *Runtime) Shutdown() error {
	var firstErr error
	if r.MemoryMgr != nil {
		_ = r.MemoryMgr.Close()
	}
	if r.Scheduler != nil {
		r.Scheduler.Stop()
	}
	if r.DB != nil {
		if err := r.DB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.Logger != nil {
		r.Logger.Info("runtime shutdown complete")
		if err := r.Logger.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
