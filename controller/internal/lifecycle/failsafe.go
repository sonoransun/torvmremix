package lifecycle

import (
	"sync"

	"github.com/user/extorvm/controller/internal/logging"
	"github.com/user/extorvm/controller/internal/network"
)

// FailSafe blocks all network traffic when the VM dies unexpectedly,
// preventing unprotected network access.
type FailSafe struct {
	netMgr network.Manager
	logger *logging.Logger

	mu     sync.Mutex
	active bool
}

// NewFailSafe creates a new failsafe controller.
func NewFailSafe(netMgr network.Manager, logger *logging.Logger) *FailSafe {
	return &FailSafe{
		netMgr: netMgr,
		logger: logger,
	}
}

// Activate enables the failsafe, tearing down routing to block traffic.
func (f *FailSafe) Activate() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.active {
		return
	}

	f.logger.Error("failsafe: ACTIVATING - blocking all network traffic")
	if err := f.netMgr.TeardownRouting(); err != nil {
		f.logger.Error("failsafe: teardown routing: %v", err)
	}
	f.active = true
}

// Deactivate disables the failsafe.
func (f *FailSafe) Deactivate() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.active {
		return
	}

	f.logger.Info("failsafe: deactivating")
	f.active = false
}

// IsActive reports whether the failsafe is currently engaged.
func (f *FailSafe) IsActive() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}
