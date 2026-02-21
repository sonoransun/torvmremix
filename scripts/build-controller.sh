#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT/controller"

echo "Building controller..."
PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

for platform in $PLATFORMS; do
  OS="${platform%/*}"
  ARCH="${platform#*/}"
  EXT=""
  [ "$OS" = "windows" ] && EXT=".exe"
  echo "  Building torvm-${OS}-${ARCH}${EXT}..."
  GOOS=$OS GOARCH=$ARCH go build -o "../dist/controller/torvm-${OS}-${ARCH}${EXT}" ./cmd/torvm/
done

echo "Controller binaries in dist/controller/"
