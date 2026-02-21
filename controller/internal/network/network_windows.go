//go:build windows

package network

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type windowsManager struct {
	stateDir string
}

// NewManager returns a Windows network manager.
// Ported from torvm.c: configtap(), savenetconfig(), restorenetconfig().
func NewManager() Manager {
	return &windowsManager{
		stateDir: filepath.Join(".", "state"),
	}
}

func (m *windowsManager) CreateTAP(name string, hostIP, vmIP net.IP, mask net.IPMask) error {
	// TAP-Windows6 adapter is expected to be pre-installed.
	// Configure the adapter IP address via netsh, matching legacy configtap().
	if err := run("netsh", "interface", "ip", "set", "address",
		name, "static", hostIP.String(), net.IP(mask).String(), vmIP.String(), "1"); err != nil {
		return fmt.Errorf("configure tap address: %w", err)
	}
	return nil
}

func (m *windowsManager) DestroyTAP(name string) error {
	// Remove the IP configuration; the adapter itself persists.
	_ = run("netsh", "interface", "ip", "delete", "address", name, "all")
	return nil
}

// validateNetshDump checks that every non-empty, non-comment line in a netsh
// dump starts with a known-safe prefix to prevent command injection.
func validateNetshDump(data []byte) error {
	safePrefixes := []string{
		"set address", "add dns", "set dns",
		"add wins", "set wins",
		"pushd", "popd",
		"set interface",
		"add address",
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "rem ") {
			continue
		}
		safe := false
		lower := strings.ToLower(line)
		for _, prefix := range safePrefixes {
			if strings.HasPrefix(lower, prefix) {
				safe = true
				break
			}
		}
		if !safe {
			return fmt.Errorf("netsh dump line %d has unexpected content: %q", lineNum, line)
		}
	}
	return scanner.Err()
}

func (m *windowsManager) SaveConfig() (*SavedConfig, error) {
	// Capture current IP configuration via netsh, matching legacy savenetconfig().
	out, err := exec.Command("netsh", "interface", "ip", "dump").Output()
	if err != nil {
		return nil, fmt.Errorf("netsh dump: %w", err)
	}

	// Save to a file for later restore.
	savePath := filepath.Join(m.stateDir, "netcfg.save")
	if err := os.MkdirAll(m.stateDir, 0750); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	if err := os.WriteFile(savePath, out, 0600); err != nil {
		return nil, fmt.Errorf("write netcfg save: %w", err)
	}

	return &SavedConfig{Data: out, Platform: "windows"}, nil
}

func (m *windowsManager) RestoreConfig(cfg *SavedConfig) error {
	if cfg == nil || cfg.Platform != "windows" {
		return fmt.Errorf("invalid saved config for windows")
	}

	// Validate the saved dump before executing it.
	if err := validateNetshDump(cfg.Data); err != nil {
		return fmt.Errorf("netsh dump validation failed: %w", err)
	}

	// Write the saved config to a temp file and execute it with netsh,
	// matching legacy restorenetconfig().
	savePath := filepath.Join(m.stateDir, "netcfg.save")
	if err := os.WriteFile(savePath, cfg.Data, 0600); err != nil {
		return fmt.Errorf("write netcfg for restore: %w", err)
	}

	if err := run("netsh", "exec", savePath); err != nil {
		os.Remove(savePath)
		return fmt.Errorf("netsh exec restore: %w", err)
	}

	os.Remove(savePath)
	return nil
}

func (m *windowsManager) SetupRouting(tapName string, vmIP net.IP) error {
	// Set DNS servers on the TAP adapter, matching legacy configtap().
	if err := run("netsh", "interface", "ip", "set", "dns", tapName, "static", "4.2.2.4"); err != nil {
		return fmt.Errorf("set dns1: %w", err)
	}
	if err := run("netsh", "interface", "ip", "add", "dns", tapName, "4.2.2.2"); err != nil {
		return fmt.Errorf("set dns2: %w", err)
	}
	return nil
}

func (m *windowsManager) TeardownRouting() error {
	return nil
}

func (m *windowsManager) FlushDNS() error {
	return run("ipconfig", "/flushdns")
}

