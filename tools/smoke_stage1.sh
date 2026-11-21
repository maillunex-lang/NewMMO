#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GODOT="${GODOT:-$HOME/bin/godot}"
if [[ ! -x "$GODOT" ]]; then
  GODOT="$(command -v godot || true)"
fi
if [[ -z "${GODOT}" ]]; then
  echo "Godot binary not found" >&2
  exit 1
fi

echo "== Import project =="
"$GODOT" --headless --path "$ROOT/client" --import --quit-after 3 || true

echo "== Editor boot smoke =="
"$GODOT" --headless --path "$ROOT/client" --quit-after 2

echo "== Server tests =="
(cd "$ROOT/server" && go test ./...)

echo "Smoke OK"
