package updater_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/app/config"
	"github.com/janayuv/import-manager-next/backend/services/updater"
	winpaths "github.com/janayuv/import-manager-next/backend/windows"
)

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

func TestCheckForUpdate_UpdateAvailable(t *testing.T) {
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
				BrowserDownloadURL: "https://github.com/janayuv/import-manager-next/releases/download/v1.1.0/import-manager-installer.exe",
				ContentType:        "application/octet-stream",
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/janayuv/import-manager-next/releases/latest" {
			http.NotFound(w, r)
			return
		}
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

	if !res.UpdateAvailable {
		t.Fatalf("expected update to be available for 1.1.0 > 1.0.0")
	}
	if res.LatestVersion != "1.1.0" {
		t.Errorf("expected LatestVersion '1.1.0', got %q", res.LatestVersion)
	}
	if res.AssetName != "import-manager-installer.exe" {
		t.Errorf("expected AssetName 'import-manager-installer.exe', got %q", res.AssetName)
	}
	if res.DownloadURL != mockRelease.Assets[0].BrowserDownloadURL {
		t.Errorf("expected DownloadURL %q, got %q", mockRelease.Assets[0].BrowserDownloadURL, res.DownloadURL)
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

func TestDownloadUpdate(t *testing.T) {
	testContent := "test-installer-binary-bytes"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(testContent))
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	cfg := config.Config{
		AppName:    "Import Manager",
		AppVersion: "1.0.0",
		Paths:      winpaths.AppPaths{DataDir: tempDir},
	}

	svc := updater.NewService(cfg, nil, nil)

	downloadPath, err := svc.DownloadUpdate(context.Background(), ts.URL+"/installer.exe", "test-installer.exe")
	if err != nil {
		t.Fatalf("DownloadUpdate failed: %v", err)
	}

	if _, statErr := os.Stat(downloadPath); statErr != nil {
		t.Fatalf("downloaded file stat failed: %v", statErr)
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
}
