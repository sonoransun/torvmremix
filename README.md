# TorVM - Transparent Tor Proxy Virtual Machine

TorVM routes all host network traffic through the Tor network transparently using a lightweight Linux virtual machine. The host communicates through a TAP adapter to an Alpine Linux VM running inside QEMU, which applies iptables rules to redirect all TCP and DNS traffic through Tor's transparent proxy.

**WARNING**: This is experimental software. Do NOT rely on it for privacy-sensitive purposes without thorough review.

## Architecture

TorVM uses a three-layer architecture:

```
Host (torvm controller)
    | TAP adapter (10.10.10.2)
QEMU VM running Alpine Linux (10.10.10.1)
    | iptables transparent proxy
Tor network
```

1. **Host Controller** -- A Go binary that manages the VM lifecycle: creates TAP adapters, launches QEMU, monitors health, and handles graceful shutdown.
2. **Alpine Linux VM** -- A minimal Linux kernel with initramfs that runs Tor and applies iptables rules for transparent proxying. All traffic arriving on the TAP interface is redirected through Tor.
3. **Tor Network** -- TCP connections are transparently proxied via Tor's TransPort. DNS queries are resolved through Tor's DNSPort.

## Prerequisites

- **Docker** -- Required to build the VM image (multistage build)
- **Go 1.22+** -- Required to build the controller
- **QEMU 8.0+** -- Required at runtime to run the VM

## Building

```bash
# Build everything (VM image + controller binaries)
make

# Build only the VM image
make vm

# Build only the controller (cross-compiled for all platforms)
make controller

# Install to /usr/local (Linux/macOS)
sudo make install

# Clean build artifacts
make clean
```

The VM build uses Docker to produce `dist/vm/vmlinuz`, `dist/vm/initramfs.gz`, and `dist/vm/state.img`.

The controller build cross-compiles for: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64. Binaries are placed in `dist/controller/`.

## Usage

### Linux

```bash
# Run directly
sudo torvm

# Run headless (no UI)
sudo torvm --headless

# Install as systemd service
sudo cp installer/linux/torvm.service /etc/systemd/system/
sudo systemctl enable --now torvm
```

### macOS

```bash
# Run directly
sudo torvm

# Install as launchd service
sudo cp installer/macos/torvm.plist /Library/LaunchDaemons/org.torproject.torvm.plist
sudo launchctl load /Library/LaunchDaemons/org.torproject.torvm.plist
```

### Windows

```powershell
# Run directly (requires Administrator)
torvm.exe

# Or install via MSI (built from installer/windows/torvm.wxs)
```

## Network Topology

| Interface | IP Address | Role |
|---|---|---|
| TAP adapter (Host) | 10.10.10.2 | Host endpoint |
| VM gateway (Alpine) | 10.10.10.1 | Transparent proxy gateway |
| Tor TransPort | 10.10.10.1:9095 | Transparent TCP proxy |
| Tor DNSPort | 10.10.10.1:9093 | DNS resolution via Tor |

## Security Model

The VM provides network-level isolation between the host and Tor:

- The host has no direct internet access; all traffic must pass through the VM
- The VM runs a minimal Alpine Linux with only Tor and supporting services
- iptables rules enforce that all TCP traffic is redirected to Tor's TransPort
- DNS queries are intercepted and resolved through Tor's DNSPort
- The controller runs with elevated privileges only for TAP adapter setup

## Directory Structure

```
extorvm/
  controller/       Go controller application
    cmd/torvm/       Main entry point
    internal/        Internal packages (vm, network, config, health)
  vm/
    alpine/          Alpine Linux VM configuration
    scripts/         VM build helper scripts
  installer/
    linux/           systemd service unit
    macos/           launchd plist
    windows/         WiX MSI installer
  scripts/           Build scripts
  dist/              Build output (generated)
  legacy/            Original 2008-2009 codebase (historical reference)
  doc/               Documentation
```

## Legacy Code

The `legacy/` directory contains the original 2008-2009 TorVM codebase from The Tor Project, Inc., preserved for historical reference. It targeted Windows with OpenWRT and a C controller. The modern version replaces this with Alpine Linux, a Go controller, and cross-platform support.

## License

See the [LICENSE](LICENSE) file for rights and terms.

Software Copyright (C) 2008-2009 The Tor Project, Inc.
