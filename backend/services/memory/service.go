package memory

import (
	"github.com/janayuv/import-manager-next/backend/memory"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service provides Wails desktop bindings for memory telemetry, on-demand optimization,
// and background trim policies.
type Service struct {
	manager *memory.Manager
	sec     *security.Service
	audit   *audit.Service
}

// NewService constructs a memory binding service.
func NewService(mgr *memory.Manager, sec *security.Service, aud *audit.Service) *Service {
	return &Service{
		manager: mgr,
		sec:     sec,
		audit:   aud,
	}
}

// Manager returns the underlying memory manager instance.
func (s *Service) Manager() *memory.Manager {
	return s.manager
}

// GetMemoryStats returns real-time operating system working set and Go heap allocation metrics.
func (s *Service) GetMemoryStats() (memory.Stats, error) {
	if s.sec != nil {
		if _, err := s.sec.RequireAnyPermission(security.PermAdminSystemHealth, security.PermAdminSystemTools, security.PermDataRead); err != nil {
			return memory.Stats{}, err
		}
	}
	s.manager.Touch()
	return s.manager.GetStats(), nil
}

// OptimizeMemory performs an immediate memory sweep (SQLite page cache release, Go GC,
// OS memory reclamation, and Windows working set trimming).
func (s *Service) OptimizeMemory(aggressive bool) (memory.TrimResult, error) {
	var userID string
	if s.sec != nil {
		sess, err := s.sec.RequireAnyPermission(security.PermAdminSystemTools, security.PermDataWrite)
		if err != nil {
			return memory.TrimResult{}, err
		}
		userID = sess.UserID
	}

	res, err := s.manager.Trim(aggressive)
	if err != nil {
		return memory.TrimResult{}, err
	}

	if s.audit != nil && userID != "" {
		_ = s.audit.RecordUserActivity(
			userID,
			"memory.optimize",
			"system",
			"memory_manager",
			"SUCCESS",
			"INFO",
			map[string]any{
				"workingSetBefore": res.WorkingSetBefore,
				"workingSetAfter":  res.WorkingSetAfter,
				"heapAllocBefore":  res.HeapAllocBefore,
				"heapAllocAfter":   res.HeapAllocAfter,
				"reclaimedBytes":   res.ReclaimedBytes,
				"durationMs":       res.DurationMs,
				"aggressive":       aggressive,
			},
		)
	}

	return res, nil
}

// SetMemoryLimitMB updates the Go runtime soft memory limit in Megabytes.
func (s *Service) SetMemoryLimitMB(limitMB int64) error {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermAdminSystemTools); err != nil {
			return err
		}
	}
	if limitMB < 64 {
		limitMB = 64
	}
	s.manager.SetMemoryLimit(limitMB * 1024 * 1024)
	return nil
}

// SetAutoTrim configures whether background idle memory trimming is enabled.
func (s *Service) SetAutoTrim(enabled bool) error {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermAdminSystemTools); err != nil {
			return err
		}
	}
	s.manager.SetAutoTrim(enabled)
	return nil
}
