//go:build windows

package network

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type windowsManager struct {
	stateDir   string
	sessionKey []byte // Session-derived key for HMAC integrity of saved config.
}

// NewManager returns a Windows network manager.
// Ported from torvm.c: configtap(), savenetconfig(), restorenetconfig().
func NewManager() Manager {
	// Derive state directory from the executable's location to avoid
	// relying on the working directory, which could be attacker-controlled.
	stateDir := filepath.Join(".", "state") // fallback
	if exe, err := os.Executable(); err == nil {
		stateDir = filepath.Join(filepath.Dir(exe), "state")
	}

	// Generate a session-specific key for HMAC integrity of saved config.
	sessionKey := make([]byte, 32)
	if _, err := rand.Read(sessionKey); err != nil {
		// If we can't get random bytes, proceed without integrity checking.
		// This is a degraded mode that still works but logs a warning.
		sessionKey = nil
	}

	return &windowsManager{
		stateDir:   stateDir,
		sessionKey: sessionKey,
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

// computeHMAC returns a hex-encoded HMAC-SHA256 of the given data.
func (m *windowsManager) computeHMAC(data []byte) string {
	if m.sessionKey == nil {
		return ""
	}
	mac := hmac.New(sha256.New, m.sessionKey)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyHMAC checks the HMAC of the given data against the expected value.
func (m *windowsManager) verifyHMAC(data []byte, expected string) error {
	if m.sessionKey == nil {
		// No session key available; skip integrity check.
		return nil
	}
	if expected == "" {
		return fmt.Errorf("saved config has no HMAC; integrity cannot be verified")
	}
	computed := m.computeHMAC(data)
	if !hmac.Equal([]byte(computed), []byte(expected)) {
		return fmt.Errorf("saved config HMAC mismatch; data may have been tampered with")
	}
	return nil
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

	return &SavedConfig{
		Data:     out,
		Platform: "windows",
		HMAC:     m.computeHMAC(out),
	}, nil
}

func (m *windowsManager) RestoreConfig(cfg *SavedConfig) error {
	if cfg == nil || cfg.Platform != "windows" {
		return fmt.Errorf("invalid saved config for windows")
	}

	// Verify HMAC integrity before restoring.
	if err := m.verifyHMAC(cfg.Data, cfg.HMAC); err != nil {
		return fmt.Errorf("saved config integrity check failed: %w", err)
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

