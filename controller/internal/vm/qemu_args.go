package vm

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/security"
)

// BuildArgs constructs the QEMU command-line arguments from the
// instance configuration. Ported from the legacy buildcmdline() and
// launchtorvm() functions in torvm.c.
func (inst *Instance) BuildArgs() ([]string, error) {
	cfg := inst.Config

	// Reject paths containing null bytes to prevent injection.
	for _, pair := range []struct{ name, path string }{
		{"KernelPath", cfg.KernelPath},
		{"InitrdPath", cfg.InitrdPath},
		{"StateDiskPath", cfg.StateDiskPath},
		{"QMPSocketPath", cfg.QMPSocketPath},
	} {
		if strings.Contains(pair.path, "\x00") {
			return nil, fmt.Errorf("%s contains null byte", pair.name)
		}
	}

	accel := cfg.Accel
	if accel == "" {
		accel = "tcg"
	}

	cpu := "host"
	if accel == "tcg" {
		cpu = "qemu64"
	}

	entropy, err := security.EntropyHexString(32)
	if err != nil {
		return nil, fmt.Errorf("generate entropy: %w", err)
	}

	kernelAppend := fmt.Sprintf(
		"quiet IP=%s MASK=%s GW=%s MTU=1500 PRIVIP=%s CTLSOCK=%s:%d ENTROPY=%s",
		cfg.HostIP,
		cfg.SubnetMask,
		cfg.VMIP,
		cfg.VMIP,
		cfg.VMIP,
		cfg.ControlPort,
		entropy,
	)

	args := []string{
		"-name", "TorVM",
		"-machine", "q35",
		"-cpu", cpu,
		"-accel", accel,
		"-m", fmt.Sprintf("%d", cfg.VMMemoryMB),
		"-kernel", cfg.KernelPath,
		"-initrd", cfg.InitrdPath,
		"-append", kernelAppend,
		"-drive", fmt.Sprintf("file=%s,if=virtio,format=raw", cfg.StateDiskPath),
		"-nographic",
	}

	// Network device: platform-specific TAP configuration.
	args = append(args, tapArgs(cfg)...)

	// QMP monitor socket.
	if runtime.GOOS == "windows" {
		// Named pipe on Windows.
		args = append(args,
			"-qmp", fmt.Sprintf("pipe:%s,server,nowait", cfg.QMPSocketPath),
		)
	} else {
		args = append(args,
			"-qmp", fmt.Sprintf("unix:%s,server,nowait", cfg.QMPSocketPath),
		)
	}

	return args, nil
}

func tapArgs(cfg *config.Config) []string {
	if runtime.GOOS == "darwin" {
		// On macOS, use vmnet-shared for networking.
		return []string{
			"-netdev", fmt.Sprintf("vmnet-shared,id=net0,start-address=%s,end-address=%s,subnet-mask=%s",
				cfg.VMIP, cfg.HostIP, cfg.SubnetMask),
			"-device", "virtio-net-pci,netdev=net0",
		}
	}

	// Linux and Windows use TAP devices.
	return []string{
		"-netdev", fmt.Sprintf("tap,id=net0,ifname=%s,script=no,downscript=no", cfg.TAPName),
		"-device", "virtio-net-pci,netdev=net0",
	}
}
