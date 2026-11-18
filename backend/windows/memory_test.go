package windows_test

import (
	"runtime"
	"testing"

	win "github.com/janayuv/import-manager-next/backend/windows"
)

func TestOSMemoryStatsAndWorkingSetTrim(t *testing.T) {
	stats, err := win.GetOSMemoryStats()
	if runtime.GOOS == "windows" {
		if err != nil {
			t.Fatalf("GetOSMemoryStats failed on Windows: %v", err)
		}
		if stats.WorkingSetBytes == 0 {
			t.Fatalf("expected non-zero WorkingSetBytes on Windows, got %+v", stats)
		}
	} else {
		if err != nil {
			t.Fatalf("GetOSMemoryStats failed on non-windows: %v", err)
		}
	}

	// Trigger working set trim
	trimErr := win.TrimProcessWorkingSet()
	if trimErr != nil {
		t.Fatalf("TrimProcessWorkingSet failed: %v", trimErr)
	}

	statsAfter, err := win.GetOSMemoryStats()
	if err != nil {
		t.Fatalf("GetOSMemoryStats after trim failed: %v", err)
	}
	t.Logf("Memory before trim: WorkingSet=%d, Peak=%d | Memory after trim: WorkingSet=%d, Peak=%d",
		stats.WorkingSetBytes, stats.PeakWorkingSetBytes,
		statsAfter.WorkingSetBytes, statsAfter.PeakWorkingSetBytes)
}
