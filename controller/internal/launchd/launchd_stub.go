//go:build !darwin

package launchd

import (
	"fmt"
	"runtime"
)

// Status describes the current state of the launchd service.
type Status struct {
	Installed  bool
	Running    bool
	RunAtLoad  bool
	PID        int
	LastExit   int
	PlistPath  string
}

func errUnsupported() error {
	return fmt.Errorf("launchd: not supported on %s", runtime.GOOS)
}

// QueryStatus is not supported on non-macOS platforms.
func QueryStatus() *Status { return &Status{} }

// Install is not supported on non-macOS platforms.
func Install(_ bool) error { return errUnsupported() }

// Uninstall is not supported on non-macOS platforms.
func Uninstall() error { return errUnsupported() }

// Start is not supported on non-macOS platforms.
func Start() error { return errUnsupported() }

// Stop is not supported on non-macOS platforms.
func Stop() error { return errUnsupported() }

// SetRunAtLoad is not supported on non-macOS platforms.
func SetRunAtLoad(_ bool) error { return errUnsupported() }

// ReadLog is not supported on non-macOS platforms.
func ReadLog(_ int) (string, error) { return "", errUnsupported() }
