package network

import (
	"fmt"
	"net"
	"os/exec"
)

// Manager provides platform-specific network configuration.
type Manager interface {
	// CreateTAP creates and configures a TAP adapter.
	CreateTAP(name string, hostIP, vmIP net.IP, mask net.IPMask) error

	// DestroyTAP removes a TAP adapter.
	DestroyTAP(name string) error

	// SaveConfig captures the current network configuration so it
	// can be restored later.
	SaveConfig() (*SavedConfig, error)

	// RestoreConfig restores a previously saved network configuration.
	RestoreConfig(cfg *SavedConfig) error

	// SetupRouting configures routes so traffic flows through the VM.
	SetupRouting(tapName string, vmIP net.IP) error

	// TeardownRouting removes routes added by SetupRouting.
	TeardownRouting() error

	// FlushDNS clears the system DNS cache.
	FlushDNS() error
}

// SavedConfig holds opaque platform-specific network state.
type SavedConfig struct {
	Data     []byte
	Platform string
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %v: %s: %w", name, args, string(out), err)
	}
	return nil
}
