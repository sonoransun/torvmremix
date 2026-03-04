//go:build !windows

package winsvc

// TAPStatus describes the current state of the TAP-Windows adapter.
type TAPStatus struct {
	Installed   bool
	AdapterName string
	DriverPath  string
}

// QueryTAPStatus is not supported on non-Windows platforms.
func QueryTAPStatus() *TAPStatus { return &TAPStatus{} }

// InstallTAPAdapter is not supported on non-Windows platforms.
func InstallTAPAdapter() error { return errUnsupported() }

// RemoveTAPAdapter is not supported on non-Windows platforms.
func RemoveTAPAdapter() error { return errUnsupported() }
