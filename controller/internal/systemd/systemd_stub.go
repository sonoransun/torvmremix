//go:build !linux

package systemd

// IsRunningUnderSystemd always returns false on non-Linux platforms.
func IsRunningUnderSystemd() bool { return false }

// Ready is a no-op on non-Linux platforms.
func Ready() error { return nil }

// Stopping is a no-op on non-Linux platforms.
func Stopping() error { return nil }

// Watchdog is a no-op on non-Linux platforms.
func Watchdog() error { return nil }

// Status is a no-op on non-Linux platforms.
func Status(_ string) error { return nil }

// MainPID is a no-op on non-Linux platforms.
func MainPID(_ int) error { return nil }
