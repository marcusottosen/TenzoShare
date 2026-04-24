#!/usr/bin/env bash
# generate-proto.sh — regenerates all Go and TypeScript code from .proto files.
# Run from the repository root: ./scripts/generate-proto.sh
# Requires: buf CLI (https://buf.build/docs/installation)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$REPO_ROOT/proto"

echo "==> Checking buf CLI..."
if ! command -v buf &>/dev/null; then
  echo "ERROR: 'buf' is not installed. Install it from https://buf.build/docs/installation"
  exit 1
fi
echo "    buf version: $(buf --version)"

echo ""
echo "==> Linting proto files..."
cd "$PROTO_DIR"
buf lint

echo ""
echo "==> Checking for breaking changes against HEAD..."
buf breaking --against ".git#branch=main" 2>/dev/null || true

echo ""
echo "==> Generating Go code..."
buf generate

echo ""
echo "==> Done. Generated files are in proto/gen/"
