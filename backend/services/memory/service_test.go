package memory_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/memory"
	memService "github.com/janayuv/import-manager-next/backend/services/memory"
)

func TestMemoryServiceDirect(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "svc_mem_test.db")
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	dbMgr, err := connection.OpenEncrypted(dbPath, key)
	if err != nil {
		t.Fatalf("OpenEncrypted failed: %v", err)
	}
	defer dbMgr.Close()

	cfg := memory.Config{
		MemoryLimitBytes:  256 * 1024 * 1024,
		AutoTrimEnabled:   false,
		IdleTrimThreshold: 1 * time.Minute,
		MinTrimInterval:   10 * time.Millisecond,
	}

	mgr := memory.NewManager(dbMgr, nil, cfg)
	defer mgr.Close()

	svc := memService.NewService(mgr, nil, nil)

	stats, err := svc.GetMemoryStats()
	if err != nil {
		t.Fatalf("GetMemoryStats failed: %v", err)
	}
	if stats.MemoryLimitBytes != 256*1024*1024 {
		t.Fatalf("unexpected limit: %d", stats.MemoryLimitBytes)
	}

	res, err := svc.OptimizeMemory(true)
	if err != nil {
		t.Fatalf("OptimizeMemory failed: %v", err)
	}
	if !res.Aggressive {
		t.Fatalf("expected aggressive trim")
	}

	if err := svc.SetMemoryLimitMB(512); err != nil {
		t.Fatalf("SetMemoryLimitMB failed: %v", err)
	}
	if err := svc.SetAutoTrim(true); err != nil {
		t.Fatalf("SetAutoTrim failed: %v", err)
	}
}
