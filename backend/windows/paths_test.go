package windows_test

import (
	"os"
	"path/filepath"
	"testing"

	winpaths "github.com/janayuv/import-manager-next/backend/windows"
)

func TestResolveAppPathsCreatesRestrictedDirs(t *testing.T) {
	base := filepath.Join(t.TempDir(), "ImportManagerTest")
	paths, err := winpaths.ResolveAppPaths(base)
	if err != nil {
		t.Fatalf("ResolveAppPaths failed: %v", err)
	}

	if paths.DataDir != base {
		t.Fatalf("expected DataDir %s, got %s", base, paths.DataDir)
	}
	expectedDB := filepath.Join(base, "import_manager.db")
	if paths.DatabasePath != expectedDB {
		t.Fatalf("expected DatabasePath %s, got %s", expectedDB, paths.DatabasePath)
	}

	for _, dir := range []string{
		paths.DataDir,
		paths.BackupsDir,
		paths.AttachmentsDir,
		paths.LogsDir,
		paths.ExportsDir,
		paths.TempDir,
	} {
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			t.Fatalf("expected directory %s to exist: %v", dir, err)
		}
	}
}
