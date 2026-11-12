package workflow

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"

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
	}
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
