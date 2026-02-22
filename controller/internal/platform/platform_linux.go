//go:build linux

package platform

import "os"

func detect() (*Info, error) {
	info := &Info{Accel: TCG}

	// Detect KVM hardware acceleration.
	if f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0); err == nil {
		f.Close()
		info.Accel = KVM
	}

	// Detect vhost-net kernel module for accelerated virtio networking.
	// When available, QEMU delegates packet processing to the kernel
	// vhost-net driver, bypassing userspace for significant throughput gains.
	if _, err := os.Stat("/dev/vhost-net"); err == nil {
		info.VhostNet = true
	}

	// Detect IOMMU (Intel VT-d / AMD-Vi) support.
	// If IOMMU groups exist, the kernel has IOMMU enabled and devices
	// can be isolated for secure DMA and interrupt remapping.
	entries, err := os.ReadDir("/sys/kernel/iommu_groups")
	if err == nil && len(entries) > 0 {
		info.IOMMUSupport = true
	}

	return info, nil
}

func detectAccel() (AccelType, error) {
	info, _ := detect()
	return info.Accel, nil
}
