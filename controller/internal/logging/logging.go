package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity.
type Level int

const (
	LevelError Level = iota
	LevelInfo
	LevelDebug
)

func (l Level) String() string {
	switch l {
	case LevelError:
		return "ERROR"
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// Logger provides thread-safe logging with configurable outputs.
type Logger struct {
	mu      sync.Mutex
	level   Level
	writers []io.Writer
}

// Options configures the logger.
type Options struct {
	LogFile  string
	Verbose  bool
}

// NewLogger creates a logger. It always writes to stderr.
// If opts.LogFile is set, it also writes to that file.
// If opts.Verbose is true, debug messages are included.
func NewLogger(opts Options) (*Logger, error) {
	level := LevelInfo
	if opts.Verbose {
		level = LevelDebug
	}

	writers := []io.Writer{os.Stderr}

	if opts.LogFile != "" {
		f, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		writers = append(writers, f)
	}

	return &Logger{
		level:   level,
		writers: writers,
	}, nil
}

func (l *Logger) log(lvl Level, format string, args ...any) {
	if lvl > l.level {
		return
	}

	now := time.Now().UTC()
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s UTC] %s: %s\n",
		now.Format("2006/01/02 15:04:05.000"),
		lvl.String(),
		msg,
	)

	l.mu.Lock()
	defer l.mu.Unlock()
	for _, w := range l.writers {
		_, _ = io.WriteString(w, line)
	}
}

// AddWriter appends an additional writer to the logger output.
func (l *Logger) AddWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writers = append(l.writers, w)
}

// Error logs at ERROR level.
func (l *Logger) Error(format string, args ...any) {
	l.log(LevelError, format, args...)
}

// Info logs at INFO level.
func (l *Logger) Info(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}
