package tor

import "testing"

func TestParseRelayPath(t *testing.T) {
	tests := []struct {
		input string
		fp    string
		nick  string
	}{
		{"$AAAA~Guard1", "$AAAA", "Guard1"},
		{"$BBBB", "$BBBB", ""},
		{"$CCCC~", "$CCCC", ""},
		{"~NoFingerprint", "", "NoFingerprint"},
		{"", "", ""},
		{"$ABCDEF0123456789ABCDEF0123456789ABCDEF01~MyRelay", "$ABCDEF0123456789ABCDEF0123456789ABCDEF01", "MyRelay"},
	}
	for _, tt := range tests {
		fp, nick := ParseRelayPath(tt.input)
		if fp != tt.fp || nick != tt.nick {
			t.Errorf("ParseRelayPath(%q) = (%q, %q), want (%q, %q)", tt.input, fp, nick, tt.fp, tt.nick)
		}
	}
}

func TestCloseCircuitValidation(t *testing.T) {
	// CloseCircuit should reject non-numeric IDs without needing a connection.
	c := &ControlClient{
		done: make(chan struct{}),
	}
	close(c.done)

	if err := c.CloseCircuit("abc"); err == nil {
		t.Error("expected error for non-numeric circuit ID")
	}
	if err := c.CloseCircuit("12; DROP TABLE"); err == nil {
		t.Error("expected error for injection attempt")
	}
	if err := c.CloseCircuit(""); err == nil {
		t.Error("expected error for empty circuit ID")
	}
}
