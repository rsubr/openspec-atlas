#!/usr/bin/env bash
set -euo pipefail

OUTPUT="structure.json"
ALL=""
DIRS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)    OUTPUT="$2"; shift 2 ;;
    -all)  ALL="-all"; shift ;;
    *)     DIRS+=("$1"); shift ;;
  esac
done

if [[ ${#DIRS[@]} -eq 0 ]]; then
  DIRS=(".")
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
[[ "$ARCH" == "aarch64" ]] && ARCH="arm64"

BINARY="./dist/openspec-atlas-${OS}-${ARCH}"
if [[ ! -x "$BINARY" ]]; then
  echo "error: no binary found for ${OS}/${ARCH} at ${BINARY}" >&2
  exit 1
fi

"$BINARY" -o "$OUTPUT" $ALL "${DIRS[@]}"
