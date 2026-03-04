package network

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	HMAC     string // Hex-encoded HMAC for integrity verification.
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %v: %s: %w", name, args, string(out), err)
	}
	return nil
}

// newSessionKey generates a 32-byte random key for HMAC integrity verification
// of saved network configs. Returns nil if random bytes are unavailable.
func newSessionKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil
	}
	return key
}

// computeHMAC returns a hex-encoded HMAC-SHA256 of the given data using the
// provided session key. Returns "" if key is nil.
func computeHMAC(key, data []byte) string {
	if key == nil {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyHMAC checks the HMAC of the given data against the expected hex string.
// Returns nil if the key is nil (degraded mode) or if the HMAC matches.
func verifyHMAC(key, data []byte, expected string) error {
	if key == nil {
		return nil
	}
	if expected == "" {
		return fmt.Errorf("saved config has no HMAC; integrity cannot be verified")
	}
	computed := computeHMAC(key, data)
	if !hmac.Equal([]byte(computed), []byte(expected)) {
		return fmt.Errorf("saved config HMAC mismatch; data may have been tampered with")
	}
	return nil
}
