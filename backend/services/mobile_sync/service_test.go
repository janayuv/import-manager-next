package mobile_sync_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/services/mobile_sync"
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
}
