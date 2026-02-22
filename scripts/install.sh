#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY="torvm-${OS}-${ARCH}"
INSTALL_DIR="/usr/local/bin"
VM_DIR="/usr/local/share/torvm"

if [ ! -f "$PROJECT_ROOT/dist/controller/$BINARY" ]; then
  echo "Error: Controller binary not found: dist/controller/$BINARY"
  echo "Run 'make' first to build."
  exit 1
fi

echo "Installing TorVM..."
echo "  Binary: $INSTALL_DIR/torvm"
sudo install -m 755 "$PROJECT_ROOT/dist/controller/$BINARY" "$INSTALL_DIR/torvm"

echo "  VM artifacts: $VM_DIR/"
sudo mkdir -p "$VM_DIR"
sudo install -m 644 "$PROJECT_ROOT/dist/vm/vmlinuz" "$VM_DIR/"
sudo install -m 644 "$PROJECT_ROOT/dist/vm/initramfs.gz" "$VM_DIR/"

if [ -f "$PROJECT_ROOT/dist/vm/state.img" ]; then
  sudo install -m 644 "$PROJECT_ROOT/dist/vm/state.img" "$VM_DIR/"
fi

# macOS: install launchd plist and create log directory.
if [ "$OS" = "darwin" ]; then
  echo "  launchd plist: /Library/LaunchDaemons/org.torproject.torvm.plist"
  sudo cp "$PROJECT_ROOT/installer/macos/torvm.plist" /Library/LaunchDaemons/org.torproject.torvm.plist
  sudo chmod 644 /Library/LaunchDaemons/org.torproject.torvm.plist

  echo "  Log directory: /var/log/torvm/"
  sudo mkdir -p /var/log/torvm
fi

echo "Installation complete."
