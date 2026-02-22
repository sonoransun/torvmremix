package vm

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/security"
)

// BuildArgs constructs the QEMU command-line arguments from the
// instance configuration, applying platform-specific optimizations
// for maximum virtualization performance.
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

	// Machine type with platform-specific optimizations.
	machine := machineArgs(cfg)

	args := []string{
		"-name", "TorVM",
		"-machine", machine,
		"-cpu", cpu,
		"-accel", accel,
		"-smp", fmt.Sprintf("%d", cfg.VMCPUs),
		"-m", fmt.Sprintf("%d", cfg.VMMemoryMB),
		"-kernel", cfg.KernelPath,
		"-initrd", cfg.InitrdPath,
		"-append", kernelAppend,
	}

	// Block device: explicit virtio-blk-pci with optimized caching.
	args = append(args, blockArgs(cfg)...)

	// IOMMU device (VT-d) when supported with KVM.
	if cfg.IOMMUEnabled && accel == "kvm" {
		args = append(args,
			"-device", "intel-iommu,intremap=on,caching-mode=on",
		)
	}

	// Virtio entropy device: high-quality RNG from host.
	args = append(args, rngArgs()...)

	// Virtio memory balloon for dynamic memory management.
	args = append(args, "-device", "virtio-balloon-pci")

	args = append(args, "-nographic")

	// Network device: platform-specific TAP with vhost acceleration.
	args = append(args, tapArgs(cfg)...)

	// QMP monitor socket.
	if runtime.GOOS == "windows" {
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

// machineArgs returns the -machine argument value with platform-specific
// optimizations for interrupt handling.
func machineArgs(cfg *config.Config) string {
	accel := cfg.Accel
	if accel == "" {
		accel = "tcg"
	}

	switch accel {
	case "kvm":
		if cfg.IOMMUEnabled {
			// IOMMU requires split irqchip: kernel handles LAPIC,
			// QEMU handles IOAPIC with interrupt remapping through
			// the virtual IOMMU for secure interrupt delivery.
			return "q35,kernel-irqchip=split"
		}
		// Offload full interrupt controller to KVM for lowest latency.
		return "q35,kernel-irqchip=on"
	default:
		return "q35"
	}
}

// blockArgs returns QEMU arguments for the state disk using an explicit
// virtio-blk-pci device with optimized cache and I/O settings.
func blockArgs(cfg *config.Config) []string {
	accel := cfg.Accel
	if accel == "" {
		accel = "tcg"
	}

	var driveOpts string
	switch accel {
	case "kvm":
		// Direct I/O with kernel-level async I/O bypasses host page
		// cache for lowest latency and avoids double-caching.
		driveOpts = fmt.Sprintf(
			"file=%s,id=drive0,if=none,format=raw,cache=none,aio=native",
			cfg.StateDiskPath,
		)
	case "hvf", "whpx":
		// Thread-based AIO with writeback cache; native AIO not
		// available on macOS/Windows.
		driveOpts = fmt.Sprintf(
			"file=%s,id=drive0,if=none,format=raw,cache=writeback,aio=threads",
			cfg.StateDiskPath,
		)
	default:
		// TCG: safe defaults.
		driveOpts = fmt.Sprintf(
			"file=%s,id=drive0,if=none,format=raw,cache=writeback",
			cfg.StateDiskPath,
		)
	}

	return []string{
		"-drive", driveOpts,
		"-device", "virtio-blk-pci,drive=drive0",
	}
}

// rngArgs returns QEMU arguments for a virtio-rng entropy device backed
// by the host's random number generator. This provides high-quality
// entropy to the VM for Tor's cryptographic operations without relying
// on slow kernel command-line seeding alone.
func rngArgs() []string {
	var rngBackend string
	if runtime.GOOS == "windows" {
		// Windows: use QEMU's built-in PRNG (backed by CryptGenRandom).
		rngBackend = "rng-builtin,id=rng0"
	} else {
		// Linux/macOS: read directly from host /dev/urandom.
		rngBackend = "rng-random,id=rng0,filename=/dev/urandom"
	}

	return []string{
		"-object", rngBackend,
		"-device", "virtio-rng-pci,rng=rng0,max-bytes=1024,period=1000",
	}
}

// tapArgs returns QEMU arguments for the network device with
// platform-specific optimizations including vhost-net acceleration.
func tapArgs(cfg *config.Config) []string {
	if runtime.GOOS == "darwin" {
		// On macOS, use vmnet-shared for networking.
		return []string{
			"-netdev", fmt.Sprintf(
				"vmnet-shared,id=net0,start-address=%s,end-address=%s,subnet-mask=%s",
				cfg.VMIP, cfg.HostIP, cfg.SubnetMask),
			"-device", "virtio-net-pci,netdev=net0",
		}
	}

	// Linux and Windows use TAP devices.
	netdev := fmt.Sprintf("tap,id=net0,ifname=%s,script=no,downscript=no", cfg.TAPName)

	// Enable vhost-net on Linux when the kernel module is available.
	// vhost-net moves virtio packet processing into the kernel,
	// eliminating QEMU userspace overhead for each packet.
	if cfg.VhostNet && runtime.GOOS == "linux" {
		netdev += ",vhost=on"
	}

	return []string{
		"-netdev", netdev,
		"-device", "virtio-net-pci,netdev=net0",
	}
}
