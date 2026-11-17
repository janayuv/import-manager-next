package google_drive_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/models"
)

func setupDriveEnv(t *testing.T) *bootstrap.Runtime {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "gdrive-test-key-0123456789abcdef0123")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	dir := filepath.Join(t.TempDir(), "appdata")
	rt, err := bootstrap.Initialize(nil, dir)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	t.Cleanup(func() { _ = rt.Shutdown() })

	if _, err := rt.Security.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup failed: %v", err)
	}
	return rt
}

func TestGoogleDriveOAuthUploadListRetentionAndDownload(t *testing.T) {
	rt := setupDriveEnv(t)

	var mu sync.Mutex
	deletedIDs := []string{}
	extraOldFiles := 31

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "mock-access-token",
				"refresh_token": "mock-refresh-token",
				"expires_in":    3600,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/oauth2/v2/userinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"email": "ops@importmanager.local"})
		case r.Method == http.MethodGet && r.URL.Path == "/drive/v3/files":
			q := r.URL.Query().Get("q")
			if strings.Contains(q, "application/vnd.google-apps.folder") {
				_ = json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
				return
			}
			var files []map[string]any
			for i := 1; i <= extraOldFiles; i++ {
				files = append(files, map[string]any{
					"id":           fmt.Sprintf("file-%02d", i),
					"name":         fmt.Sprintf("backup_%02d.imbk2", i),
					"size":         "128",
					"createdTime":  fmt.Sprintf("2026-11-01T%02d:00:00Z", i%24),
					"modifiedTime": fmt.Sprintf("2026-11-01T%02d:00:00Z", i%24),
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"files": files})
		case r.Method == http.MethodPost && r.URL.Path == "/drive/v3/files":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "folder-import-backups"})
		case r.Method == http.MethodPost && r.URL.Path == "/upload/drive/v3/files":
			raw, _ := io.ReadAll(r.Body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "file-new",
				"name":         "nightly.imbk2",
				"size":         fmt.Sprintf("%d", len(raw)),
				"createdTime":  "2026-11-12T10:00:00Z",
				"modifiedTime": "2026-11-12T10:00:00Z",
			})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/drive/v3/files/"):
			id := strings.TrimPrefix(r.URL.Path, "/drive/v3/files/")
			deletedIDs = append(deletedIDs, id)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/drive/v3/files/") && r.URL.Query().Get("alt") == "media":
			_, _ = w.Write([]byte("IMBK2-MOCK-ENCRYPTED-ARCHIVE"))
		default:
			http.Error(w, "unexpected route: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	rt.GoogleDrive.AuthBaseURL = srv.URL + "/auth"
	rt.GoogleDrive.TokenURL = srv.URL + "/token"
	rt.GoogleDrive.APIBaseURL = srv.URL
	rt.GoogleDrive.RedirectURI = "http://127.0.0.1:0/callback"

	if err := rt.GoogleDrive.SaveClientConfig(models.GoogleDriveConfigInput{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}); err != nil {
		t.Fatalf("SaveClientConfig failed: %v", err)
	}

	authURL, err := rt.GoogleDrive.StartOAuthFlow()
	if err != nil || !strings.Contains(authURL, "test-client-id") {
		t.Fatalf("StartOAuthFlow failed: url=%s err=%v", authURL, err)
	}

	status, err := rt.GoogleDrive.CompleteOAuthCode("mock-auth-code", "")
	if err != nil || !status.Connected || status.AccountEmail != "ops@importmanager.local" {
		t.Fatalf("CompleteOAuthCode failed: status=%+v err=%v", status, err)
	}

	localFile := filepath.Join(t.TempDir(), "nightly.imbk2")
	_ = os.WriteFile(localFile, []byte("encrypted-backup-payload"), 0o600)

	remote, err := rt.GoogleDrive.UploadBackup(localFile)
	if err != nil || remote.Locator != "gdrive:file-new" {
		t.Fatalf("UploadBackup failed: remote=%+v err=%v", remote, err)
	}
	if len(deletedIDs) != 1 {
		t.Fatalf("expected 1 oldest backup pruned for >30 retention, got %v", deletedIDs)
	}

	list, err := rt.GoogleDrive.ListBackups()
	if err != nil || len(list) != 31 {
		t.Fatalf("ListBackups failed: len=%d err=%v", len(list), err)
	}

	dlPath, err := rt.GoogleDrive.DownloadBackup(remote.Locator, "restored_nightly.imbk2")
	if err != nil {
		t.Fatalf("DownloadBackup failed: %v", err)
	}
	dlRaw, _ := os.ReadFile(dlPath)
	if string(dlRaw) != "IMBK2-MOCK-ENCRYPTED-ARCHIVE" {
		t.Fatalf("unexpected downloaded bytes: %q", string(dlRaw))
	}

	// Test DeleteBackup
	if err := rt.GoogleDrive.DeleteBackup(remote.Locator); err != nil {
		t.Fatalf("DeleteBackup failed: %v", err)
	}
	if len(deletedIDs) != 2 || deletedIDs[1] != "file-new" {
		t.Fatalf("expected 2 deleted IDs with file-new, got %v", deletedIDs)
	}
	if err := rt.GoogleDrive.DeleteBackup(""); err == nil {
		t.Fatalf("expected error for empty locator")
	}

	if err := rt.GoogleDrive.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if err := rt.GoogleDrive.DeleteBackup("file-new"); err == nil {
		t.Fatalf("expected error after disconnect")
	}
	if _, err := rt.GoogleDrive.ListBackups(); err == nil || !strings.Contains(err.Error(), "GDRIVE_ERROR:") {
		t.Fatalf("expected GDRIVE_ERROR after disconnect, got %v", err)
	}
}
