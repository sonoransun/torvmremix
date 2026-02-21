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
	Accel AccelType
}

// DetectAccel probes the current platform for the best available
// hardware acceleration and returns an Info with the result.
// Falls back to TCG (software emulation) if no hardware accel is found.
func DetectAccel() (*Info, error) {
	accel, err := detectAccel()
	if err != nil {
		return &Info{Accel: TCG}, nil
	}
	return &Info{Accel: accel}, nil
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
