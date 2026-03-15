package update

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.1.0", 0},
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.9.9", "1.0.0", -1},
		{"v1.0.0", "1.0.0", 0},
		{"0.1.0", "v0.1.1", -1},
		{"1.0", "1.0.0", 0},
		{"2.0.0", "1.99.99", 1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
