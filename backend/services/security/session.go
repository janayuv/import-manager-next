package security

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultSessionTTL        = 12 * time.Hour
	RememberMeSessionTTL     = 30 * 24 * time.Hour
	DefaultIdleTimeoutMins   = 30
)

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrForbidden       = errors.New("insufficient permissions")
)

// SessionInfo represents the active desktop user session and resolved RBAC permissions.
type SessionInfo struct {
	SessionID   string   `json:"sessionId"`
	UserID      string   `json:"userId"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	RememberMe  bool     `json:"rememberMe"`
	CreatedAt   string   `json:"createdAt"`
	LastActive  string   `json:"lastActive"`
	ExpiresAt   string   `json:"expiresAt"`
}

type internalSession struct {
	info       SessionInfo
	created    time.Time
	lastActive time.Time
	expires    time.Time
}

// SessionStore manages active desktop sessions in-process with idle timeout & TTL enforcement.
type SessionStore struct {
	mu       sync.RWMutex
	activeID string
	sessions map[string]*internalSession
	now      func() time.Time
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*internalSession),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (s *SessionStore) Create(userID, username string, role Role, rememberMe bool) SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	ttl := DefaultSessionTTL
	if rememberMe {
		ttl = RememberMeSessionTTL
	}
	id := uuid.NewString()
	info := SessionInfo{
		SessionID:   id,
		UserID:      userID,
		Username:    username,
		Role:        string(role),
		Permissions: PermissionsForRole(role),
		RememberMe:  rememberMe,
		CreatedAt:   now.Format(time.RFC3339),
		LastActive:  now.Format(time.RFC3339),
		ExpiresAt:   now.Add(ttl).Format(time.RFC3339),
	}
	s.sessions[id] = &internalSession{
		info:       info,
		created:    now,
		lastActive: now,
		expires:    now.Add(ttl),
	}
	s.activeID = id
	return info
}

func (s *SessionStore) Current(idleTimeoutMins int) (SessionInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeID == "" {
		return SessionInfo{}, ErrUnauthenticated
	}
	sess, ok := s.sessions[s.activeID]
	if !ok {
		s.activeID = ""
		return SessionInfo{}, ErrUnauthenticated
	}

	now := s.now()
	if now.After(sess.expires) {
		delete(s.sessions, s.activeID)
		s.activeID = ""
		return SessionInfo{}, fmt.Errorf("%w: session expired", ErrUnauthenticated)
	}
	if idleTimeoutMins > 0 && now.Sub(sess.lastActive) > time.Duration(idleTimeoutMins)*time.Minute {
		delete(s.sessions, s.activeID)
		s.activeID = ""
		return SessionInfo{}, fmt.Errorf("%w: idle timeout exceeded", ErrUnauthenticated)
	}

	sess.lastActive = now
	sess.info.LastActive = now.Format(time.RFC3339)
	return sess.info, nil
}

func (s *SessionStore) ClearActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeID != "" {
		delete(s.sessions, s.activeID)
		s.activeID = ""
	}
}

func (s *SessionStore) Terminate(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sessionID]; !ok {
		return false
	}
	delete(s.sessions, sessionID)
	if s.activeID == sessionID {
		s.activeID = ""
	}
	return true
}

func (s *SessionStore) ListActive(idleTimeoutMins int) []SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	var out []SessionInfo
	for id, sess := range s.sessions {
		expired := now.After(sess.expires) || (idleTimeoutMins > 0 && now.Sub(sess.lastActive) > time.Duration(idleTimeoutMins)*time.Minute)
		if expired {
			delete(s.sessions, id)
			if s.activeID == id {
				s.activeID = ""
			}
			continue
		}
		out = append(out, sess.info)
	}
	return out
}

// RequirePermission enforces backend authentication and permission verification against the active session.
func (s *SessionStore) RequirePermission(perm string, idleTimeoutMins int) (SessionInfo, error) {
	sess, err := s.Current(idleTimeoutMins)
	if err != nil {
		return SessionInfo{}, err
	}
	role, ok := ParseRole(sess.Role)
	if !ok || !RoleHasPermission(role, perm) {
		return SessionInfo{}, fmt.Errorf("%w: missing permission %s for role %s", ErrForbidden, perm, sess.Role)
	}
	return sess, nil
}
