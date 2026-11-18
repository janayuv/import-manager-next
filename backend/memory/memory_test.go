package memory_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/memory"
)

func TestMemoryManagerLifecycleAndTrim(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mem_test.db")
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	dbMgr, err := connection.OpenEncrypted(dbPath, key)
	if err != nil {
		t.Fatalf("OpenEncrypted failed: %v", err)
	}
	defer dbMgr.Close()

	cfg := memory.Config{
		MemoryLimitBytes:  256 * 1024 * 1024,
		AutoTrimEnabled:   false,
		IdleTrimThreshold: 100 * time.Millisecond,
		MinTrimInterval:   10 * time.Millisecond,
	}

	mgr := memory.NewManager(dbMgr, nil, cfg)
	defer mgr.Close()

	stats := mgr.GetStats()
	if stats.MemoryLimitBytes != 256*1024*1024 {
		t.Fatalf("expected MemoryLimitBytes=256MB, got %d", stats.MemoryLimitBytes)
	}

	// Allocate temporary memory to simulate workload
	sink := make([][]byte, 20)
	for i := range sink {
		sink[i] = make([]byte, 1024*1024) // 20 MB total
	}
	sink = nil
	_ = sink

	// Test light trim
	resLight, err := mgr.Trim(false)
	if err != nil {
		t.Fatalf("Trim(false) failed: %v", err)
	}
	if resLight.Aggressive {
		t.Fatalf("expected light trim, got aggressive")
	}

	time.Sleep(15 * time.Millisecond)

	// Test aggressive trim
	resAggressive, err := mgr.Trim(true)
	if err != nil {
		t.Fatalf("Trim(true) failed: %v", err)
	}
	if !resAggressive.Aggressive {
		t.Fatalf("expected aggressive trim")
	}

	mgr.Touch()
	statsAfter := mgr.GetStats()
	if statsAfter.LastTrimTime.IsZero() {
		t.Fatalf("expected LastTrimTime to be set")
	}

	mgr.SetMemoryLimit(512 * 1024 * 1024)
	if mgr.GetStats().MemoryLimitBytes != 512*1024*1024 {
		t.Fatalf("expected updated memory limit")
	}
}

func TestMemoryManagerAutoTrimWatchdog(t *testing.T) {
	cfg := memory.Config{
		MemoryLimitBytes:  128 * 1024 * 1024,
		AutoTrimEnabled:   true,
		IdleTrimThreshold: 50 * time.Millisecond,
		MinTrimInterval:   10 * time.Millisecond,
	}

	mgr := memory.NewManager(nil, nil, cfg)
	defer mgr.Close()

	mgr.Touch()
	stats := mgr.GetStats()
	if !stats.AutoTrimEnabled {
		t.Fatalf("expected AutoTrimEnabled to be true")
	}

	mgr.SetAutoTrim(false)
	if mgr.GetStats().AutoTrimEnabled {
		t.Fatalf("expected AutoTrimEnabled to be false after toggle")
	}
}
