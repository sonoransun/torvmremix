//go:build windows

package network

import (
	"fmt"
	"os/exec"
	"strings"
)

// VerifyRoutes checks that the routing table is correctly configured
// to route traffic through the TAP adapter. It parses the output of
// "route print" and verifies the expected entries exist.
func VerifyRoutes(tapName string, vmIP string) []string {
	var warnings []string

	out, err := exec.Command("route", "print").CombinedOutput()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to run 'route print': %v", err))
		return warnings
	}

	routeTable := string(out)

	// Check that the VM IP appears as a gateway in the routing table.
	if !strings.Contains(routeTable, vmIP) {
		warnings = append(warnings, fmt.Sprintf("VM IP %s not found in routing table", vmIP))
	}

	// Check for the two /1 routes (0.0.0.0/1 and 128.0.0.0/1) that
	// override the default route without replacing it.
	if !strings.Contains(routeTable, "0.0.0.0") {
		warnings = append(warnings, "default route 0.0.0.0 not found in routing table")
	}
	if !strings.Contains(routeTable, "128.0.0.0") {
		warnings = append(warnings, "128.0.0.0/1 route not found in routing table")
	}

	return warnings
}
