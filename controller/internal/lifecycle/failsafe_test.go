package lifecycle

import (
	"sync"
	"testing"

	"github.com/user/extorvm/controller/internal/testutil"
)

func TestFailSafeActivate(t *testing.T) {
	net := &mockNetwork{}
	logger, _ := testutil.NewTestLogger()
	fs := NewFailSafe(net, logger)

	fs.Activate()
	if !fs.IsActive() {
		t.Error("failsafe should be active")
	}

	net.mu.Lock()
	teardowns := net.teardownCount
	net.mu.Unlock()
	if teardowns != 1 {
		t.Errorf("TeardownRouting called %d times, want 1", teardowns)
	}
}

func TestFailSafeActivateIdempotent(t *testing.T) {
	net := &mockNetwork{}
	logger, _ := testutil.NewTestLogger()
	fs := NewFailSafe(net, logger)

	fs.Activate()
	fs.Activate()
	fs.Activate()

	net.mu.Lock()
	teardowns := net.teardownCount
	net.mu.Unlock()
	if teardowns != 1 {
		t.Errorf("TeardownRouting called %d times, want 1 (idempotent)", teardowns)
	}
}

func TestFailSafeDeactivate(t *testing.T) {
	net := &mockNetwork{}
	logger, _ := testutil.NewTestLogger()
	fs := NewFailSafe(net, logger)

	fs.Activate()
	fs.Deactivate()
	if fs.IsActive() {
		t.Error("failsafe should not be active after deactivate")
	}
}

func TestFailSafeDeactivateIdempotent(t *testing.T) {
	net := &mockNetwork{}
	logger, _ := testutil.NewTestLogger()
	fs := NewFailSafe(net, logger)

	// Deactivating when already inactive should be fine.
	fs.Deactivate()
	fs.Deactivate()
	if fs.IsActive() {
		t.Error("failsafe should not be active")
	}
}

func TestFailSafeInitialState(t *testing.T) {
	net := &mockNetwork{}
	logger, _ := testutil.NewTestLogger()
	fs := NewFailSafe(net, logger)

	if fs.IsActive() {
		t.Error("failsafe should not be active initially")
	}
}

func TestFailSafeConcurrentSafety(t *testing.T) {
	net := &mockNetwork{}
	logger, _ := testutil.NewTestLogger()
	fs := NewFailSafe(net, logger)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			fs.Activate()
		}()
		go func() {
			defer wg.Done()
			fs.Deactivate()
		}()
	}
	wg.Wait()

	// Just verifying no race/panic occurred. Final state is non-deterministic.
	_ = fs.IsActive()
}
