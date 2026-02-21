//go:build linux

package platform

import "os"

func detectAccel() (AccelType, error) {
	f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
	if err != nil {
		return TCG, err
	}
	f.Close()
	return KVM, nil
}
