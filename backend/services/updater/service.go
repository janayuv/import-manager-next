package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	ChecksumURL     string `json:"checksumURL,omitempty"`
	ExpectedSHA256  string `json:"expectedSha256,omitempty"`
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
	cfg               config.Config
	paths             winpaths.AppPaths
	sec               *security.Service
	audit             *audit.Service
	httpClient        *http.Client
	apiBaseURL        string
	repo              string
	mu                sync.Mutex
	expectedChecksums map[string]string
	verifiedFiles     map[string]string
	allowedTestHosts  map[string]bool
	skipLaunchInTest  bool
}

// NewService constructs an update service with standard defaults.
func NewService(cfg config.Config, sec *security.Service, auditSvc *audit.Service) *Service {
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		repo = DefaultGitHubRepo
	}

	svc := &Service{
		cfg:               cfg,
		paths:             cfg.Paths,
		sec:               sec,
		audit:             auditSvc,
		apiBaseURL:        "https://api.github.com",
		repo:              repo,
		expectedChecksums: make(map[string]string),
		verifiedFiles:     make(map[string]string),
		allowedTestHosts:  make(map[string]bool),
	}
	svc.httpClient = &http.Client{
		Timeout: DefaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			req.Header.Del("Authorization")
			if err := svc.validateRedirectURL(req.URL.String()); err != nil {
				return fmt.Errorf("blocked redirect target: %w", err)
			}
			return nil
		},
	}
	return svc
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

// validateGitHubURL accepts only HTTPS release-asset URLs of the configured repository
// (https://github.com/<owner>/<repo>/releases/download/<tag>/<asset>). It guards every URL
// supplied by the frontend and the SHA256SUMS.txt URL derived from it, so a download can
// never be pointed at another GitHub repository or a raw githubusercontent.com file.
func (s *Service) validateGitHubURL(rawURL string) error {
	return s.validateUpdateURL(rawURL, false)
}

// validateRedirectURL additionally admits GitHub's release-asset CDN hosts
// (*.githubusercontent.com), which github.com redirects release downloads to.
// It is only used for redirects that start from an already-validated URL.
func (s *Service) validateRedirectURL(rawURL string) error {
	return s.validateUpdateURL(rawURL, true)
}

func (s *Service) validateUpdateURL(rawURL string, allowAssetCDN bool) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return errors.New("URL host is empty")
	}

	s.mu.Lock()
	isTestHost := s.allowedTestHosts[host]
	s.mu.Unlock()

	if isTestHost {
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
		}
		return nil
	}

	if u.Scheme != "https" {
		return fmt.Errorf("download URL must use HTTPS (got %q)", u.Scheme)
	}
	if u.User != nil {
		return errors.New("download URL must not contain credentials")
	}
	switch {
	case host == "github.com":
		if !s.isRepoReleaseAssetPath(u.Path) {
			return fmt.Errorf("update URL is not a release asset of github.com/%s", s.repo)
		}
		return nil
	case allowAssetCDN && strings.HasSuffix(host, ".githubusercontent.com"):
		return nil
	}
	return fmt.Errorf("untrusted update host %q (only release assets of github.com/%s are allowed)", host, s.repo)
}

// isRepoReleaseAssetPath reports whether p is exactly /<owner>/<repo>/releases/download/<tag>/<asset>
// for the configured repository, rejecting dot-segments and extra path levels.
func (s *Service) isRepoReleaseAssetPath(p string) bool {
	if p == "" || path.Clean(p) != p {
		return false
	}
	repo := strings.ToLower(strings.Trim(strings.TrimSpace(s.repo), "/"))
	if strings.Count(repo, "/") != 1 {
		return false
	}
	prefix := "/" + repo + "/releases/download/"
	lp := strings.ToLower(p)
	if !strings.HasPrefix(lp, prefix) {
		return false
	}
	parts := strings.Split(lp[len(prefix):], "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
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

		if checksumAsset := selectChecksumAsset(release.Assets); checksumAsset != nil {
			result.ChecksumURL = checksumAsset.BrowserDownloadURL
			if sums, sumErr := s.fetchSHA256Sums(ctx, checksumAsset.BrowserDownloadURL); sumErr == nil {
				s.mu.Lock()
				for name, hash := range sums {
					s.expectedChecksums[name] = hash
				}
				s.mu.Unlock()
				if expected, ok := sums[asset.Name]; ok {
					result.ExpectedSHA256 = expected
				}
			}
		}
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

// DownloadUpdate downloads the update executable from a trusted GitHub URL to the updates folder
// and verifies its SHA-256 digest against SHA256SUMS.txt before finalizing it.
func (s *Service) DownloadUpdate(ctx context.Context, downloadURL, assetName string) (string, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermAdminSystemTools); err != nil {
			return "", err
		}
	}

	if strings.TrimSpace(downloadURL) == "" {
		return "", errors.New("downloadURL cannot be empty")
	}
	if err := s.validateGitHubURL(downloadURL); err != nil {
		return "", err
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

	expectedHex, err := s.resolveExpectedSHA256(ctx, downloadURL, cleanAssetName)
	if err != nil {
		return "", err
	}

	destPath := filepath.Join(updatesDir, cleanAssetName)
	tempPath := destPath + ".download"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("ImportManager-Desktop/%s", s.GetCurrentVersion()))

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

	if err := VerifySHA256(tempPath, expectedHex); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("update integrity check failed: %w", err)
	}

	// Atomically replace destination
	_ = os.Remove(destPath)
	if err := os.Rename(tempPath, destPath); err != nil {
		return "", fmt.Errorf("finalize update file: %w", err)
	}

	s.mu.Lock()
	s.verifiedFiles[filepath.Clean(destPath)] = expectedHex
	s.mu.Unlock()

	if s.audit != nil {
		_ = s.audit.RecordUserActivity("system", "system.update_downloaded", "update", cleanAssetName, "success", "INFO", map[string]any{
			"assetName": cleanAssetName,
			"destPath":  destPath,
			"url":       downloadURL,
			"sha256":    expectedHex,
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

// ApplyUpdate launches the downloaded installer executable in background after verifying admin permission,
// path containment inside the updates directory, and SHA-256 integrity.
func (s *Service) ApplyUpdate(installerPath string) error {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermAdminSystemTools); err != nil {
			return err
		}
	}

	updatesDir := filepath.Clean(filepath.Join(s.paths.DataDir, "updates"))
	cleanPath := filepath.Clean(installerPath)
	rel, relErr := filepath.Rel(updatesDir, cleanPath)
	if relErr != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return errors.New("installer path must reside inside the updates directory")
	}

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

	s.mu.Lock()
	expectedHex := s.verifiedFiles[cleanPath]
	if expectedHex == "" {
		expectedHex = s.expectedChecksums[filepath.Base(cleanPath)]
	}
	skipLaunch := s.skipLaunchInTest
	s.mu.Unlock()

	if expectedHex == "" {
		return errors.New("installer SHA-256 checksum not verified; re-download the update from a release with SHA256SUMS.txt")
	}
	if err := VerifySHA256(cleanPath, expectedHex); err != nil {
		return fmt.Errorf("installer checksum verification failed: %w", err)
	}

	if !skipLaunch {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd.exe", "/c", "start", "", cleanPath)
		} else {
			cmd = exec.Command(cleanPath)
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("launch installer: %w", err)
		}
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity("system", "system.update_launched", "update", filepath.Base(cleanPath), "success", "INFO", map[string]any{
			"installerPath": cleanPath,
			"fileSize":      info.Size(),
			"sha256":        expectedHex,
		})
	}

	return nil
}

func (s *Service) resolveExpectedSHA256(ctx context.Context, downloadURL, assetName string) (string, error) {
	s.mu.Lock()
	cached := s.expectedChecksums[assetName]
	s.mu.Unlock()
	if cached != "" {
		return cached, nil
	}

	u, err := url.Parse(downloadURL)
	if err != nil {
		return "", fmt.Errorf("parse download URL: %w", err)
	}
	idx := strings.LastIndex(u.Path, "/")
	if idx == -1 {
		return "", fmt.Errorf("cannot derive SHA256SUMS.txt URL for %s", assetName)
	}
	u.Path = u.Path[:idx+1] + "SHA256SUMS.txt"
	u.RawQuery = ""
	u.Fragment = ""

	sums, err := s.fetchSHA256Sums(ctx, u.String())
	if err != nil {
		return "", fmt.Errorf("fetch SHA256SUMS.txt for update verification: %w", err)
	}

	s.mu.Lock()
	for name, hash := range sums {
		s.expectedChecksums[name] = hash
	}
	s.mu.Unlock()

	expected, ok := sums[assetName]
	if !ok || expected == "" {
		return "", fmt.Errorf("SHA256SUMS.txt does not contain a checksum for %q", assetName)
	}
	return expected, nil
}

func (s *Service) fetchSHA256Sums(ctx context.Context, checksumURL string) (map[string]string, error) {
	if err := s.validateGitHubURL(checksumURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("ImportManager-Desktop/%s", s.GetCurrentVersion()))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("checksum download failed with status %d", resp.StatusCode)
	}

	return parseSHA256Sums(io.LimitReader(resp.Body, 1<<20))
}

func parseSHA256Sums(r io.Reader) (map[string]string, error) {
	out := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// Windows PowerShell 5 writes UTF-8 with a byte-order mark; strip it so the first entry is kept.
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		digest := strings.ToLower(strings.TrimSpace(fields[0]))
		if len(digest) != 64 {
			continue
		}
		if _, err := hex.DecodeString(digest); err != nil {
			continue
		}
		name := filepath.Base(strings.TrimPrefix(fields[1], "*"))
		if name != "" && name != "." {
			out[name] = digest
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func selectChecksumAsset(assets []GitHubAsset) *GitHubAsset {
	for i := range assets {
		if strings.EqualFold(strings.TrimSpace(assets[i].Name), "SHA256SUMS.txt") {
			return &assets[i]
		}
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
