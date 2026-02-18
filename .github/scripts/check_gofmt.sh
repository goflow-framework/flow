#!/usr/bin/env bash
set -euo pipefail

# check_gofmt.sh
# Check formatting using gofmt for tracked .go files only.
# Exits non-zero with a list of files if formatting differences exist.

cd "${GITHUB_WORKSPACE:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}" || exit 0

# Find tracked .go files
files=$(git ls-files '*.go' || true)
if [ -z "$files" ]; then
  echo "No tracked .go files to check."
  exit 0
fi

# Run gofmt only on tracked files
out=$(printf "%s\n" $files | xargs -r gofmt -l || true)
if [ -n "$out" ]; then
  echo "gofmt found files that need formatting:" >&2
  printf "%s\n" "$out" >&2
  exit 1
fi

echo "All tracked .go files are formatted."
