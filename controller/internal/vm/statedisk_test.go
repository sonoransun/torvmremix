package vm

import (
	"strings"
	"testing"
)

func TestValidateGuestPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid simple", "etc/tor/torrc", false},
		{"valid single file", "torrc", false},
		{"valid with dots", "etc/tor/torrc.conf", false},
		{"valid with hyphens", "etc/tor-browser/config", false},
		{"valid with underscore", "etc/tor_config", false},
		{"empty", "", true},
		{"contains dotdot", "etc/../passwd", true},
		{"starts with slash", "/etc/tor/torrc", true},
		{"starts with dot", ".hidden", true},
		{"contains space", "etc/my file", true},
		{"contains backtick", "etc/`whoami`", true},
		{"contains semicolon", "etc;rm", true},
		{"contains null", "etc/\x00bad", true},
		{"too long", strings.Repeat("a", 256), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGuestPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGuestPath(%q): got err=%v, wantErr=%v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSafeHostPathRegex(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		match bool
	}{
		{"valid unix path", "/tmp/torvm-overlay-12345", true},
		{"valid relative", "dist/vm/state.img", true},
		{"valid with hyphens", "/usr/local/share/torvm/state.img", true},
		{"valid with underscore", "/tmp/torvm_test", true},
		{"contains space", "/tmp/my file", false},
		{"contains backtick", "/tmp/`whoami`", false},
		{"contains dollar", "/tmp/$HOME", false},
		{"contains semicolon", "/tmp/a;b", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeHostPathRe.MatchString(tt.path)
			if got != tt.match {
				t.Errorf("safeHostPathRe.MatchString(%q) = %v, want %v", tt.path, got, tt.match)
			}
		})
	}
}
