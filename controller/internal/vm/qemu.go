package vm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/logging"
)

// qemuAllowedDirs lists directories where the QEMU binary is expected to
// reside, per platform. Resolved binary paths outside these directories
// are rejected to prevent PATH-based binary substitution attacks.
var qemuAllowedDirs = map[string][]string{
	"linux":   {"/usr/bin", "/usr/local/bin", "/usr/sbin", "/usr/local/sbin", "/snap/bin"},
	"darwin":  {"/usr/bin", "/usr/local/bin", "/opt/homebrew/bin", "/opt/local/bin"},
	"windows": {`C:\Program Files`, `C:\Program Files (x86)`},
}

// resolveQEMUBinary locates the QEMU binary via exec.LookPath, resolves
// symlinks, and validates the resolved path is under an allowed directory.
func resolveQEMUBinary() (string, error) {
	path, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		return "", fmt.Errorf("qemu-system-x86_64 not found in PATH: %w", err)
	}

	// Resolve symlinks to get the real path.
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve qemu binary path: %w", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("absolute qemu binary path: %w", err)
	}

	// Validate the resolved path is under an allowed directory.
	allowed := qemuAllowedDirs[runtime.GOOS]
	if len(allowed) == 0 {
		// Unknown platform; accept any resolved path but log a warning.
		return resolved, nil
	}

	resolvedDir := filepath.Dir(resolved)
	for _, dir := range allowed {
		if runtime.GOOS == "windows" {
			// Case-insensitive comparison on Windows.
			if strings.EqualFold(resolvedDir, dir) || strings.HasPrefix(strings.ToLower(resolvedDir), strings.ToLower(dir)+string(filepath.Separator)) {
				return resolved, nil
			}
		} else {
			if resolvedDir == dir || strings.HasPrefix(resolvedDir, dir+string(filepath.Separator)) {
				return resolved, nil
			}
		}
	}

	return "", fmt.Errorf("qemu binary %q is not under an allowed directory %v", resolved, allowed)
}

// Instance manages a QEMU virtual machine process.
type Instance struct {
	Config   *config.Config
	Logger   *logging.Logger
	Process  *exec.Cmd
	QEMUPath string // Resolved and validated QEMU binary path.

	mu       sync.Mutex
	qmp      *QMPClient
	running  bool
	waitErr  chan error
}

// NewInstance creates a new VM instance. It resolves the QEMU binary
// path at construction time so it can be validated once and reused.
func NewInstance(cfg *config.Config, logger *logging.Logger) *Instance {
	inst := &Instance{
		Config:  cfg,
		Logger:  logger,
		waitErr: make(chan error, 1),
	}

	// Resolve QEMU binary path eagerly. Errors will be reported at Start().
	if qemuPath, err := resolveQEMUBinary(); err != nil {
		logger.Error("QEMU binary resolution failed: %v", err)
	} else {
		inst.QEMUPath = qemuPath
		logger.Info("resolved QEMU binary: %s", qemuPath)
	}

	return inst
}

// Start launches the QEMU process with the configured arguments.
func (inst *Instance) Start(ctx context.Context) error {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.running {
		return fmt.Errorf("vm: already running")
	}

	// Write torrc overlay to state disk if bridge or proxy settings are configured.
	overlay, err := inst.Config.TorrcOverlay()
	if err != nil {
		return fmt.Errorf("vm: torrc overlay: %w", err)
	}
	if overlay != "" {
		if err := WriteStateDiskFile(inst.Config.StateDiskPath, "torrc.override", overlay); err != nil {
			return fmt.Errorf("vm: write torrc overlay: %w", err)
		}
		inst.Logger.Info("wrote torrc overlay to state disk")
	}

	// Verify VM image files exist before launching QEMU.
	for _, pair := range []struct{ name, path string }{
		{"kernel", inst.Config.KernelPath},
		{"initrd", inst.Config.InitrdPath},
		{"state disk", inst.Config.StateDiskPath},
	} {
		if _, err := os.Stat(pair.path); err != nil {
			return fmt.Errorf("vm: %s file not found: %w", pair.name, err)
		}
	}

	// Create QMP socket directory with restrictive permissions.
	if runtime.GOOS != "windows" {
		qmpDir := filepath.Dir(inst.Config.QMPSocketPath)
		if err := os.MkdirAll(qmpDir, 0700); err != nil {
			return fmt.Errorf("vm: create QMP socket dir: %w", err)
		}
	}

	args, err := inst.BuildArgs()
	if err != nil {
		return fmt.Errorf("vm: build args: %w", err)
	}

	if inst.QEMUPath == "" {
		return fmt.Errorf("vm: QEMU binary not resolved; cannot start")
	}

	inst.Logger.Info("starting QEMU with %d args", len(args))
	inst.Logger.Debug("qemu binary: %s, args: %v", inst.QEMUPath, args)

	inst.Process = exec.CommandContext(ctx, inst.QEMUPath, args...)

	if err := inst.Process.Start(); err != nil {
		return fmt.Errorf("vm: start qemu: %w", err)
	}

	inst.running = true

	// Wait for the process in a goroutine.
	go func() {
		err := inst.Process.Wait()
		inst.mu.Lock()
		inst.running = false
		inst.mu.Unlock()
		inst.waitErr <- err
	}()

	return nil
}

// Stop gracefully shuts down the VM. It first attempts a QMP
// system_powerdown, then falls back to killing the process.
func (inst *Instance) Stop(ctx context.Context) error {
	inst.mu.Lock()
	if !inst.running {
		inst.mu.Unlock()
		return nil
	}
	// Capture process reference while holding the lock to avoid race.
	proc := inst.Process
	inst.mu.Unlock()

	// Try graceful shutdown via QMP.
	qmp, err := NewQMPClient(inst.Config.QMPSocketPath)
	if err == nil {
		inst.Logger.Info("sending QMP system_powerdown")
		if err := qmp.SystemPowerdown(); err != nil {
			inst.Logger.Error("QMP powerdown failed: %v", err)
		}
		qmp.Close()

		// Wait a bit for graceful shutdown.
		select {
		case <-ctx.Done():
		case err := <-inst.waitErr:
			return err
		}
	}

	// Fallback: kill the process using captured reference.
	inst.mu.Lock()
	defer inst.mu.Unlock()
	if inst.running && proc != nil && proc.Process != nil {
		inst.Logger.Info("killing QEMU process")
		return proc.Process.Kill()
	}
	return nil
}

// IsRunning reports whether the QEMU process is still alive.
func (inst *Instance) IsRunning() bool {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	return inst.running
}

// Wait blocks until the QEMU process exits.
func (inst *Instance) Wait(ctx context.Context) error {
	select {
	case err := <-inst.waitErr:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
