package connection

import (
	"errors"
	"sync"
	"time"
)

// ErrRestoreInProgress indicates a global restore barrier is currently active.
var ErrRestoreInProgress = errors.New("database restore is currently in progress; background jobs and mutating writes are temporarily paused")

// RestoreControlStatus reports the current global restore barrier state.
type RestoreControlStatus struct {
	Restoring bool   `json:"restoring"`
	Reason    string `json:"reason"`
	StartedAt string `json:"startedAt,omitempty"`
}

// RestoreControl coordinates a process-wide barrier during backup restore and recovery operations.
type RestoreControl struct {
	mu        sync.RWMutex
	restoring bool
	reason    string
	startedAt time.Time
}

// NewRestoreControl creates an unlocked RestoreControl coordinator.
func NewRestoreControl() *RestoreControl {
	return &RestoreControl{}
}

// SetRestoring activates or releases the global restore lock.
func (rc *RestoreControl) SetRestoring(active bool, reason string) {
	if rc == nil {
		return
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.restoring = active
	if active {
		if reason == "" {
			reason = "Database restore in progress"
		}
		rc.reason = reason
		rc.startedAt = time.Now().UTC()
	} else {
		rc.reason = ""
		rc.startedAt = time.Time{}
	}
}

// IsRestoring returns true while a restore operation holds the barrier.
func (rc *RestoreControl) IsRestoring() bool {
	if rc == nil {
		return false
	}
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.restoring
}

// RequireIdle returns ErrRestoreInProgress when the restore lock is active.
func (rc *RestoreControl) RequireIdle() error {
	if rc.IsRestoring() {
		return ErrRestoreInProgress
	}
	return nil
}

// Status returns a point-in-time snapshot of the restore barrier.
func (rc *RestoreControl) Status() RestoreControlStatus {
	if rc == nil {
		return RestoreControlStatus{}
	}
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	out := RestoreControlStatus{
		Restoring: rc.restoring,
		Reason:    rc.reason,
	}
	if !rc.startedAt.IsZero() {
		out.StartedAt = rc.startedAt.Format(time.RFC3339)
	}
	return out
}
