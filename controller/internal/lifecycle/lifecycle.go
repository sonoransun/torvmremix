package lifecycle

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/logging"
	"github.com/user/extorvm/controller/internal/network"
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

// StateObserver is called when the lifecycle state changes.
type StateObserver func(from, to State)

// Engine drives the VM lifecycle state machine.
type Engine struct {
	Config   *config.Config
	Logger   *logging.Logger
	VM       *vm.Instance
	Network  network.Manager
	FailSafe *FailSafe

	state     State
	savedNet  *network.SavedConfig
	observers []StateObserver
}

// OnStateChange registers a callback for state transitions.
func (e *Engine) OnStateChange(fn StateObserver) {
	e.observers = append(e.observers, fn)
}

// State returns the current lifecycle state.
func (e *Engine) State() State { return e.state }

// NewEngine creates a lifecycle engine.
func NewEngine(cfg *config.Config, logger *logging.Logger) *Engine {
	inst := vm.NewInstance(cfg, logger)
	netMgr := network.NewManager()

	return &Engine{
		Config:   cfg,
		Logger:   logger,
		VM:       inst,
		Network:  netMgr,
		FailSafe: NewFailSafe(netMgr, logger),
		state:    StateInit,
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
			e.Logger.Error("lifecycle: %s failed: %v", e.state, err)
			e.FailSafe.Activate()
			e.transition(StateShutdown)
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
	e.state = next
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
			conn.Close()
			e.transition(StateConfigureTAP)
			return nil
		}
		time.Sleep(time.Second)
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
	e.transition(StateWaitBootstrap)
	return nil
}

func (e *Engine) doWaitBootstrap(ctx context.Context) error {
	// Wait up to 5 minutes for Tor to bootstrap.
	timeout := 5 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !e.VM.IsRunning() {
			return fmt.Errorf("VM exited during bootstrap")
		}
		// Check SOCKS port availability as a bootstrap indicator.
		conn, err := net.DialTimeout("tcp",
			fmt.Sprintf("%s:%d", e.Config.VMIP, e.Config.SOCKSPort),
			2*time.Second)
		if err == nil {
			conn.Close()
			e.Logger.Info("Tor SOCKS port is reachable, bootstrap likely complete")
			e.transition(StateRunning)
			return nil
		}
		time.Sleep(2 * time.Second)
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
	e.Network.TeardownRouting()

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
