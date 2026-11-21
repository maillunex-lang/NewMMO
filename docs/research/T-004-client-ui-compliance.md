# T-004 — Client UI compliance audit + fixes

**Lane:** client · **Branch:** `agent/client/T-004`  
**Date:** 2026-11-21  
**Engine:** Godot 4.3 (unchanged)

## Smoke before / after

| Gate | Before | After |
|------|--------|-------|
| `./tools/smoke_stage1.sh` | Smoke OK | Smoke OK |
| `mq01_smoke.gd` | `MQ01_SMOKE_OK` xp=500 rep=100 evidence×2 | identical |
| `stage1_smoke.gd` | (not re-run before; after OK) | `STAGE1_SMOKE_OK` |
| `godot --headless --path client --quit-after 2` | OK | OK |
| `python3 tools/content/check_client_sync.py` | — | **0 errors** (28 info stubs/punct) |

## Compliance table

| Screen / file | Rule | Status | Evidence | Action |
|---------------|------|--------|----------|--------|
| `hud.tscn` / `dialogue.tscn` | Editable Control `.tscn` + Theme | **OK** | Both use `res://themes/lunex_hud.tres` | kept |
| `dialogue_ui.gd` | Scripts must not rebuild full layout | **Was violation** | Previously `Button.new()` for choices | **Fixed:** ChoiceAccept/Ask/TurnIn authored in `.tscn`; script toggles visibility |
| HUD / Dialogue | Landscape + safe-area | **Was missing** | No `DisplayServer.get_display_safe_area` usage | **Fixed:** `SafeArea` MarginContainer + `safe_area_margins.gd` |
| Corner chrome | Anchors vs absolute | **PARTIAL → OK** | Anchored corners; pixel offsets remain (normal for Godot) | inside SafeArea/Chrome |
| Touch ≥48 units | Skill/Next/Interact | **Was violation** | `dialogue.tscn` Next `custom_minimum_size` height **40** | **Fixed:** Next + choices **48**; skills/AUTO/TALK **56** |
| `hud.tscn` QuestText | No stale canon in scene | **Was violation** | Default `"Prologue · Talk to Elder Mira"` | **Fixed:** empty; filled from `quest_manager` |
| `dialogue.tscn` Speaker | No stale NPC name | **Was violation** | Default `"Elder Mira"` | **Fixed:** `"…"` |
| `hud_controller.gd` | No hardcode class label | **Was violation** | `"Guardian · Lv 1"` L44 | **Fixed:** placeholder export; no class claim |
| `dialogue_ui.gd` | Dialogue text from data | **OK / was fallback** | Body from `get_line`; fallback had canon proverb | **Fixed:** empty fallback if missing |
| Choice chrome RU labels | UI chrome in scene | **OK after** | `Принять`/`Спросить`/`Сдать`/`Далее` on `.tscn` | scene-authored |
| Quest **log** screen | Separate journal UI | **MISSING** | Only tracker panel `Quest` on HUD | **Deferred** — propose dedicated `.tscn` later |
| Full localization keys | UI string table | **PARTIAL** | Chrome still literal RU/EN on controls | **Deferred** — needs director locale contract |
| `project.godot` features | Stay on 4.3 | **OK** | not modified | — |
| Client authority | No new server-bypass | **OK** | UI-only | — |

## Deferred / proposals (out of small-fix budget)

1. Dedicated quest journal scene (list of MQ/SQ, not only tracker Label).  
2. Locale resource / `tr()` keys for AUTO/TALK/Далее (director content/locale contract).  
3. Optional Dialogue Manager balloon skin **after** owner OK + Godot pin path (see research).  
4. Desktop preview margins for cutouts (safe-area only applied when `mobile`/`android`/`ios`).

## Files touched (lane)

- `client/scripts/ui/safe_area_margins.gd` (new)
- `client/scenes/ui/hud.tscn`
- `client/scenes/ui/dialogue.tscn`
- `client/scripts/ui/hud_controller.gd`
- `client/scripts/ui/dialogue_ui.gd`
- `docs/research/T-004-client-ui-research.md`
- `docs/research/T-004-client-ui-compliance.md`
