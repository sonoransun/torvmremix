//go:build darwin

package platform

import (
	"os/exec"
	"strings"
)

func detectAccel() (AccelType, error) {
	out, err := exec.Command("sysctl", "-n", "kern.hv_support").Output()
	if err != nil {
		return TCG, err
	}
	if strings.TrimSpace(string(out)) == "1" {
		return HVF, nil
	}
	return TCG, nil
}
