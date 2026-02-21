#!/bin/sh
# Extract VM image artifacts from Docker build.
# Usage: ./extract-vmimage.sh [output_dir]
#
# Builds the VM Docker image and copies vmlinuz, initramfs.gz,
# and state.img to the output directory (default: dist/vm/).

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VM_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_DIR="$(cd "$VM_DIR/.." && pwd)"
OUTPUT_DIR="${1:-$PROJECT_DIR/dist/vm}"

echo "Building VM image..."
docker build --target output -o "type=local,dest=$OUTPUT_DIR" "$VM_DIR"

echo ""
echo "VM image artifacts extracted to: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"/vmlinuz "$OUTPUT_DIR"/initramfs.gz "$OUTPUT_DIR"/state.img
