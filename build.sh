#!/usr/bin/env bash
set -euo pipefail

LDFLAGS="-s -w -extldflags '-static'"

mkdir -p dist

# build <os> <filename-arch> <go-arch> <cc>
build() {
  local os="$1" arch="$2" goarch="$3" cc="$4"
  local out="dist/openspec-atlas-${os}-${arch}"
  echo "building ${out}..."
  CGO_ENABLED=1 GOOS="${os}" GOARCH="${goarch}" CC="${cc}" \
    go build -ldflags="${LDFLAGS}" -o "${out}" ./cmd/openspec-atlas
  echo "done: ${out}"
}

build linux x86_64 amd64 gcc
build linux arm64  arm64 aarch64-linux-gnu-gcc
