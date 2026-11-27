package mobile_sync_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/mobile_sync"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

type mockSecrets struct {
	data map[string]string
}

func (m *mockSecrets) GetSecret(acc string) (string, error) {
	if val, ok := m.data[acc]; ok {
		return val, nil
	}
	return "", fmt.Errorf("secret not found")
}

func (m *mockSecrets) SetSecret(acc, secret string) error {
	m.data[acc] = secret
	return nil
}

func (m *mockSecrets) DeleteSecret(acc string) error {
	delete(m.data, acc)
	return nil
}

func TestMobileSyncServerLifecycleAndEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	backupsDir := filepath.Join(tempDir, "backups")
	_ = os.MkdirAll(backupsDir, 0o755)

	// Create a dummy .imbk2 file
	dummyBackup := filepath.Join(backupsDir, "import_manager_backup_20261122.imbk2")
	_ = os.WriteFile(dummyBackup, []byte("IMBK2_TEST_PAYLOAD_1234567890"), 0o600)

	sec := &mockSecrets{data: map[string]string{
		"ImportManager_BackupPassphrase_v1": "Jana@1438",
	}}

	svc := mobile_sync.NewService(nil, nil, nil, nil, sec, backupsDir, "v1.9.4", nil)
	db := openMigratedDB(t, "lifecycle.db")
	recordBackup(t, db, dummyBackup, "2026-11-22 10:00:00")
	svc.SetDB(db)
	err := svc.Start(18766)
	if err != nil {
		t.Fatalf("failed to start mobile sync server: %v", err)
	}
	defer svc.Stop()

	time.Sleep(100 * time.Millisecond)

	info, err := svc.GetServerInfo()
	if err != nil {
		t.Fatalf("failed to get server info: %v", err)
	}
	if !info.IsRunning || info.Port == 0 {
		t.Fatalf("expected server running, got %+v", info)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", info.Port)

	// 1. Test Status endpoint
	resp, err := http.Get(baseURL + "/api/mobile/status")
	if err != nil {
		t.Fatalf("GET /api/mobile/status failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code %d, expected 200", resp.StatusCode)
	}
	var stat mobile_sync.StatusResponse
	_ = json.NewDecoder(resp.Body).Decode(&stat)
	if stat.Status != "online" || stat.LastBackup != "import_manager_backup_20261122.imbk2" {
		t.Fatalf("unexpected status response: %+v", stat)
	}

	// 2. Test Login endpoint with Backup Passphrase
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": "Jana@1438",
	})
	respLogin, err := http.Post(baseURL+"/api/mobile/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("POST /api/mobile/login failed: %v", err)
	}
	defer respLogin.Body.Close()

	if respLogin.StatusCode != http.StatusOK {
		t.Fatalf("login status code %d, expected 200", respLogin.StatusCode)
	}
	var loginResp mobile_sync.LoginResponse
	_ = json.NewDecoder(respLogin.Body).Decode(&loginResp)
	if loginResp.Token == "" {
		t.Fatalf("expected non-empty token, got %+v", loginResp)
	}

	// 3. Test Sync Latest endpoint with Bearer token
	req, _ := http.NewRequest("GET", baseURL+"/api/mobile/sync/latest", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	respSync, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/mobile/sync/latest failed: %v", err)
	}
	defer respSync.Body.Close()

	if respSync.StatusCode != http.StatusOK {
		t.Fatalf("sync status code %d, expected 200", respSync.StatusCode)
	}
	bodyBytes := make([]byte, 100)
	n, _ := respSync.Body.Read(bodyBytes)
	if string(bodyBytes[:n]) != "IMBK2_TEST_PAYLOAD_1234567890" {
		t.Fatalf("unexpected backup payload: %s", string(bodyBytes[:n]))
	}

	// 4. Test unauthorized request
	reqUnauth, _ := http.NewRequest("GET", baseURL+"/api/mobile/sync/latest", nil)
	respUnauth, err := http.DefaultClient.Do(reqUnauth)
	if err != nil {
		t.Fatalf("GET unauth failed: %v", err)
	}
	defer respUnauth.Body.Close()
	if respUnauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized, got %d", respUnauth.StatusCode)
	}
	if cors := respUnauth.Header.Get("Access-Control-Allow-Origin"); cors != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin header, got %q", cors)
	}

	// 5. Test Basic-auth fallback lockout on /api/mobile/sync/latest
	for i := 0; i < 5; i++ {
		reqBad, _ := http.NewRequest("GET", baseURL+"/api/mobile/sync/latest", nil)
		reqBad.SetBasicAuth("user@example.com", "wrong-passphrase")
		rBad, err := http.DefaultClient.Do(reqBad)
		if err != nil {
			t.Fatalf("basic auth attempt %d failed: %v", i+1, err)
		}
		_ = rBad.Body.Close()
		if rBad.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 on failed basic auth attempt %d, got %d", i+1, rBad.StatusCode)
		}
	}

	// 6th attempt even with valid passphrase must be locked out (429)
	reqLocked, _ := http.NewRequest("GET", baseURL+"/api/mobile/sync/latest", nil)
	reqLocked.SetBasicAuth("user@example.com", "Jana@1438")
	rLocked, err := http.DefaultClient.Do(reqLocked)
	if err != nil {
		t.Fatalf("locked basic auth request failed: %v", err)
	}
	_ = rLocked.Body.Close()
	if rLocked.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 lockout on basic auth fallback, got %d", rLocked.StatusCode)
	}
}

func TestMobileSyncOptInPersistence(t *testing.T) {
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "msync.db"), "msync-test-key")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer mgr.Close()

	if _, err := migrations.Run(context.Background(), mgr.DB(), "1.9.5", nil); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	aud := audit.NewService(mgr.DB())
	sec := security.NewService(mgr.DB(), false, aud)
	if _, err := sec.CompleteAdminSetup("admin", "Admin!Pass#2026"); err != nil {
		t.Fatalf("CompleteAdminSetup: %v", err)
	}

	svc := mobile_sync.NewService(sec, nil, nil, aud, nil, t.TempDir(), "v1.9.5", nil)
	svc.SetDB(mgr.DB())
	defer svc.Stop()

	if svc.IsEnabled() {
		t.Fatalf("expected mobile sync to be disabled by default (opt-in)")
	}

	info, err := svc.SetEnabled(true)
	if err != nil {
		t.Fatalf("SetEnabled(true) failed: %v", err)
	}
	if !info.Enabled || !info.IsRunning {
		t.Fatalf("expected enabled & running after opt-in, got %+v", info)
	}
	if !svc.IsEnabled() {
		t.Fatalf("expected persisted setting to remain enabled")
	}

	info, err = svc.SetEnabled(false)
	if err != nil {
		t.Fatalf("SetEnabled(false) failed: %v", err)
	}
	if info.IsRunning || svc.IsEnabled() {
		t.Fatalf("expected stopped & disabled after SetEnabled(false), got %+v", info)
	}
}

// openMigratedDB opens a fresh encrypted company database with the full schema applied.
func openMigratedDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), name), "msync-test-key")
	if err != nil {
		t.Fatalf("open db %s: %v", name, err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	if _, err := migrations.Run(context.Background(), mgr.DB(), "2.3.0", nil); err != nil {
		t.Fatalf("migrations for %s: %v", name, err)
	}
	return mgr.DB()
}

// recordBackup registers a backup file in a company's backups table, as the backup service does.
func recordBackup(t *testing.T, db *sql.DB, path, createdAt string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO backups (filename, path, destination, status, created_at) VALUES (?, ?, 'local', 'completed', ?)`,
		filepath.Base(path), path, createdAt,
	); err != nil {
		t.Fatalf("record backup %s: %v", path, err)
	}
}

func optInMobileSync(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO app_settings (key, value, updated_at) VALUES ('mobile_sync.enabled', 'true', datetime('now', 'utc'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`); err != nil {
		t.Fatalf("opt in mobile sync: %v", err)
	}
}

func TestMobileSyncFollowsActiveCompany(t *testing.T) {
	backupsDir := t.TempDir()
	writeBackup := func(name, payload string) string {
		p := filepath.Join(backupsDir, name)
		if err := os.WriteFile(p, []byte(payload), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}
	backupA := writeBackup("company_a_backup.imbk2", "PAYLOAD_COMPANY_A")
	backupB := writeBackup("company_b_backup.imbk2", "PAYLOAD_COMPANY_B")
	// Newest file in the shared folder, but recorded by neither company: must never be served.
	_ = writeBackup("zz_unrecorded_newest.imbk2", "PAYLOAD_UNRECORDED")

	dbA := openMigratedDB(t, "company_a.db")
	dbB := openMigratedDB(t, "company_b.db")
	recordBackup(t, dbA, backupA, "2026-11-20 10:00:00")
	recordBackup(t, dbB, backupB, "2026-11-21 10:00:00")
	optInMobileSync(t, dbA)

	const passphrase = "Sync#Pass2026"
	secrets := &mockSecrets{data: map[string]string{"ImportManager_BackupPassphrase_v1": passphrase}}
	svc := mobile_sync.NewService(nil, nil, nil, nil, secrets, backupsDir, "v2.3.0", nil)
	defer svc.Stop()

	baseURL := func() string {
		info, err := svc.GetServerInfo()
		if err != nil || !info.IsRunning {
			t.Fatalf("expected mobile sync listener running, got %+v (err=%v)", info, err)
		}
		return fmt.Sprintf("http://127.0.0.1:%d", info.Port)
	}
	lastBackup := func(base string) string {
		resp, err := http.Get(base + "/api/mobile/status")
		if err != nil {
			t.Fatalf("GET status: %v", err)
		}
		defer resp.Body.Close()
		var stat mobile_sync.StatusResponse
		_ = json.NewDecoder(resp.Body).Decode(&stat)
		return stat.LastBackup
	}
	login := func(base string) string {
		body, _ := json.Marshal(map[string]string{"email": "phone@example.com", "password": passphrase})
		resp, err := http.Post(base+"/api/mobile/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST login: %v", err)
		}
		defer resp.Body.Close()
		var lr mobile_sync.LoginResponse
		_ = json.NewDecoder(resp.Body).Decode(&lr)
		if lr.Token == "" {
			t.Fatalf("expected login token (status %d)", resp.StatusCode)
		}
		return lr.Token
	}
	fetch := func(base, token string) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, base+"/api/mobile/sync/latest", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET sync/latest: %v", err)
		}
		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return resp.StatusCode, buf.String()
	}

	// Company A has opted in: listener starts and serves only A's recorded backup.
	svc.SetDB(dbA)
	if err := svc.ApplyEnabledSetting(); err != nil {
		t.Fatalf("ApplyEnabledSetting (A): %v", err)
	}
	base := baseURL()
	if got := lastBackup(base); got != "company_a_backup.imbk2" {
		t.Fatalf("company A status: expected company_a_backup.imbk2, got %q", got)
	}
	tokenA := login(base)
	if code, body := fetch(base, tokenA); code != http.StatusOK || body != "PAYLOAD_COMPANY_A" {
		t.Fatalf("company A sync: got %d %q", code, body)
	}

	// Switching to company B, which has not opted in, stops the listener.
	svc.SetDB(dbB)
	if err := svc.ApplyEnabledSetting(); err != nil {
		t.Fatalf("ApplyEnabledSetting (B, disabled): %v", err)
	}
	if info, _ := svc.GetServerInfo(); info.IsRunning {
		t.Fatalf("expected listener stopped after switching to a company without mobile sync opt-in")
	}

	// Once B opts in, only B's backup is served and A's token is no longer accepted.
	optInMobileSync(t, dbB)
	if err := svc.ApplyEnabledSetting(); err != nil {
		t.Fatalf("ApplyEnabledSetting (B, enabled): %v", err)
	}
	base = baseURL()
	if got := lastBackup(base); got != "company_b_backup.imbk2" {
		t.Fatalf("company B status: expected company_b_backup.imbk2, got %q", got)
	}
	if code, _ := fetch(base, tokenA); code != http.StatusUnauthorized {
		t.Fatalf("expected company A token to be revoked after switching company, got %d", code)
	}
	if code, body := fetch(base, login(base)); code != http.StatusOK || body != "PAYLOAD_COMPANY_B" {
		t.Fatalf("company B sync: got %d %q", code, body)
	}
}
