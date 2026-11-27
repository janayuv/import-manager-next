package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/workflow"
)

// BackupRunner abstracts scheduled backup execution and retention pruning.
type BackupRunner interface {
	CheckAndRunDueSchedules(ctx context.Context) ([]models.BackupRecord, error)
}

// Scheduler manages periodic background operations (daily exception/incident ticks & scheduled backups) cleanly under RestoreControl.
type Scheduler struct {
	workflowSvc  *workflow.Service
	backupRunner BackupRunner
	restoreCtrl  *connection.RestoreControl
	interval     time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// New constructs a Scheduler with a default 5-minute tick interval when interval <= 0.
func New(workflowSvc *workflow.Service, backupRunner BackupRunner, restoreCtrl *connection.RestoreControl, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Scheduler{
		workflowSvc:  workflowSvc,
		backupRunner: backupRunner,
		restoreCtrl:  restoreCtrl,
		interval:     interval,
	}
}

// Start launches the scheduler goroutine idempotently and catches up on missed schedules immediately.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true

	// Immediate startup catch-up execution (e.g. computer was switched off)
	go s.RunOnce()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RunOnce()
			}
		}
	}()
}

// RunOnce executes one scheduler cycle if RestoreControl is not locked.
func (s *Scheduler) RunOnce() {
	if s.restoreCtrl != nil && s.restoreCtrl.IsRestoring() {
		return
	}
	if s.workflowSvc != nil {
		_ = s.workflowSvc.RunDailyOperationsTick("scheduler")
	}
	if s.backupRunner != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_, _ = s.backupRunner.CheckAndRunDueSchedules(ctx)
	}
}

// Stop cancels the scheduler loop and waits for its goroutine to exit cleanly before returning.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	wasRunning := s.running
	s.running = false
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if wasRunning {
		s.wg.Wait()
	}
}
