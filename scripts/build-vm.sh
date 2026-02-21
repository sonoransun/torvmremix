#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo "Building VM image..."
docker build --target output --output type=local,dest=./dist/vm ./vm
echo "VM artifacts in dist/vm/: vmlinuz, initramfs.gz, state.img"
