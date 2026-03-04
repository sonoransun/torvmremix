//go:build !windows

package winsvc

import (
	"fmt"
	"runtime"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/logging"
)

func errUnsupported() error {
	return fmt.Errorf("winsvc: not supported on %s", runtime.GOOS)
}

// EventLogWriter is a stub for non-Windows platforms.
type EventLogWriter struct{}

// NewEventLogWriter returns an error on non-Windows platforms.
func NewEventLogWriter() (*EventLogWriter, error) {
	return nil, errUnsupported()
}

// Write is a stub that always returns an error.
func (w *EventLogWriter) Write(p []byte) (int, error) {
	return 0, errUnsupported()
}

// Close is a no-op stub.
func (w *EventLogWriter) Close() error {
	return nil
}

// RunService is not supported on non-Windows platforms.
func RunService(_ *config.Config, _ *logging.Logger) error {
	return errUnsupported()
}

// InstallService is not supported on non-Windows platforms.
func InstallService() error {
	return errUnsupported()
}

// RemoveService is not supported on non-Windows platforms.
func RemoveService() error {
	return errUnsupported()
}
