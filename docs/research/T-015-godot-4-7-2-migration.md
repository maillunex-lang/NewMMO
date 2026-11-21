# T-015 — Client migration Godot 4.3 → **4.7.2**

**Date:** 2026-11-21 · **Lane:** client  
**Owner decision:** move now (C-GODOT-001 updated); device metrics waived for this gate.  
**Extra-Write:** `tools/android/**`, `tools/smoke_stage1.sh`

## 1. Sources (≥3)

| # | Source | Role |
|---|--------|------|
| 1 | Official migration guides 4.3→4.4, 4.4→4.5, 4.5→4.6, 4.6→4.7 | Breaking/behavior changes |
| 2 | In-repo gate `docs/research/godot-4-7-2-upgrade.md` + T-012 Android baseline | Risk table, SDK pins for 4.7.2 tag |
| 3 | Hysen / GitHub: godot-ci, setup-godot, godot-export | CI packaging patterns (not installed) |
| 4 | Godot 4.7 Android export docs + `config.gradle` on tag 4.7.2 | platforms;android-36, build-tools;36.1.0; prebuilt APK without NDK |

## 2. What likely touches Lunex

| Area | Expectation for this repo |
|------|---------------------------|
| `config/features` | `"4.3"` → `"4.7"`; keep GL Compatibility, 1280×720, canvas_items/expand |
| UID / `.gd.uid` | Clean import on 4.7.2 may add script UIDs — commit if explained |
| GDScript | Typed returns / `CONFUSABLE_TEMPORARY_MODIFICATION` on JSON→Variant paths (quest/net) |
| UI | Control/Theme/safe-area mostly compatible; re-check HUD/dialogue/journal screenshots |
| Translations | Re-import `ui.csv` → `.translation` (gitignored); keep CSV |
| World Studio | EditorPlugin APIs mostly additive; verify dock/undo in GUI editor |
| Android | Prebuilt APK; install SDK 36 / build-tools 36.1.0; **no NDK/CMake/Studio** unless export fails |

## 3. Install pins (official)

| Artifact | SHA-512 (official) |
|----------|--------------------|
| `Godot_v4.7.2-stable_linux.x86_64.zip` | `9aa00f7a605200940bce3027a567b782f49bd8e940dd06ae9e987bd65aee1b1467edd56ed84fcdcbdd44354bf613bdbb4e5d2913e925850368e150c59ed54c65` |
| `Godot_v4.7.2-stable_export_templates.tpz` | `ca4d71c4d7b81dfc15d1a98baa07534aa95b03fdda78a0075b06672e1648d2e5f40980c9adc28d23e1b92e732ee7bf3461997aa804af74ec2fcd7a93ccb84079` |

Keep Godot **4.3** binary + templates for rollback.

## 4. Rollback plan

1. Restore `project.godot` `config/features` to `"4.3"` (and any scene/UID commits from this branch).
2. Run with `$HOME/bin/godot` 4.3 + templates `4.3.stable`.
3. Re-import; `./tools/smoke_stage1.sh` with `GODOT` pointing at 4.3.
4. Android: use build-tools 34.0.0 path in script / env; templates 4.3.

## 5. CI requirements (for director after accept)

- Godot **4.7.2-stable** Standard Linux x86_64 (SHA above)
- Export templates **4.7.2** (SHA above)
- JDK **17**
- Android SDK: `platform-tools`, `platforms;android-36`, `build-tools;36.1.0`, cmdline-tools
- Debug keystore via `GODOT_ANDROID_KEYSTORE_DEBUG_*` env (never in git)
- NDK/CMake **not** required for current prebuilt preset

## 6. Execution notes

Filled during T-015: warnings before→after, files changed, APK before/after SDK levels, screenshot comparison.
