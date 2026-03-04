package lifecycle

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/logging"
	"github.com/user/extorvm/controller/internal/network"
	"github.com/user/extorvm/controller/internal/tor"
	"github.com/user/extorvm/controller/internal/vm"
)

// State represents a lifecycle phase.
type State int

const (
	StateInit State = iota
	StateCheckPrivileges
	StateSaveNetwork
	StateCreateTAP
	StateLaunchVM
	StateWaitTAP
	StateConfigureTAP
	StateFlushDNS
	StateWaitBootstrap
	StateRunning
	StateShutdown
	StateRestoreNetwork
	StateCleanup
	StateFailed
)

func (s State) String() string {
	names := [...]string{
		"Init", "CheckPrivileges", "SaveNetwork", "CreateTAP",
		"LaunchVM", "WaitTAP", "ConfigureTAP", "FlushDNS",
		"WaitBootstrap", "Running", "Shutdown", "RestoreNetwork",
		"Cleanup", "Failed",
	}
	if int(s) < len(names) {
		return names[s]
	}
	return fmt.Sprintf("State(%d)", s)
}

// VMController abstracts VM operations so the lifecycle engine can be
// tested without a real QEMU process.
type VMController interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
	Wait(ctx context.Context) error
}

// StateObserver is called when the lifecycle state changes.
type StateObserver func(from, to State)

// MetricsRecorder is an optional interface for recording lifecycle metrics.
type MetricsRecorder interface {
	RecordTransition(from, to string)
}

// BootstrapObserver is called when bootstrap progress changes.
type BootstrapObserver func(progress int, summary string)

// Engine drives the VM lifecycle state machine.
type Engine struct {
	Config   *config.Config
	Logger   *logging.Logger
	VM       VMController
	Network  network.Manager
	FailSafe *FailSafe
	Metrics  MetricsRecorder

	TorControl         *tor.ControlClient
	bootstrapObservers []BootstrapObserver

	state       State
	savedNet    *network.SavedConfig
	observers   []StateObserver
	retryPolicy map[State]*RetryPolicy
	attempts    map[State]int
}

// OnStateChange registers a callback for state transitions.
func (e *Engine) OnStateChange(fn StateObserver) {
	e.observers = append(e.observers, fn)
}

// State returns the current lifecycle state.
func (e *Engine) State() State { return e.state }

// OnBootstrapProgress registers a callback for bootstrap progress updates.
func (e *Engine) OnBootstrapProgress(fn BootstrapObserver) {
	e.bootstrapObservers = append(e.bootstrapObservers, fn)
}

// NewIdentity sends a NEWNYM signal via the Tor Control Protocol to
// obtain a new Tor identity (new circuits).
func (e *Engine) NewIdentity() error {
	if e.TorControl == nil {
		return fmt.Errorf("tor control not connected")
	}
	return e.TorControl.Signal("NEWNYM")
}

// ReloadConfig applies a new configuration to the running engine.
// Hot-reloadable changes (bridges, proxy, verbose) are applied via the Tor
// Control Protocol. Changes that require a VM restart are logged as warnings.
func (e *Engine) ReloadConfig(newCfg *config.Config) error {
	diff := config.Diff(e.Config, newCfg)
	if !diff.HasChanges() {
		e.Logger.Debug("config reload: no changes detected")
		return nil
	}

	// Log restart-required changes as warnings.
	for _, field := range diff.RestartRequired {
		e.Logger.Info("config reload: %s changed but requires VM restart to take effect", field)
	}

	// Apply hot-reloadable changes via Tor Control Protocol.
	if len(diff.HotReloadable) > 0 {
		e.Logger.Info("config reload: applying hot-reloadable changes: %v", diff.HotReloadable)

		if e.TorControl != nil && e.state == StateRunning {
			// Generate torrc overlay from the new config and push it.
			overlay, err := newCfg.TorrcOverlay()
			if err != nil {
				return fmt.Errorf("config reload: generate torrc overlay: %w", err)
			}

			if overlay != "" {
				directives := parseTorrcOverlay(overlay)
				if err := e.TorControl.SetConf(directives); err != nil {
					return fmt.Errorf("config reload: setconf: %w", err)
				}
			}

			if err := e.TorControl.Signal("RELOAD"); err != nil {
				e.Logger.Error("config reload: RELOAD signal failed (non-fatal): %v", err)
			}
		} else {
			e.Logger.Info("config reload: tor control not available, changes will apply on next restart")
		}
	}

	// Update verbose logging level immediately.
	if newCfg.Verbose != e.Config.Verbose {
		e.Logger.SetVerbose(newCfg.Verbose)
	}

	e.Config = newCfg
	return nil
}

// parseTorrcOverlay converts a torrc overlay string into a map of key=value
// directives suitable for SetConf.
func parseTorrcOverlay(overlay string) map[string]string {
	directives := make(map[string]string)
	for _, line := range strings.Split(overlay, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.IndexByte(line, ' ')
		if idx < 0 {
			directives[line] = ""
		} else {
			directives[line[:idx]] = line[idx+1:]
		}
	}
	return directives
}

// NewEngine creates a lifecycle engine.
func NewEngine(cfg *config.Config, logger *logging.Logger) *Engine {
	inst := vm.NewInstance(cfg, logger)
	netMgr := network.NewManager()

	return &Engine{
		Config:      cfg,
		Logger:      logger,
		VM:          inst,
		Network:     netMgr,
		FailSafe:    NewFailSafe(netMgr, logger),
		state:       StateInit,
		retryPolicy: DefaultRetryPolicy(),
		attempts:    make(map[State]int),
	}
}

// NewEngineWithDeps creates a lifecycle engine with explicit dependencies,
// enabling testing with mock VM and network implementations.
func NewEngineWithDeps(cfg *config.Config, logger *logging.Logger, vmCtrl VMController, netMgr network.Manager) *Engine {
	return &Engine{
		Config:      cfg,
		Logger:      logger,
		VM:          vmCtrl,
		Network:     netMgr,
		FailSafe:    NewFailSafe(netMgr, logger),
		state:       StateInit,
		retryPolicy: DefaultRetryPolicy(),
		attempts:    make(map[State]int),
	}
}

// Run progresses through the lifecycle states. It blocks until
// the VM exits or the context is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			e.transition(StateShutdown)
		}

		e.Logger.Info("lifecycle: entering state %s", e.state)

		var err error
		switch e.state {
		case StateInit:
			e.transition(StateCheckPrivileges)

		case StateCheckPrivileges:
			if err := checkPrivileges(); err != nil {
				return err
			}
			e.transition(StateSaveNetwork)

		case StateSaveNetwork:
			err = e.doSaveNetwork()

		case StateCreateTAP:
			err = e.doCreateTAP()

		case StateLaunchVM:
			err = e.doLaunchVM(ctx)

		case StateWaitTAP:
			err = e.doWaitTAP(ctx)

		case StateConfigureTAP:
			err = e.doConfigureTAP()

		case StateFlushDNS:
			err = e.doFlushDNS()

		case StateWaitBootstrap:
			err = e.doWaitBootstrap(ctx)

		case StateRunning:
			err = e.doRunning(ctx)

		case StateShutdown:
			err = e.doShutdown(ctx)

		case StateRestoreNetwork:
			err = e.doRestoreNetwork()

		case StateCleanup:
			return e.doCleanup()

		case StateFailed:
			return fmt.Errorf("lifecycle: entered failed state")
		}

		if err != nil {
			policy := e.retryPolicy[e.state]
			if retry, delay := ShouldRetry(e.state, err, e.attempts[e.state], policy); retry {
				e.attempts[e.state]++
				e.Logger.Info("lifecycle: %s failed (attempt %d/%d), retrying in %v: %v",
					e.state, e.attempts[e.state], policy.MaxAttempts, delay, err)
				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					e.transition(StateShutdown)
				}
			} else {
				e.Logger.Error("lifecycle: %s failed permanently: %v", e.state, err)
				e.FailSafe.Activate()
				e.transition(StateShutdown)
			}
		}
	}
}

// Start runs the lifecycle loop in a background goroutine,
// returning a channel that receives the result.
func (e *Engine) Start(ctx context.Context) <-chan error {
	ch := make(chan error, 1)
	go func() { ch <- e.Run(ctx) }()
	return ch
}

func (e *Engine) transition(next State) {
	prev := e.state
	e.Logger.Debug("lifecycle: %s -> %s", prev, next)
	delete(e.attempts, prev)
	e.state = next
	if e.Metrics != nil {
		e.Metrics.RecordTransition(prev.String(), next.String())
	}
	for _, fn := range e.observers {
		fn(prev, next)
	}
}

func (e *Engine) fail(err error) {
	e.Logger.Error("lifecycle: fatal: %v", err)
	e.state = StateFailed
}

func (e *Engine) doSaveNetwork() error {
	saved, err := e.Network.SaveConfig()
	if err != nil {
		return err
	}
	e.savedNet = saved
	e.transition(StateCreateTAP)
	return nil
}

func (e *Engine) doCreateTAP() error {
	hostIP := net.ParseIP(e.Config.HostIP)
	if hostIP == nil {
		return fmt.Errorf("invalid HostIP: %q", e.Config.HostIP)
	}
	vmIP := net.ParseIP(e.Config.VMIP)
	if vmIP == nil {
		return fmt.Errorf("invalid VMIP: %q", e.Config.VMIP)
	}
	maskIP := net.ParseIP(e.Config.SubnetMask)
	if maskIP == nil {
		return fmt.Errorf("invalid SubnetMask: %q", e.Config.SubnetMask)
	}
	mask := net.IPMask(maskIP.To4())

	if err := e.Network.CreateTAP(e.Config.TAPName, hostIP, vmIP, mask); err != nil {
		return err
	}
	e.transition(StateLaunchVM)
	return nil
}

func (e *Engine) doLaunchVM(ctx context.Context) error {
	if err := e.VM.Start(ctx); err != nil {
		return err
	}
	e.transition(StateWaitTAP)
	return nil
}

func (e *Engine) doWaitTAP(ctx context.Context) error {
	// Wait up to 60 seconds for the TAP device to become connected.
	timeout := 60 * time.Second
	deadline := time.Now().Add(timeout)
	backoff := 500 * time.Millisecond
	const maxBackoff = 10 * time.Second

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !e.VM.IsRunning() {
			return fmt.Errorf("VM exited during TAP wait")
		}
		// Check if we can reach the VM IP.
		conn, err := net.DialTimeout("tcp",
			fmt.Sprintf("%s:%d", e.Config.VMIP, e.Config.ControlPort),
			2*time.Second)
		if err == nil {
			// Set linger to 0 to close immediately without TIME_WAIT,
			// avoiding file descriptor exhaustion from probe connections.
			if tc, ok := conn.(*net.TCPConn); ok {
				tc.SetLinger(0)
			}
			conn.Close()
			e.transition(StateConfigureTAP)
			return nil
		}
		time.Sleep(backoff)
		// Exponential backoff capped at maxBackoff.
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	return fmt.Errorf("TAP connect timeout after %v", timeout)
}

func (e *Engine) doConfigureTAP() error {
	vmIP := net.ParseIP(e.Config.VMIP)
	if vmIP == nil {
		return fmt.Errorf("invalid VMIP: %q", e.Config.VMIP)
	}
	if err := e.Network.SetupRouting(e.Config.TAPName, vmIP); err != nil {
		return err
	}
	e.transition(StateFlushDNS)
	return nil
}

func (e *Engine) doFlushDNS() error {
	if err := e.Network.FlushDNS(); err != nil {
		e.Logger.Error("flush DNS failed (non-fatal): %v", err)
	}

	// Establish Tor Control Protocol connection.
	ctrlAddr := fmt.Sprintf("%s:%d", e.Config.VMIP, e.Config.ControlPort)
	client, err := tor.NewControlClient(ctrlAddr, 10*time.Second)
	if err != nil {
		e.Logger.Error("tor control connect failed (falling back to port probe): %v", err)
	} else {
		// Authenticate with empty password (CookieAuthentication or HashedControlPassword
		// is configured in the VM's torrc; empty AUTHENTICATE works for CookieAuth when
		// connecting from the expected interface).
		if err := client.Authenticate(""); err != nil {
			e.Logger.Error("tor control auth failed: %v", err)
			client.Close()
		} else {
			e.TorControl = client
			e.Logger.Info("tor control connected to %s", ctrlAddr)
		}
	}

	e.transition(StateWaitBootstrap)
	return nil
}

func (e *Engine) doWaitBootstrap(ctx context.Context) error {
	// Wait up to 5 minutes for Tor to bootstrap.
	timeout := 5 * time.Minute
	deadline := time.Now().Add(timeout)
	backoff := time.Second
	const maxBackoff = 10 * time.Second

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !e.VM.IsRunning() {
			return fmt.Errorf("VM exited during bootstrap")
		}

		// Use Tor Control Protocol if available for accurate bootstrap status.
		if e.TorControl != nil {
			status, err := e.TorControl.GetBootstrapStatus()
			if err == nil {
				for _, fn := range e.bootstrapObservers {
					fn(status.Progress, status.Summary)
				}
				if status.Progress >= 100 {
					e.Logger.Info("Tor bootstrap complete: %s", status.Summary)
					e.transition(StateRunning)
					return nil
				}
				e.Logger.Debug("bootstrap: %d%% - %s", status.Progress, status.Summary)
			} else {
				e.Logger.Debug("bootstrap query failed: %v", err)
			}
		} else {
			// Fallback: check SOCKS port availability as a bootstrap indicator.
			conn, err := net.DialTimeout("tcp",
				fmt.Sprintf("%s:%d", e.Config.VMIP, e.Config.SOCKSPort),
				2*time.Second)
			if err == nil {
				if tc, ok := conn.(*net.TCPConn); ok {
					tc.SetLinger(0)
				}
				conn.Close()
				e.Logger.Info("Tor SOCKS port is reachable, bootstrap likely complete")
				e.transition(StateRunning)
				return nil
			}
		}

		time.Sleep(backoff)
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	return fmt.Errorf("Tor bootstrap timeout after %v", timeout)
}

func (e *Engine) doRunning(ctx context.Context) error {
	e.Logger.Info("TorVM is running")
	e.FailSafe.Deactivate()

	// Block until the VM exits or context is cancelled.
	err := e.VM.Wait(ctx)
	if err != nil && ctx.Err() == nil {
		e.Logger.Error("VM exited unexpectedly: %v", err)
		e.FailSafe.Activate()
	}
	e.transition(StateShutdown)
	return nil
}

func (e *Engine) doShutdown(ctx context.Context) error {
	// Close Tor Control connection if open.
	if e.TorControl != nil {
		e.TorControl.Close()
		e.TorControl = nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if e.VM.IsRunning() {
		if err := e.VM.Stop(shutdownCtx); err != nil {
			e.Logger.Error("VM stop error: %v", err)
		}
	}
	e.transition(StateRestoreNetwork)
	return nil
}

func (e *Engine) doRestoreNetwork() error {
	if err := e.Network.TeardownRouting(); err != nil {
		e.Logger.Error("teardown routing failed: %v", err)
		// Activate failsafe to block unprotected traffic if routing
		// teardown fails, since traffic may still be flowing without
		// Tor protection.
		e.FailSafe.Activate()
	}

	if e.savedNet != nil {
		if err := e.Network.RestoreConfig(e.savedNet); err != nil {
			e.Logger.Error("restore network failed: %v", err)
		}
	}

	e.Network.DestroyTAP(e.Config.TAPName)
	e.transition(StateCleanup)
	return nil
}

func (e *Engine) doCleanup() error {
	e.FailSafe.Deactivate()
	e.Logger.Info("lifecycle: cleanup complete")
	return nil
}

func checkPrivileges() error {
	if runtime.GOOS == "windows" {
		// Windows privilege check is handled by the OS when creating TAP adapters.
		return nil
	}
	if os.Getuid() != 0 {
		return fmt.Errorf("must run as root (current uid=%d)", os.Getuid())
	}
	return nil
}
