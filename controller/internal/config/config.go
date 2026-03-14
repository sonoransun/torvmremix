package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// tapNameUnixRe matches valid Unix TAP interface names: starts with a letter,
// followed by up to 14 alphanumeric characters.
var tapNameUnixRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]{0,14}$`)

// tapNameWindowsRe matches valid Windows TAP adapter names: letters, digits,
// spaces, and hyphens, up to 64 characters.
var tapNameWindowsRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9 -]{0,63}$`)

// validateTAPName checks that the TAP adapter name matches a strict whitelist.
func validateTAPName(name string) error {
	if name == "" {
		return fmt.Errorf("TAPName must not be empty")
	}
	if runtime.GOOS == "windows" {
		if !tapNameWindowsRe.MatchString(name) {
			return fmt.Errorf("TAPName %q does not match allowed pattern (letters, digits, spaces, hyphens; max 64 chars)", name)
		}
	} else {
		if !tapNameUnixRe.MatchString(name) {
			return fmt.Errorf("TAPName %q does not match allowed pattern (letter followed by up to 14 alphanumeric chars)", name)
		}
	}
	return nil
}

// BridgeConfig holds Tor bridge and pluggable transport settings.
type BridgeConfig struct {
	UseBridges bool     `json:"use_bridges"`
	Transport  string   `json:"transport"` // "none", "obfs4", "meek-azure", "snowflake"
	Bridges    []string `json:"bridges"`   // bridge lines (address:port fingerprint)
}

// ProxyConfig holds upstream proxy settings for Tor.
type ProxyConfig struct {
	Type     string `json:"type"`     // "", "http", "https", "socks5"
	Address  string `json:"address"`  // host:port
	Username string `json:"username"`
	Password string `json:"password"`
}

// RetryConfig holds retry/recovery settings for lifecycle state transitions.
type RetryConfig struct {
	Enabled     bool `json:"retry_enabled"`
	MaxAttempts int  `json:"retry_max_attempts"`
}

// ServiceConfig holds launchd service settings (macOS).
type ServiceConfig struct {
	RunAtLoad bool `json:"run_at_load"`
}

// EntropyConfig holds hardware entropy and RNG settings for the VM.
type EntropyConfig struct {
	// EnableHaveged starts the haveged daemon inside the VM for
	// CPU timing jitter entropy (HAVEGE algorithm).
	EnableHaveged bool `json:"enable_haveged"`

	// EnableRngd starts the rngd daemon inside the VM to harvest
	// entropy from /dev/hwrng, RDRAND, and other hardware sources.
	EnableRngd bool `json:"enable_rngd"`

	// ExposeRDRAND adds the +rdrand CPU flag when running under TCG
	// (software emulation). Under KVM/HVF with -cpu host, RDRAND
	// passes through natively and this setting is ignored.
	ExposeRDRAND bool `json:"expose_rdrand"`

	// SerialEntropyDevice is an optional host device path for an
	// external hardware RNG (e.g., "/dev/ttyUSB0"). When set, QEMU
	// creates a chardev and exposes it as a serial port to the guest.
	SerialEntropyDevice string `json:"serial_entropy_device"`

	// VirtioRNGMaxBytes sets the rate limit for the virtio-rng-pci
	// device (max bytes per period). Range: 64-65536. Default: 1024.
	VirtioRNGMaxBytes int `json:"virtio_rng_max_bytes"`

	// VirtioRNGPeriod sets the rate limit period in milliseconds
	// for the virtio-rng-pci device. Range: 100-60000. Default: 1000.
	VirtioRNGPeriod int `json:"virtio_rng_period"`

	// KernelEntropyBytes is the number of random bytes passed to the
	// VM via the kernel command line ENTROPY= parameter.
	// Range: 16-256. Default: 64.
	KernelEntropyBytes int `json:"kernel_entropy_bytes"`
}

// RelayConfig holds relay exclusion settings for Tor circuit selection.
type RelayConfig struct {
	ExcludeNodes     []string `json:"exclude_nodes"`      // $fingerprint or {CC} entries
	ExcludeExitNodes []string `json:"exclude_exit_nodes"`  // same format, exit-only
	StrictNodes      bool     `json:"strict_nodes"`        // Tor StrictNodes 1|0
}

// Config holds all configuration for the TorVM controller.
type Config struct {
	TAPName       string `json:"tap_name"`
	HostIP        string `json:"host_ip"`
	VMIP          string `json:"vm_ip"`
	SubnetMask    string `json:"subnet_mask"`
	DNS1          string `json:"dns1"`
	DNS2          string `json:"dns2"`
	SOCKSPort     int    `json:"socks_port"`
	ControlPort   int    `json:"control_port"`
	TransPort     int    `json:"trans_port"`
	DNSPort       int    `json:"dns_port"`
	VMMemoryMB    int    `json:"vm_memory_mb"`
	VMCPUs        int    `json:"vm_cpus"`
	KernelPath    string `json:"kernel_path"`
	InitrdPath    string `json:"initrd_path"`
	StateDiskPath string `json:"state_disk_path"`
	QMPSocketPath string `json:"qmp_socket_path"`
	Verbose       bool   `json:"verbose"`
	Accel         string `json:"accel"`
	Headless      bool   `json:"headless"`

	// Runtime-detected platform capabilities (not persisted).
	VhostNet     bool `json:"-"`
	IOMMUEnabled bool `json:"-"`

	Bridge        BridgeConfig  `json:"bridge"`
	Proxy         ProxyConfig   `json:"proxy"`
	Service       ServiceConfig `json:"service"`
	Retry         RetryConfig   `json:"retry"`
	Entropy       EntropyConfig `json:"entropy"`
	Relays        RelayConfig   `json:"relays"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	tapName := "torvm0"
	if runtime.GOOS == "windows" {
		tapName = "TorVM Tap"
	}

	return &Config{
		TAPName:       tapName,
		HostIP:        "10.10.10.2",
		VMIP:          "10.10.10.1",
		SubnetMask:    "255.255.255.252",
		DNS1:          "4.2.2.4",
		DNS2:          "4.2.2.2",
		SOCKSPort:     9050,
		ControlPort:   9051,
		TransPort:     9095,
		DNSPort:       9093,
		VMMemoryMB:    128,
		VMCPUs:        2,
		KernelPath:    filepath.Join("dist", "vm", "vmlinuz"),
		InitrdPath:    filepath.Join("dist", "vm", "initramfs.gz"),
		StateDiskPath: filepath.Join("dist", "vm", "state.img"),
		QMPSocketPath: defaultQMPPath(),
		Verbose:       false,
		Accel:         "",
		Retry: RetryConfig{
			Enabled:     true,
			MaxAttempts: 3,
		},
		Entropy: EntropyConfig{
			EnableHaveged:      true,
			EnableRngd:         true,
			ExposeRDRAND:       true,
			VirtioRNGMaxBytes:  1024,
			VirtioRNGPeriod:    1000,
			KernelEntropyBytes: 64,
		},
	}
}

// Load reads configuration from a JSON file and merges it with defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("default config invalid: %w", err)
		}
		return cfg, nil
	}

	// Check file permissions before reading. Refuse world-writable or
	// group-writable config files to prevent tampering.
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat config file: %w", err)
		}
		if err == nil {
			perm := fi.Mode().Perm()
			if perm&0022 != 0 {
				return nil, fmt.Errorf("config file %s has insecure permissions %04o; must not be group-writable or world-writable (expected 0600 or 0644)", path, perm)
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("default config invalid: %w", err)
			}
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

// Validate checks all config fields for safety and correctness.
func (c *Config) Validate() error {
	// Validate IP addresses.
	for _, pair := range []struct{ name, val string }{
		{"HostIP", c.HostIP},
		{"VMIP", c.VMIP},
		{"SubnetMask", c.SubnetMask},
		{"DNS1", c.DNS1},
		{"DNS2", c.DNS2},
	} {
		if net.ParseIP(pair.val) == nil {
			return fmt.Errorf("invalid IP for %s: %q", pair.name, pair.val)
		}
	}

	// Validate ports.
	if err := validatePort("SOCKSPort", c.SOCKSPort); err != nil {
		return err
	}
	if err := validatePort("ControlPort", c.ControlPort); err != nil {
		return err
	}
	if err := validatePort("TransPort", c.TransPort); err != nil {
		return err
	}
	if err := validatePort("DNSPort", c.DNSPort); err != nil {
		return err
	}

	// Validate VM memory.
	if c.VMMemoryMB < 32 || c.VMMemoryMB > 4096 {
		return fmt.Errorf("VMMemoryMB must be 32-4096, got %d", c.VMMemoryMB)
	}

	// Validate VM CPUs.
	if c.VMCPUs < 1 || c.VMCPUs > 16 {
		return fmt.Errorf("VMCPUs must be 1-16, got %d", c.VMCPUs)
	}

	// Required paths must be non-empty.
	for _, pair := range []struct{ name, val string }{
		{"KernelPath", c.KernelPath},
		{"InitrdPath", c.InitrdPath},
		{"StateDiskPath", c.StateDiskPath},
		{"QMPSocketPath", c.QMPSocketPath},
	} {
		if pair.val == "" {
			return fmt.Errorf("%s must not be empty", pair.name)
		}
	}

	// TAPName must match a strict whitelist pattern.
	if err := validateTAPName(c.TAPName); err != nil {
		return err
	}

	// Whitelist acceleration backends.
	switch c.Accel {
	case "", "kvm", "hvf", "whpx", "tcg":
		// valid
	default:
		return fmt.Errorf("invalid Accel: %q", c.Accel)
	}

	// Whitelist proxy types.
	switch c.Proxy.Type {
	case "", "http", "https", "socks5":
		// valid
	default:
		return fmt.Errorf("invalid Proxy.Type: %q", c.Proxy.Type)
	}

	// Whitelist bridge transports.
	switch c.Bridge.Transport {
	case "", "none", "obfs4", "meek-azure", "snowflake":
		// valid
	default:
		return fmt.Errorf("invalid Bridge.Transport: %q", c.Bridge.Transport)
	}

	// Validate entropy settings.
	if c.Entropy.VirtioRNGMaxBytes < 64 || c.Entropy.VirtioRNGMaxBytes > 65536 {
		return fmt.Errorf("Entropy.VirtioRNGMaxBytes must be 64-65536, got %d", c.Entropy.VirtioRNGMaxBytes)
	}
	if c.Entropy.VirtioRNGPeriod < 100 || c.Entropy.VirtioRNGPeriod > 60000 {
		return fmt.Errorf("Entropy.VirtioRNGPeriod must be 100-60000, got %d", c.Entropy.VirtioRNGPeriod)
	}
	if c.Entropy.KernelEntropyBytes < 16 || c.Entropy.KernelEntropyBytes > 256 {
		return fmt.Errorf("Entropy.KernelEntropyBytes must be 16-256, got %d", c.Entropy.KernelEntropyBytes)
	}
	if c.Entropy.SerialEntropyDevice != "" {
		if strings.Contains(c.Entropy.SerialEntropyDevice, "\x00") {
			return fmt.Errorf("Entropy.SerialEntropyDevice contains null byte")
		}
		if !strings.HasPrefix(c.Entropy.SerialEntropyDevice, "/dev/") {
			return fmt.Errorf("Entropy.SerialEntropyDevice must be a /dev/ path, got %q", c.Entropy.SerialEntropyDevice)
		}
		if strings.Contains(c.Entropy.SerialEntropyDevice, "..") {
			return fmt.Errorf("Entropy.SerialEntropyDevice must not contain '..'")
		}
	}

	return nil
}

func validatePort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be 1-65535, got %d", name, port)
	}
	return nil
}

func defaultQMPPath() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\torvm-qmp`
	}
	return "/run/torvm/qmp.sock"
}
