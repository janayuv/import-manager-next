package main

import (
	"context"
	"runtime"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	winpaths "github.com/janayuv/import-manager-next/backend/windows"
)

// BootstrapInfo exposes core runtime diagnostics, schema status, and SQLCipher health to the React frontend.
type BootstrapInfo struct {
	AppName       string                `json:"appName"`
	AppVersion    string                `json:"appVersion"`
	GoVersion     string                `json:"goVersion"`
	OS            string                `json:"os"`
	Arch          string                `json:"arch"`
	RecoveryMode  bool                  `json:"recoveryMode"`
	SchemaVersion int                   `json:"schemaVersion"`
	Paths         winpaths.AppPaths     `json:"paths"`
	Database      connection.HealthInfo `json:"database"`
	Integrity     integrity.Report      `json:"integrity"`
}

// AppService provides application metadata, runtime paths, and SQLCipher database health bindings.
type AppService struct {
	rt *bootstrap.Runtime
}

// NewAppService constructs the root Wails-bound service.
func NewAppService(rt *bootstrap.Runtime) *AppService {
	return &AppService{rt: rt}
}

// GetBootstrapInfo returns verified startup configuration, migration version, and SQLCipher health status.
func (s *AppService) GetBootstrapInfo() (BootstrapInfo, error) {
	health, err := s.rt.DB.Health()
	if err != nil {
		return BootstrapInfo{}, err
	}
	integ, err := integrity.Verify(s.rt.DB.DB())
	if err != nil {
		return BootstrapInfo{}, err
	}
	integ.RecoveredWrites = s.rt.Integrity.RecoveredWrites
	return BootstrapInfo{
		AppName:       s.rt.Config.AppName,
		AppVersion:    s.rt.Config.AppVersion,
		GoVersion:     runtime.Version(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		RecoveryMode:  s.rt.Config.RecoveryMode,
		SchemaVersion: s.rt.MigrationResult.CurrentVersion,
		Paths:         s.rt.Config.Paths,
		Database:      health,
		Integrity:     integ,
	}, nil
}

// CheckDatabaseHealth executes a live SQLCipher cipher/WAL/FK/integrity probe.
func (s *AppService) CheckDatabaseHealth() (connection.HealthInfo, error) {
	return s.rt.DB.Health()
}

// ListSchemaMigrations returns applied migration history with SHA-256 checksums and execution timings.
func (s *AppService) ListSchemaMigrations() ([]migrations.AppliedRecord, error) {
	return migrations.ListApplied(context.Background(), s.rt.DB.DB())
}
