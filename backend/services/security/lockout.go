package security

import (
	"database/sql"
	"fmt"
	"time"
)

// LockoutStatus describes current failed-attempt lockout state for an account.
type LockoutStatus struct {
	UserID          string `json:"userId"`
	IsLocked        bool   `json:"isLocked"`
	FailureCount    int    `json:"failureCount"`
	Threshold       int    `json:"threshold"`
	WindowStartedAt string `json:"windowStartedAt,omitempty"`
	LockedUntil     string `json:"lockedUntil,omitempty"`
	RemainingSecs   int64  `json:"remainingSecs"`
}

type LockoutManager struct {
	db  *sql.DB
	now func() time.Time
}

func NewLockoutManager(db *sql.DB) *LockoutManager {
	return &LockoutManager{db: db, now: func() time.Time { return time.Now().UTC() }}
}

// GetStatus returns whether userId is currently locked out based on active threshold settings.
func (m *LockoutManager) GetStatus(userID string, threshold int) (LockoutStatus, error) {
	status := LockoutStatus{
		UserID:    userID,
		Threshold: threshold,
	}
	var count int
	var windowStartStr string
	var lockedUntil sql.NullString

	err := m.db.QueryRow(`
		SELECT failure_count, window_started_at, locked_until
		FROM auth_lockout_state
		WHERE user_id = ?`, userID,
	).Scan(&count, &windowStartStr, &lockedUntil)
	if err == sql.ErrNoRows {
		return status, nil
	}
	if err != nil {
		return status, fmt.Errorf("query auth_lockout_state: %w", err)
	}

	status.FailureCount = count
	status.WindowStartedAt = windowStartStr
	if lockedUntil.Valid && lockedUntil.String != "" {
		if luTime, parseErr := time.Parse(time.RFC3339, lockedUntil.String); parseErr == nil {
			now := m.now()
			if now.Before(luTime) {
				status.IsLocked = true
				status.LockedUntil = lockedUntil.String
				status.RemainingSecs = int64(luTime.Sub(now).Seconds())
				if status.RemainingSecs < 1 {
					status.RemainingSecs = 1
				}
			}
		}
	}
	return status, nil
}

// RecordFailure logs a failed attempt and activates lockout when failureCount >= threshold inside windowMins.
func (m *LockoutManager) RecordFailure(userID, reason, source string, threshold, windowMins, durationMins int) (LockoutStatus, error) {
	now := m.now()
	nowStr := now.Format(time.RFC3339)

	if _, err := m.db.Exec(`
		INSERT INTO auth_failed_attempts (user_id, attempted_at, reason, source)
		VALUES (?, ?, ?, ?)`,
		userID, nowStr, reason, source,
	); err != nil {
		return LockoutStatus{}, fmt.Errorf("insert auth_failed_attempts: %w", err)
	}

	var count int
	var windowStartStr string
	err := m.db.QueryRow(`
		SELECT failure_count, window_started_at
		FROM auth_lockout_state
		WHERE user_id = ?`, userID,
	).Scan(&count, &windowStartStr)

	resetWindow := err == sql.ErrNoRows
	if err == nil {
		if ws, parseErr := time.Parse(time.RFC3339, windowStartStr); parseErr == nil {
			if now.Sub(ws) > time.Duration(windowMins)*time.Minute {
				resetWindow = true
			}
		} else {
			resetWindow = true
		}
	} else if err != sql.ErrNoRows {
		return LockoutStatus{}, fmt.Errorf("read auth_lockout_state: %w", err)
	}

	if resetWindow {
		count = 1
		windowStartStr = nowStr
	} else {
		count++
	}

	var lockedUntilVal any = nil
	if count >= threshold {
		lockedUntilVal = now.Add(time.Duration(durationMins) * time.Minute).Format(time.RFC3339)
	}

	if _, err := m.db.Exec(`
		INSERT INTO auth_lockout_state (user_id, failure_count, window_started_at, locked_until, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			failure_count = excluded.failure_count,
			window_started_at = excluded.window_started_at,
			locked_until = excluded.locked_until,
			updated_at = excluded.updated_at`,
		userID, count, windowStartStr, lockedUntilVal, nowStr,
	); err != nil {
		return LockoutStatus{}, fmt.Errorf("upsert auth_lockout_state: %w", err)
	}

	return m.GetStatus(userID, threshold)
}

// Clear resets lockout state and sliding window failure counters for userID.
func (m *LockoutManager) Clear(userID string) error {
	if _, err := m.db.Exec(`DELETE FROM auth_lockout_state WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("clear auth_lockout_state: %w", err)
	}
	return nil
}
