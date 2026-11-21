package memory

import (
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/logging"
	"github.com/janayuv/import-manager-next/backend/windows"
)

// Default limits and thresholds.
const (
	DefaultMemoryLimitBytes = 512 * 1024 * 1024 // 512 MiB soft memory ceiling for desktop ERP
	DefaultIdleThreshold    = 3 * time.Minute
	DefaultMinTrimInterval  = 30 * time.Second
)

// Config configures memory management behavior.
type Config struct {
	MemoryLimitBytes  int64         `json:"memoryLimitBytes"`
	AutoTrimEnabled   bool          `json:"autoTrimEnabled"`
	IdleTrimThreshold time.Duration `json:"idleTrimThreshold"`
	MinTrimInterval   time.Duration `json:"minTrimInterval"`
}

// Stats provides comprehensive runtime and OS memory diagnostics.
type Stats struct {
	WorkingSetBytes     uint64    `json:"workingSetBytes"`
	PeakWorkingSetBytes uint64    `json:"peakWorkingSetBytes"`
	PrivateUsageBytes   uint64    `json:"privateUsageBytes"`
	HeapAlloc           uint64    `json:"heapAlloc"`
	HeapSys             uint64    `json:"heapSys"`
	HeapIdle            uint64    `json:"heapIdle"`
	HeapInuse           uint64    `json:"heapInuse"`
	HeapReleased        uint64    `json:"heapReleased"`
	StackInuse          uint64    `json:"stackInuse"`
	NumGC               uint32    `json:"numGC"`
	LastGCTime          time.Time `json:"lastGCTime"`
	MemoryLimitBytes    int64     `json:"memoryLimitBytes"`
	LastTrimTime        time.Time `json:"lastTrimTime"`
	LastTrimReclaimed   uint64    `json:"lastTrimReclaimed"`
	AutoTrimEnabled     bool      `json:"autoTrimEnabled"`
	IsIdle              bool      `json:"isIdle"`
}

// TrimResult reports the memory savings after a trim operation.
type TrimResult struct {
	WorkingSetBefore uint64 `json:"workingSetBefore"`
	WorkingSetAfter  uint64 `json:"workingSetAfter"`
	HeapAllocBefore  uint64 `json:"heapAllocBefore"`
	HeapAllocAfter   uint64 `json:"heapAllocAfter"`
	ReclaimedBytes   uint64 `json:"reclaimedBytes"`
	DurationMs       int64  `json:"durationMs"`
	Aggressive       bool   `json:"aggressive"`
}

// Manager orchestrates process, database, and runtime memory optimization.
type Manager struct {
	mu                sync.RWMutex
	db                *connection.Manager
	logger            *logging.Logger
	cfg               Config
	lastActivity      time.Time
	lastTrim          time.Time
	lastTrimReclaimed uint64
	stopCh            chan struct{}
	stopped           bool
}

// NewManager creates an initialized memory manager.
func NewManager(db *connection.Manager, logger *logging.Logger, cfg Config) *Manager {
	if cfg.MemoryLimitBytes <= 0 {
		cfg.MemoryLimitBytes = DefaultMemoryLimitBytes
	}
	if cfg.IdleTrimThreshold <= 0 {
		cfg.IdleTrimThreshold = DefaultIdleThreshold
	}
	if cfg.MinTrimInterval <= 0 {
		cfg.MinTrimInterval = DefaultMinTrimInterval
	}

	// Apply Go runtime soft memory limit
	debug.SetMemoryLimit(cfg.MemoryLimitBytes)

	mgr := &Manager{
		db:           db,
		logger:       logger,
		cfg:          cfg,
		lastActivity: time.Now(),
		stopCh:       make(chan struct{}),
	}

	if cfg.AutoTrimEnabled {
		mgr.startWatchdog()
	}

	return mgr
}

// SetDB rebinds the memory manager to an active database connection manager.
func (m *Manager) SetDB(db *connection.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.db = db
}

// Touch registers user or API activity, resetting the idle timer.
func (m *Manager) Touch() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActivity = time.Now()
}

// GetStats compiles current operating system and Go runtime memory metrics.
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	lastAct := m.lastActivity
	lastTr := m.lastTrim
	lastRec := m.lastTrimReclaimed
	limit := m.cfg.MemoryLimitBytes
	autoTrim := m.cfg.AutoTrimEnabled
	idleThresh := m.cfg.IdleTrimThreshold
	m.mu.RUnlock()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	osStats, _ := windows.GetOSMemoryStats()

	var lastGC time.Time
	if mem.LastGC > 0 {
		lastGC = time.Unix(0, int64(mem.LastGC))
	}

	isIdle := time.Since(lastAct) >= idleThresh

	return Stats{
		WorkingSetBytes:     osStats.WorkingSetBytes,
		PeakWorkingSetBytes: osStats.PeakWorkingSetBytes,
		PrivateUsageBytes:   osStats.PrivateUsageBytes,
		HeapAlloc:           mem.Alloc,
		HeapSys:             mem.HeapSys,
		HeapIdle:            mem.HeapIdle,
		HeapInuse:           mem.HeapInuse,
		HeapReleased:        mem.HeapReleased,
		StackInuse:          mem.StackInuse,
		NumGC:               mem.NumGC,
		LastGCTime:          lastGC,
		MemoryLimitBytes:    limit,
		LastTrimTime:        lastTr,
		LastTrimReclaimed:   lastRec,
		AutoTrimEnabled:     autoTrim,
		IsIdle:              isIdle,
	}
}

// Trim executes database cache shrinking, Go runtime garbage collection,
// OS memory de-allocation, and Windows process working set trimming.
func (m *Manager) Trim(aggressive bool) (TrimResult, error) {
	m.mu.Lock()
	now := time.Now()
	// Rate-limit consecutive auto-trims unless forced
	if !aggressive && !m.lastTrim.IsZero() && now.Sub(m.lastTrim) < m.cfg.MinTrimInterval {
		m.mu.Unlock()
		return TrimResult{}, nil
	}
	m.lastTrim = now
	m.mu.Unlock()

	startTime := time.Now()

	// Initial measurements
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)
	initialOS, _ := windows.GetOSMemoryStats()

	// 1. Release SQLite in-memory page cache & checkpoint WAL
	if m.db != nil {
		_ = m.db.ShrinkMemory()
		if aggressive {
			_ = m.db.CheckpointWAL()
		}
	}

	// 2. Invoke Go runtime garbage collector
	runtime.GC()

	// 3. Return unmapped physical pages to the OS
	debug.FreeOSMemory()

	// 4. On Windows, empty or trim the process working set
	if aggressive {
		_ = windows.TrimProcessWorkingSet()
	}

	// Post-trim measurements
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalOS, _ := windows.GetOSMemoryStats()

	var reclaimed uint64
	if initialOS.WorkingSetBytes > finalOS.WorkingSetBytes {
		reclaimed = initialOS.WorkingSetBytes - finalOS.WorkingSetBytes
	} else if initialMem.Alloc > finalMem.Alloc {
		reclaimed = initialMem.Alloc - finalMem.Alloc
	}

	m.mu.Lock()
	m.lastTrimReclaimed = reclaimed
	m.mu.Unlock()

	duration := time.Since(startTime).Milliseconds()

	if m.logger != nil {
		m.logger.Info("memory trim executed",
			"aggressive", aggressive,
			"durationMs", duration,
			"wsBefore", initialOS.WorkingSetBytes,
			"wsAfter", finalOS.WorkingSetBytes,
			"heapBefore", initialMem.Alloc,
			"heapAfter", finalMem.Alloc,
			"reclaimed", reclaimed,
		)
	}

	return TrimResult{
		WorkingSetBefore: initialOS.WorkingSetBytes,
		WorkingSetAfter:  finalOS.WorkingSetBytes,
		HeapAllocBefore:  initialMem.Alloc,
		HeapAllocAfter:   finalMem.Alloc,
		ReclaimedBytes:   reclaimed,
		DurationMs:       duration,
		Aggressive:       aggressive,
	}, nil
}

// SetMemoryLimit adjusts the Go runtime soft memory limit dynamically.
func (m *Manager) SetMemoryLimit(limitBytes int64) {
	if limitBytes <= 0 {
		limitBytes = DefaultMemoryLimitBytes
	}
	m.mu.Lock()
	m.cfg.MemoryLimitBytes = limitBytes
	m.mu.Unlock()
	debug.SetMemoryLimit(limitBytes)
}

// SetAutoTrim enables or disables automatic idle background memory optimization.
func (m *Manager) SetAutoTrim(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg.AutoTrimEnabled == enabled {
		return
	}
	m.cfg.AutoTrimEnabled = enabled
	if enabled && !m.stopped {
		m.startWatchdog()
	}
}

// startWatchdog runs a background monitor that triggers memory trimming during user inactivity.
func (m *Manager) startWatchdog() {
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.mu.RLock()
				enabled := m.cfg.AutoTrimEnabled
				idleDuration := time.Since(m.lastActivity)
				idleThreshold := m.cfg.IdleTrimThreshold
				lastTrim := m.lastTrim
				m.mu.RUnlock()

				if enabled && idleDuration >= idleThreshold {
					// Only trim if we haven't trimmed since going idle
					if lastTrim.Before(m.lastActivity) || time.Since(lastTrim) >= 5*time.Minute {
						_, _ = m.Trim(true)
					}
				}
			}
		}
	}()
}

// Close gracefully stops the memory watchdog routine.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.stopped {
		m.stopped = true
		close(m.stopCh)
	}
	return nil
}
