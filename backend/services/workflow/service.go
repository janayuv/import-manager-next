package workflow

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"sync"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service orchestrates ERP exception workflows, automation rule promotion/canary,
// background job telemetry, incident correlation/suppression/forecasts, error memory, and diagnostics.
type Service struct {
	db          *sql.DB
	security    *security.Service
	audit       *audit.Service
	restoreCtrl *connection.RestoreControl
	exportsDir  string
	logsDir     string
	appVersion  string

	runningMu   sync.Mutex
	runningJobs map[string]bool
}

// NewService constructs a Workflow & Observability Service instance.
func NewService(
	db *sql.DB,
	sec *security.Service,
	aud *audit.Service,
	restoreCtrl *connection.RestoreControl,
	exportsDir string,
	logsDir string,
	appVersion string,
) *Service {
	return &Service{
		db:          db,
		security:    sec,
		audit:       aud,
		restoreCtrl: restoreCtrl,
		exportsDir:  exportsDir,
		logsDir:     logsDir,
		appVersion:  appVersion,
		runningJobs: make(map[string]bool),
	}
}

func (s *Service) acquireJobLock(jobID string) (func(), bool) {
	s.runningMu.Lock()
	if s.runningJobs == nil {
		s.runningJobs = make(map[string]bool)
	}
	if s.runningJobs[jobID] {
		s.runningMu.Unlock()
		return nil, false
	}
	s.runningJobs[jobID] = true
	s.runningMu.Unlock()

	return func() {
		s.runningMu.Lock()
		delete(s.runningJobs, jobID)
		s.runningMu.Unlock()
	}, true
}

// SetDB rebinds the service to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
