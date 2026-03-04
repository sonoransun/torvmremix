//go:build darwin

package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// verifyRoutes checks that macOS split routes (0.0.0.0/1 and 128.0.0.0/1)
// point to the expected VM gateway IP.
func verifyRoutes(expectedTAP string, expectedVMIP net.IP) error {
	out, err := exec.Command("netstat", "-rn").Output()
	if err != nil {
		return fmt.Errorf("netstat -rn: %w", err)
	}

	vmStr := expectedVMIP.String()
	lines := strings.Split(string(out), "\n")

	var found0, found128 bool
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		dest := fields[0]
		gateway := fields[1]

		switch dest {
		case "0/1":
			found0 = true
			if gateway != vmStr {
				return fmt.Errorf("route 0/1 points to %s, expected %s", gateway, vmStr)
			}
		case "128.0/1":
			found128 = true
			if gateway != vmStr {
				return fmt.Errorf("route 128.0/1 points to %s, expected %s", gateway, vmStr)
			}
		}
	}

	if !found0 {
		return fmt.Errorf("route 0/1 via %s not found", vmStr)
	}
	if !found128 {
		return fmt.Errorf("route 128.0/1 via %s not found", vmStr)
	}

	return nil
}
