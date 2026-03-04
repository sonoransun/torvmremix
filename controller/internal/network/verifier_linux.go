//go:build linux

package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// verifyRoutes checks that the Linux default route goes through the
// expected TAP device and VM gateway IP.
func verifyRoutes(expectedTAP string, expectedVMIP net.IP) error {
	out, err := exec.Command("ip", "route", "show").Output()
	if err != nil {
		return fmt.Errorf("ip route show: %w", err)
	}

	vmStr := expectedVMIP.String()
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// Look for: default via <IP> dev <DEV> ...
		if fields[0] != "default" {
			continue
		}

		var gateway, dev string
		for i := 1; i < len(fields)-1; i++ {
			switch fields[i] {
			case "via":
				gateway = fields[i+1]
			case "dev":
				dev = fields[i+1]
			}
		}

		if gateway == vmStr && dev == expectedTAP {
			return nil
		}
	}

	return fmt.Errorf("no default route via %s dev %s found", vmStr, expectedTAP)
}
