package security

import (
	"encoding/hex"
	"testing"
)

func TestEntropyHexStringLength(t *testing.T) {
	tests := []struct {
		bytes   int
		hexLen  int
	}{
		{16, 32},
		{32, 64},
		{1, 2},
		{64, 128},
	}
	for _, tt := range tests {
		s, err := EntropyHexString(tt.bytes)
		if err != nil {
			t.Fatalf("EntropyHexString(%d): %v", tt.bytes, err)
		}
		if len(s) != tt.hexLen {
			t.Errorf("EntropyHexString(%d) length = %d, want %d", tt.bytes, len(s), tt.hexLen)
		}
	}
}

func TestEntropyHexStringValidHex(t *testing.T) {
	s, err := EntropyHexString(32)
	if err != nil {
		t.Fatalf("EntropyHexString: %v", err)
	}
	_, err = hex.DecodeString(s)
	if err != nil {
		t.Errorf("EntropyHexString returned invalid hex: %v", err)
	}
}

func TestEntropyHexStringUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, err := EntropyHexString(16)
		if err != nil {
			t.Fatalf("EntropyHexString: %v", err)
		}
		if seen[s] {
			t.Fatalf("duplicate entropy string on iteration %d: %s", i, s)
		}
		seen[s] = true
	}
}
