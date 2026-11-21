#!/usr/bin/env bash
# Builds the downloadable asset pack(s) for the in-game background downloader (client/scripts/core/asset_pack.gd).
# The APK ships WITHOUT the world/enemy models (Android preset exclude_filter; hero_*, npc_* and animations stay in the APK); this script packs the rest into
# build/packs/<name>-<sha8>.pck plus build/packs/manifest.json (sha256 + size). Host the folder at
# $NEWMMO_ASSETS_URL (default https://rikagro.ru/newmmo/assets/). Needs only Godot (no export templates for packs).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GODOT="${GODOT:-$HOME/bin/godot472}"
OUT="${OUT:-$ROOT/build/packs}"
CFG="$ROOT/client/export_presets.cfg"
mkdir -p "$OUT"
cp "$CFG" "$CFG.bak"
trap 'mv -f "$CFG.bak" "$CFG"' EXIT
python3 - "$ROOT" <<'PY'
import glob, os, sys
root = sys.argv[1]
# hero_* and npc_* models (and the humanoid animations) stay inside the APK so the login/character flow never waits for a download
files = sorted(f for f in glob.glob(os.path.join(root, "client/assets/models/*.glb")) if not os.path.basename(f).startswith(("hero_", "npc_")))
res = ", ".join('"res://' + os.path.relpath(f, os.path.join(root, "client")).replace(os.sep, "/") + '"' for f in files)
cfg = os.path.join(root, "client/export_presets.cfg")
n = open(cfg).read().count("[preset.") // 2  # existing presets are [preset.N] and [preset.N.options]; the new one takes the next index
open(cfg, "a").write(f'''
[preset.{n}]
name="AssetPack"
platform="Linux"
runnable=false
advanced_options=false
dedicated_server=false
custom_features=""
export_filter="resources"
export_files=PackedStringArray({res})
include_filter=""
exclude_filter=""
export_path=""
patches=PackedStringArray()
encryption_include_filters=""
encryption_exclude_filters=""
encrypt_pck=false
encrypt_directory=false

[preset.{n}.options]
binary_format/embed_pck=false
''')
PY
TMP="$OUT/_world3d.pck"
"$GODOT" --headless --path "$ROOT/client" --export-pack "AssetPack" "$TMP"
SHA="$(sha256sum "$TMP" | cut -d' ' -f1)"
NAME="world3d-${SHA:0:8}.pck"
mv -f "$TMP" "$OUT/$NAME"
python3 - "$OUT" "$NAME" "$SHA" <<'PY'
import json, os, sys, time
out, name, sha = sys.argv[1:4]
json.dump({"version": int(time.time()), "packs": [{"name": "world3d", "file": name, "sha256": sha,
            "size": os.path.getsize(os.path.join(out, name)), "required": False}]},
          open(os.path.join(out, "manifest.json"), "w"), indent=1)
PY
echo "packs: $(ls -1 "$OUT" | tr '\n' ' ')"
cat "$OUT/manifest.json"
