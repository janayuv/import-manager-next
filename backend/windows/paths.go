package windows

import (
	"fmt"
	"os"
	"path/filepath"
)

const AppDirName = "ImportManager"

// AppPaths holds the Windows application data directories used at runtime.
// The database is never stored inside the application's installation directory.
type AppPaths struct {
	DataDir        string `json:"dataDir"`
	DatabasePath   string `json:"databasePath"`
	BackupsDir     string `json:"backupsDir"`
	AttachmentsDir string `json:"attachmentsDir"`
	LogsDir        string `json:"logsDir"`
	ExportsDir     string `json:"exportsDir"`
	TempDir        string `json:"tempDir"`
}

// ResolveAppPaths resolves and creates the Windows AppData directory layout.
// When overrideBaseDir is non-empty (e.g. during tests), directories are placed under it.
func ResolveAppPaths(overrideBaseDir string) (AppPaths, error) {
	base := overrideBaseDir
	if base == "" {
		if envOverride := os.Getenv("IMPORT_MANAGER_DATA_DIR"); envOverride != "" {
			base = envOverride
		} else {
			userConfig, err := os.UserConfigDir()
			if err != nil {
				return AppPaths{}, fmt.Errorf("resolve user config dir: %w", err)
			}
			base = filepath.Join(userConfig, AppDirName)
		}
	}

	paths := AppPaths{
		DataDir:        base,
		DatabasePath:   filepath.Join(base, "import_manager.db"),
		BackupsDir:     filepath.Join(base, "backups"),
		AttachmentsDir: filepath.Join(base, "attachments"),
		LogsDir:        filepath.Join(base, "logs"),
		ExportsDir:     filepath.Join(base, "exports"),
		TempDir:        filepath.Join(base, "tmp"),
	}

	dirs := []string{
		paths.DataDir,
		paths.BackupsDir,
		paths.AttachmentsDir,
		paths.LogsDir,
		paths.ExportsDir,
		paths.TempDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return AppPaths{}, fmt.Errorf("create app directory %s: %w", dir, err)
		}
	}

	return paths, nil
}
