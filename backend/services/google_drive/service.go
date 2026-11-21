package google_drive

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/janayuv/import-manager-next/backend/crypto"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const (
	DefaultRedirectURI    = "http://127.0.0.1:8765/callback"
	DefaultFolderName     = "ImportManagerBackups"
	DefaultRetentionLimit = 30
	DefaultAuthBaseURL    = "https://accounts.google.com/o/oauth2/v2/auth"
	DefaultTokenURL       = "https://oauth2.googleapis.com/token"
	DefaultAPIBaseURL     = "https://www.googleapis.com"
	OAuthScopes           = "https://www.googleapis.com/auth/drive https://www.googleapis.com/auth/userinfo.email"
)

type oauthClientConfig struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type storedTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Expiry       string `json:"expiry"`
	Email        string `json:"email"`
	FolderID     string `json:"folderId"`
}

// Service manages Google Drive OAuth2 loopback authorization and encrypted backup cloud synchronization.
type Service struct {
	db          *sql.DB
	security    *security.Service
	audit       *audit.Service
	secrets     crypto.SecretStore
	backupsDir  string
	httpClient  *http.Client
	AuthBaseURL string
	TokenURL    string
	APIBaseURL  string
	RedirectURI string

	mu           sync.Mutex
	pendingState string
}

// NewService creates a Google Drive sync service.
func NewService(db *sql.DB, sec *security.Service, aud *audit.Service, secrets crypto.SecretStore, backupsDir string) *Service {
	return &Service{
		db:          db,
		security:    sec,
		audit:       aud,
		secrets:     secrets,
		backupsDir:  backupsDir,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		AuthBaseURL: DefaultAuthBaseURL,
		TokenURL:    DefaultTokenURL,
		APIBaseURL:  DefaultAPIBaseURL,
		RedirectURI: DefaultRedirectURI,
	}
}

// SetDB rebinds the service to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = db
}

func gdriveErr(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("GDRIVE_ERROR: %s: %w", msg, err)
	}
	return fmt.Errorf("GDRIVE_ERROR: %s", msg)
}

func formatDriveAPIError(action string, statusCode int, body []byte) error {
	raw := string(body)
	if statusCode == http.StatusForbidden || statusCode == http.StatusUnauthorized {
		if strings.Contains(raw, "ACCESS_TOKEN_SCOPE_INSUFFICIENT") || strings.Contains(raw, "insufficientPermissions") {
			return gdriveErr("Google Drive permission missing on your account. Please click 'Disconnect' and re-connect, and make sure to check the 'See, edit, create, and delete all your Google Drive files' checkbox during Google sign-in", nil)
		}
		if strings.Contains(raw, "invalid_grant") || strings.Contains(raw, "Token has been expired or revoked") {
			return gdriveErr("Google session expired or revoked. Please click 'Disconnect' and re-connect your Google account", nil)
		}
	}
	return gdriveErr(fmt.Sprintf("%s HTTP %d: %s", action, statusCode, raw), nil)
}

// SaveClientConfig persists Google OAuth2 desktop client credentials in Windows Credential Manager.
func (s *Service) SaveClientConfig(input models.GoogleDriveConfigInput) error {
	sess, err := s.security.RequirePermission(security.PermBackupSettings)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.ClientID) == "" || strings.TrimSpace(input.ClientSecret) == "" {
		return gdriveErr("client_id and client_secret are required", nil)
	}
	raw, err := json.Marshal(oauthClientConfig{
		ClientID:     strings.TrimSpace(input.ClientID),
		ClientSecret: strings.TrimSpace(input.ClientSecret),
	})
	if err != nil {
		return gdriveErr("marshal client config", err)
	}
	if err := s.secrets.SetSecret(crypto.GoogleDriveOAuthClientAccount, string(raw)); err != nil {
		return gdriveErr("save client config", err)
	}
	_ = s.audit.RecordTableAudit("google_drive", "client_config", "UPSERT", sess.UserID, nil, map[string]string{"configured": "true"}, nil)
	return nil
}

// GetStatus returns current Google Drive client & OAuth connection status.
func (s *Service) GetStatus() (models.GoogleDriveStatus, error) {
	if _, err := s.security.RequireAnyPermission(security.PermBackupCreate, security.PermBackupRestore, security.PermBackupSettings); err != nil {
		return models.GoogleDriveStatus{}, err
	}
	status := models.GoogleDriveStatus{
		FolderName:     DefaultFolderName,
		RetentionLimit: DefaultRetentionLimit,
	}
	if _, err := s.loadClientConfig(); err == nil {
		status.HasClientConfig = true
	}
	if tok, err := s.loadTokens(); err == nil && tok.AccessToken != "" {
		status.Connected = true
		status.AccountEmail = tok.Email
		status.FolderID = tok.FolderID
		status.TokenExpiry = tok.Expiry
	}
	return status, nil
}

// Disconnect removes stored OAuth tokens from Credential Manager.
func (s *Service) Disconnect() error {
	sess, err := s.security.RequirePermission(security.PermBackupSettings)
	if err != nil {
		return err
	}
	if err := s.secrets.DeleteSecret(crypto.GoogleDriveTokensAccount); err != nil {
		return gdriveErr("disconnect account", err)
	}
	_ = s.audit.RecordTableAudit("google_drive", "oauth", "DISCONNECT", sess.UserID, nil, nil, nil)
	return nil
}

// StartOAuthFlow generates the Google consent URL and spins up a single-shot 127.0.0.1:8765 callback listener.
func (s *Service) StartOAuthFlow() (string, error) {
	if _, err := s.security.RequirePermission(security.PermBackupSettings); err != nil {
		return "", err
	}
	cfg, err := s.loadClientConfig()
	if err != nil {
		return "", err
	}

	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	s.mu.Lock()
	s.pendingState = state
	s.mu.Unlock()

	authURL := s.buildAuthorizeURL(cfg.ClientID, s.RedirectURI, state)

	parsedRedirect, err := url.Parse(s.RedirectURI)
	if err == nil && parsedRedirect.Host != "" {
		if ln, listenErr := net.Listen("tcp", parsedRedirect.Host); listenErr == nil {
			go s.serveLoopbackCallback(ln, state)
		}
	}
	return authURL, nil
}

func (s *Service) buildAuthorizeURL(clientID, redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", OAuthScopes)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("state", state)
	return s.AuthBaseURL + "?" + q.Encode()
}

func (s *Service) serveLoopbackCallback(ln net.Listener, expectedState string) {
	defer ln.Close()
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			go func() {
				time.Sleep(150 * time.Millisecond)
				_ = srv.Shutdown(context.Background())
			}()
		}()
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errParam := r.URL.Query().Get("error")
		if errParam != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Connection Error</title><style>body{font-family:system-ui,-apple-system,sans-serif;background:#0f172a;color:#f8fafc;padding:3rem;text-align:center}h2{color:#ef4444}.box{background:#1e293b;border:1px solid #334155;border-radius:8px;padding:2rem;max-width:520px;margin:2rem auto;line-height:1.6}</style></head><body><div class="box"><h2>Authentication Canceled or Denied</h2><p>Google reported: <strong>%s</strong></p><p>You may close this tab and try again in Import Manager.</p></div></body></html>`, errParam))
			return
		}
		if state != expectedState || code == "" {
			http.Error(w, "Invalid OAuth callback state or missing authorization code", http.StatusBadRequest)
			return
		}
		if err := s.exchangeCodeAndPersist(r.Context(), code); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Connection Failed</title><style>body{font-family:system-ui,-apple-system,sans-serif;background:#0f172a;color:#f8fafc;padding:3rem;text-align:center}h2{color:#ef4444}.box{background:#1e293b;border:1px solid #334155;border-radius:8px;padding:2rem;max-width:520px;margin:2rem auto;line-height:1.6}</style></head><body><div class="box"><h2>Authorization Incomplete</h2><p>%s</p><p>Please return to Import Manager and try again.</p></div></body></html>`, err.Error()))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Connected to Google Drive</title><style>body{font-family:system-ui,-apple-system,sans-serif;background:#0f172a;color:#f8fafc;padding:3rem;text-align:center}h2{color:#22c55e}.box{background:#1e293b;border:1px solid #334155;border-radius:8px;padding:2rem;max-width:520px;margin:2rem auto;line-height:1.6}</style></head><body><div class="box"><h2>✓ Google Drive Connected!</h2><p>Your Google Drive account has been successfully linked to Import Manager for backups.</p><p style="color:#94a3b8">You may close this browser tab and return to Import Manager.</p></div></body></html>`)
	})

	go func() {
		time.Sleep(3 * time.Minute)
		_ = srv.Shutdown(context.Background())
	}()
	_ = srv.Serve(ln)
}

// CompleteOAuthCode manually exchanges an authorization code (used in headless/CLI fallback or tests).
func (s *Service) CompleteOAuthCode(code, state string) (models.GoogleDriveStatus, error) {
	sess, err := s.security.RequirePermission(security.PermBackupSettings)
	if err != nil {
		return models.GoogleDriveStatus{}, err
	}
	s.mu.Lock()
	expected := s.pendingState
	s.mu.Unlock()
	if expected != "" && state != "" && state != expected {
		return models.GoogleDriveStatus{}, gdriveErr("state mismatch during OAuth exchange", nil)
	}
	if err := s.exchangeCodeAndPersist(context.Background(), code); err != nil {
		return models.GoogleDriveStatus{}, err
	}
	_ = s.audit.RecordTableAudit("google_drive", "oauth", "CONNECT", sess.UserID, nil, map[string]string{"connected": "true"}, nil)
	return s.GetStatus()
}

func (s *Service) exchangeCodeAndPersist(ctx context.Context, code string) error {
	if strings.TrimSpace(code) == "" {
		return gdriveErr("authorization code is required", nil)
	}
	cfg, err := s.loadClientConfig()
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("redirect_uri", s.RedirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return gdriveErr("build token request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return gdriveErr("token exchange request failed", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return gdriveErr(fmt.Sprintf("token endpoint returned HTTP %d: %s", resp.StatusCode, string(body)), nil)
	}

	var parsed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return gdriveErr("parse token response", err)
	}
	if parsed.AccessToken == "" {
		return gdriveErr("empty access_token from Google OAuth", nil)
	}
	if parsed.Scope != "" && !strings.Contains(parsed.Scope, "drive") {
		return gdriveErr("Google Drive permission was not granted by your Google account. In the Google sign-in consent window, you must check the box to allow Google Drive access", nil)
	}

	expiry := time.Now().UTC().Add(time.Duration(max(parsed.ExpiresIn, 3600)) * time.Second).Format(time.RFC3339)
	tok := storedTokens{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		Expiry:       expiry,
	}
	if tok.RefreshToken == "" {
		if prevTok, prevErr := s.loadTokens(); prevErr == nil && prevTok.RefreshToken != "" {
			tok.RefreshToken = prevTok.RefreshToken
		}
	}
	if email, err := s.fetchUserEmail(ctx, tok.AccessToken); err == nil {
		tok.Email = email
	}
	return s.saveTokens(tok)
}

func (s *Service) fetchUserEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.APIBaseURL+"/oauth2/v2/userinfo", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.Email, nil
}

func (s *Service) ensureValidAccessToken(ctx context.Context) (storedTokens, error) {
	tok, err := s.loadTokens()
	if err != nil {
		return storedTokens{}, err
	}
	if tok.Expiry != "" {
		if exp, parseErr := time.Parse(time.RFC3339, tok.Expiry); parseErr == nil {
			if time.Now().UTC().Add(60 * time.Second).Before(exp) {
				return tok, nil
			}
		}
	}
	if tok.RefreshToken == "" {
		return tok, nil
	}
	cfg, err := s.loadClientConfig()
	if err != nil {
		return storedTokens{}, err
	}

	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("refresh_token", tok.RefreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return storedTokens{}, gdriveErr("build refresh request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return storedTokens{}, gdriveErr("refresh token request failed", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return storedTokens{}, gdriveErr(fmt.Sprintf("refresh HTTP %d: %s", resp.StatusCode, string(body)), nil)
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.AccessToken == "" {
		return storedTokens{}, gdriveErr("parse refresh response", err)
	}
	tok.AccessToken = parsed.AccessToken
	tok.Expiry = time.Now().UTC().Add(time.Duration(max(parsed.ExpiresIn, 3600)) * time.Second).Format(time.RFC3339)
	_ = s.saveTokens(tok)
	return tok, nil
}

func (s *Service) ensureBackupFolder(ctx context.Context, tok storedTokens) (string, error) {
	if tok.FolderID != "" {
		return tok.FolderID, nil
	}
	query := url.QueryEscape(fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and trashed=false", DefaultFolderName))
	listURL := fmt.Sprintf("%s/drive/v3/files?q=%s&fields=files(id,name)", s.APIBaseURL, query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return "", gdriveErr("build folder lookup request", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", gdriveErr("folder lookup failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", formatDriveAPIError("folder lookup", resp.StatusCode, body)
	}

	var listResp struct {
		Files []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err == nil && len(listResp.Files) > 0 {
		tok.FolderID = listResp.Files[0].ID
		_ = s.saveTokens(tok)
		return tok.FolderID, nil
	}

	metaBody, _ := json.Marshal(map[string]any{
		"name":     DefaultFolderName,
		"mimeType": "application/vnd.google-apps.folder",
	})
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIBaseURL+"/drive/v3/files?fields=id", bytes.NewReader(metaBody))
	if err != nil {
		return "", gdriveErr("build create folder request", err)
	}
	createReq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := s.httpClient.Do(createReq)
	if err != nil {
		return "", gdriveErr("create backup folder failed", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode < 200 || createResp.StatusCode >= 300 {
		body, _ := io.ReadAll(createResp.Body)
		return "", formatDriveAPIError("create folder", createResp.StatusCode, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil || created.ID == "" {
		return "", gdriveErr("parse created folder id", err)
	}
	tok.FolderID = created.ID
	_ = s.saveTokens(tok)
	return created.ID, nil
}

// UploadBackup uploads a local encrypted backup file into Google Drive's ImportManagerBackups folder
// and enforces retention of at most 30 cloud backups.
func (s *Service) UploadBackup(localFilePath string) (models.GoogleDriveRemoteFile, error) {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermBackupCreate)
	if err != nil {
		return models.GoogleDriveRemoteFile{}, err
	}
	tok, err := s.ensureValidAccessToken(ctx)
	if err != nil {
		return models.GoogleDriveRemoteFile{}, err
	}
	folderID, err := s.ensureBackupFolder(ctx, tok)
	if err != nil {
		return models.GoogleDriveRemoteFile{}, err
	}

	payload, err := os.ReadFile(localFilePath)
	if err != nil {
		return models.GoogleDriveRemoteFile{}, gdriveErr("read local backup file", err)
	}
	filename := filepath.Base(localFilePath)

	var bodyBuf bytes.Buffer
	writer := multipart.NewWriter(&bodyBuf)
	metaHeader := make(textproto.MIMEHeader)
	metaHeader.Set("Content-Type", "application/json; charset=UTF-8")
	metaPart, err := writer.CreatePart(metaHeader)
	if err != nil {
		return models.GoogleDriveRemoteFile{}, gdriveErr("create metadata part", err)
	}
	metaJSON, _ := json.Marshal(map[string]any{
		"name":    filename,
		"parents": []string{folderID},
	})
	_, _ = metaPart.Write(metaJSON)

	mediaHeader := make(textproto.MIMEHeader)
	mediaHeader.Set("Content-Type", "application/octet-stream")
	mediaPart, err := writer.CreatePart(mediaHeader)
	if err != nil {
		return models.GoogleDriveRemoteFile{}, gdriveErr("create media part", err)
	}
	_, _ = mediaPart.Write(payload)
	_ = writer.Close()

	uploadURL := s.APIBaseURL + "/upload/drive/v3/files?uploadType=multipart&fields=id,name,size,createdTime,modifiedTime"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &bodyBuf)
	if err != nil {
		return models.GoogleDriveRemoteFile{}, gdriveErr("build upload request", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "multipart/related; boundary="+writer.Boundary())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return models.GoogleDriveRemoteFile{}, gdriveErr("upload request failed", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.GoogleDriveRemoteFile{}, formatDriveAPIError("upload", resp.StatusCode, rawResp)
	}

	var uploaded struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Size         string `json:"size"`
		CreatedTime  string `json:"createdTime"`
		ModifiedTime string `json:"modifiedTime"`
	}
	if err := json.Unmarshal(rawResp, &uploaded); err != nil {
		return models.GoogleDriveRemoteFile{}, gdriveErr("parse upload response", err)
	}
	sizeVal, _ := strconv.ParseInt(uploaded.Size, 10, 64)
	if sizeVal == 0 {
		sizeVal = int64(len(payload))
	}

	remote := models.GoogleDriveRemoteFile{
		ID:           uploaded.ID,
		Name:         uploaded.Name,
		SizeBytes:    sizeVal,
		CreatedTime:  uploaded.CreatedTime,
		ModifiedTime: uploaded.ModifiedTime,
		Locator:      "gdrive:" + uploaded.ID,
	}

	_ = s.enforceRetention(ctx, tok, folderID)
	_ = s.audit.RecordTableAudit("google_drive", uploaded.ID, "UPLOAD", sess.UserID, nil, remote, nil)
	return remote, nil
}

// ListBackups lists encrypted backups inside the ImportManagerBackups Google Drive folder.
func (s *Service) ListBackups() ([]models.GoogleDriveRemoteFile, error) {
	ctx := context.Background()
	if _, err := s.security.RequireAnyPermission(security.PermBackupCreate, security.PermBackupRestore); err != nil {
		return nil, err
	}
	tok, err := s.ensureValidAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	folderID, err := s.ensureBackupFolder(ctx, tok)
	if err != nil {
		return nil, err
	}
	return s.listFolderFiles(ctx, tok, folderID)
}

func (s *Service) listFolderFiles(ctx context.Context, tok storedTokens, folderID string) ([]models.GoogleDriveRemoteFile, error) {
	q := url.QueryEscape(fmt.Sprintf("'%s' in parents and trashed=false", folderID))
	listURL := fmt.Sprintf("%s/drive/v3/files?q=%s&orderBy=createdTime%%20desc&fields=files(id,name,size,createdTime,modifiedTime)", s.APIBaseURL, q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, gdriveErr("build list request", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, gdriveErr("list remote backups failed", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, formatDriveAPIError("list remote backups", resp.StatusCode, body)
	}
	var parsed struct {
		Files []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Size         string `json:"size"`
			CreatedTime  string `json:"createdTime"`
			ModifiedTime string `json:"modifiedTime"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, gdriveErr("parse drive file list", err)
	}
	out := make([]models.GoogleDriveRemoteFile, 0, len(parsed.Files))
	for _, f := range parsed.Files {
		sz, _ := strconv.ParseInt(f.Size, 10, 64)
		out = append(out, models.GoogleDriveRemoteFile{
			ID:           f.ID,
			Name:         f.Name,
			SizeBytes:    sz,
			CreatedTime:  f.CreatedTime,
			ModifiedTime: f.ModifiedTime,
			Locator:      "gdrive:" + f.ID,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedTime > out[j].CreatedTime
	})
	return out, nil
}

func (s *Service) enforceRetention(ctx context.Context, tok storedTokens, folderID string) error {
	files, err := s.listFolderFiles(ctx, tok, folderID)
	if err != nil || len(files) <= DefaultRetentionLimit {
		return err
	}
	for _, extra := range files[DefaultRetentionLimit:] {
		delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.APIBaseURL+"/drive/v3/files/"+url.PathEscape(extra.ID), nil)
		if err != nil {
			continue
		}
		delReq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		if resp, err := s.httpClient.Do(delReq); err == nil {
			_ = resp.Body.Close()
		}
	}
	return nil
}

// DownloadBackup downloads a cloud backup (accepts raw fileId or "gdrive:<fileId>") into local backupsDir.
func (s *Service) DownloadBackup(fileIDOrLocator, targetFilename string) (string, error) {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermBackupRestore)
	if err != nil {
		return "", err
	}
	fileID := strings.TrimPrefix(strings.TrimSpace(fileIDOrLocator), "gdrive:")
	if fileID == "" {
		return "", gdriveErr("file id or gdrive:<id> locator is required", nil)
	}
	tok, err := s.ensureValidAccessToken(ctx)
	if err != nil {
		return "", err
	}

	dlURL := fmt.Sprintf("%s/drive/v3/files/%s?alt=media", s.APIBaseURL, url.PathEscape(fileID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return "", gdriveErr("build download request", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", gdriveErr("download request failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", formatDriveAPIError("download backup", resp.StatusCode, body)
	}
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", gdriveErr("read download payload", err)
	}

	if err := os.MkdirAll(s.backupsDir, 0o700); err != nil {
		return "", err
	}
	name := filepath.Base(strings.TrimSpace(targetFilename))
	if name == "" || name == "." {
		name = fmt.Sprintf("gdrive_%s.imbk2", fileID)
	}
	outPath := filepath.Join(s.backupsDir, name)
	if err := os.WriteFile(outPath, payload, 0o600); err != nil {
		return "", gdriveErr("save downloaded backup", err)
	}
	_, _ = crypto.WriteSidecarSHA256(outPath, payload)
	_ = s.audit.RecordTableAudit("google_drive", fileID, "DOWNLOAD", sess.UserID, nil, map[string]string{"localPath": outPath}, nil)
	return outPath, nil
}

// DeleteBackup deletes a cloud backup file from Google Drive (accepts raw fileId or "gdrive:<fileId>").
func (s *Service) DeleteBackup(fileIDOrLocator string) error {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermBackupCreate)
	if err != nil {
		return err
	}
	fileID := strings.TrimPrefix(strings.TrimSpace(fileIDOrLocator), "gdrive:")
	if fileID == "" {
		return gdriveErr("file id or gdrive:<id> locator is required", nil)
	}
	tok, err := s.ensureValidAccessToken(ctx)
	if err != nil {
		return err
	}

	delURL := fmt.Sprintf("%s/drive/v3/files/%s", s.APIBaseURL, url.PathEscape(fileID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, delURL, nil)
	if err != nil {
		return gdriveErr("build delete request", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return gdriveErr("delete remote backup failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return formatDriveAPIError("delete backup", resp.StatusCode, body)
	}

	_ = s.audit.RecordTableAudit("google_drive", fileID, "DELETE", sess.UserID, nil, nil, nil)
	return nil
}

func (s *Service) loadClientConfig() (oauthClientConfig, error) {
	if s.secrets == nil {
		return oauthClientConfig{}, gdriveErr("credential store unavailable", nil)
	}
	raw, err := s.secrets.GetSecret(crypto.GoogleDriveOAuthClientAccount)
	if err != nil {
		if errors.Is(err, crypto.ErrSecretNotFound) {
			return oauthClientConfig{}, gdriveErr("Google OAuth client credentials not configured", nil)
		}
		return oauthClientConfig{}, gdriveErr("read client config", err)
	}
	var cfg oauthClientConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return oauthClientConfig{}, gdriveErr("parse client config", err)
	}
	return cfg, nil
}

func (s *Service) loadTokens() (storedTokens, error) {
	if s.secrets == nil {
		return storedTokens{}, gdriveErr("credential store unavailable", nil)
	}
	raw, err := s.secrets.GetSecret(crypto.GoogleDriveTokensAccount)
	if err != nil {
		if errors.Is(err, crypto.ErrSecretNotFound) {
			return storedTokens{}, gdriveErr("Google Drive account is not connected", nil)
		}
		return storedTokens{}, gdriveErr("read OAuth tokens", err)
	}
	var tok storedTokens
	if err := json.Unmarshal([]byte(raw), &tok); err != nil {
		return storedTokens{}, gdriveErr("parse OAuth tokens", err)
	}
	return tok, nil
}

func (s *Service) saveTokens(tok storedTokens) error {
	raw, err := json.Marshal(tok)
	if err != nil {
		return gdriveErr("encode OAuth tokens", err)
	}
	if err := s.secrets.SetSecret(crypto.GoogleDriveTokensAccount, string(raw)); err != nil {
		return gdriveErr("persist OAuth tokens", err)
	}
	return nil
}
