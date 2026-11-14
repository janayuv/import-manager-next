package config

import (
	"os"
	"slices"

	winpaths "github.com/janayuv/import-manager-next/backend/windows"
)

const (
	AppName    = "Import Manager"
	AppVersion = "1.0.2"
)

// Config holds immutable startup configuration derived from CLI args, environment, and Windows AppData paths.
type Config struct {
	AppName      string
	AppVersion   string
	RecoveryMode bool
	Paths        winpaths.AppPaths
}

// Load resolves application paths and startup flags without mutating state.
func Load(args []string, dataDirOverride string) (Config, error) {
	paths, err := winpaths.ResolveAppPaths(dataDirOverride)
	if err != nil {
		return Config{}, err
	}

	recovery := slices.Contains(args, "--recovery") || os.Getenv("IMPORT_MANAGER_RECOVERY") == "1"

	return Config{
		AppName:      AppName,
		AppVersion:   AppVersion,
		RecoveryMode: recovery,
		Paths:        paths,
	}, nil
}
