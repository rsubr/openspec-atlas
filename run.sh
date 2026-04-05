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

./dist/openspec-atlas -o "$OUTPUT" $ALL "${DIRS[@]}"
