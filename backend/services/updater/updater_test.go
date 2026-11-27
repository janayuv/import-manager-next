package updater_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/app/config"
	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/updater"
	winpaths "github.com/janayuv/import-manager-next/backend/windows"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func setupUpdaterSecurity(t *testing.T, role string) (*security.Service, func()) {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "upd.db"), "updater-sec-test-key")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := migrations.Run(context.Background(), mgr.DB(), "1.0.0", nil); err != nil {
		_ = mgr.Close()
		t.Fatalf("migrations: %v", err)
	}
	aud := audit.NewService(mgr.DB())
	sec := security.NewService(mgr.DB(), false, aud)
	if _, err := sec.CompleteAdminSetup("admin", "Admin!Pass#2026"); err != nil {
		_ = mgr.Close()
		t.Fatalf("CompleteAdminSetup: %v", err)
	}
	if role == "operator" {
		if err := sec.AssignUserRole("admin", "operator"); err != nil {
			_ = mgr.Close()
			t.Fatalf("AssignUserRole: %v", err)
		}
		_ = sec.Logout()
		if _, err := sec.Authenticate("admin", "Admin!Pass#2026", false); err != nil {
			_ = mgr.Close()
			t.Fatalf("Authenticate operator: %v", err)
		}
	} else if role == "none" {
		_ = sec.Logout()
	}
	return sec, func() { _ = mgr.Close() }
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.1", "1.0.0", 1},
		{"v1.0.1", "1.0.0", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"1.1.0", "1.0.9", 1},
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0-beta.1", "1.0.0", 0},
		{"0.9.9", "1.0.0", -1},
		{"1.0.0", "1.0.1", -1},
		{"1.0.0", "2.0.0", -1},
	}

	for _, tt := range tests {
		got := updater.CompareSemver(tt.v1, tt.v2)
		if got != tt.expected {
			t.Errorf("CompareSemver(%q, %q) = %d; expected %d", tt.v1, tt.v2, got, tt.expected)
		}
	}
}

func TestCheckForUpdate_UpdateAvailableAndNoTokenLeak(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_secret_should_never_be_sent")
	t.Setenv("GH_TOKEN", "ghp_secret_should_never_be_sent")

	installerBytes := "fake-signed-installer-payload"
	expectedHash := sha256Hex(installerBytes)

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("unexpected Authorization header leaked to updater request: %q", auth)
		}
		switch r.URL.Path {
		case "/repos/janayuv/import-manager-next/releases/latest":
			mockRelease := updater.GitHubRelease{
				TagName:     "v1.1.0",
				Name:        "Import Manager 1.1.0",
				Body:        "## Release Notes\n* Fixed dropdowns\n* Added in-app updater",
				PublishedAt: time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
				HTMLURL:     "https://github.com/janayuv/import-manager-next/releases/tag/v1.1.0",
				Assets: []updater.GitHubAsset{
					{
						Name:               "import-manager-installer.exe",
						Size:               52428800,
						BrowserDownloadURL: ts.URL + "/releases/download/v1.1.0/import-manager-installer.exe",
						ContentType:        "application/octet-stream",
					},
					{
						Name:               "SHA256SUMS.txt",
						Size:               128,
						BrowserDownloadURL: ts.URL + "/releases/download/v1.1.0/SHA256SUMS.txt",
						ContentType:        "text/plain",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockRelease)
		case "/releases/download/v1.1.0/SHA256SUMS.txt":
			_, _ = w.Write([]byte(expectedHash + "  import-manager-installer.exe\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	cfg := config.Config{
		AppName:    "Import Manager",
		AppVersion: "1.0.0",
		Paths:      winpaths.AppPaths{DataDir: t.TempDir()},
	}

	svc := updater.NewService(cfg, nil, nil)
	svc.SetAPIBaseURL(ts.URL)
	svc.AllowTestURLHost(ts.URL)

	res, err := svc.CheckForUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdate error: %v", err)
	}

	if !res.UpdateAvailable {
		t.Fatalf("expected update to be available for 1.1.0 > 1.0.0")
	}
	if res.LatestVersion != "1.1.0" {
		t.Errorf("expected LatestVersion '1.1.0', got %q", res.LatestVersion)
	}
	if res.AssetName != "import-manager-installer.exe" {
		t.Errorf("expected AssetName 'import-manager-installer.exe', got %q", res.AssetName)
	}
	if res.ExpectedSHA256 != expectedHash {
		t.Errorf("expected SHA256 %q, got %q", expectedHash, res.ExpectedSHA256)
	}
}

func TestCheckForUpdate_AlreadyUpToDate(t *testing.T) {
	mockRelease := updater.GitHubRelease{
		TagName:     "v1.0.0",
		Name:        "Import Manager 1.0.0",
		Body:        "Initial release",
		PublishedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		HTMLURL:     "https://github.com/janayuv/import-manager-next/releases/tag/v1.0.0",
		Assets:      []updater.GitHubAsset{},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer ts.Close()

	cfg := config.Config{
		AppName:    "Import Manager",
		AppVersion: "1.0.0",
		Paths:      winpaths.AppPaths{DataDir: t.TempDir()},
	}

	svc := updater.NewService(cfg, nil, nil)
	svc.SetAPIBaseURL(ts.URL)

	res, err := svc.CheckForUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdate error: %v", err)
	}

	if res.UpdateAvailable {
		t.Fatalf("expected update NOT to be available when versions match")
	}
	if res.LatestVersion != "1.0.0" {
		t.Errorf("expected LatestVersion '1.0.0', got %q", res.LatestVersion)
	}
}

func TestCheckForUpdate_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	cfg := config.Config{
		AppName:    "Import Manager",
		AppVersion: "1.0.0",
		Paths:      winpaths.AppPaths{DataDir: t.TempDir()},
	}

	svc := updater.NewService(cfg, nil, nil)
	svc.SetAPIBaseURL(ts.URL)

	res, err := svc.CheckForUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdate unexpected error: %v", err)
	}

	if res.UpdateAvailable {
		t.Fatalf("expected updateAvailable false when release not found")
	}
	if res.Error == "" {
		t.Errorf("expected informative error message on 404")
	}
}

func TestDownloadAndApplyUpdate_VerificationAndPermissions(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_should_not_leak_on_download")

	testContent := "test-installer-binary-bytes"
	expectedHash := sha256Hex(testContent)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("unexpected Authorization header on download: %q", auth)
		}
		switch r.URL.Path {
		case "/test-installer.exe":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(testContent))
		case "/SHA256SUMS.txt":
			_, _ = w.Write([]byte(expectedHash + "  test-installer.exe\n"))
		case "/tampered-installer.exe":
			_, _ = w.Write([]byte("tampered-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	cfg := config.Config{
		AppName:    "Import Manager",
		AppVersion: "1.0.0",
		Paths:      winpaths.AppPaths{DataDir: tempDir},
	}

	// Non-admin operator (has PermDataWrite, lacks PermAdminSystemTools) must be rejected
	opSec, opCleanup := setupUpdaterSecurity(t, "operator")
	defer opCleanup()
	opSvc := updater.NewService(cfg, opSec, nil)
	opSvc.AllowTestURLHost(ts.URL)
	if _, err := opSvc.DownloadUpdate(context.Background(), ts.URL+"/test-installer.exe", "test-installer.exe"); err == nil {
		t.Fatalf("expected operator without PermAdminSystemTools to be rejected by DownloadUpdate")
	}

	// Admin succeeds
	adminSec, adminCleanup := setupUpdaterSecurity(t, "admin")
	defer adminCleanup()
	svc := updater.NewService(cfg, adminSec, nil)
	svc.AllowTestURLHost(ts.URL)
	svc.SetSkipLaunchInTest(true)

	// Reject non-GitHub untrusted hosts
	if _, err := svc.DownloadUpdate(context.Background(), "https://evil.example.com/installer.exe", "installer.exe"); err == nil || !strings.Contains(err.Error(), "untrusted update host") {
		t.Fatalf("expected untrusted host error, got %v", err)
	}

	// Reject plain HTTP on GitHub host
	if _, err := svc.DownloadUpdate(context.Background(), "http://github.com/janayuv/import-manager-next/releases/download/v1.0.0/installer.exe", "installer.exe"); err == nil {
		t.Fatalf("expected non-HTTPS GitHub URL to be rejected")
	}

	// Only release assets of the pinned repository are accepted on github.com
	svc.SetRepository("janayuv/import-manager-next")
	for _, bad := range []string{
		"https://github.com/attacker/fake-repo/releases/download/v9.9.9/installer.exe",
		"https://github.com/janayuv/import-manager-next/releases/download/../../../attacker/fake-repo/releases/download/v1/installer.exe",
		"https://github.com/janayuv/import-manager-next/archive/refs/heads/main.zip",
	} {
		if _, err := svc.DownloadUpdate(context.Background(), bad, "installer.exe"); err == nil || !strings.Contains(err.Error(), "not a release asset") {
			t.Fatalf("expected %q to be rejected as a foreign release asset, got %v", bad, err)
		}
	}

	// githubusercontent.com is only valid as a redirect target, never as a frontend-supplied URL
	if _, err := svc.DownloadUpdate(context.Background(), "https://raw.githubusercontent.com/attacker/fake-repo/main/installer.exe", "installer.exe"); err == nil || !strings.Contains(err.Error(), "untrusted update host") {
		t.Fatalf("expected direct githubusercontent.com URL to be rejected, got %v", err)
	}

	downloadPath, err := svc.DownloadUpdate(context.Background(), ts.URL+"/test-installer.exe", "test-installer.exe")
	if err != nil {
		t.Fatalf("DownloadUpdate failed: %v", err)
	}

	content, readErr := os.ReadFile(downloadPath)
	if readErr != nil {
		t.Fatalf("read downloaded file error: %v", readErr)
	}
	if string(content) != testContent {
		t.Errorf("expected downloaded content %q, got %q", testContent, string(content))
	}

	expectedDir := filepath.Join(tempDir, "updates")
	if filepath.Dir(downloadPath) != expectedDir {
		t.Errorf("expected download in %q, got %q", expectedDir, filepath.Dir(downloadPath))
	}

	// Operator cannot call ApplyUpdate even on a verified file
	opSvc2 := updater.NewService(cfg, opSec, nil)
	if err := opSvc2.ApplyUpdate(downloadPath); err == nil {
		t.Fatalf("expected operator to be rejected by ApplyUpdate")
	}

	// Unverified arbitrary path outside updates dir must fail
	outsideExe := filepath.Join(tempDir, "outside.exe")
	_ = os.WriteFile(outsideExe, []byte(testContent), 0o600)
	if err := svc.ApplyUpdate(outsideExe); err == nil {
		t.Fatalf("expected ApplyUpdate to reject installer outside updates dir")
	}

	// Valid verified installer succeeds for admin
	if err := svc.ApplyUpdate(downloadPath); err != nil {
		t.Fatalf("ApplyUpdate failed for verified admin download: %v", err)
	}

	// Tampering with downloaded file on disk before ApplyUpdate must fail checksum verification
	if err := os.WriteFile(downloadPath, []byte("corrupted-after-download"), 0o600); err != nil {
		t.Fatalf("tamper write: %v", err)
	}
	if err := svc.ApplyUpdate(downloadPath); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected checksum mismatch after tampering, got %v", err)
	}
}
