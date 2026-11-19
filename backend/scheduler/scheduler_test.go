package scheduler_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/scheduler"
)

func TestSchedulerRestoreLockAndCleanShutdown(t *testing.T) {
	t.Setenv("IMPORT_MANAGER_DB_KEY", "sched-test-key-0123456789abcdef012")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	t.Setenv("IMPORT_MANAGER_RECOVERY", "1")
	dir := filepath.Join(t.TempDir(), "appdata")

	rt, err := bootstrap.Initialize([]string{"--recovery-mode"}, dir)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	t.Cleanup(func() { _ = rt.Shutdown() })

	_, _ = rt.DB.DB().Exec(`DELETE FROM workflow_failure_forecast`)

	// Lock RestoreControl -> RunOnce must safely skip without writing forecasts
	rt.RestoreControl.SetRestoring(true, "unit test restore")
	sched := scheduler.New(rt.Workflow, rt.Backup, rt.RestoreControl, 50*time.Millisecond)
	sched.RunOnce()

	var countLocked int
	_ = rt.DB.DB().QueryRow(`SELECT COUNT(*) FROM workflow_failure_forecast`).Scan(&countLocked)
	if countLocked != 0 {
		t.Fatalf("expected scheduler RunOnce to skip under RestoreControl lock, got %d forecasts", countLocked)
	}

	// Unlock RestoreControl -> RunOnce must run cleanly
	rt.RestoreControl.SetRestoring(false, "")
	if err := rt.Workflow.RunDailyOperationsTick("test"); err != nil {
		t.Fatalf("RunDailyOperationsTick failed: %v", err)
	}
	sched.RunOnce()
	var countUnlocked int
	_ = rt.DB.DB().QueryRow(`SELECT COUNT(*) FROM workflow_failure_forecast`).Scan(&countUnlocked)
	if countUnlocked == 0 {
		t.Fatalf("expected scheduler RunOnce to produce forecast when unlocked")
	}

	sched.Start()
	time.Sleep(80 * time.Millisecond)
	sched.Stop()
	// Calling Stop twice must be safe and idempotent
	sched.Stop()
}
