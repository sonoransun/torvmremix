#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== TorVM Build ==="
echo ""

"${SCRIPT_DIR}/build-vm.sh"
echo ""
"${SCRIPT_DIR}/build-controller.sh"

echo ""
echo "=== Build Complete ==="
echo "VM artifacts:         dist/vm/"
echo "Controller binaries:  dist/controller/"
