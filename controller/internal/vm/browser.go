package vm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/logging"
)

// BrowserInstance manages a hardened Chromium browser QEMU VM process.
type BrowserInstance struct {
	Config   *config.Config
	Logger   *logging.Logger
	QEMUPath string

	mu      sync.Mutex
	process *exec.Cmd
	qmp     *QMPClient
	running bool
	waitErr chan error
}

// NewBrowserInstance creates a browser VM instance.
func NewBrowserInstance(cfg *config.Config, logger *logging.Logger) *BrowserInstance {
	return &BrowserInstance{
		Config:  cfg,
		Logger:  logger,
		waitErr: make(chan error, 1),
	}
}

// Start launches the browser VM QEMU process.
func (b *BrowserInstance) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("browser VM already running")
	}

	bcfg := &b.Config.Browser

	// Verify image files exist.
	for _, pair := range []struct{ name, path string }{
		{"BrowserKernel", bcfg.KernelPath},
		{"BrowserInitrd", bcfg.InitrdPath},
		{"BrowserStateDisk", bcfg.StateDiskPath},
	} {
		if _, err := os.Stat(pair.path); err != nil {
			return fmt.Errorf("%s not found: %w", pair.name, err)
		}
	}

	qemuPath, err := resolveQEMUBinary()
	if err != nil {
		return err
	}
	b.QEMUPath = qemuPath

	args, err := BuildBrowserArgs(b.Config)
	if err != nil {
		return fmt.Errorf("build browser QEMU args: %w", err)
	}

	b.Logger.Info("browser VM: launching %s with %d args", qemuPath, len(args))
	b.Logger.Debug("browser VM args: %v", args)

	// Create QMP socket directory.
	qmpDir := filepath.Dir(bcfg.QMPSocketPath)
	if err := os.MkdirAll(qmpDir, 0700); err != nil {
		return fmt.Errorf("create QMP socket dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, qemuPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start browser QEMU: %w", err)
	}

	b.process = cmd
	b.running = true
	b.waitErr = make(chan error, 1)

	go func() {
		err := cmd.Wait()
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		b.waitErr <- err
	}()

	return nil
}

// Stop gracefully shuts down the browser VM.
func (b *BrowserInstance) Stop(ctx context.Context) error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()

	// Try graceful QMP shutdown first.
	bcfg := &b.Config.Browser
	client, err := NewQMPClient(bcfg.QMPSocketPath)
	if err == nil {
		b.Logger.Info("browser VM: sending QMP powerdown")
		if err := client.SystemPowerdown(); err != nil {
			b.Logger.Error("browser VM: QMP powerdown: %v", err)
		}
		client.Close()
	}

	// Wait for process exit or force kill.
	select {
	case <-b.waitErr:
		return nil
	case <-ctx.Done():
		b.mu.Lock()
		if b.process != nil && b.process.Process != nil {
			b.Logger.Info("browser VM: force killing")
			b.process.Process.Kill()
		}
		b.mu.Unlock()
		return ctx.Err()
	}
}

// IsRunning returns whether the browser VM process is alive.
func (b *BrowserInstance) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// Wait blocks until the browser VM exits or context is cancelled.
func (b *BrowserInstance) Wait(ctx context.Context) error {
	select {
	case err := <-b.waitErr:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
