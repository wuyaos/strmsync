#!/usr/bin/env bash
# 3-cleanup.sh
# Remove empty directories and temp folders.
# Run from repo root.

set -euo pipefail
IFS=$'\n\t'

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
BACKEND="$REPO_ROOT/backend"

if [ ! -d "$BACKEND" ]; then
  echo "backend/ not found at $BACKEND" >&2
  exit 1
fi

echo "Removing temp dirs"
find "$BACKEND" -maxdepth 1 -type d -name ".tmp-restructure-*" -print -exec rm -rf {} \;

echo "Removing empty directories under backend/"
find "$BACKEND" -type d -empty -print -delete

echo "OK"
