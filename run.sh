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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION_FILE="${SCRIPT_DIR}/VERSION"
if [[ -r "$VERSION_FILE" ]]; then
  VERSION="$(cat "$VERSION_FILE")"
else
  VERSION=""
fi

# Prefer the versioned binary produced by build.sh, but fall back to an
# unversioned one if a user has symlinked it into ./dist.
BINARY=""
for candidate in \
  "${SCRIPT_DIR}/dist/openspec-atlas-${OS}-${ARCH}-v${VERSION}" \
  "${SCRIPT_DIR}/dist/openspec-atlas-${OS}-${ARCH}"; do
  if [[ -x "$candidate" ]]; then
    BINARY="$candidate"
    break
  fi
done

if [[ -z "$BINARY" ]]; then
  echo "error: no binary found for ${OS}/${ARCH} in ${SCRIPT_DIR}/dist/" >&2
  exit 1
fi

exec "$BINARY" -o "$OUTPUT" ${ALL:+$ALL} "${DIRS[@]}"
