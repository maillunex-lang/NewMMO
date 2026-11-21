#!/usr/bin/env bash
# Stage-1 smoke: import + boot + GDScript smokes + Go tests.
# Success = every step exit 0 AND required stdout markers for script smokes.
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

run_godot_capture() {
  # Usage: run_godot_capture <label> <required_marker_or_empty> -- <godot args...>
  local label="$1"
  local marker="$2"
  shift 2
  if [[ "${1:-}" == "--" ]]; then
    shift
  fi
  local log
  log="$(mktemp)"
  echo "== ${label} =="
  set +e
  "$GODOT" "$@" >"$log" 2>&1
  local code=$?
  set -e
  cat "$log"
  if [[ "$code" -ne 0 ]]; then
    echo "FAIL: ${label} exited ${code}" >&2
    rm -f "$log"
    exit "$code"
  fi
  if [[ -n "$marker" ]] && ! grep -qF "$marker" "$log"; then
    echo "FAIL: ${label} missing success marker: ${marker}" >&2
    rm -f "$log"
    exit 1
  fi
  rm -f "$log"
}

run_godot_capture "Import project" "" -- \
  --headless --path "$ROOT/client" --import --quit-after 3

run_godot_capture "Editor boot smoke" "" -- \
  --headless --path "$ROOT/client" --quit-after 2

run_godot_capture "Stage1 smoke script" "STAGE1_SMOKE_OK" -- \
  --headless --path "$ROOT/client" --script res://scripts/dev/stage1_smoke.gd

run_godot_capture "MQ01 smoke script" "MQ01_SMOKE_OK" -- \
  --headless --path "$ROOT/client" --script res://scripts/dev/mq01_smoke.gd

echo "== Server tests =="
(cd "$ROOT/server" && go test ./...)

echo "Smoke OK"
