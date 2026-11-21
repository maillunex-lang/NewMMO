#!/usr/bin/env bash
# Stage-1 smoke: import + boot + GDScript smokes + Go tests.
# Success = every step exit 0 AND required stdout markers for script smokes.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# Prefer Godot 4.7.2 (T-015); fall back to 4.3 symlink / PATH for rollback.
GODOT="${GODOT:-}"
if [[ -z "$GODOT" ]]; then
  if [[ -x "$HOME/bin/godot472" ]]; then
    GODOT="$HOME/bin/godot472"
  elif [[ -x "$HOME/bin/godot" ]]; then
    GODOT="$HOME/bin/godot"
  else
    GODOT="$(command -v godot || true)"
  fi
fi
if [[ -z "${GODOT}" || ! -x "$GODOT" ]]; then
  echo "Godot binary not found" >&2
  exit 1
fi
GODOT_VERSION_STR="$("$GODOT" --version 2>/dev/null || echo '?')"
echo "Using GODOT=$GODOT ($GODOT_VERSION_STR)"
# Project pin is Godot 4.7.x (owner decision 2026-11-21). Rollback to 4.3 must be explicit.
if [[ "$GODOT_VERSION_STR" != 4.7.* && "${ALLOW_GODOT_43:-0}" != "1" ]]; then
  echo "FAIL: Godot 4.7.x required (got $GODOT_VERSION_STR). Set ALLOW_GODOT_43=1 only for a deliberate rollback." >&2
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

run_godot_capture "Net protocol smoke" "NET_SMOKE_OK" -- \
  --headless --path "$ROOT/client" --script res://scripts/dev/net_smoke.gd

run_godot_capture "Auth REST smoke" "AUTH_SMOKE_OK" -- \
  --headless --path "$ROOT/client" --script res://scripts/dev/auth_smoke.gd

run_godot_capture "Online wiring smoke" "ONLINE_SMOKE_OK" -- \
  --headless --path "$ROOT/client" --script res://scripts/dev/online_smoke.gd

echo "== Server tests =="
(cd "$ROOT/server" && go test ./...)

echo "Smoke OK"
