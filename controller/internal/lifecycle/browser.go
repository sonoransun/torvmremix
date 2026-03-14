package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/logging"
	"github.com/user/extorvm/controller/internal/secwatch"
	"github.com/user/extorvm/controller/internal/vm"
)

// BrowserState represents a browser VM lifecycle phase.
type BrowserState int

const (
	BrowserIdle BrowserState = iota
	BrowserWaitingForTor
	BrowserStarting
	BrowserRunning
	BrowserCompromised
	BrowserStopping
)

func (s BrowserState) String() string {
	names := [...]string{
		"Idle", "WaitingForTor", "Starting", "Running", "Compromised", "Stopping",
	}
	if int(s) < len(names) {
		return names[s]
	}
	return fmt.Sprintf("BrowserState(%d)", s)
}

// BrowserStateObserver is called when the browser lifecycle state changes.
type BrowserStateObserver func(from, to BrowserState)

// BrowserEngine manages the lifecycle of the hardened browser VM,
// subordinate to the Tor VM lifecycle engine.
type BrowserEngine struct {
	Config     *config.Config
	Logger     *logging.Logger
	VM         *vm.BrowserInstance
	SecMonitor *secwatch.Monitor
	TorEngine  *Engine

	mu        sync.Mutex
	state     BrowserState
	observers []BrowserStateObserver
	cancel    context.CancelFunc
}

// NewBrowserEngine creates a browser lifecycle engine subordinate to the
// given Tor engine.
func NewBrowserEngine(cfg *config.Config, logger *logging.Logger, torEngine *Engine) *BrowserEngine {
	inst := vm.NewBrowserInstance(cfg, logger)
	mon := secwatch.NewMonitor(vm.SecwatchSocketPath(), logger)

	return &BrowserEngine{
		Config:     cfg,
		Logger:     logger,
		VM:         inst,
		SecMonitor: mon,
		TorEngine:  torEngine,
		state:      BrowserIdle,
	}
}

// State returns the current browser lifecycle state.
func (b *BrowserEngine) State() BrowserState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// OnStateChange registers a callback for browser state transitions.
func (b *BrowserEngine) OnStateChange(fn BrowserStateObserver) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.observers = append(b.observers, fn)
}

// Start launches the browser VM. The Tor VM must be in StateRunning.
func (b *BrowserEngine) Start(ctx context.Context) <-chan error {
	ch := make(chan error, 1)
	go func() { ch <- b.run(ctx) }()
	return ch
}

// Stop gracefully shuts down the browser VM.
func (b *BrowserEngine) Stop() {
	b.mu.Lock()
	cancel := b.cancel
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (b *BrowserEngine) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.cancel = cancel
	b.mu.Unlock()
	defer cancel()

	// Verify Tor VM is running.
	if b.TorEngine.State() != StateRunning {
		b.transition(BrowserWaitingForTor)
		b.Logger.Info("browser: waiting for Tor VM to reach Running state")
		for {
			if b.TorEngine.State() == StateRunning {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}

	// Launch browser VM.
	b.transition(BrowserStarting)
	if err := b.VM.Start(ctx); err != nil {
		b.transition(BrowserIdle)
		return fmt.Errorf("browser VM start: %w", err)
	}

	// Start security event monitor.
	b.SecMonitor.OnEvent(func(ev secwatch.SecurityEvent) {
		if ev.IsCritical() {
			b.Logger.Error("browser: CRITICAL security event: [%s] %s", ev.Type, ev.Detail)
			b.handleCompromise(ctx, ev)
		}
	})
	b.SecMonitor.Start()

	b.transition(BrowserRunning)
	b.Logger.Info("browser VM is running")

	// Block until VM exits or context cancelled.
	err := b.VM.Wait(ctx)

	b.SecMonitor.Stop()
	b.transition(BrowserStopping)

	// Graceful shutdown.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if b.VM.IsRunning() {
		b.VM.Stop(shutdownCtx)
	}

	b.transition(BrowserIdle)
	return err
}

func (b *BrowserEngine) handleCompromise(ctx context.Context, ev secwatch.SecurityEvent) {
	b.transition(BrowserCompromised)

	// Stop the browser VM immediately.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if b.VM.IsRunning() {
		b.Logger.Info("browser: stopping compromised VM")
		b.VM.Stop(shutdownCtx)
	}

	// Auto-restart if configured.
	if b.Config.Browser.AutoRemediate {
		b.Logger.Info("browser: auto-remediating, restarting in 3 seconds")
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return
		}
		if b.TorEngine.State() == StateRunning {
			b.transition(BrowserStarting)
			if err := b.VM.Start(ctx); err != nil {
				b.Logger.Error("browser: restart failed: %v", err)
				b.transition(BrowserIdle)
				return
			}
			b.SecMonitor.Start()
			b.transition(BrowserRunning)
			b.Logger.Info("browser: restarted after compromise remediation")
		}
	}
}

func (b *BrowserEngine) transition(next BrowserState) {
	b.mu.Lock()
	prev := b.state
	b.state = next
	snap := make([]BrowserStateObserver, len(b.observers))
	copy(snap, b.observers)
	b.mu.Unlock()

	b.Logger.Debug("browser lifecycle: %s -> %s", prev, next)
	for _, fn := range snap {
		fn(prev, next)
	}
}
