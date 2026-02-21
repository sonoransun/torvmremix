//go:build linux

package platform

import (
	"os"
	"syscall"
)

func detectAccel() (AccelType, error) {
	fi, err := os.Stat("/dev/kvm")
	if err != nil {
		return TCG, err
	}

	// Check that we can actually open it (have permissions).
	f, err := os.OpenFile("/dev/kvm", syscall.O_RDWR, 0)
	if err != nil {
		_ = fi
		return TCG, err
	}
	f.Close()
	return KVM, nil
}
