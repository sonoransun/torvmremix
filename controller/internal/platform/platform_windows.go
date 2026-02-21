//go:build windows

package platform

import (
	"os/exec"
	"strings"
)

func detectAccel() (AccelType, error) {
	// Check for WHPX support by querying the Windows Hypervisor Platform
	// capability via systeminfo. A more reliable check would use the
	// WHvGetCapability API, but for simplicity we check if the
	// Hyper-V hypervisor is present via the systeminfo output.
	out, err := exec.Command("systeminfo").Output()
	if err == nil {
		info := strings.ToLower(string(out))
		if strings.Contains(info, "hypervisor has been detected") {
			return WHPX, nil
		}
	}

	return TCG, nil
}
