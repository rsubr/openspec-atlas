#!/usr/bin/env bash
set -euo pipefail

BINARY="dist/openspec-atlas"
LDFLAGS="-s -w"

# Linux supports fully static binaries via -extldflags; macOS does not.
if [[ "$(uname -s)" == "Linux" ]]; then
  LDFLAGS="${LDFLAGS} -extldflags '-static'"
fi

mkdir -p dist
echo "building openspec-atlas..."
CGO_ENABLED=1 go build -ldflags="${LDFLAGS}" -o "${BINARY}" .
echo "done: ${BINARY}"
