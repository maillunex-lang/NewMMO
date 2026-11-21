#!/usr/bin/env bash
# Owner/device protocol for CT-020-style Android metrics (Godot 4.3 baseline).
# Requires: adb, installed package com.lunex.newmmo, optional GODOT gfx monitor notes.
# Does NOT invent numbers — prints raw dumpsys excerpts for three cold starts + one 5‑min session helper.
set -euo pipefail
PKG="${PKG:-com.lunex.newmmo}"
APK="${1:-}"
ANDROID_HOME="${ANDROID_HOME:-$HOME/android-sdk}"
ADB="${ADB:-$ANDROID_HOME/platform-tools/adb}"
OUT_DIR="${OUT_DIR:-$HOME/lunex-android/metrics}"
mkdir -p "$OUT_DIR"

if [[ ! -x "$ADB" ]]; then
  echo "adb not found at $ADB" >&2
  exit 1
fi
DEVICES="$("$ADB" devices | awk 'NR>1 && $2=="device"{print $1}')"
if [[ -z "$DEVICES" ]]; then
  echo "DEVICE_UNAVAILABLE: no adb device in 'device' state"
  echo "Connect a phone with USB debugging, accept the RSA prompt, re-run."
  exit 2
fi

echo "== devices =="
"$ADB" devices -l

if [[ -n "$APK" ]]; then
  echo "== install $APK =="
  "$ADB" install -r "$APK"
fi

run_cold_start() {
  local n="$1"
  local log="$OUT_DIR/cold_start_$n.txt"
  "$ADB" shell am force-stop "$PKG" || true
  sleep 2
  "$ADB" shell "echo 3 > /proc/sys/vm/drop_caches" 2>/dev/null || true
  local t0
  t0="$(date +%s%3N)"
  "$ADB" shell am start -W -n "$PKG/com.godot.game.GodotApp" >"$log" 2>&1 || \
    "$ADB" shell monkey -p "$PKG" -c android.intent.category.LAUNCHER 1 >>"$log" 2>&1
  local t1
  t1="$(date +%s%3N)"
  echo "cold_start_$n wall_ms=$((t1 - t0))" | tee -a "$log"
  "$ADB" shell dumpsys meminfo "$PKG" >"$OUT_DIR/meminfo_cold_$n.txt" || true
}

echo "== three cold starts =="
for i in 1 2 3; do
  run_cold_start "$i"
  sleep 3
done

echo "== gfxinfo reset =="
"$ADB" shell dumpsys gfxinfo "$PKG" reset || true
WAIT_SEC="${MEASURE_WAIT_SEC:-0}"
if [[ "$WAIT_SEC" -gt 0 ]]; then
  echo "Waiting ${WAIT_SEC}s for a play session (set MEASURE_WAIT_SEC, default 0=skip long wait)..."
  sleep "$WAIT_SEC"
elif [[ -t 0 ]]; then
  echo "Leave the game in landscape, move ~5 minutes on low, then press Enter (or set MEASURE_WAIT_SEC=300)."
  read -r _
else
  echo "Non-interactive: skipping 5-min wait. Re-run with MEASURE_WAIT_SEC=300 after manual play."
fi
"$ADB" shell dumpsys gfxinfo "$PKG" >"$OUT_DIR/gfxinfo_5min.txt" || true
"$ADB" shell dumpsys meminfo "$PKG" >"$OUT_DIR/meminfo_5min.txt" || true
"$ADB" shell dumpsys batterystats --checkin >"$OUT_DIR/batterystats_checkin.txt" 2>/dev/null || true

echo "== summarize files in $OUT_DIR =="
ls -la "$OUT_DIR"
echo "Parse TotalPss / frame stats manually; target median FPS ≥ 30 on low (CT-020)."
echo "MEASURE_DEVICE_OK"
