package mobile_sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	DefaultMobileSyncPort = 8766
	SessionTTLHours       = 24
)

// ServerInfo represents the status of the mobile sync companion server.
type ServerInfo struct {
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

	handler := s.corsMiddleware(mux)

	srv := &http.Server{
		Handler:      handler,
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ServerInfo{
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

func (s *Service) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Auth-Token, X-Requested-With, X-Passphrase")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, X-Backup-Filename, X-Backup-Size")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
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

	remoteIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		remoteIP = host
	}

	s.mu.Lock()
	if rec, exists := s.failedLogins[remoteIP]; exists {
		if time.Now().Before(rec.LockoutUntil) {
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Too many failed login attempts. Please try again later."})
			return
		}
	}
	s.mu.Unlock()

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
	if !authenticated && s.secrets != nil {
		savedPass, err := s.secrets.GetSecret(crypto.BackupPassphraseAccount)
		if err == nil && savedPass != "" && savedPass == password {
			authenticated = true
			username = identifier
			userID = "usr_backup_passphrase"
			role = "admin"
		}
	}

	if !authenticated {
		s.mu.Lock()
		rec := s.failedLogins[remoteIP]
		rec.Count++
		if rec.Count >= 5 {
			rec.LockoutUntil = time.Now().Add(15 * time.Minute)
		}
		s.failedLogins[remoteIP] = rec
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email, username, or password"})
		return
	}

	s.mu.Lock()
	delete(s.failedLogins, remoteIP)
	s.mu.Unlock()

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
		// Also check Basic Auth as fallback
		user, pass, ok := r.BasicAuth()
		if !ok || !s.authenticateBasic(user, pass) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized. Please login first."})
			return
		}
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
	if s.secrets != nil {
		if savedPass, err := s.secrets.GetSecret(crypto.BackupPassphraseAccount); err == nil && savedPass != "" && savedPass == password {
			return true
		}
	}
	return false
}

func (s *Service) getLatestBackupFileName() string {
	files, err := filepath.Glob(filepath.Join(s.backupsDir, "*.imbk2"))
	if err != nil || len(files) == 0 {
		return ""
	}
	sort.Slice(files, func(i, j int) bool {
		fi, errI := os.Stat(files[i])
		fj, errJ := os.Stat(files[j])
		if errI != nil || errJ != nil {
			return files[i] > files[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return filepath.Base(files[0])
}

func (s *Service) getOrGenerateLatestBackup() (path string, filename string, err error) {
	files, globErr := filepath.Glob(filepath.Join(s.backupsDir, "*.imbk2"))
	if globErr == nil && len(files) > 0 {
		sort.Slice(files, func(i, j int) bool {
			fi, errI := os.Stat(files[i])
			fj, errJ := os.Stat(files[j])
			if errI != nil || errJ != nil {
				return files[i] > files[j]
			}
			return fi.ModTime().After(fj.ModTime())
		})
		path = files[0]
		filename = filepath.Base(path)
		return path, filename, nil
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
