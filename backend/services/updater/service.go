package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/app/config"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	winpaths "github.com/janayuv/import-manager-next/backend/windows"
)

const (
	DefaultGitHubRepo = "janayuv/import-manager-next"
	DefaultTimeout    = 15 * time.Second
)

// UpdateCheckResult holds GitHub release metadata and comparison with current app version.
type UpdateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseName     string `json:"releaseName"`
	ReleaseNotes    string `json:"releaseNotes"`
	PublishedAt     string `json:"publishedAt"`
	ReleaseURL      string `json:"releaseURL"`
	DownloadURL     string `json:"downloadURL"`
	AssetName       string `json:"assetName"`
	AssetSize       int64  `json:"assetSize"`
	CheckedAt       string `json:"checkedAt"`
	Repository      string `json:"repository"`
	Error           string `json:"error,omitempty"`
}

// GitHubRelease describes the JSON payload returned by the GitHub Releases API.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt time.Time     `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset describes an individual asset attached to a GitHub release.
type GitHubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// Service manages GitHub-backed software updates for the Import Manager desktop client.
type Service struct {
	cfg        config.Config
	paths      winpaths.AppPaths
	sec        *security.Service
	audit      *audit.Service
	httpClient *http.Client
	apiBaseURL string
	repo       string
}

// NewService constructs an update service with standard defaults.
func NewService(cfg config.Config, sec *security.Service, auditSvc *audit.Service) *Service {
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		repo = DefaultGitHubRepo
	}

	return &Service{
		cfg:   cfg,
		paths: cfg.Paths,
		sec:   sec,
		audit: auditSvc,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
				if len(via) > 0 && req.URL.Host != via[0].URL.Host {
					req.Header.Del("Authorization")
				}
				return nil
			},
		},
		apiBaseURL: "https://api.github.com",
		repo:       repo,
	}
}

// SetAPIBaseURL overrides the GitHub API base URL (useful for unit testing).
func (s *Service) SetAPIBaseURL(url string) {
	s.apiBaseURL = strings.TrimRight(url, "/")
}

// SetHTTPClient injects a custom HTTP client (useful for unit testing).
func (s *Service) SetHTTPClient(client *http.Client) {
	s.httpClient = client
}

// SetRepository sets the target GitHub repository (owner/repo).
func (s *Service) SetRepository(repo string) {
	if strings.TrimSpace(repo) != "" {
		s.repo = strings.TrimSpace(repo)
	}
}

// resolveToken retrieves any available GitHub token from environment variables or gh CLI.
func (s *Service) resolveToken() string {
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t
	}
	if t := strings.TrimSpace(os.Getenv("GH_TOKEN")); t != "" {
		return t
	}
	// Try gh CLI token if available in the host environment
	if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		if t := strings.TrimSpace(string(out)); t != "" && !strings.Contains(t, " ") {
			return t
		}
	}
	return ""
}

// GetCurrentVersion returns the running app version.
func (s *Service) GetCurrentVersion() string {
	if s.cfg.AppVersion != "" {
		return s.cfg.AppVersion
	}
	return "1.0.0"
}

// CheckForUpdate queries the latest GitHub release and compares it against currentVersion.
func (s *Service) CheckForUpdate(ctx context.Context) (UpdateCheckResult, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	currentVer := s.GetCurrentVersion()

	result := UpdateCheckResult{
		CurrentVersion: currentVer,
		LatestVersion:  currentVer,
		CheckedAt:      now,
		Repository:     s.repo,
	}

	reqURL := fmt.Sprintf("%s/repos/%s/releases/latest", s.apiBaseURL, s.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("create update request: %v", err)
		return result, nil
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("ImportManager-Desktop/%s", currentVer))
	if token := s.resolveToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("check update connection error: %v", err)
		return result, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		result.Error = fmt.Sprintf("No releases found on GitHub repository %q", s.repo)
		return result, nil
	}

	if resp.StatusCode == http.StatusForbidden {
		result.Error = "GitHub API rate limit exceeded or access forbidden. Please try again later."
		return result, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		result.Error = fmt.Sprintf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
		return result, nil
	}

	var release GitHubRelease
	if decodeErr := json.NewDecoder(resp.Body).Decode(&release); decodeErr != nil {
		result.Error = fmt.Sprintf("decode release payload: %v", decodeErr)
		return result, nil
	}

	latestVer := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(release.TagName, "v"), "V"))
	result.LatestVersion = latestVer
	result.ReleaseName = release.Name
	result.ReleaseNotes = release.Body
	result.PublishedAt = release.PublishedAt.Format(time.RFC3339)
	result.ReleaseURL = release.HTMLURL

	// Pick preferred Windows installer / executable asset
	asset := selectBestAsset(release.Assets)
	if asset != nil {
		result.DownloadURL = asset.BrowserDownloadURL
		result.AssetName = asset.Name
		result.AssetSize = asset.Size
	} else {
		result.DownloadURL = release.HTMLURL
		result.AssetName = "Release Page"
	}

	// Compare versions: if latestVer > currentVer, update is available.
	if CompareSemver(latestVer, currentVer) > 0 {
		result.UpdateAvailable = true
	}

	return result, nil
}

// DownloadUpdate downloads the update executable from downloadURL to the updates folder.
func (s *Service) DownloadUpdate(ctx context.Context, downloadURL, assetName string) (string, error) {
	if s.sec != nil {
		if _, err := s.sec.RequireAnyPermission(security.PermAdminSystemTools, security.PermDataWrite); err != nil {
			return "", err
		}
	}

	if strings.TrimSpace(downloadURL) == "" {
		return "", errors.New("downloadURL cannot be empty")
	}

	// Target directory: %APPDATA%\ImportManager\updates
	updatesDir := filepath.Join(s.paths.DataDir, "updates")
	if err := os.MkdirAll(updatesDir, 0755); err != nil {
		return "", fmt.Errorf("create updates directory: %w", err)
	}

	cleanAssetName := filepath.Base(assetName)
	if cleanAssetName == "" || cleanAssetName == "." || cleanAssetName == "/" {
		cleanAssetName = filepath.Base(downloadURL)
	}
	if cleanAssetName == "" || cleanAssetName == "." {
		cleanAssetName = "import-manager-installer.exe"
	}

	destPath := filepath.Join(updatesDir, cleanAssetName)
	tempPath := destPath + ".download"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("ImportManager-Desktop/%s", s.GetCurrentVersion()))
	if token := s.resolveToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("create update file: %w", err)
	}

	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("write update data: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close update file: %w", closeErr)
	}

	// Atomically replace destination
	_ = os.Remove(destPath)
	if err := os.Rename(tempPath, destPath); err != nil {
		return "", fmt.Errorf("finalize update file: %w", err)
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity("system", "system.update_downloaded", "update", cleanAssetName, "success", "INFO", map[string]any{
			"assetName": cleanAssetName,
			"destPath":  destPath,
			"url":       downloadURL,
		})
	}

	return destPath, nil
}

// ComputeSHA256 calculates the hexadecimal SHA-256 checksum of a file.
func ComputeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read file for checksum: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifySHA256 verifies that the file at filePath matches expectedHex (case-insensitive).
func VerifySHA256(filePath, expectedHex string) error {
	cleanExpected := strings.ToLower(strings.TrimSpace(expectedHex))
	if cleanExpected == "" {
		return errors.New("expected checksum is empty")
	}
	actual, err := ComputeSHA256(filePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, cleanExpected) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", cleanExpected, actual)
	}
	return nil
}

// ApplyUpdate launches the downloaded installer executable in background after verifying file safety.
func (s *Service) ApplyUpdate(installerPath string) error {
	if s.sec != nil {
		if _, err := s.sec.RequireAnyPermission(security.PermAdminSystemTools, security.PermDataWrite); err != nil {
			return err
		}
	}

	cleanPath := filepath.Clean(installerPath)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("installer file not found: %w", err)
	}
	if info.Size() == 0 {
		return errors.New("installer file is empty")
	}

	ext := strings.ToLower(filepath.Ext(cleanPath))
	if ext != ".exe" && ext != ".msi" {
		return fmt.Errorf("invalid installer file extension %q (expected .exe or .msi)", ext)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", "start", "", cleanPath)
	} else {
		cmd = exec.Command(cleanPath)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch installer: %w", err)
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity("system", "system.update_launched", "update", filepath.Base(cleanPath), "success", "INFO", map[string]any{
			"installerPath": cleanPath,
			"fileSize":      info.Size(),
		})
	}

	return nil
}

// OpenURL opens an external HTTP/HTTPS URL in the user's default web browser.
func (s *Service) OpenURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return fmt.Errorf("invalid URL scheme: %s", rawURL)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", trimmed)
	case "darwin":
		cmd = exec.Command("open", trimmed)
	default:
		cmd = exec.Command("xdg-open", trimmed)
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// selectBestAsset chooses the most suitable asset for the client OS from release assets.
func selectBestAsset(assets []GitHubAsset) *GitHubAsset {
	if len(assets) == 0 {
		return nil
	}

	// 1. Look for installer exe (e.g. import-manager-installer.exe or *-setup.exe)
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ".exe") && (strings.Contains(name, "installer") || strings.Contains(name, "setup")) {
			return &assets[i]
		}
	}

	// 2. Look for any Windows .exe
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ".exe") {
			return &assets[i]
		}
	}

	// 3. Look for .zip
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ".zip") {
			return &assets[i]
		}
	}

	return &assets[0]
}

// CompareSemver compares two semantic version strings (e.g. "1.0.1" vs "1.0.0").
// Returns 1 if v1 > v2, -1 if v1 < v2, and 0 if v1 == v2.
func CompareSemver(v1, v2 string) int {
	clean1 := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(v1), "v"), "V")
	clean2 := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(v2), "v"), "V")

	// Strip prerelease or metadata if present
	if idx := strings.IndexAny(clean1, "-+"); idx != -1 {
		clean1 = clean1[:idx]
	}
	if idx := strings.IndexAny(clean2, "-+"); idx != -1 {
		clean2 = clean2[:idx]
	}

	parts1 := strings.Split(clean1, ".")
	parts2 := strings.Split(clean2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}
