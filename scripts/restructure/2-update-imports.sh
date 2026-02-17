#!/usr/bin/env bash
# 2-update-imports.sh
# Update Go import paths using find + sed (GNU/BSD compatible).
# Run from repo root.

set -euo pipefail
IFS=$'\n\t'

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
BACKEND="$REPO_ROOT/backend"

if [ ! -d "$BACKEND" ]; then
  echo "backend/ not found at $BACKEND" >&2
  exit 1
fi

sedi() {
  local expr="$1"
  local file="$2"
  if sed --version >/dev/null 2>&1; then
    sed -i'' -e "$expr" "$file"
  else
    sed -i '' -e "$expr" "$file"
  fi
}

echo "Updating imports under $BACKEND"

while IFS= read -r -d '' file; do
  # More specific replacements first
  sedi 's@internal/clients/clouddrive2/proto@proto@g' "$file"
  sedi 's@internal/service/dataserver@internal/dataserver@g' "$file"
  sedi 's@internal/service/mediaserver@internal/mediaserver@g' "$file"

  sedi 's@internal/clients/clouddrive2@internal/dataserver@g' "$file"
  sedi 's@internal/database@internal@g' "$file"
  sedi 's@internal/config@internal@g' "$file"
  sedi 's@internal/handlers@internal/handler@g' "$file"
  sedi 's@internal/utils@internal/tools@g' "$file"

  # Flattened service packages
  sedi 's@internal/service/executor@internal/service@g' "$file"
  sedi 's@internal/service/filemonitor@internal/service@g' "$file"
  sedi 's@internal/service/job@internal/service@g' "$file"
  sedi 's@internal/service/planner@internal/service@g' "$file"
  sedi 's@internal/service/strm@internal/service@g' "$file"
  sedi 's@internal/service/taskrun@internal/service@g' "$file"
  sedi 's@internal/service/types@internal/service@g' "$file"
done < <(find "$BACKEND" -type f -name "*.go" -print0)

echo "OK"
