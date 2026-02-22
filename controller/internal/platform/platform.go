package platform

import "fmt"

// AccelType represents a QEMU acceleration backend.
type AccelType string

const (
	KVM  AccelType = "kvm"
	HVF  AccelType = "hvf"
	WHPX AccelType = "whpx"
	TCG  AccelType = "tcg"
)

// Info holds detected platform capabilities.
type Info struct {
	Accel        AccelType
	VhostNet     bool // Linux: kernel vhost-net available for virtio-net
	IOMMUSupport bool // Linux: IOMMU (VT-d / AMD-Vi) available
}

// Detect probes the current platform for hardware virtualization
// capabilities including acceleration backend, vhost-net, and IOMMU.
// Falls back to TCG (software emulation) if no hardware accel is found.
func Detect() (*Info, error) {
	info, err := detect()
	if err != nil {
		return &Info{Accel: TCG}, nil
	}
	return info, nil
}

// DetectAccel probes the current platform for the best available
// hardware acceleration and returns an Info with the result.
// Falls back to TCG (software emulation) if no hardware accel is found.
//
// Deprecated: Use Detect() for full capability detection.
func DetectAccel() (*Info, error) {
	return Detect()
}

// ParseAccel converts a user-supplied string to an AccelType.
func ParseAccel(s string) (AccelType, error) {
	switch s {
	case "kvm":
		return KVM, nil
	case "hvf":
		return HVF, nil
	case "whpx":
		return WHPX, nil
	case "tcg":
		return TCG, nil
	default:
		return "", fmt.Errorf("unknown accelerator: %q", s)
	}
}
