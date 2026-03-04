package config

import (
	"strings"
	"testing"
)

func TestSanitizeTorrcLine(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"normal ascii", "hello world 123", false},
		{"contains newline", "hello\nworld", true},
		{"contains carriage return", "hello\rworld", true},
		{"contains tab", "hello\tworld", true},
		{"contains null byte", "hello\x00world", true},
		{"contains DEL", "hello\x7fworld", true},
		{"empty string", "", false},
		{"printable special chars", "obfs4 192.168.1.1:443 cert=abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizeTorrcLine("test", tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeTorrcLine(%q): got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBridgeLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantErr bool
	}{
		{"valid simple", "obfs4 192.168.1.1:443 AAAA", false},
		{"valid with fingerprint", "obfs4 1.2.3.4:9001 ABCDEF1234567890ABCDEF1234567890ABCDEF12 cert=abc iat-mode=0", false},
		{"valid ipv6", "obfs4 [::1]:443 ABCD", false},
		{"too long", strings.Repeat("a", 1025), true},
		{"contains semicolon", "obfs4 1.2.3.4:443;rm -rf /", true},
		{"contains backtick", "obfs4 1.2.3.4:443`whoami`", true},
		{"contains newline", "obfs4\n1.2.3.4:443", true},
		{"contains dollar sign", "obfs4 $HOME:443", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBridgeLine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBridgeLine(%q): got err=%v, wantErr=%v", tt.line, err, tt.wantErr)
			}
		})
	}
}

func TestValidateProxyAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid host port", "192.168.1.1:8080", false},
		{"valid hostname", "proxy.example.com:3128", false},
		{"missing port", "192.168.1.1", true},
		{"empty host", ":8080", true},
		{"empty string", "", true},
		{"contains newline", "192.168.1.1\n:8080", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProxyAddress(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProxyAddress(%q): got err=%v, wantErr=%v", tt.addr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCredential(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty allowed", "", false},
		{"simple alphanumeric", "user123", false},
		{"special chars", "p@ss!w0rd#", false},
		{"too long", strings.Repeat("a", 256), true},
		{"contains space", "user name", true},
		{"contains newline", "user\nname", true},
		{"contains tab", "user\tname", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCredential("test", tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCredential(%q): got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestTorrcOverlayEmpty(t *testing.T) {
	cfg := DefaultConfig()
	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overlay != "" {
		t.Errorf("expected empty overlay for default config, got %q", overlay)
	}
}

func TestTorrcOverlayObfs4(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Bridge.UseBridges = true
	cfg.Bridge.Transport = "obfs4"
	cfg.Bridge.Bridges = []string{"obfs4 1.2.3.4:443 ABCD cert=xyz iat-mode=0"}

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "UseBridges 1") {
		t.Error("expected UseBridges 1")
	}
	if !strings.Contains(overlay, "ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy") {
		t.Error("expected obfs4 transport plugin line")
	}
	if !strings.Contains(overlay, "Bridge obfs4 1.2.3.4:443 ABCD cert=xyz iat-mode=0") {
		t.Error("expected bridge line")
	}
}

func TestTorrcOverlayMeekAzure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Bridge.UseBridges = true
	cfg.Bridge.Transport = "meek-azure"

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "ClientTransportPlugin meek_lite exec /usr/bin/obfs4proxy") {
		t.Error("expected meek_lite transport plugin line")
	}
}

func TestTorrcOverlaySnowflake(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Bridge.UseBridges = true
	cfg.Bridge.Transport = "snowflake"

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "ClientTransportPlugin snowflake exec /usr/bin/snowflake-client") {
		t.Error("expected snowflake transport plugin line")
	}
}

func TestTorrcOverlayNoneTransport(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Bridge.UseBridges = true
	cfg.Bridge.Transport = "none"
	cfg.Bridge.Bridges = []string{"1.2.3.4:443 ABCD"}

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "UseBridges 1") {
		t.Error("expected UseBridges 1")
	}
	if strings.Contains(overlay, "ClientTransportPlugin") {
		t.Error("expected no transport plugin for 'none' transport")
	}
}

func TestTorrcOverlayHTTPProxy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Proxy.Type = "http"
	cfg.Proxy.Address = "proxy.example.com:8080"

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "HTTPProxy proxy.example.com:8080") {
		t.Error("expected HTTPProxy line")
	}
}

func TestTorrcOverlayHTTPProxyWithAuth(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Proxy.Type = "http"
	cfg.Proxy.Address = "proxy.example.com:8080"
	cfg.Proxy.Username = "user"
	cfg.Proxy.Password = "pass"

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "HTTPProxyAuthenticator user:pass") {
		t.Error("expected HTTPProxyAuthenticator line")
	}
}

func TestTorrcOverlayHTTPSProxy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Proxy.Type = "https"
	cfg.Proxy.Address = "proxy.example.com:443"
	cfg.Proxy.Username = "user"
	cfg.Proxy.Password = "pass"

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "HTTPSProxy proxy.example.com:443") {
		t.Error("expected HTTPSProxy line")
	}
	if !strings.Contains(overlay, "HTTPSProxyAuthenticator user:pass") {
		t.Error("expected HTTPSProxyAuthenticator line")
	}
}

func TestTorrcOverlaySocks5Proxy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Proxy.Type = "socks5"
	cfg.Proxy.Address = "proxy.example.com:1080"
	cfg.Proxy.Username = "user"
	cfg.Proxy.Password = "pass"

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "Socks5Proxy proxy.example.com:1080") {
		t.Error("expected Socks5Proxy line")
	}
	if !strings.Contains(overlay, "Socks5ProxyUsername user") {
		t.Error("expected Socks5ProxyUsername line")
	}
	if !strings.Contains(overlay, "Socks5ProxyPassword pass") {
		t.Error("expected Socks5ProxyPassword line")
	}
}

func TestTorrcOverlayUnsupportedTransport(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Bridge.UseBridges = true
	cfg.Bridge.Transport = "scramblesuit"

	_, err := cfg.TorrcOverlay()
	if err == nil {
		t.Error("expected error for unsupported transport")
	}
}

func TestTorrcOverlayBridgeAndProxy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Bridge.UseBridges = true
	cfg.Bridge.Transport = "obfs4"
	cfg.Bridge.Bridges = []string{"obfs4 1.2.3.4:443 ABCD cert=xyz iat-mode=0"}
	cfg.Proxy.Type = "socks5"
	cfg.Proxy.Address = "127.0.0.1:1080"

	overlay, err := cfg.TorrcOverlay()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(overlay, "UseBridges 1") {
		t.Error("expected UseBridges")
	}
	if !strings.Contains(overlay, "Socks5Proxy 127.0.0.1:1080") {
		t.Error("expected Socks5Proxy")
	}
}
