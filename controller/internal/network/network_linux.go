//go:build linux

package network

import (
	"fmt"
	"net"
	"os/exec"
)

type linuxManager struct{}

// NewManager returns a Linux network manager.
func NewManager() Manager {
	return &linuxManager{}
}

func (m *linuxManager) CreateTAP(name string, hostIP, vmIP net.IP, mask net.IPMask) error {
	// Create the TAP device.
	if err := run("ip", "tuntap", "add", "dev", name, "mode", "tap"); err != nil {
		return fmt.Errorf("create tap: %w", err)
	}

	// Assign the host IP address.
	ones, _ := mask.Size()
	cidr := fmt.Sprintf("%s/%d", hostIP.String(), ones)
	if err := run("ip", "addr", "add", cidr, "dev", name); err != nil {
		return fmt.Errorf("set tap address: %w", err)
	}

	// Bring the interface up.
	if err := run("ip", "link", "set", name, "up"); err != nil {
		return fmt.Errorf("bring tap up: %w", err)
	}

	return nil
}

func (m *linuxManager) DestroyTAP(name string) error {
	return run("ip", "tuntap", "del", "dev", name, "mode", "tap")
}

func (m *linuxManager) SaveConfig() (*SavedConfig, error) {
	out, err := exec.Command("ip", "route", "show").Output()
	if err != nil {
		return nil, fmt.Errorf("save routes: %w", err)
	}
	return &SavedConfig{Data: out, Platform: "linux"}, nil
}

func (m *linuxManager) RestoreConfig(cfg *SavedConfig) error {
	if cfg == nil || cfg.Platform != "linux" {
		return fmt.Errorf("invalid saved config for linux")
	}
	// Route restoration is handled by TeardownRouting, which removes
	// the specific routes we added. The kernel restores defaults
	// when the TAP device is removed.
	return nil
}

func (m *linuxManager) SetupRouting(tapName string, vmIP net.IP) error {
	// Add a default route through the VM.
	if err := run("ip", "route", "add", "default", "via", vmIP.String(), "dev", tapName, "metric", "50"); err != nil {
		return fmt.Errorf("add default route: %w", err)
	}
	return nil
}

func (m *linuxManager) TeardownRouting() error {
	// Remove our added route. Errors are expected if it was already cleaned up.
	_ = run("ip", "route", "del", "default", "metric", "50")
	return nil
}

func (m *linuxManager) FlushDNS() error {
	// systemd-resolved
	_ = run("resolvectl", "flush-caches")
	return nil
}

