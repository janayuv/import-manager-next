package security

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	appcrypto "github.com/janayuv/import-manager-next/backend/crypto"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/services/audit"
)

const (
	settingAdminUsername     = "auth.admin_username"
	settingAdminPasswordHash = "auth.admin_password_hash"
	settingIdleTimeoutMins   = "auth.idle_timeout_minutes"
	settingLockoutThreshold  = "auth.lockout_threshold"
	settingLockoutWindowMins = "auth.lockout_window_minutes"
	settingLockoutDurMins    = "auth.lockout_duration_minutes"
	settingPasswordHistDepth = "auth.password_history_depth"
	settingPolicyVersion     = "security.policy_version"

	MinPasswordHistoryDepth = 1
	MaxPasswordHistoryDepth = 24
)

// AuthSetupStatus indicates whether the initial desktop administrator account has been configured.
type AuthSetupStatus struct {
	NeedsSetup    bool   `json:"needsSetup"`
	AdminUsername string `json:"adminUsername,omitempty"`
	RecoveryMode  bool   `json:"recoveryMode"`
}

// SecuritySettings mirrors configurable authentication, lockout, and password hygiene policy.
type SecuritySettings struct {
	IdleTimeoutMinutes     int `json:"idleTimeoutMinutes"`
	LockoutThreshold       int `json:"lockoutThreshold"`
	LockoutWindowMinutes   int `json:"lockoutWindowMinutes"`
	LockoutDurationMinutes int `json:"lockoutDurationMinutes"`
	PasswordHistoryDepth   int `json:"passwordHistoryDepth"`
	PolicyVersion          int `json:"policyVersion"`
}

// UserRoleRow mirrors a row in user_roles.
type UserRoleRow struct {
	ID          int64  `json:"id"`
	UserID      string `json:"userId"`
	Role        string `json:"role"`
	Permissions string `json:"permissions,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	CreatedBy   string `json:"createdBy,omitempty"`
	UpdatedBy   string `json:"updatedBy,omitempty"`
}

// SecurityDashboardMetrics summarizes current security posture for Security Center.
type SecurityDashboardMetrics struct {
	ActiveSessions      int `json:"activeSessions"`
	LockedAccounts      int `json:"lockedAccounts"`
	FailedLogins24h     int `json:"failedLogins24h"`
	TotalUsersWithRoles int `json:"totalUsersWithRoles"`
	PolicyVersion       int `json:"policyVersion"`
}

// SecurityTrendPoint represents a daily authentication event count bucket.
type SecurityTrendPoint struct {
	Date           string `json:"date"`
	FailedAttempts int    `json:"failedAttempts"`
	AuthEvents     int    `json:"authEvents"`
}

// Service implements authentication, desktop sessions, RBAC, password policy, lockout, and emergency recovery.
type Service struct {
	db           *sql.DB
	recoveryMode bool
	sessions     *SessionStore
	lockout      *LockoutManager
	audit        *audit.Service
}

// NewService constructs a SecurityService backed by SQLCipher DB, session store, lockout manager, and audit service.
func NewService(db *sql.DB, recoveryMode bool, auditSvc *audit.Service) *Service {
	return &Service{
		db:           db,
		recoveryMode: recoveryMode,
		sessions:     NewSessionStore(),
		lockout:      NewLockoutManager(db),
		audit:        auditSvc,
	}
}

// SetDB rebinds the service and its lockout manager to an active database handle without dropping active user sessions.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
	s.lockout = NewLockoutManager(db)
}

// RequirePermission validates an active desktop session and checks canonical permission authorization.
// Backend services must always use this helper rather than trusting client-supplied role parameters.
func (s *Service) RequirePermission(perm string) (SessionInfo, error) {
	settings, err := s.loadSettingsUnlocked()
	idle := DefaultIdleTimeoutMins
	if err == nil && settings.IdleTimeoutMinutes > 0 {
		idle = settings.IdleTimeoutMinutes
	}
	return s.sessions.RequirePermission(perm, idle)
}

// RequireAnyPermission enforces that the active desktop session holds at least one of the listed permissions.
func (s *Service) RequireAnyPermission(perms ...string) (SessionInfo, error) {
	var lastErr error
	for _, p := range perms {
		sess, err := s.RequirePermission(p)
		if err == nil {
			return sess, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return SessionInfo{}, lastErr
	}
	return SessionInfo{}, errors.New("permission denied")
}

// GetAuthSetupStatus reports whether an initial administrator password is configured.
func (s *Service) GetAuthSetupStatus() (AuthSetupStatus, error) {
	username := s.readSetting(settingAdminUsername, "")
	hash := s.readSetting(settingAdminPasswordHash, "")
	return AuthSetupStatus{
		NeedsSetup:    hash == "",
		AdminUsername: username,
		RecoveryMode:  s.recoveryMode,
	}, nil
}

// CompleteAdminSetup creates the initial administrator account if not yet configured and signs them in.
func (s *Service) CompleteAdminSetup(username, password string) (SessionInfo, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return SessionInfo{}, errors.New("username is required")
	}
	if existingHash := s.readSetting(settingAdminPasswordHash, ""); existingHash != "" {
		return SessionInfo{}, errors.New("administrator account is already configured")
	}
	if err := appcrypto.ValidatePasswordComplexity(password); err != nil {
		return SessionInfo{}, err
	}
	hash, err := appcrypto.HashPassword(password)
	if err != nil {
		return SessionInfo{}, err
	}

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if err := upsertSettingTx(tx, settingAdminUsername, username); err != nil {
			return err
		}
		if err := upsertSettingTx(tx, settingAdminPasswordHash, hash); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT INTO auth_password_history (user_id, password_hash, algorithm)
			VALUES (?, ?, 'argon2id')`, username, hash,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT INTO user_roles (user_id, role, created_by, updated_by)
			VALUES (?, 'administrator', 'system', 'system')
			ON CONFLICT(user_id) DO UPDATE SET role = 'administrator', updated_by = 'system', updated_at = CURRENT_TIMESTAMP`,
			username,
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return SessionInfo{}, fmt.Errorf("persist admin setup: %w", err)
	}

	sess := s.sessions.Create(username, username, RoleAdministrator, false)
	_ = s.audit.RecordUserActivity(username, "COMPLETE_ADMIN_SETUP", "user", username, "SUCCESS", "SECURITY", map[string]any{"role": "administrator"})
	return sess, nil
}

// Authenticate verifies username + password using Argon2id (or legacy bcrypt with transparent upgrade),
// checks and updates sliding-window lockout state, and creates a desktop session.
func (s *Service) Authenticate(username, password string, rememberMe bool) (SessionInfo, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return SessionInfo{}, errors.New("username and password are required")
	}

	settings, err := s.loadSettingsUnlocked()
	if err != nil {
		return SessionInfo{}, err
	}

	adminUser := s.readSetting(settingAdminUsername, "")
	adminHash := s.readSetting(settingAdminPasswordHash, "")
	if adminHash == "" {
		return SessionInfo{}, errors.New("administrator account is not configured; complete setup first")
	}

	// Single-user desktop identity check (case-insensitive username match against configured admin).
	targetUser := adminUser
	if targetUser == "" {
		targetUser = username
	}

	lockStatus, err := s.lockout.GetStatus(targetUser, settings.LockoutThreshold)
	if err != nil {
		return SessionInfo{}, err
	}
	if lockStatus.IsLocked {
		_ = s.audit.RecordUserActivity(username, "LOGIN_BLOCKED_LOCKED", "session", targetUser, "DENIED", "WARNING", map[string]any{
			"lockedUntil": lockStatus.LockedUntil,
		})
		return SessionInfo{}, fmt.Errorf("account is locked until %s (%ds remaining)", lockStatus.LockedUntil, lockStatus.RemainingSecs)
	}

	userMatches := strings.EqualFold(username, targetUser)
	passMatches := false
	needsRehash := false
	if userMatches {
		passMatches, needsRehash, err = appcrypto.VerifyPassword(password, adminHash)
		if err != nil {
			return SessionInfo{}, err
		}
	}

	if !userMatches || !passMatches {
		reason := "invalid_password"
		if !userMatches {
			reason = "unknown_username"
		}
		afterLock, recErr := s.lockout.RecordFailure(
			targetUser,
			reason,
			"desktop_login",
			settings.LockoutThreshold,
			settings.LockoutWindowMinutes,
			settings.LockoutDurationMinutes,
		)
		if recErr != nil {
			return SessionInfo{}, recErr
		}
		_ = s.audit.RecordUserActivity(username, "LOGIN_FAILED", "session", targetUser, "FAILURE", "WARNING", map[string]any{
			"reason":       reason,
			"failureCount": afterLock.FailureCount,
			"isLocked":     afterLock.IsLocked,
		})
		if afterLock.IsLocked {
			return SessionInfo{}, fmt.Errorf("invalid credentials; account locked until %s", afterLock.LockedUntil)
		}
		rem := settings.LockoutThreshold - afterLock.FailureCount
		if rem < 0 {
			rem = 0
		}
		return SessionInfo{}, fmt.Errorf("invalid username or password (%d attempts remaining before lockout)", rem)
	}

	// Successful authentication: clear lockout counters and transparently rehash legacy bcrypt -> Argon2id.
	_ = s.lockout.Clear(targetUser)
	if needsRehash {
		if newHash, hErr := appcrypto.HashPassword(password); hErr == nil {
			_ = upsertSettingDB(s.db, settingAdminPasswordHash, newHash)
		}
	}

	role := s.resolveUserRole(targetUser)
	sess := s.sessions.Create(targetUser, targetUser, role, rememberMe)
	_ = s.audit.RecordUserActivity(targetUser, "LOGIN_SUCCESS", "session", sess.SessionID, "SUCCESS", "INFO", map[string]any{
		"role":       string(role),
		"rememberMe": rememberMe,
	})
	return sess, nil
}

// GetMySession returns the active authenticated desktop session or ErrUnauthenticated.
func (s *Service) GetMySession() (SessionInfo, error) {
	settings, _ := s.loadSettingsUnlocked()
	idle := settings.IdleTimeoutMinutes
	if idle <= 0 {
		idle = DefaultIdleTimeoutMins
	}
	return s.sessions.Current(idle)
}

// GetMyPermissions returns the current session's canonical RBAC permission list.
func (s *Service) GetMyPermissions() ([]string, error) {
	sess, err := s.GetMySession()
	if err != nil {
		return nil, err
	}
	return sess.Permissions, nil
}

// Logout terminates the active desktop session cleanly.
func (s *Service) Logout() error {
	if sess, err := s.GetMySession(); err == nil {
		_ = s.audit.RecordUserActivity(sess.UserID, "LOGOUT", "session", sess.SessionID, "SUCCESS", "INFO", nil)
	}
	s.sessions.ClearActive()
	return nil
}

// ChangePassword updates the admin password after validating complexity, current password, and password history.
func (s *Service) ChangePassword(currentPassword, newPassword string) error {
	sess, err := s.RequirePermission(PermSecurityChangePassword)
	if err != nil {
		return err
	}
	if err := appcrypto.ValidatePasswordComplexity(newPassword); err != nil {
		return err
	}

	curHash := s.readSetting(settingAdminPasswordHash, "")
	ok, _, err := appcrypto.VerifyPassword(currentPassword, curHash)
	if err != nil || !ok {
		return errors.New("current password is incorrect")
	}

	settings, err := s.loadSettingsUnlocked()
	if err != nil {
		return err
	}
	if err := s.checkPasswordHistory(sess.UserID, newPassword, settings.PasswordHistoryDepth); err != nil {
		return err
	}

	newHash, err := appcrypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if err := upsertSettingTx(tx, settingAdminPasswordHash, newHash); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT INTO auth_password_history (user_id, password_hash, algorithm)
			VALUES (?, ?, 'argon2id')`, sess.UserID, newHash,
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	_ = s.audit.RecordUserActivity(sess.UserID, "CHANGE_PASSWORD", "user", sess.UserID, "SUCCESS", "SECURITY", nil)
	return nil
}

func (s *Service) checkPasswordHistory(userID, candidate string, depth int) error {
	if depth < MinPasswordHistoryDepth {
		depth = MinPasswordHistoryDepth
	}
	if depth > MaxPasswordHistoryDepth {
		depth = MaxPasswordHistoryDepth
	}
	rows, err := s.db.Query(`
		SELECT password_hash FROM auth_password_history
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT ?`, userID, depth,
	)
	if err != nil {
		return fmt.Errorf("query auth_password_history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var oldHash string
		if scanErr := rows.Scan(&oldHash); scanErr != nil {
			return scanErr
		}
		if match, _, _ := appcrypto.VerifyPassword(candidate, oldHash); match {
			return fmt.Errorf("%w: new password cannot reuse any of your last %d passwords", appcrypto.ErrPasswordPolicy, depth)
		}
	}
	return rows.Err()
}

// GetSecuritySettings loads active security policy thresholds and version.
func (s *Service) GetSecuritySettings() (SecuritySettings, error) {
	if _, err := s.GetMySession(); err != nil {
		return SecuritySettings{}, err
	}
	return s.loadSettingsUnlocked()
}

// UpdateSecuritySettings persists policy settings, increments policy version, and writes security_policy_versions.
func (s *Service) UpdateSecuritySettings(input SecuritySettings) (SecuritySettings, error) {
	sess, err := s.RequirePermission(PermSecurityLockoutManage)
	if err != nil {
		return SecuritySettings{}, err
	}

	idle := clampInt(input.IdleTimeoutMinutes, 1, 1440)
	thresh := clampInt(input.LockoutThreshold, 1, 50)
	win := clampInt(input.LockoutWindowMinutes, 1, 1440)
	dur := clampInt(input.LockoutDurationMinutes, 1, 10080)
	hist := clampInt(input.PasswordHistoryDepth, MinPasswordHistoryDepth, MaxPasswordHistoryDepth)

	cur, err := s.loadSettingsUnlocked()
	if err != nil {
		return SecuritySettings{}, err
	}
	nextVer := cur.PolicyVersion + 1

	updated := SecuritySettings{
		IdleTimeoutMinutes:     idle,
		LockoutThreshold:       thresh,
		LockoutWindowMinutes:   win,
		LockoutDurationMinutes: dur,
		PasswordHistoryDepth:   hist,
		PolicyVersion:          nextVer,
	}

	policyJSON, _ := json.Marshal(updated)
	rulesJSON, _ := json.Marshal(map[string]any{
		"minLength":         appcrypto.MinPasswordLen,
		"requireUpper":      true,
		"requireLower":      true,
		"requireDigit":      true,
		"requireSymbol":     true,
		"disallowSpace":     true,
		"historyDepth":      hist,
		"hashAlgorithm":     "argon2id",
	})

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		pairs := map[string]string{
			settingIdleTimeoutMins:   strconv.Itoa(idle),
			settingLockoutThreshold:  strconv.Itoa(thresh),
			settingLockoutWindowMins: strconv.Itoa(win),
			settingLockoutDurMins:    strconv.Itoa(dur),
			settingPasswordHistDepth: strconv.Itoa(hist),
			settingPolicyVersion:     strconv.Itoa(nextVer),
		}
		for k, v := range pairs {
			if err := upsertSettingTx(tx, k, v); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`
			INSERT INTO security_policy_versions (version, policy_json, password_rules_json, changed_by)
			VALUES (?, ?, ?, ?)`,
			nextVer, string(policyJSON), string(rulesJSON), sess.UserID,
		)
		return err
	})
	if err != nil {
		return SecuritySettings{}, fmt.Errorf("save security policy: %w", err)
	}

	_ = s.audit.RecordUserActivity(sess.UserID, "UPDATE_SECURITY_POLICY", "security_policy", strconv.Itoa(nextVer), "SUCCESS", "SECURITY", updated)
	return updated, nil
}

// GetLockoutStatus inspects lockout status for target user (defaults to admin username if empty).
func (s *Service) GetLockoutStatus(userID string) (LockoutStatus, error) {
	if _, err := s.RequirePermission(PermSecurityLockoutManage); err != nil {
		return LockoutStatus{}, err
	}
	if strings.TrimSpace(userID) == "" {
		userID = s.readSetting(settingAdminUsername, "admin")
	}
	settings, err := s.loadSettingsUnlocked()
	if err != nil {
		return LockoutStatus{}, err
	}
	return s.lockout.GetStatus(userID, settings.LockoutThreshold)
}

// ResetAccountLockout clears failed-attempt lockout state for userID.
func (s *Service) ResetAccountLockout(userID string) error {
	sess, err := s.RequirePermission(PermSecurityLockoutManage)
	if err != nil {
		return err
	}
	if strings.TrimSpace(userID) == "" {
		userID = s.readSetting(settingAdminUsername, "admin")
	}
	if err := s.lockout.Clear(userID); err != nil {
		return err
	}
	_ = s.audit.RecordUserActivity(sess.UserID, "RESET_LOCKOUT", "user", userID, "SUCCESS", "SECURITY", nil)
	return nil
}

// GetActiveSessions lists currently active in-memory sessions.
func (s *Service) GetActiveSessions() ([]SessionInfo, error) {
	if _, err := s.RequirePermission(PermSecuritySessionRead); err != nil {
		return nil, err
	}
	settings, _ := s.loadSettingsUnlocked()
	return s.sessions.ListActive(settings.IdleTimeoutMinutes), nil
}

// TerminateSessionForAdmin revokes a specific active desktop session.
func (s *Service) TerminateSessionForAdmin(sessionID string) error {
	sess, err := s.RequirePermission(PermSecurityLockoutManage)
	if err != nil {
		return err
	}
	if !s.sessions.Terminate(sessionID) {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	_ = s.audit.RecordUserActivity(sess.UserID, "TERMINATE_SESSION", "session", sessionID, "SUCCESS", "SECURITY", nil)
	return nil
}

// GetRolePermissionsMatrix returns the canonical 4-role x 25-permission matrix.
func (s *Service) GetRolePermissionsMatrix() ([]RoleMatrixRow, error) {
	if _, err := s.GetMySession(); err != nil {
		return nil, err
	}
	return GetRoleMatrix(), nil
}

// ListUserRoles lists all records in user_roles.
func (s *Service) ListUserRoles() ([]UserRoleRow, error) {
	if _, err := s.RequirePermission(PermRoleRead); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, role, COALESCE(permissions, ''), COALESCE(created_at, ''),
		       COALESCE(updated_at, ''), COALESCE(created_by, ''), COALESCE(updated_by, '')
		FROM user_roles
		ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query user_roles: %w", err)
	}
	defer rows.Close()

	var out []UserRoleRow
	for rows.Next() {
		var r UserRoleRow
		if scanErr := rows.Scan(&r.ID, &r.UserID, &r.Role, &r.Permissions, &r.CreatedAt, &r.UpdatedAt, &r.CreatedBy, &r.UpdatedBy); scanErr != nil {
			return nil, scanErr
		}
		if canon, ok := ParseRole(r.Role); ok {
			r.Role = string(canon)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AssignUserRole upserts a canonical role assignment in user_roles.
func (s *Service) AssignUserRole(userID, rawRole string) error {
	sess, err := s.RequirePermission(PermRoleWrite)
	if err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("userId is required")
	}
	canon, ok := ParseRole(rawRole)
	if !ok {
		return fmt.Errorf("invalid role %q (allowed: administrator, manager, operator, viewer)", rawRole)
	}
	if _, err := s.db.Exec(`
		INSERT INTO user_roles (user_id, role, created_by, updated_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET role = excluded.role, updated_by = excluded.updated_by, updated_at = CURRENT_TIMESTAMP`,
		userID, string(canon), sess.UserID, sess.UserID,
	); err != nil {
		return fmt.Errorf("upsert user_role: %w", err)
	}
	_ = s.audit.RecordUserActivity(sess.UserID, "ASSIGN_ROLE", "user_role", userID, "SUCCESS", "SECURITY", map[string]any{"role": string(canon)})
	return nil
}

// BootstrapFirstAdminIfEligible grants administrator role if user_roles currently has zero administrator rows.
func (s *Service) BootstrapFirstAdminIfEligible(userID string) (bool, error) {
	if _, err := s.GetMySession(); err != nil {
		return false, err
	}
	var adminCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_roles WHERE LOWER(role) IN ('administrator', 'admin')`).Scan(&adminCount); err != nil {
		return false, err
	}
	if adminCount > 0 {
		return false, nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, errors.New("userId is required")
	}
	if _, err := s.db.Exec(`
		INSERT INTO user_roles (user_id, role, created_by, updated_by)
		VALUES (?, 'administrator', 'bootstrap', 'bootstrap')
		ON CONFLICT(user_id) DO UPDATE SET role = 'administrator', updated_at = CURRENT_TIMESTAMP`, userID,
	); err != nil {
		return false, err
	}
	_ = s.audit.RecordUserActivity(userID, "BOOTSTRAP_FIRST_ADMIN", "user_role", userID, "SUCCESS", "SECURITY", nil)
	return true, nil
}

// GetSecurityDashboardMetrics returns real DB-backed metrics for Security Center.
func (s *Service) GetSecurityDashboardMetrics() (SecurityDashboardMetrics, error) {
	if _, err := s.RequirePermission(PermSecuritySessionRead); err != nil {
		return SecurityDashboardMetrics{}, err
	}
	settings, err := s.loadSettingsUnlocked()
	if err != nil {
		return SecurityDashboardMetrics{}, err
	}
	active := len(s.sessions.ListActive(settings.IdleTimeoutMinutes))

	var locked int
	nowStr := time.Now().UTC().Format(time.RFC3339)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM auth_lockout_state WHERE locked_until IS NOT NULL AND locked_until > ?`, nowStr).Scan(&locked)

	var failed24h int
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM auth_failed_attempts WHERE attempted_at >= ?`, cutoff).Scan(&failed24h)

	var roleCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM user_roles`).Scan(&roleCount)

	return SecurityDashboardMetrics{
		ActiveSessions:      active,
		LockedAccounts:      locked,
		FailedLogins24h:     failed24h,
		TotalUsersWithRoles: roleCount,
		PolicyVersion:       settings.PolicyVersion,
	}, nil
}

// GetSecurityTrendSeries returns daily authentication failures & events for the last N days.
func (s *Service) GetSecurityTrendSeries(days int) ([]SecurityTrendPoint, error) {
	if _, err := s.RequirePermission(PermSecuritySessionRead); err != nil {
		return nil, err
	}
	if days <= 0 || days > 90 {
		days = 14
	}
	rows, err := s.db.Query(`
		SELECT substr(attempted_at, 1, 10) AS d, COUNT(*)
		FROM auth_failed_attempts
		WHERE date(substr(attempted_at, 1, 10)) >= date('now', ?)
		GROUP BY d
		ORDER BY d ASC`, fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	failByDay := make(map[string]int)
	for rows.Next() {
		var d string
		var c int
		if scanErr := rows.Scan(&d, &c); scanErr == nil {
			failByDay[d] = c
		}
	}

	actRows, err := s.db.Query(`
		SELECT substr(timestamp, 1, 10) AS d, COUNT(*)
		FROM user_activity_audit_logs
		WHERE entity_type IN ('session', 'user', 'security_policy')
		  AND date(substr(timestamp, 1, 10)) >= date('now', ?)
		GROUP BY d`, fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return nil, err
	}
	defer actRows.Close()

	actByDay := make(map[string]int)
	for actRows.Next() {
		var d string
		var c int
		if scanErr := actRows.Scan(&d, &c); scanErr == nil {
			actByDay[d] = c
		}
	}

	out := make([]SecurityTrendPoint, 0, days)
	today := time.Now().UTC()
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, SecurityTrendPoint{
			Date:           d,
			FailedAttempts: failByDay[d],
			AuthEvents:     actByDay[d],
		})
	}
	return out, nil
}

// GetRecentAuthEvents returns recent security/auth audit entries.
func (s *Service) GetRecentAuthEvents(limit int) ([]audit.UserActivityEntry, error) {
	if _, err := s.RequirePermission(PermSecuritySessionRead); err != nil {
		return nil, err
	}
	return s.audit.QueryUserActivityLogs("", limit)
}

// --- Emergency Recovery Mode (--recovery / IMPORT_MANAGER_RECOVERY=1) ---

// IsRecoveryModeActive reports whether startup recovery flag is enabled.
func (s *Service) IsRecoveryModeActive() bool {
	return s.recoveryMode
}

// RecoveryClearLockout clears account lockout only when emergency recovery mode is active.
func (s *Service) RecoveryClearLockout(userID string) error {
	if !s.recoveryMode {
		return errors.New("recovery mode is not active")
	}
	if strings.TrimSpace(userID) == "" {
		userID = s.readSetting(settingAdminUsername, "admin")
	}
	if err := s.lockout.Clear(userID); err != nil {
		return err
	}
	_ = s.audit.RecordUserActivity("RECOVERY_MODE", "RECOVERY_CLEAR_LOCKOUT", "user", userID, "SUCCESS", "CRITICAL", nil)
	return nil
}

// RecoveryResetSecurityPolicy resets lockout/idle/history settings to defaults when recovery mode is active.
func (s *Service) RecoveryResetSecurityPolicy() error {
	if !s.recoveryMode {
		return errors.New("recovery mode is not active")
	}
	err := transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		defaults := map[string]string{
			settingIdleTimeoutMins:   "30",
			settingLockoutThreshold:  "5",
			settingLockoutWindowMins: "15",
			settingLockoutDurMins:    "30",
			settingPasswordHistDepth: "5",
		}
		for k, v := range defaults {
			if err := upsertSettingTx(tx, k, v); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	_ = s.audit.RecordUserActivity("RECOVERY_MODE", "RECOVERY_RESET_POLICY", "security_policy", "default", "SUCCESS", "CRITICAL", nil)
	return nil
}

// RecoverySetAdminPassword sets a new Argon2id admin password when launched in emergency recovery mode.
func (s *Service) RecoverySetAdminPassword(newPassword string) error {
	if !s.recoveryMode {
		return errors.New("recovery mode is not active")
	}
	if err := appcrypto.ValidatePasswordComplexity(newPassword); err != nil {
		return err
	}
	hash, err := appcrypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	adminUser := s.readSetting(settingAdminUsername, "admin")
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if err := upsertSettingTx(tx, settingAdminUsername, adminUser); err != nil {
			return err
		}
		if err := upsertSettingTx(tx, settingAdminPasswordHash, hash); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO auth_password_history (user_id, password_hash, algorithm) VALUES (?, ?, 'argon2id')`, adminUser, hash)
		return err
	})
	if err != nil {
		return err
	}
	_ = s.lockout.Clear(adminUser)
	_ = s.audit.RecordUserActivity("RECOVERY_MODE", "RECOVERY_SET_ADMIN_PASSWORD", "user", adminUser, "SUCCESS", "CRITICAL", nil)
	return nil
}

func (s *Service) resolveUserRole(userID string) Role {
	var raw string
	err := s.db.QueryRow(`SELECT role FROM user_roles WHERE LOWER(user_id) = LOWER(?)`, userID).Scan(&raw)
	if err == nil {
		if r, ok := ParseRole(raw); ok {
			return r
		}
	}
	return RoleAdministrator
}

func (s *Service) loadSettingsUnlocked() (SecuritySettings, error) {
	return SecuritySettings{
		IdleTimeoutMinutes:     s.readSettingInt(settingIdleTimeoutMins, 30),
		LockoutThreshold:       s.readSettingInt(settingLockoutThreshold, 5),
		LockoutWindowMinutes:   s.readSettingInt(settingLockoutWindowMins, 15),
		LockoutDurationMinutes: s.readSettingInt(settingLockoutDurMins, 30),
		PasswordHistoryDepth:   s.readSettingInt(settingPasswordHistDepth, 5),
		PolicyVersion:          s.readSettingInt(settingPolicyVersion, 0),
	}, nil
}

func (s *Service) readSetting(key, fallback string) string {
	var val string
	if err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&val); err == nil && val != "" {
		return val
	}
	return fallback
}

func (s *Service) readSettingInt(key string, fallback int) int {
	raw := s.readSetting(key, "")
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return n
}

func upsertSettingDB(db *sql.DB, key, val string) error {
	_, err := db.Exec(`
		INSERT INTO app_settings (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, val,
	)
	return err
}

func upsertSettingTx(tx *sql.Tx, key, val string) error {
	_, err := tx.Exec(`
		INSERT INTO app_settings (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, val,
	)
	return err
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
