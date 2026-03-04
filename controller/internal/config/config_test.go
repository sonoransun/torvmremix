package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig should be valid, got: %v", err)
	}
}

func TestValidateInvalidIPs(t *testing.T) {
	fields := []string{"HostIP", "VMIP", "SubnetMask", "DNS1", "DNS2"}
	for _, field := range fields {
		cfg := DefaultConfig()
		switch field {
		case "HostIP":
			cfg.HostIP = "999.999.999.999"
		case "VMIP":
			cfg.VMIP = "not-an-ip"
		case "SubnetMask":
			cfg.SubnetMask = ""
		case "DNS1":
			cfg.DNS1 = "abc"
		case "DNS2":
			cfg.DNS2 = "1.2.3"
		}
		if err := cfg.Validate(); err == nil {
			t.Errorf("expected validation error for invalid %s", field)
		}
	}
}

func TestValidateInvalidPorts(t *testing.T) {
	tests := []struct {
		name string
		set  func(*Config)
	}{
		{"SOCKSPort zero", func(c *Config) { c.SOCKSPort = 0 }},
		{"SOCKSPort negative", func(c *Config) { c.SOCKSPort = -1 }},
		{"ControlPort too high", func(c *Config) { c.ControlPort = 70000 }},
		{"TransPort zero", func(c *Config) { c.TransPort = 0 }},
		{"DNSPort too high", func(c *Config) { c.DNSPort = 65536 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.set(cfg)
			if err := cfg.Validate(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestValidateMemoryBounds(t *testing.T) {
	tests := []struct {
		name    string
		memory  int
		wantErr bool
	}{
		{"too low", 16, true},
		{"minimum", 32, false},
		{"maximum", 4096, false},
		{"too high", 4097, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.VMMemoryMB = tt.memory
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("VMMemoryMB=%d: got err=%v, wantErr=%v", tt.memory, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCPUBounds(t *testing.T) {
	tests := []struct {
		name    string
		cpus    int
		wantErr bool
	}{
		{"zero", 0, true},
		{"minimum", 1, false},
		{"maximum", 16, false},
		{"too high", 17, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.VMCPUs = tt.cpus
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("VMCPUs=%d: got err=%v, wantErr=%v", tt.cpus, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTAPNameUnix(t *testing.T) {
	tests := []struct {
		name    string
		tap     string
		wantErr bool
	}{
		{"valid short", "tap0", false},
		{"valid long", "torvm0", false},
		{"valid max length", "abcdefghijklmno", false},
		{"empty", "", true},
		{"too long", "abcdefghijklmnop", true},
		{"starts with digit", "0tap", true},
		{"contains space", "tap 0", true},
		{"contains hyphen", "tap-0", true},
		{"contains underscore", "tap_0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTAPName(tt.tap)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTAPName(%q): got err=%v, wantErr=%v", tt.tap, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAccel(t *testing.T) {
	tests := []struct {
		accel   string
		wantErr bool
	}{
		{"", false},
		{"kvm", false},
		{"hvf", false},
		{"whpx", false},
		{"tcg", false},
		{"xen", true},
		{"KVM", true},
	}
	for _, tt := range tests {
		t.Run("accel="+tt.accel, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Accel = tt.accel
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Accel=%q: got err=%v, wantErr=%v", tt.accel, err, tt.wantErr)
			}
		})
	}
}

func TestValidateProxyType(t *testing.T) {
	tests := []struct {
		proxyType string
		wantErr   bool
	}{
		{"", false},
		{"http", false},
		{"https", false},
		{"socks5", false},
		{"socks4", true},
		{"ftp", true},
	}
	for _, tt := range tests {
		t.Run("proxy="+tt.proxyType, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Proxy.Type = tt.proxyType
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Proxy.Type=%q: got err=%v, wantErr=%v", tt.proxyType, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBridgeTransport(t *testing.T) {
	tests := []struct {
		transport string
		wantErr   bool
	}{
		{"", false},
		{"none", false},
		{"obfs4", false},
		{"meek-azure", false},
		{"snowflake", false},
		{"scramblesuit", true},
	}
	for _, tt := range tests {
		t.Run("transport="+tt.transport, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Bridge.Transport = tt.transport
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Bridge.Transport=%q: got err=%v, wantErr=%v", tt.transport, err, tt.wantErr)
			}
		})
	}
}

func TestLoadEmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with empty path should return defaults, got: %v", err)
	}
	if cfg.HostIP != "10.10.10.2" {
		t.Errorf("expected default HostIP, got %q", cfg.HostIP)
	}
}

func TestLoadNonexistentPath(t *testing.T) {
	cfg, err := Load("/tmp/nonexistent-torvm-config-test.json")
	if err != nil {
		t.Fatalf("Load with nonexistent path should return defaults, got: %v", err)
	}
	if cfg.HostIP != "10.10.10.2" {
		t.Errorf("expected default HostIP, got %q", cfg.HostIP)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig()
	cfg.VMMemoryMB = 256
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load valid config: %v", err)
	}
	if loaded.VMMemoryMB != 256 {
		t.Errorf("expected VMMemoryMB=256, got %d", loaded.VMMemoryMB)
	}
}

func TestLoadInsecurePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	// Explicitly set group-writable permissions (umask may strip them from WriteFile).
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	_, err = Load(path)
	if err == nil {
		t.Error("expected error for insecure permissions")
	}
}

func TestLoadConfigWithInvalidValues(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	data := []byte(`{"vm_memory_mb": 8}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for VMMemoryMB=8")
	}
}
