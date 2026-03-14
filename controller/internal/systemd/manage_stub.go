//go:build !linux

package systemd

// ServiceStatus holds the current state of the systemd service.
type ServiceStatus struct {
	Installed bool
	Active    bool
	Enabled   bool
	PID       string
	Status    string
}

// Install is a no-op on non-Linux platforms.
func Install() error { return nil }

// Uninstall is a no-op on non-Linux platforms.
func Uninstall() error { return nil }

// Start is a no-op on non-Linux platforms.
func Start() error { return nil }

// Stop is a no-op on non-Linux platforms.
func Stop() error { return nil }

// Restart is a no-op on non-Linux platforms.
func Restart() error { return nil }

// Enable is a no-op on non-Linux platforms.
func Enable() error { return nil }

// Disable is a no-op on non-Linux platforms.
func Disable() error { return nil }

// QueryStatus returns an empty status on non-Linux platforms.
func QueryStatus() ServiceStatus { return ServiceStatus{} }

// ReadJournalLogs returns empty on non-Linux platforms.
func ReadJournalLogs(_ int) (string, error) { return "", nil }
