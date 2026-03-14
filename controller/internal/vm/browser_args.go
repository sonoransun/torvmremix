package vm

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/security"
)

// BuildBrowserArgs constructs QEMU command-line arguments for the
// hardened Chromium browser VM with maximum device reduction and
// network isolation via user-mode networking.
func BuildBrowserArgs(cfg *config.Config) ([]string, error) {
	bcfg := &cfg.Browser

	// Reject paths containing null bytes.
	for _, pair := range []struct{ name, path string }{
		{"Browser.KernelPath", bcfg.KernelPath},
		{"Browser.InitrdPath", bcfg.InitrdPath},
		{"Browser.StateDiskPath", bcfg.StateDiskPath},
		{"Browser.QMPSocketPath", bcfg.QMPSocketPath},
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
		if cfg.Entropy.ExposeRDRAND {
			cpu = "qemu64,+rdrand"
		}
	}

	entropy, err := security.EntropyHexString(64)
	if err != nil {
		return nil, fmt.Errorf("generate entropy: %w", err)
	}

	// Kernel command line passes Tor VM connection info and security settings.
	kernelAppend := fmt.Sprintf(
		"quiet TORIP=%s SOCKSPORT=%d DNSPORT=%d ENTROPY=%s CANARY_INTERVAL=%d",
		cfg.VMIP,
		cfg.SOCKSPort,
		cfg.DNSPort,
		entropy,
		bcfg.CanaryIntervalSec,
	)
	if bcfg.HoneyTokens {
		kernelAppend += " HONEY_TOKENS=1"
	}
	if bcfg.AutoRemediate {
		kernelAppend += " AUTO_REMEDIATE=1"
	}

	machine := "q35"
	if accel == "kvm" {
		machine = "q35,kernel-irqchip=on"
	}

	args := []string{
		"-name", "BrowserVM",
		"-nodefaults",
		"-machine", machine,
		"-cpu", cpu,
		"-accel", accel,
		"-smp", fmt.Sprintf("%d", bcfg.VMCPUs),
		"-m", fmt.Sprintf("%d", bcfg.VMMemoryMB),
		"-kernel", bcfg.KernelPath,
		"-initrd", bcfg.InitrdPath,
		"-append", kernelAppend,
	}

	// State disk with platform-optimized caching.
	args = append(args, browserBlockArgs(bcfg, accel)...)

	// User-mode networking: restrict=on blocks host-initiated connections.
	// The guest can only make outbound TCP to the Tor VM SOCKS port.
	args = append(args,
		"-netdev", "user,id=net0,restrict=on",
		"-device", "virtio-net-pci,netdev=net0",
	)

	// Virtio entropy device.
	args = append(args, browserRngArgs()...)

	// Virtio memory balloon.
	args = append(args, "-device", "virtio-balloon-pci")

	// Virtio-serial for security event channel (secwatchd → host).
	secwatchSocket := browserSecwatchSocketPath(bcfg)
	args = append(args,
		"-device", "virtio-serial-pci",
		"-chardev", fmt.Sprintf("socket,id=secwatch,path=%s,server=on,wait=off", secwatchSocket),
		"-device", "virtserialport,chardev=secwatch,name=com.torvm.secwatch",
	)

	// VNC display (localhost only) or headless.
	if bcfg.VNCDisplay > 0 {
		args = append(args, "-vnc", fmt.Sprintf("127.0.0.1:%d", bcfg.VNCDisplay))
	} else {
		args = append(args, "-nographic")
	}

	// QMP monitor socket.
	if runtime.GOOS == "windows" {
		args = append(args,
			"-qmp", fmt.Sprintf("pipe:%s,server,nowait", bcfg.QMPSocketPath),
		)
	} else {
		args = append(args,
			"-qmp", fmt.Sprintf("unix:%s,server,nowait", bcfg.QMPSocketPath),
		)
	}

	return args, nil
}

// browserBlockArgs returns QEMU block device arguments for the browser VM state disk.
func browserBlockArgs(bcfg *config.BrowserConfig, accel string) []string {
	var driveOpts string
	switch accel {
	case "kvm":
		driveOpts = fmt.Sprintf(
			"file=%s,id=drive0,if=none,format=raw,cache=none,aio=native",
			bcfg.StateDiskPath,
		)
	case "hvf", "whpx":
		driveOpts = fmt.Sprintf(
			"file=%s,id=drive0,if=none,format=raw,cache=writeback,aio=threads",
			bcfg.StateDiskPath,
		)
	default:
		driveOpts = fmt.Sprintf(
			"file=%s,id=drive0,if=none,format=raw,cache=writeback",
			bcfg.StateDiskPath,
		)
	}
	return []string{
		"-drive", driveOpts,
		"-device", "virtio-blk-pci,drive=drive0",
	}
}

// browserRngArgs returns virtio-rng arguments for the browser VM.
func browserRngArgs() []string {
	var rngBackend string
	if runtime.GOOS == "windows" {
		rngBackend = "rng-builtin,id=rng0"
	} else {
		rngBackend = "rng-random,id=rng0,filename=/dev/urandom"
	}
	return []string{
		"-object", rngBackend,
		"-device", "virtio-rng-pci,rng=rng0,max-bytes=1024,period=1000",
	}
}

// browserSecwatchSocketPath returns the Unix socket path for the secwatch
// virtio-serial channel.
func browserSecwatchSocketPath(bcfg *config.BrowserConfig) string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\torvm-secwatch`
	}
	return "/run/torvm/secwatch.sock"
}

// SecwatchSocketPath returns the host-side socket path for the security
// event monitor to connect to.
func SecwatchSocketPath() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\torvm-secwatch`
	}
	return "/run/torvm/secwatch.sock"
}
