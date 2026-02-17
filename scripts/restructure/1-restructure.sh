#!/usr/bin/env bash
# 1-restructure.sh
# Restructure backend/ using git mv with safety checks and temp dirs.
# Run from repo root.

set -euo pipefail
IFS=$'\n\t'

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
BACKEND="$REPO_ROOT/backend"
TMP="$BACKEND/.tmp-restructure-$$"

if [ ! -d "$BACKEND" ]; then
  echo "backend/ not found at $BACKEND" >&2
  exit 1
fi

if [ -e "$BACKEND/main.go" ]; then
  echo "Target exists: $BACKEND/main.go" >&2
  exit 1
fi

if [ -e "$BACKEND/proto" ]; then
  echo "Target exists: $BACKEND/proto" >&2
  exit 1
fi

if [ -e "$BACKEND/internal/store" ]; then
  echo "Target exists: $BACKEND/internal/store" >&2
  exit 1
fi

if [ -e "$BACKEND/internal/handler" ]; then
  echo "Target exists: $BACKEND/internal/handler" >&2
  exit 1
fi

if [ -e "$BACKEND/internal/dataserver" ]; then
  echo "Target exists: $BACKEND/internal/dataserver" >&2
  exit 1
fi

if [ -e "$BACKEND/internal/mediaserver" ]; then
  echo "Target exists: $BACKEND/internal/mediaserver" >&2
  exit 1
fi

if [ -e "$TMP" ]; then
  echo "Temp dir exists: $TMP" >&2
  exit 1
fi

git_mv() {
  local src="$1"
  local dst="$2"
  if [ ! -e "$src" ]; then
    echo "Source missing: $src" >&2
    exit 1
  fi
  if [ -e "$dst" ]; then
    echo "Target exists: $dst" >&2
    exit 1
  fi
  git mv "$src" "$dst"
  local rc=$?
  if [ $rc -ne 0 ]; then
    echo "git mv failed: $src -> $dst" >&2
    exit $rc
  fi
}

echo "Step 1: create temp dir"
mkdir -p "$TMP"
echo "OK"

echo "Step 2: move main.go"
git_mv "$BACKEND/cmd/server/main.go" "$BACKEND/main.go"
echo "OK"

echo "Step 3: move database files to internal/"
git_mv "$BACKEND/internal/database/database.go" "$BACKEND/internal/database.go"
git_mv "$BACKEND/internal/database/models.go" "$BACKEND/internal/models.go"
if [ -f "$BACKEND/internal/database/repository.go" ]; then
  git_mv "$BACKEND/internal/database/repository.go" "$BACKEND/internal/repository.go"
fi
echo "OK"

echo "Step 4: move config to internal/"
git_mv "$BACKEND/internal/config/config.go" "$BACKEND/internal/config.go"
echo "OK"

echo "Step 5: rename handlers -> handler"
git_mv "$BACKEND/internal/handlers" "$BACKEND/internal/handler"
echo "OK"

echo "Step 6: move service dataserver and mediaserver to temp"
git_mv "$BACKEND/internal/service/dataserver" "$TMP/service-dataserver"
git_mv "$BACKEND/internal/service/mediaserver" "$TMP/service-mediaserver"
echo "OK"

echo "Step 7: move clouddrive2 to temp"
git_mv "$BACKEND/internal/clients/clouddrive2" "$TMP/clouddrive2"
echo "OK"

echo "Step 8: move proto to backend/proto"
git_mv "$TMP/clouddrive2/proto" "$BACKEND/proto"
echo "OK"

echo "Step 9: create internal/dataserver and merge sources"
mkdir -p "$BACKEND/internal/dataserver"

for item in "$TMP/service-dataserver"/*; do
  [ -e "$item" ] || continue
  dst="$BACKEND/internal/dataserver/$(basename "$item")"
  git_mv "$item" "$dst"
done

for item in "$TMP/clouddrive2"/*; do
  [ -e "$item" ] || continue
  base="$(basename "$item")"
  if [ "$base" = "proto" ]; then
    continue
  fi
  dst="$BACKEND/internal/dataserver/$base"
  git_mv "$item" "$dst"
done
echo "OK"

echo "Step 10: move mediaserver into internal/mediaserver"
git_mv "$TMP/service-mediaserver" "$BACKEND/internal/mediaserver"
echo "OK"

echo "Step 11: rename utils -> tools"
git_mv "$BACKEND/internal/utils" "$BACKEND/internal/tools"
echo "OK"

echo "Step 12: flatten service subpackages into internal/service"
if [ ! -d "$BACKEND/internal/service" ]; then
  echo "Missing: $BACKEND/internal/service" >&2
  exit 1
fi

while IFS= read -r -d '' file; do
  base="$(basename "$file")"
  dst="$BACKEND/internal/service/$base"
  if [ -e "$dst" ]; then
    echo "Name conflict while flattening: $dst" >&2
    exit 1
  fi
  git mv "$file" "$dst"
  rc=$?
  if [ $rc -ne 0 ]; then
    echo "git mv failed: $file -> $dst" >&2
    exit $rc
  fi
done < <(find "$BACKEND/internal/service" -mindepth 2 -maxdepth 2 -type f -name "*.go" -print0)

echo "OK"

echo "Step 13: verify status"
git status --short
echo "Done"
