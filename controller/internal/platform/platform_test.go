package platform

import (
	"testing"
)

func TestParseAccelValid(t *testing.T) {
	tests := []struct {
		input string
		want  AccelType
	}{
		{"kvm", KVM},
		{"hvf", HVF},
		{"whpx", WHPX},
		{"tcg", TCG},
	}
	for _, tt := range tests {
		got, err := ParseAccel(tt.input)
		if err != nil {
			t.Errorf("ParseAccel(%q): unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseAccel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseAccelInvalid(t *testing.T) {
	invalid := []string{"", "xen", "KVM", "HVF", "qemu", "vmx"}
	for _, s := range invalid {
		_, err := ParseAccel(s)
		if err == nil {
			t.Errorf("ParseAccel(%q): expected error, got nil", s)
		}
	}
}

func TestDetectReturnsNonNil(t *testing.T) {
	info, err := Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if info == nil {
		t.Fatal("Detect returned nil Info")
	}
	// Accel should be one of the known types.
	switch info.Accel {
	case KVM, HVF, WHPX, TCG:
		// valid
	default:
		t.Errorf("Detect returned unknown accel: %q", info.Accel)
	}
}
