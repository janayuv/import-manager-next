package config_test

import (
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/app/config"
)

func TestLoadConfigRecoveryFlags(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "cfg")
	cfg, err := config.Load(nil, tmp)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.RecoveryMode {
		t.Fatalf("expected RecoveryMode=false by default")
	}

	cfgFlag, err := config.Load([]string{"--recovery"}, tmp)
	if err != nil {
		t.Fatalf("config.Load(--recovery) failed: %v", err)
	}
	if !cfgFlag.RecoveryMode {
		t.Fatalf("expected RecoveryMode=true with --recovery flag")
	}

	cfgModeFlag, err := config.Load([]string{"--recovery-mode"}, tmp)
	if err != nil {
		t.Fatalf("config.Load(--recovery-mode) failed: %v", err)
	}
	if !cfgModeFlag.RecoveryMode {
		t.Fatalf("expected RecoveryMode=true with --recovery-mode flag")
	}

	t.Setenv("IMPORT_MANAGER_RECOVERY", "1")
	cfgEnv, err := config.Load(nil, tmp)
	if err != nil {
		t.Fatalf("config.Load(env) failed: %v", err)
	}
	if !cfgEnv.RecoveryMode {
		t.Fatalf("expected RecoveryMode=true with IMPORT_MANAGER_RECOVERY=1")
	}
}
