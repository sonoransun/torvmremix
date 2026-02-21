package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	KernelPath    string `json:"kernel_path"`
	InitrdPath    string `json:"initrd_path"`
	StateDiskPath string `json:"state_disk_path"`
	QMPSocketPath string `json:"qmp_socket_path"`
	Verbose       bool   `json:"verbose"`
	Accel         string `json:"accel"`
	Headless      bool   `json:"headless"`
	Bridge        BridgeConfig `json:"bridge"`
	Proxy         ProxyConfig  `json:"proxy"`
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
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultQMPPath() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\torvm-qmp`
	}
	dir := os.TempDir()
	return filepath.Join(dir, "torvm-qmp.sock")
}
