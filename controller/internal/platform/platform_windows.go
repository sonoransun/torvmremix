//go:build windows

package platform

import (
	"os/exec"
	"strings"
)

func detect() (*Info, error) {
	info := &Info{Accel: TCG}

	out, err := exec.Command("systeminfo").Output()
	if err == nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "hypervisor has been detected") {
			info.Accel = WHPX
		}
	}

	// Windows: no vhost-net kernel module.
	// Windows: no IOMMU passthrough in QEMU (WHPX does not expose IOMMU).

	return info, nil
}

func detectAccel() (AccelType, error) {
	info, _ := detect()
	return info.Accel, nil
}
