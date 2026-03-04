//go:build windows

package network

import (
	"net"
)

// verifyRoutes is a stub on Windows. Parsing `route print` output is
// complex and unreliable across Windows versions. A future implementation
// could use Win32 GetIpForwardTable2 via syscall.
func verifyRoutes(_ string, _ net.IP) error {
	return nil
}
