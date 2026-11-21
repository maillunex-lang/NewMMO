# T-015 — Client migration Godot 4.3 → **4.7.2**

**Date:** 2026-11-21 · **Lane:** client  
**Owner decision:** move now (C-GODOT-001 updated); device metrics waived for this gate.  
**Extra-Write:** `tools/android/**`, `tools/smoke_stage1.sh`

## 1. Sources (≥3)

| # | Source | Role |
|---|--------|------|
| 1 | Official migration guides 4.3→4.4, 4.4→4.5, 4.5→4.6, 4.6→4.7 (docs.godotengine.org) | Breaking/behavior changes; UID `.gd.uid`; Android SDK bumps |
| 2 | In-repo gate `docs/research/godot-4-7-2-upgrade.md` + T-012 Android baseline | Risk table, SDK pins for 4.7.2 tag |
| 3 | Hysen Labs catalog + GitHub godot-ci / setup-godot patterns | CI packaging patterns (not installed); migration checklist habits |
| 4 | Godot 4.7.2 release assets + SHA512-SUMS.txt (github.com/godotengine/godot) | Verified engine + export templates |

## 2. What actually touched Lunex

| Area | Result |
|------|--------|
| `config/features` | `"4.3"` → `"4.7"`; GL Compatibility, 1280×720, canvas_items/expand kept |
| UID / `.gd.uid` | 4.7.2 import created `.gd.uid` for all client scripts (committed) |
| `project.godot` | Editor-normalized comments/blank lines + locale key order; values unchanged |
| GDScript / smokes | No SCRIPT ERROR / CONFUSABLE warnings on 4.7.2; STAGE1/MQ01/NET green |
| UI / Theme / safe-area | Layout preserved in before/after frames |
| Translations | `ui.csv` re-imported to `.translation` (gitignored); chrome labels via `tr()` OK |
| World Studio | Plugin loads on import; dock + `get_undo_redo()` place path unchanged; no API edits needed |
| Android | Prebuilt APK OK **without** NDK/CMake; SDK 36 / build-tools 36.1.0 |

## 3. Install pins (verified)

| Artifact | SHA-512 | Path |
|----------|---------|------|
| `Godot_v4.7.2-stable_linux.x86_64.zip` | `9aa00f7a605200940bce3027a567b782f49bd8e940dd06ae9e987bd65aee1b1467edd56ed84fcdcbdd44354bf613bdbb4e5d2913e925850368e150c59ed54c65` | `$HOME/godot/Godot_v4.7.2-stable_linux.x86_64` → `$HOME/bin/godot472` |
| `Godot_v4.7.2-stable_export_templates.tpz` | `ca4d71c4d7b81dfc15d1a98baa07534aa95b03fdda78a0075b06672e1648d2e5f40980c9adc28d23e1b92e732ee7bf3461997aa804af74ec2fcd7a93ccb84079` | `$HOME/.local/share/godot/export_templates/4.7.2.stable/` |
| Godot 4.3 (kept) | (prior T-012) | `$HOME/bin/godot` → `Godot_v4.3-stable_linux.x86_64` |
| JDK 17 | Temurin 17.0.21.1+2 | `$HOME/jdk/jdk-17.0.21.1+2` |
| Android SDK | `platforms;android-36`, `build-tools;36.1.0` (+34 kept) | `$HOME/android-sdk` |

Engine version string: `4.7.2.stable.official.ed1daf0bf`

## 4. Warnings before → after

| Kind | 4.3 | 4.7.2 |
|------|-----|-------|
| SCRIPT / CONFUSABLE / Parse | none in smoke | none in smoke |
| ALSA / no audio device | env dummy driver | same (env) |
| adb daemon refused | env (no device) | same |
| Editor Vulkan extension | n/a (not needed for GL game) | editor may warn; game uses GL Compatibility |

## 5. Screenshot compare

Artifacts: `/opt/cursor/artifacts/t015-before/` vs `t015-after/`  
(`stage1_dawn_valley.png`, `stage1_dialogue.png`, `stage1_journal.png`)

- Resolution 1280×720 both engines.
- Layers/text/HUD/safe-area/journal/dialogue chrome match.
- Small pixel deltas (≈1–2% pixels, mean Δ≪1) attributed to sprite animation frame timing + llvmpipe, not layout regressions.
- Dialogue frame shows the same raw line id on **both** 4.3 and 4.7.2 when driven by the screenshot harness (not a 4.7 regression).

## 6. Android APK before → after

| | 4.3 (T-012) | 4.7.2 (T-015) |
|--|-------------|----------------|
| godot | 4.3.stable | 4.7.2.stable |
| apk_bytes | 24686211 | 29156536 |
| package | com.lunex.newmmo | com.lunex.newmmo |
| abi | arm64-v8a | arm64-v8a |
| minSdk | 21 | **24** |
| targetSdk / compileSdk | 34 | **36** |
| signed | debug apksigner OK | debug apksigner OK |
| JSON in APK | quests+dialogues | quests+dialogues |

NDK/CMake **not** required for `gradle_build/use_gradle_build=false`.

## 7. CI requirements (for director after accept)

- Godot **4.7.2-stable** Standard Linux x86_64 (SHA above)
- Export templates **4.7.2** (SHA above)
- JDK **17**
- Android SDK: `platform-tools`, `platforms;android-36`, `build-tools;36.1.0`, cmdline-tools
- Env: `GODOT` → 4.7.2 binary (repo smoke prefers `$HOME/bin/godot472`)
- Debug keystore via `GODOT_ANDROID_KEYSTORE_DEBUG_*` (never in git)
- NDK/CMake **not** required for current prebuilt preset

## 8. Rollback plan

1. Revert this branch commits (or restore `project.godot` `config/features` to `"4.3"` and drop new `.gd.uid` if undesired).
2. Run with `$HOME/bin/godot` (4.3) + templates `4.3.stable`.
3. `GODOT=$HOME/bin/godot ./tools/smoke_stage1.sh`
4. Android: `GODOT=$HOME/bin/godot ANDROID_BUILD_TOOLS_VER=34.0.0 ./tools/android/build_debug_apk.sh` (4.3 templates; platforms;android-34 still present).

## 9. Residual risks

- Device metrics (CT-020) still required before release; waived only for this migration gate.
- minSdk rose 21→24 (engine template default) — confirm owner OK for older devices.
- APK ~18% larger (engine binary growth).
- World Studio interactive place/undo verified by code path + plugin load; full GUI click not exercised in this Vulkan-limited VM.
