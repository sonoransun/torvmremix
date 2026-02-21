//go:build darwin

package network

import (
	"fmt"
	"net"
	"os/exec"
)

type darwinManager struct{}

// NewManager returns a macOS network manager.
func NewManager() Manager {
	return &darwinManager{}
}

func (m *darwinManager) CreateTAP(name string, hostIP, vmIP net.IP, mask net.IPMask) error {
	// On macOS, QEMU uses vmnet-shared for networking. The TAP device
	// is managed by QEMU itself via the Virtualization.framework.
	// We only need to ensure the host-side routing is configured.
	return nil
}

func (m *darwinManager) DestroyTAP(name string) error {
	// vmnet-shared TAP is managed by QEMU.
	return nil
}

func (m *darwinManager) SaveConfig() (*SavedConfig, error) {
	out, err := exec.Command("netstat", "-rn").Output()
	if err != nil {
		return nil, fmt.Errorf("save routes: %w", err)
	}
	return &SavedConfig{Data: out, Platform: "darwin"}, nil
}

func (m *darwinManager) RestoreConfig(cfg *SavedConfig) error {
	if cfg == nil || cfg.Platform != "darwin" {
		return fmt.Errorf("invalid saved config for darwin")
	}
	// Route restoration is handled by TeardownRouting.
	return nil
}

func (m *darwinManager) SetupRouting(tapName string, vmIP net.IP) error {
	if err := run("route", "-n", "add", "-net", "0.0.0.0/1", vmIP.String()); err != nil {
		return fmt.Errorf("add route 0.0.0.0/1: %w", err)
	}
	if err := run("route", "-n", "add", "-net", "128.0.0.0/1", vmIP.String()); err != nil {
		return fmt.Errorf("add route 128.0.0.0/1: %w", err)
	}
	return nil
}

func (m *darwinManager) TeardownRouting() error {
	_ = run("route", "-n", "delete", "-net", "0.0.0.0/1")
	_ = run("route", "-n", "delete", "-net", "128.0.0.0/1")
	return nil
}

func (m *darwinManager) FlushDNS() error {
	_ = run("dscacheutil", "-flushcache")
	_ = run("killall", "-HUP", "mDNSResponder")
	return nil
}

