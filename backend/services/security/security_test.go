package security_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func setupSecurityDB(t *testing.T, recoveryMode bool) (*security.Service, *connection.Manager, *audit.Service) {
	t.Helper()
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "sec.db"), "sec-test-key-001")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := migrations.Run(context.Background(), mgr.DB(), "1.0.0", nil); err != nil {
		_ = mgr.Close()
		t.Fatalf("run migrations: %v", err)
	}
	aud := audit.NewService(mgr.DB())
	return security.NewService(mgr.DB(), recoveryMode, aud), mgr, aud
}

func TestRBACMatrix25PermissionsAndAliases(t *testing.T) {
	if len(security.AllPermissions) != 25 {
		t.Fatalf("expected 25 canonical permissions, got %d", len(security.AllPermissions))
	}
	matrix := security.GetRoleMatrix()
	if len(matrix) != 4 {
		t.Fatalf("expected 4 roles, got %d", len(matrix))
	}

	if role, ok := security.ParseRole("admin"); !ok || role != security.RoleAdministrator {
		t.Fatalf("expected admin -> administrator")
	}
	if role, ok := security.ParseRole("db_manager"); !ok || role != security.RoleManager {
		t.Fatalf("expected db_manager -> manager")
	}
	if !security.RoleHasPermission(security.RoleAdministrator, security.PermRoleWrite) {
		t.Fatalf("administrator should have role.write")
	}
	if security.RoleHasPermission(security.RoleManager, security.PermRoleWrite) {
		t.Fatalf("manager must NOT have role.write")
	}
	if !security.RoleHasPermission(security.RoleOperator, security.PermDataWrite) || security.RoleHasPermission(security.RoleOperator, security.PermDataDelete) {
		t.Fatalf("operator permission mismatch")
	}
	if !security.RoleHasPermission(security.RoleViewer, security.PermDataRead) || security.RoleHasPermission(security.RoleViewer, security.PermDataWrite) {
		t.Fatalf("viewer permission mismatch")
	}
}

func TestAuthSetupLockoutPasswordHistoryAndRecovery(t *testing.T) {
	svc, mgr, aud := setupSecurityDB(t, false)
	defer mgr.Close()

	st, err := svc.GetAuthSetupStatus()
	if err != nil || !st.NeedsSetup {
		t.Fatalf("expected NeedsSetup=true initially, got %+v err=%v", st, err)
	}

	pass1 := "Init!Pass#2026"
	sess, err := svc.CompleteAdminSetup("Jana", pass1)
	if err != nil {
		t.Fatalf("CompleteAdminSetup failed: %v", err)
	}
	if sess.Username != "Jana" || sess.Role != "administrator" || len(sess.Permissions) != 25 {
		t.Fatalf("unexpected setup session: %+v", sess)
	}

	if err := svc.Logout(); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// Trigger 5 failed attempts -> account lock.
	for i := 0; i < 5; i++ {
		if _, err := svc.Authenticate("Jana", "Wrong!Pass1", false); err == nil {
			t.Fatalf("expected failure on attempt %d", i+1)
		}
	}
	if _, err := svc.Authenticate("Jana", pass1, false); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected lockout block even with correct password, got %v", err)
	}

	// Recovery commands must refuse execution when recoveryMode=false.
	if err := svc.RecoveryClearLockout("Jana"); err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("expected recovery refusal when flag off, got %v", err)
	}

	// Simulate reopening app in --recovery mode against same DB.
	svcRec := security.NewService(mgr.DB(), true, aud)
	if !svcRec.IsRecoveryModeActive() {
		t.Fatalf("expected recovery mode active")
	}
	if err := svcRec.RecoveryClearLockout("Jana"); err != nil {
		t.Fatalf("RecoveryClearLockout failed: %v", err)
	}

	// Password change + history enforcement.
	if _, err := svc.Authenticate("Jana", pass1, true); err != nil {
		t.Fatalf("Authenticate after recovery clear failed: %v", err)
	}
	if err := svc.ChangePassword(pass1, pass1); err == nil || !strings.Contains(err.Error(), "last 5 passwords") {
		t.Fatalf("expected password history rejection, got %v", err)
	}
	pass2 := "Next!Pass#2027"
	if err := svc.ChangePassword(pass1, pass2); err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}
	if err := svc.ChangePassword(pass2, pass1); err == nil || !strings.Contains(err.Error(), "last 5 passwords") {
		t.Fatalf("expected historical password rejection for pass1, got %v", err)
	}

	// Update security policy settings and confirm audit + version bump.
	updated, err := svc.UpdateSecuritySettings(security.SecuritySettings{
		IdleTimeoutMinutes:     15,
		LockoutThreshold:       3,
		LockoutWindowMinutes:   10,
		LockoutDurationMinutes: 20,
		PasswordHistoryDepth:   8,
	})
	if err != nil || updated.PolicyVersion != 1 || updated.PasswordHistoryDepth != 8 {
		t.Fatalf("UpdateSecuritySettings failed: %+v err=%v", updated, err)
	}

	logs, err := svc.GetRecentAuthEvents(20)
	if err != nil || len(logs) == 0 {
		t.Fatalf("expected auth audit events, got %d err=%v", len(logs), err)
	}
}

func TestLegacyBcryptAutoRehashesToArgon2idOnLogin(t *testing.T) {
	svc, mgr, _ := setupSecurityDB(t, false)
	defer mgr.Close()

	pass := "Legacy!Pass#1"
	bc, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	if _, err := mgr.DB().Exec(`
		INSERT INTO app_settings (key, value) VALUES ('auth.admin_username', 'Jana'), ('auth.admin_password_hash', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(bc),
	); err != nil {
		t.Fatalf("seed legacy bcrypt: %v", err)
	}

	if _, err := svc.Authenticate("Jana", pass, false); err != nil {
		t.Fatalf("login with legacy bcrypt failed: %v", err)
	}

	var storedHash string
	if err := mgr.DB().QueryRow(`SELECT value FROM app_settings WHERE key='auth.admin_password_hash'`).Scan(&storedHash); err != nil {
		t.Fatalf("read upgraded hash: %v", err)
	}
	if !strings.HasPrefix(storedHash, "$argon2id$v=19$") {
		t.Fatalf("expected transparent upgrade to Argon2id, got %s", storedHash)
	}
}
