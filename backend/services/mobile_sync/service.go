package mobile_sync

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/janayuv/import-manager-next/backend/crypto"
	"github.com/janayuv/import-manager-next/backend/logging"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/backup"
	"github.com/janayuv/import-manager-next/backend/services/company"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const (
	DefaultMobileSyncPort       = 8766
	SessionTTLHours             = 24
	mobileSyncEnabledSettingKey = "mobile_sync.enabled"
)

// ServerInfo represents the status of the mobile sync companion server.
type ServerInfo struct {
	Enabled   bool     `json:"enabled"`
	IsRunning bool     `json:"isRunning"`
	Port      int      `json:"port"`
	Addresses []string `json:"addresses"`
	Version   string   `json:"version"`
}

// LoginRequest models mobile credentials payload.
type LoginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse models authentication result.
type LoginResponse struct {
	Token        string `json:"token"`
	Username     string `json:"username"`
	UserID       string `json:"userId"`
	Role         string `json:"role"`
	ExpiresAt    string `json:"expiresAt"`
	ActiveCompany string `json:"activeCompany,omitempty"`
}

// StatusResponse models the public health check of the sync server.
type StatusResponse struct {
	Status        string `json:"status"`
	AppName       string `json:"appName"`
	Version       string `json:"version"`
	ActiveCompany string `json:"activeCompany"`
	LastBackup    string `json:"lastBackup,omitempty"`
}

type tokenRecord struct {
	Username  string
	UserID    string
	Role      string
	ExpiresAt time.Time
}

type failedLoginRecord struct {
	Count        int
	LockoutUntil time.Time
}

// Service provides a local HTTP server allowing the Android mobile companion app
// to authenticate with Email/Username and Password, and download the latest .imbk2 backup.
type Service struct {
	mu           sync.RWMutex
	db           *sql.DB
	server       *http.Server
	listener     net.Listener
	port         int
	isRunning    bool
	addresses    []string
	securitySvc  *security.Service
	backupSvc    *backup.Service
	companySvc   *company.Service
	auditSvc     *audit.Service
	secrets      crypto.SecretStore
	logger       *logging.Logger
	backupsDir   string
	appVersion   string
	activeTokens map[string]tokenRecord
	failedLogins map[string]failedLoginRecord
}

// NewService instantiates the mobile sync companion service.
func NewService(
	securitySvc *security.Service,
	backupSvc *backup.Service,
	companySvc *company.Service,
	auditSvc *audit.Service,
	secrets crypto.SecretStore,
	backupsDir string,
	appVersion string,
	logger *logging.Logger,
) *Service {
	svc := &Service{
		securitySvc:  securitySvc,
		backupSvc:    backupSvc,
		companySvc:   companySvc,
		auditSvc:     auditSvc,
		secrets:      secrets,
		backupsDir:   backupsDir,
		appVersion:   appVersion,
		logger:       logger,
		activeTokens: make(map[string]tokenRecord),
		failedLogins: make(map[string]failedLoginRecord),
	}
	return svc
}

// ControlService exposes only safe opt-in controls and status queries to the Wails frontend.
type ControlService struct {
	svc *Service
}

// NewControlService constructs a Wails-bound facade for mobile sync opt-in and status.
func NewControlService(svc *Service) *ControlService {
	return &ControlService{svc: svc}
}

// GetServerInfo returns details of the currently running mobile sync server.
func (c *ControlService) GetServerInfo() (ServerInfo, error) {
	return c.svc.GetServerInfo()
}

// GetActiveDeviceCount returns the number of active mobile companion session tokens.
func (c *ControlService) GetActiveDeviceCount() int {
	return c.svc.GetActiveDeviceCount()
}

// IsEnabled reports whether mobile sync has been opted into.
func (c *ControlService) IsEnabled() bool {
	return c.svc.IsEnabled()
}

// SetEnabled toggles and persists the opt-in mobile sync server.
func (c *ControlService) SetEnabled(enabled bool) (ServerInfo, error) {
	return c.svc.SetEnabled(enabled)
}

// SetDB binds the database handle used to persist the opt-in mobile sync setting.
// When the active company changes, previously issued mobile tokens are revoked, because they
// were authenticated against the old company's users and must not grant access to the new one.
func (s *Service) SetDB(db *sql.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil && s.db != db {
		s.activeTokens = make(map[string]tokenRecord)
	}
	s.db = db
}

// ApplyEnabledSetting starts or stops the listener so it matches the opt-in setting stored in
// the currently bound (active company) database. Call it after SetDB when the company changes.
func (s *Service) ApplyEnabledSetting() error {
	if s.IsEnabled() {
		return s.Start(DefaultMobileSyncPort)
	}
	return s.Stop()
}

// IsEnabled reports whether mobile sync has been explicitly opted into via app_settings.
func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return false
	}
	var val string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, mobileSyncEnabledSettingKey).Scan(&val); err != nil {
		return false
	}
	clean := strings.ToLower(strings.TrimSpace(val))
	return clean == "true" || clean == "1"
}

// SetEnabled persists the mobile sync opt-in setting and starts or stops the local listener accordingly.
func (s *Service) SetEnabled(enabled bool) (ServerInfo, error) {
	if s.securitySvc != nil {
		if _, err := s.securitySvc.RequireAnyPermission(security.PermAdminSystemTools, security.PermBackupSettings); err != nil {
			return ServerInfo{}, err
		}
	}

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db != nil {
		val := "false"
		if enabled {
			val = "true"
		}
		if _, err := db.Exec(`
			INSERT INTO app_settings (key, value, updated_at)
			VALUES (?, ?, datetime('now', 'utc'))
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			mobileSyncEnabledSettingKey, val,
		); err != nil {
			return ServerInfo{}, fmt.Errorf("persist mobile sync setting: %w", err)
		}
	}

	if enabled {
		if err := s.Start(DefaultMobileSyncPort); err != nil {
			return ServerInfo{}, err
		}
	} else {
		if err := s.Stop(); err != nil {
			return ServerInfo{}, err
		}
	}
	return s.GetServerInfo()
}

// Start initiates the mobile sync HTTP listener on port 8766 (or fallback range).
func (s *Service) Start(preferredPort int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return nil
	}

	if preferredPort <= 0 {
		preferredPort = DefaultMobileSyncPort
	}

	var ln net.Listener
	var err error
	var chosenPort int

	for p := preferredPort; p < preferredPort+10; p++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p))
		if err == nil {
			chosenPort = p
			break
		}
	}

	if ln == nil {
		return fmt.Errorf("failed to bind mobile sync server on port %d-%d: %w", preferredPort, preferredPort+9, err)
	}

	s.listener = ln
	s.port = chosenPort
	s.isRunning = true
	s.addresses = s.resolveServerAddresses(chosenPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/mobile/status", s.handleStatus)
	mux.HandleFunc("/api/mobile/login", s.handleLogin)
	mux.HandleFunc("/api/mobile/sync/latest", s.handleSyncLatest)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	s.server = srv

	go func(srv *http.Server, ln net.Listener) {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			if s.logger != nil {
				s.logger.Warn("mobile sync server terminated", "error", serveErr)
			}
		}
	}(srv, ln)

	if s.logger != nil {
		s.logger.Info("mobile sync server started", "port", chosenPort, "addresses", s.addresses)
	}
	return nil
}

// Stop gracefully shuts down the mobile sync HTTP server.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning || s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := s.server.Shutdown(ctx)
	s.isRunning = false
	s.server = nil
	s.listener = nil
	return err
}

// GetServerInfo returns details of the currently running mobile sync server.
func (s *Service) GetServerInfo() (ServerInfo, error) {
	enabled := s.IsEnabled()

	s.mu.RLock()
	defer s.mu.RUnlock()

	return ServerInfo{
		Enabled:   enabled || s.isRunning,
		IsRunning: s.isRunning,
		Port:      s.port,
		Addresses: s.addresses,
		Version:   s.appVersion,
	}, nil
}

// GetActiveDeviceCount returns the number of active, unexpired mobile companion session tokens.
func (s *Service) GetActiveDeviceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	count := 0
	for _, tok := range s.activeTokens {
		if now.Before(tok.ExpiresAt) {
			count++
		}
	}
	return count
}

func clientIP(r *http.Request) string {
	remoteIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return remoteIP
}

func (s *Service) isLockedOut(remoteIP string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, exists := s.failedLogins[remoteIP]
	if !exists {
		return false
	}
	if time.Now().Before(rec.LockoutUntil) {
		return true
	}
	if !rec.LockoutUntil.IsZero() {
		delete(s.failedLogins, remoteIP)
	}
	return false
}

func (s *Service) recordFailedLogin(remoteIP string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.failedLogins[remoteIP]
	rec.Count++
	if rec.Count >= 5 {
		rec.LockoutUntil = time.Now().Add(15 * time.Minute)
	}
	s.failedLogins[remoteIP] = rec
}

func (s *Service) clearFailedLogin(remoteIP string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failedLogins, remoteIP)
}

func (s *Service) verifyBackupPassphrase(candidate string) bool {
	if s.secrets == nil || candidate == "" {
		return false
	}
	savedPass, err := s.secrets.GetSecret(crypto.BackupPassphraseAccount)
	if err != nil || savedPass == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(savedPass), []byte(candidate)) == 1
}

func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var activeCompName string
	if s.companySvc != nil {
		if comp, err := s.companySvc.GetActiveCompany(); err == nil {
			activeCompName = comp.Name
		}
	}
	if activeCompName == "" {
		activeCompName = "Default Company"
	}

	latestBackup := s.getLatestBackupFileName()

	resp := StatusResponse{
		Status:        "online",
		AppName:       "Import Manager Next Generation",
		Version:       s.appVersion,
		ActiveCompany: activeCompName,
		LastBackup:    latestBackup,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	identifier := strings.TrimSpace(req.Username)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	password := strings.TrimSpace(req.Password)

	if identifier == "" || password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Username/Email and Password are required"})
		return
	}

	remoteIP := clientIP(r)
	if s.isLockedOut(remoteIP) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Too many failed login attempts. Please try again later."})
		return
	}

	// 1. Try standard user authentication
	authenticated := false
	var username, userID, role string

	if s.securitySvc != nil {
		sess, err := s.securitySvc.Authenticate(identifier, password, false)
		if err == nil {
			authenticated = true
			username = sess.Username
			userID = sess.UserID
			role = sess.Role
		}
	}

	// 2. Allow backup passphrase authentication as a fallback
	if !authenticated && s.verifyBackupPassphrase(password) {
		authenticated = true
		username = identifier
		userID = "usr_backup_passphrase"
		role = "admin"
	}

	if !authenticated {
		s.recordFailedLogin(remoteIP)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email, username, or password"})
		return
	}

	s.clearFailedLogin(remoteIP)

	token := uuid.NewString()
	expiresAt := time.Now().Add(SessionTTLHours * time.Hour)

	s.mu.Lock()
	s.activeTokens[token] = tokenRecord{
		Username:  username,
		UserID:    userID,
		Role:      role,
		ExpiresAt: expiresAt,
	}
	s.mu.Unlock()

	var activeCompName string
	if s.companySvc != nil {
		if comp, err := s.companySvc.GetActiveCompany(); err == nil {
			activeCompName = comp.Name
		}
	}

	resp := LoginResponse{
		Token:         token,
		Username:      username,
		UserID:        userID,
		Role:          role,
		ExpiresAt:     expiresAt.Format(time.RFC3339),
		ActiveCompany: activeCompName,
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.RecordUserActivity(username, "MOBILE_SYNC_LOGIN", "mobile_sync", userID, "SUCCESS", "INFO", map[string]any{
			"remoteAddr": r.RemoteAddr,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Service) handleSyncLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Authenticate token
	token := s.extractToken(r)
	if !s.validateToken(token) {
		remoteIP := clientIP(r)
		if s.isLockedOut(remoteIP) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Too many failed login attempts. Please try again later."})
			return
		}

		// Also check Basic Auth as fallback
		user, pass, ok := r.BasicAuth()
		if !ok || !s.authenticateBasic(user, pass) {
			if ok {
				s.recordFailedLogin(remoteIP)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized. Please login first."})
			return
		}
		s.clearFailedLogin(remoteIP)
	}

	// 2. Locate or generate latest .imbk2 backup
	latestPath, filename, err := s.getOrGenerateLatestBackup()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to locate backup: %v", err)})
		return
	}

	file, err := os.Open(latestPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to open backup: %v", err)})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("X-Backup-Filename", filename)
	w.Header().Set("X-Backup-Size", fmt.Sprintf("%d", stat.Size()))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	_, _ = io.Copy(w, file)
}

func (s *Service) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	if tok := r.Header.Get("X-Auth-Token"); tok != "" {
		return strings.TrimSpace(tok)
	}
	return r.URL.Query().Get("token")
}

func (s *Service) validateToken(token string) bool {
	if token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.activeTokens[token]
	if !ok {
		return false
	}
	if time.Now().After(rec.ExpiresAt) {
		delete(s.activeTokens, token)
		return false
	}
	return true
}

func (s *Service) authenticateBasic(identifier, password string) bool {
	identifier = strings.TrimSpace(identifier)
	password = strings.TrimSpace(password)
	if identifier == "" || password == "" {
		return false
	}
	if s.securitySvc != nil {
		if _, err := s.securitySvc.Authenticate(identifier, password, false); err == nil {
			return true
		}
	}
	return s.verifyBackupPassphrase(password)
}

func (s *Service) getLatestBackupFileName() string {
	_, filename, ok := s.latestCompanyBackup()
	if !ok {
		return ""
	}
	return filename
}

// latestCompanyBackup returns the newest completed .imbk2 backup recorded in the active
// company's database whose file still exists. All companies share one backups folder, so the
// folder itself is never scanned: that could hand a phone another company's backup.
func (s *Service) latestCompanyBackup() (path string, filename string, ok bool) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return "", "", false
	}

	rows, err := db.Query(`
		SELECT path FROM backups
		WHERE COALESCE(status, 'completed') = 'completed' AND COALESCE(path, '') <> ''
		ORDER BY datetime(created_at) DESC, id DESC
		LIMIT 50`)
	if err != nil {
		return "", "", false
	}
	defer rows.Close()

	for rows.Next() {
		var p string
		if rows.Scan(&p) != nil {
			continue
		}
		if !strings.EqualFold(filepath.Ext(p), ".imbk2") {
			continue
		}
		if st, statErr := os.Stat(p); statErr == nil && st.Mode().IsRegular() && st.Size() > 0 {
			return p, filepath.Base(p), true
		}
	}
	return "", "", false
}

func (s *Service) getOrGenerateLatestBackup() (path string, filename string, err error) {
	if p, f, ok := s.latestCompanyBackup(); ok {
		return p, f, nil
	}

	// If no backup exists, create on-demand snapshot
	if s.backupSvc != nil {
		rec, bErr := s.backupSvc.CreateBackup(models.CreateBackupRequest{
			Destination: "local",
			Notes:       "On-demand mobile companion backup",
		})
		if bErr == nil && rec.Path != "" {
			return rec.Path, rec.Filename, nil
		}
	}

	return "", "", errors.New("no backup files (.imbk2) available")
}

func (s *Service) resolveServerAddresses(port int) []string {
	var addrs []string
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			// Skip loopback and down interfaces
			if (iface.Flags&net.FlagLoopback != 0) || (iface.Flags&net.FlagUp == 0) {
				continue
			}
			ifAddrs, addrErr := iface.Addrs()
			if addrErr != nil {
				continue
			}
			for _, a := range ifAddrs {
				if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					if ip4 := ipNet.IP.To4(); ip4 != nil {
						addrs = append(addrs, fmt.Sprintf("http://%s:%d", ip4.String(), port))
					}
				}
			}
		}
	}

	addrs = append(addrs, fmt.Sprintf("http://127.0.0.1:%d", port))
	return addrs
}
