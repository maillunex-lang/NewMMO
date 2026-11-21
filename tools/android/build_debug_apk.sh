#!/usr/bin/env bash
# Reproduce Lunex Android debug APK (Godot 4.3 prebuilt export).
# Requires env (no secrets committed):
#   GODOT, JAVA_HOME, ANDROID_HOME
#   GODOT_ANDROID_KEYSTORE_DEBUG_PATH / _USER / _PASSWORD
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GODOT="${GODOT:-$HOME/bin/godot}"
JAVA_HOME="${JAVA_HOME:-$HOME/jdk/jdk-17.0.21.1+2}"
ANDROID_HOME="${ANDROID_HOME:-$HOME/android-sdk}"
OUT_DIR="${OUT_DIR:-$ROOT/client/build}"
OUT_APK="${OUT_APK:-$OUT_DIR/LunexMMORPG-debug.apk}"

if [[ ! -x "$GODOT" ]]; then
  echo "GODOT binary missing: $GODOT" >&2
  exit 1
fi
if [[ ! -x "$JAVA_HOME/bin/java" ]]; then
  echo "JAVA_HOME invalid: $JAVA_HOME" >&2
  exit 1
fi
if [[ ! -x "$ANDROID_HOME/platform-tools/adb" ]]; then
  echo "ANDROID_HOME invalid (need platform-tools): $ANDROID_HOME" >&2
  exit 1
fi
if [[ -z "${GODOT_ANDROID_KEYSTORE_DEBUG_PATH:-}" || ! -f "${GODOT_ANDROID_KEYSTORE_DEBUG_PATH}" ]]; then
  echo "Set GODOT_ANDROID_KEYSTORE_DEBUG_PATH to an existing debug keystore (outside git)." >&2
  exit 1
fi
: "${GODOT_ANDROID_KEYSTORE_DEBUG_USER:?set GODOT_ANDROID_KEYSTORE_DEBUG_USER}"
: "${GODOT_ANDROID_KEYSTORE_DEBUG_PASSWORD:?set GODOT_ANDROID_KEYSTORE_DEBUG_PASSWORD}"

export JAVA_HOME ANDROID_HOME
export PATH="$JAVA_HOME/bin:$ANDROID_HOME/platform-tools:$ANDROID_HOME/build-tools/34.0.0:${PATH:-}"

mkdir -p "$OUT_DIR"
echo "== Toolchain =="
"$GODOT" --version
"$JAVA_HOME/bin/java" -version
echo "ANDROID_HOME=$ANDROID_HOME"
echo "OUT_APK=$OUT_APK"

echo "== Import =="
"$GODOT" --headless --path "$ROOT/client" --import --quit-after 3

echo "== Export debug Android =="
"$GODOT" --headless --path "$ROOT/client" --export-debug "Android" "$OUT_APK"

if [[ ! -s "$OUT_APK" ]]; then
  echo "APK missing or empty: $OUT_APK" >&2
  exit 1
fi

APKSIGNER="$ANDROID_HOME/build-tools/34.0.0/apksigner"
AAPT2="$ANDROID_HOME/build-tools/34.0.0/aapt2"
echo "== apksigner verify =="
"$APKSIGNER" verify --print-certs "$OUT_APK"

echo "== badging =="
BADGING="$("$AAPT2" dump badging "$OUT_APK")"
echo "$BADGING" | head -20
PKG="$(echo "$BADGING" | sed -n "s/.*package: name='\([^']*\)'.*/\1/p" | head -1)"
VER="$(echo "$BADGING" | sed -n "s/.*versionName='\([^']*\)'.*/\1/p" | head -1)"
SDK_MIN="$(echo "$BADGING" | sed -n "s/.*sdkVersion:'\([^']*\)'.*/\1/p" | head -1)"
SDK_TGT="$(echo "$BADGING" | sed -n "s/.*targetSdkVersion:'\([^']*\)'.*/\1/p" | head -1)"
ABI="$(echo "$BADGING" | sed -n "s/.*native-code: '\([^']*\)'.*/\1/p" | head -1)"
SIZE="$(wc -c <"$OUT_APK" | tr -d ' ')"

echo "== JSON presence (unzip listing) =="
LISTING="$(unzip -l "$OUT_APK")"
echo "$LISTING" | grep -E 'assets/data/(quests|dialogues)/.*\.json' || true
if ! echo "$LISTING" | grep -q 'assets/data/quests/.*\.json'; then
  echo "FAIL: quest JSON missing from APK" >&2
  exit 1
fi
if ! echo "$LISTING" | grep -q 'assets/data/dialogues/.*\.json'; then
  echo "FAIL: dialogue JSON missing from APK" >&2
  exit 1
fi
echo "json_in_apk=OK"

echo "== SUMMARY =="
echo "godot_version=$("$GODOT" --version)"
echo "apk_path=$OUT_APK"
echo "apk_bytes=$SIZE"
echo "package=$PKG"
echo "versionName=$VER"
echo "minSdk=$SDK_MIN"
echo "targetSdk=$SDK_TGT"
echo "abi=$ABI"
echo "BUILD_DEBUG_APK_OK"
