package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

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

// ServiceConfig holds launchd service settings (macOS).
type ServiceConfig struct {
	RunAtLoad bool `json:"run_at_load"`
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

	// TAPName must be non-empty and free of shell metacharacters.
	if c.TAPName == "" {
		return fmt.Errorf("TAPName must not be empty")
	}
	if strings.ContainsAny(c.TAPName, ";|&$`\\\"'<>(){}!\n\r") {
		return fmt.Errorf("TAPName contains invalid characters: %q", c.TAPName)
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
