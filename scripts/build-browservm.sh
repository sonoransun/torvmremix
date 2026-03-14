#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

echo "Building Browser VM image ..."
docker build --target output --output type=local,dest=./dist/browservm ./browservm

echo "Browser VM artifacts:"
ls -lh dist/browservm/
echo "Done."
