package lifecycle

import (
	"sync"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
)

// Coordinator tracks runtime teardown and restore/maintenance barriers.
type Coordinator struct {
	mu      sync.Mutex
	runtime *bootstrap.Runtime
	closed  bool
}

// NewCoordinator wraps an initialized runtime for controlled shutdown.
func NewCoordinator(rt *bootstrap.Runtime) *Coordinator {
	return &Coordinator{runtime: rt}
}

// Close ensures shutdown runs at most once.
func (c *Coordinator) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.runtime == nil {
		return nil
	}
	c.closed = true
	return c.runtime.Shutdown()
}
