//go:build windows

package winsvc

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/svc/eventlog"
)

// EventLogWriter implements io.Writer by sending log entries to the
// Windows Event Log. It can be attached to the logger via AddWriter.
type EventLogWriter struct {
	elog *eventlog.Log
}

// NewEventLogWriter opens a handle to the Windows Event Log for the
// TorVM service name.
func NewEventLogWriter() (*EventLogWriter, error) {
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		return nil, fmt.Errorf("winsvc: open event log: %w", err)
	}
	return &EventLogWriter{elog: elog}, nil
}

// Write sends p as an informational event log entry. It satisfies io.Writer.
func (w *EventLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	if err := w.elog.Info(1, msg); err != nil {
		return 0, fmt.Errorf("winsvc: write event log: %w", err)
	}
	return len(p), nil
}

// Close releases the event log handle.
func (w *EventLogWriter) Close() error {
	return w.elog.Close()
}
