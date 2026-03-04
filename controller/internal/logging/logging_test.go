package logging

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelError, "ERROR"},
		{LevelInfo, "INFO"},
		{LevelDebug, "DEBUG"},
		{Level(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:   LevelInfo,
		writers: []io.Writer{&buf},
	}

	logger.Error("error msg")
	logger.Info("info msg")
	logger.Debug("debug msg")

	output := buf.String()
	if !strings.Contains(output, "ERROR: error msg") {
		t.Error("expected error message in output")
	}
	if !strings.Contains(output, "INFO: info msg") {
		t.Error("expected info message in output")
	}
	if strings.Contains(output, "DEBUG: debug msg") {
		t.Error("debug message should be filtered at INFO level")
	}
}

func TestLevelFilteringDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:   LevelDebug,
		writers: []io.Writer{&buf},
	}

	logger.Debug("debug msg")
	output := buf.String()
	if !strings.Contains(output, "DEBUG: debug msg") {
		t.Error("expected debug message at DEBUG level")
	}
}

func TestLevelFilteringErrorOnly(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:   LevelError,
		writers: []io.Writer{&buf},
	}

	logger.Error("error msg")
	logger.Info("info msg")
	logger.Debug("debug msg")

	output := buf.String()
	if !strings.Contains(output, "ERROR: error msg") {
		t.Error("expected error message")
	}
	if strings.Contains(output, "INFO") {
		t.Error("info should be filtered at ERROR level")
	}
	if strings.Contains(output, "DEBUG") {
		t.Error("debug should be filtered at ERROR level")
	}
}

func TestFormatStrings(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:   LevelDebug,
		writers: []io.Writer{&buf},
	}

	logger.Info("count: %d, name: %s", 42, "test")
	output := buf.String()
	if !strings.Contains(output, "count: 42, name: test") {
		t.Errorf("format string not applied correctly, got: %s", output)
	}
}

func TestLogFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:   LevelInfo,
		writers: []io.Writer{&buf},
	}

	logger.Info("hello")
	output := buf.String()

	// Check format: [YYYY/MM/DD HH:MM:SS.mmm UTC] INFO: hello
	if !strings.Contains(output, "UTC]") {
		t.Errorf("expected UTC timestamp, got: %s", output)
	}
	if !strings.Contains(output, "INFO: hello") {
		t.Errorf("expected 'INFO: hello', got: %s", output)
	}
	if !strings.HasSuffix(output, "\n") {
		t.Error("expected trailing newline")
	}
}

func TestAddWriter(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	logger := &Logger{
		level:   LevelInfo,
		writers: []io.Writer{&buf1},
	}

	logger.Info("before")
	logger.AddWriter(&buf2)
	logger.Info("after")

	if !strings.Contains(buf1.String(), "before") {
		t.Error("buf1 should have 'before'")
	}
	if !strings.Contains(buf1.String(), "after") {
		t.Error("buf1 should have 'after'")
	}
	if strings.Contains(buf2.String(), "before") {
		t.Error("buf2 should NOT have 'before'")
	}
	if !strings.Contains(buf2.String(), "after") {
		t.Error("buf2 should have 'after'")
	}
}

func TestNewLoggerVerbose(t *testing.T) {
	logger, err := NewLogger(Options{Verbose: true})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	if logger.level != LevelDebug {
		t.Errorf("expected LevelDebug with Verbose, got %v", logger.level)
	}
}

func TestNewLoggerDefault(t *testing.T) {
	logger, err := NewLogger(Options{})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	if logger.level != LevelInfo {
		t.Errorf("expected LevelInfo by default, got %v", logger.level)
	}
}
