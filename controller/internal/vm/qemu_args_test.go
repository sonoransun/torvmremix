package vm

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/logging"
)

func testInstance(cfg *config.Config) *Instance {
	logger, _ := logging.NewLogger(logging.Options{Verbose: false})
	return &Instance{
		Config:   cfg,
		Logger:   logger,
		QEMUPath: "/usr/bin/qemu-system-x86_64",
	}
}

func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Accel = "tcg"
	return cfg
}

func TestBuildArgsBasic(t *testing.T) {
	cfg := testConfig()
	inst := testInstance(cfg)

	args, err := inst.BuildArgs()
	if err != nil {
		t.Fatal(err)
	}

	// Check essential args are present.
	assertContains(t, args, "-name", "TorVM")
	assertContains(t, args, "-cpu", "qemu64")
	assertContains(t, args, "-accel", "tcg")
	assertContains(t, args, "-smp", fmt.Sprintf("%d", cfg.VMCPUs))
	assertContains(t, args, "-m", fmt.Sprintf("%d", cfg.VMMemoryMB))
	assertContains(t, args, "-kernel", cfg.KernelPath)
	assertContains(t, args, "-initrd", cfg.InitrdPath)
	assertArgPresent(t, args, "-nographic")
}

func TestBuildArgsCPUSelection(t *testing.T) {
	tests := []struct {
		accel   string
		wantCPU string
	}{
		{"tcg", "qemu64"},
		{"kvm", "host"},
		{"hvf", "host"},
		{"whpx", "host"},
		{"", "qemu64"}, // default is tcg
	}
	for _, tt := range tests {
		t.Run(tt.accel, func(t *testing.T) {
			cfg := testConfig()
			cfg.Accel = tt.accel
			inst := testInstance(cfg)
			args, err := inst.BuildArgs()
			if err != nil {
				t.Fatal(err)
			}
			assertContains(t, args, "-cpu", tt.wantCPU)
		})
	}
}

func TestBuildArgsSMPAndMemory(t *testing.T) {
	cfg := testConfig()
	cfg.VMCPUs = 4
	cfg.VMMemoryMB = 256
	inst := testInstance(cfg)

	args, err := inst.BuildArgs()
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, args, "-smp", "4")
	assertContains(t, args, "-m", "256")
}

func TestBuildArgsNullByteRejection(t *testing.T) {
	paths := []struct {
		name string
		set  func(*config.Config, string)
	}{
		{"KernelPath", func(c *config.Config, v string) { c.KernelPath = v }},
		{"InitrdPath", func(c *config.Config, v string) { c.InitrdPath = v }},
		{"StateDiskPath", func(c *config.Config, v string) { c.StateDiskPath = v }},
		{"QMPSocketPath", func(c *config.Config, v string) { c.QMPSocketPath = v }},
	}
	for _, tt := range paths {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			tt.set(cfg, "/tmp/evil\x00inject")
			inst := testInstance(cfg)

			_, err := inst.BuildArgs()
			if err == nil {
				t.Errorf("expected error for null byte in %s", tt.name)
			}
			if err != nil && !strings.Contains(err.Error(), "null byte") {
				t.Errorf("expected null byte error, got: %v", err)
			}
		})
	}
}

func TestMachineArgs(t *testing.T) {
	tests := []struct {
		accel string
		iommu bool
		want  string
	}{
		{"tcg", false, "q35"},
		{"kvm", false, "q35,kernel-irqchip=on"},
		{"kvm", true, "q35,kernel-irqchip=split"},
		{"hvf", false, "q35"},
		{"whpx", false, "q35"},
		{"", false, "q35"}, // defaults to tcg
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_iommu=%v", tt.accel, tt.iommu), func(t *testing.T) {
			cfg := testConfig()
			cfg.Accel = tt.accel
			cfg.IOMMUEnabled = tt.iommu
			got := machineArgs(cfg)
			if got != tt.want {
				t.Errorf("machineArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBlockArgsCacheMode(t *testing.T) {
	tests := []struct {
		accel     string
		wantCache string
	}{
		{"kvm", "cache=none,aio=native"},
		{"hvf", "cache=writeback,aio=threads"},
		{"whpx", "cache=writeback,aio=threads"},
		{"tcg", "cache=writeback"},
		{"", "cache=writeback"}, // default tcg
	}
	for _, tt := range tests {
		t.Run(tt.accel, func(t *testing.T) {
			cfg := testConfig()
			cfg.Accel = tt.accel
			args := blockArgs(cfg)
			// Find the -drive arg.
			driveArg := ""
			for i, a := range args {
				if a == "-drive" && i+1 < len(args) {
					driveArg = args[i+1]
					break
				}
			}
			if driveArg == "" {
				t.Fatal("no -drive arg found")
			}
			if !strings.Contains(driveArg, tt.wantCache) {
				t.Errorf("-drive = %q, want to contain %q", driveArg, tt.wantCache)
			}
		})
	}
}

func TestBlockArgsContainVirtioBlk(t *testing.T) {
	cfg := testConfig()
	args := blockArgs(cfg)
	assertContains(t, args, "-device", "virtio-blk-pci,drive=drive0")
}

func TestRngArgsPlatform(t *testing.T) {
	args := rngArgs()
	// Find the -object arg.
	objectArg := ""
	for i, a := range args {
		if a == "-object" && i+1 < len(args) {
			objectArg = args[i+1]
			break
		}
	}
	if objectArg == "" {
		t.Fatal("no -object arg found")
	}

	if runtime.GOOS == "windows" {
		if !strings.Contains(objectArg, "rng-builtin") {
			t.Errorf("expected rng-builtin on Windows, got %q", objectArg)
		}
	} else {
		if !strings.Contains(objectArg, "rng-random") {
			t.Errorf("expected rng-random on Unix, got %q", objectArg)
		}
		if !strings.Contains(objectArg, "/dev/urandom") {
			t.Errorf("expected /dev/urandom in rng args, got %q", objectArg)
		}
	}

	// Verify rate-limiting on virtio-rng device.
	assertContains(t, args, "-device", "virtio-rng-pci,rng=rng0,max-bytes=1024,period=1000")
}

func TestTapArgsDarwinVmnet(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-specific test")
	}
	cfg := testConfig()
	args := tapArgs(cfg)

	// Should use vmnet-shared on macOS.
	netdevArg := ""
	for i, a := range args {
		if a == "-netdev" && i+1 < len(args) {
			netdevArg = args[i+1]
			break
		}
	}
	if !strings.Contains(netdevArg, "vmnet-shared") {
		t.Errorf("expected vmnet-shared on darwin, got %q", netdevArg)
	}
}

func TestTapArgsLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific test")
	}
	cfg := testConfig()
	cfg.VhostNet = false
	args := tapArgs(cfg)

	netdevArg := ""
	for i, a := range args {
		if a == "-netdev" && i+1 < len(args) {
			netdevArg = args[i+1]
			break
		}
	}
	if !strings.HasPrefix(netdevArg, "tap,") {
		t.Errorf("expected tap netdev, got %q", netdevArg)
	}
	if strings.Contains(netdevArg, "vhost=on") {
		t.Error("vhost should not be enabled when VhostNet=false")
	}
}

func TestTapArgsLinuxVhost(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific test")
	}
	cfg := testConfig()
	cfg.VhostNet = true
	args := tapArgs(cfg)

	netdevArg := ""
	for i, a := range args {
		if a == "-netdev" && i+1 < len(args) {
			netdevArg = args[i+1]
			break
		}
	}
	if !strings.Contains(netdevArg, "vhost=on") {
		t.Errorf("expected vhost=on when VhostNet=true, got %q", netdevArg)
	}
}

func TestBuildArgsIOMMU(t *testing.T) {
	cfg := testConfig()
	cfg.Accel = "kvm"
	cfg.IOMMUEnabled = true
	inst := testInstance(cfg)

	args, err := inst.BuildArgs()
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, args, "-device", "intel-iommu,intremap=on,caching-mode=on")
}

func TestBuildArgsNoIOMMUWithoutKVM(t *testing.T) {
	cfg := testConfig()
	cfg.Accel = "tcg"
	cfg.IOMMUEnabled = true // should be ignored without kvm
	inst := testInstance(cfg)

	args, err := inst.BuildArgs()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range args {
		if strings.Contains(a, "intel-iommu") {
			t.Error("IOMMU device should not be present without KVM accel")
		}
	}
}

func TestBuildArgsQMPSocket(t *testing.T) {
	cfg := testConfig()
	inst := testInstance(cfg)

	args, err := inst.BuildArgs()
	if err != nil {
		t.Fatal(err)
	}

	qmpArg := ""
	for i, a := range args {
		if a == "-qmp" && i+1 < len(args) {
			qmpArg = args[i+1]
			break
		}
	}
	if qmpArg == "" {
		t.Fatal("no -qmp arg found")
	}

	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(qmpArg, "pipe:") {
			t.Errorf("expected pipe: on Windows, got %q", qmpArg)
		}
	} else {
		if !strings.HasPrefix(qmpArg, "unix:") {
			t.Errorf("expected unix: on Unix, got %q", qmpArg)
		}
	}
	if !strings.Contains(qmpArg, "server,nowait") {
		t.Errorf("expected server,nowait in QMP arg, got %q", qmpArg)
	}
}

func TestBuildArgsKernelAppend(t *testing.T) {
	cfg := testConfig()
	inst := testInstance(cfg)

	args, err := inst.BuildArgs()
	if err != nil {
		t.Fatal(err)
	}

	appendArg := ""
	for i, a := range args {
		if a == "-append" && i+1 < len(args) {
			appendArg = args[i+1]
			break
		}
	}
	if appendArg == "" {
		t.Fatal("no -append arg found")
	}

	// Verify kernel command line contains expected parameters.
	for _, substr := range []string{
		"IP=" + cfg.HostIP,
		"MASK=" + cfg.SubnetMask,
		"GW=" + cfg.VMIP,
		"PRIVIP=" + cfg.VMIP,
		fmt.Sprintf("CTLSOCK=%s:%d", cfg.VMIP, cfg.ControlPort),
		"ENTROPY=",
	} {
		if !strings.Contains(appendArg, substr) {
			t.Errorf("-append missing %q: %s", substr, appendArg)
		}
	}
}

func TestBuildArgsContainsVirtioBalloon(t *testing.T) {
	cfg := testConfig()
	inst := testInstance(cfg)

	args, err := inst.BuildArgs()
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, args, "-device", "virtio-balloon-pci")
}

// assertContains checks that args contains a consecutive pair of flag and value.
func assertContains(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("args missing %s %s", flag, value)
}

// assertArgPresent checks that a standalone arg is present.
func assertArgPresent(t *testing.T, args []string, arg string) {
	t.Helper()
	for _, a := range args {
		if a == arg {
			return
		}
	}
	t.Errorf("args missing %s", arg)
}
