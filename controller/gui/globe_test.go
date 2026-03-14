package gui

import (
	"math"
	"testing"
)

func TestProjectVisibility(t *testing.T) {
	cx, cy, r := 200.0, 200.0, 100.0

	// Point directly facing the viewer (lat=0, lon=0, rotation=0, tilt=0)
	// should be visible.
	_, _, vis := project(0, 0, 0, 0, cx, cy, r)
	if !vis {
		t.Error("expected (0,0) to be visible with no rotation/tilt")
	}

	// Point on the far side (lon=180) should not be visible.
	_, _, vis = project(0, 180, 0, 0, cx, cy, r)
	if vis {
		t.Error("expected (0,180) to NOT be visible with rotation=0")
	}

	// Point on the far side becomes visible when we rotate to face it.
	_, _, vis = project(0, 180, math.Pi, 0, cx, cy, r)
	if !vis {
		t.Error("expected (0,180) to be visible with rotation=pi")
	}
}

func TestProjectCenter(t *testing.T) {
	cx, cy, r := 200.0, 200.0, 100.0

	// Lat=0, lon=0 with no rotation should project to center.
	x, y, vis := project(0, 0, 0, 0, cx, cy, r)
	if !vis {
		t.Fatal("expected visible")
	}
	if math.Abs(x-cx) > 1 || math.Abs(y-cy) > 1 {
		t.Errorf("expected center (%v, %v), got (%v, %v)", cx, cy, x, y)
	}
}

func TestProjectNorthPole(t *testing.T) {
	cx, cy, r := 200.0, 200.0, 100.0

	// North pole (lat=90) with no tilt should project to top of globe.
	x, y, vis := project(90, 0, 0, 0, cx, cy, r)
	if !vis {
		t.Fatal("expected visible")
	}
	// X should be near center.
	if math.Abs(x-cx) > 1 {
		t.Errorf("north pole X should be near center, got %v", x)
	}
	// Y should be above center (lower y value since y increases downward).
	if y >= cy {
		t.Errorf("north pole Y should be above center (%v), got %v", cy, y)
	}
}

func TestNormalizeRelayEntry(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  ABCDEF0123456789ABCDEF0123456789ABCDEF01  ", "$ABCDEF0123456789ABCDEF0123456789ABCDEF01"},
		{"$ABCDEF0123456789ABCDEF0123456789ABCDEF01", "$ABCDEF0123456789ABCDEF0123456789ABCDEF01"},
		{"{us}", "{US}"},
		{"{DE}", "{DE}"},
		{"  {gb}  ", "{GB}"},
		{"something else", "something else"},
	}
	for _, tt := range tests {
		got := normalizeRelayEntry(tt.input)
		if got != tt.want {
			t.Errorf("normalizeRelayEntry(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateGUIRelayEntry(t *testing.T) {
	valid := []string{
		"$ABCDEF0123456789ABCDEF0123456789ABCDEF01",
		"$abcdef0123456789abcdef0123456789abcdef01",
		"{US}",
		"{DE}",
	}
	for _, v := range valid {
		if err := validateGUIRelayEntry(v); err != nil {
			t.Errorf("expected valid for %q, got %v", v, err)
		}
	}

	invalid := []string{
		"ABCDEF", "$short", "{USA}", "US", "", "random",
	}
	for _, v := range invalid {
		if err := validateGUIRelayEntry(v); err == nil {
			t.Errorf("expected invalid for %q", v)
		}
	}
}

func TestContainsEntry(t *testing.T) {
	slice := []string{"{US}", "$AAAA"}
	if !containsEntry(slice, "{US}") {
		t.Error("expected to find {US}")
	}
	if containsEntry(slice, "{DE}") {
		t.Error("expected NOT to find {DE}")
	}
	if containsEntry(nil, "anything") {
		t.Error("expected NOT to find in nil slice")
	}
}

func TestFormatExcludeEntry(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"{US}", "{US} United States"},
		{"{DE}", "{DE} Germany"},
		{"{ZZ}", "{ZZ}"},
		{"$ABCDEF0123456789ABCDEF0123456789ABCDEF01", "$ABCDEF0123456789ABCDEF0123456789ABCDEF01"},
		{"", ""},
	}
	for _, tt := range tests {
		got := formatExcludeEntry(tt.input)
		if got != tt.want {
			t.Errorf("formatExcludeEntry(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
