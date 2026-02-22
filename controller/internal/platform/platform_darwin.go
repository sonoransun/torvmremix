//go:build darwin

package platform

import (
	"os/exec"
	"strings"
)

func detect() (*Info, error) {
	info := &Info{Accel: TCG}

	out, err := exec.Command("sysctl", "-n", "kern.hv_support").Output()
	if err == nil && strings.TrimSpace(string(out)) == "1" {
		info.Accel = HVF
	}

	// macOS: no vhost-net kernel module.
	// macOS: no IOMMU passthrough in QEMU (Hypervisor.framework provides
	// its own device model isolation).

	return info, nil
}

func detectAccel() (AccelType, error) {
	info, _ := detect()
	return info.Accel, nil
}
