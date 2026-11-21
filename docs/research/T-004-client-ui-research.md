# T-004 — Client UI / dialogue research (no install)

**Date:** 2026-11-21  
**Lane:** client  
**Task:** T-004  
**Status:** Research only — **nothing installed or vendored**  
**Engine pin:** Godot **4.3** (project); target **4.7.2** (C-GODOT-001 B — do not upgrade in this task)

## 1. Problem

Review HUD / dialogue / quest tracker for: Theme-first `.tscn` UI, landscape + Android safe-area, ≥~48 dp touch targets, no canon strings baked into GDScript layout builders. Prefer native Godot for T-004 fixes; evaluate dialogue kits only for a future director decision.

## 2. Sources searched

| Source | What |
|--------|------|
| **Hysen Labs** | https://hysenlabs.com/projects — Dialogic 2, DialogueNodes, Godot Open RPG, OpenGamepadUI |
| GitHub | nathanhoad/godot_dialogue_manager; godot-proposals#6929 safe-area |
| Godot docs | `DisplayServer.get_display_safe_area`, Control/Theme |
| Tutorials | PlayerMotion “Safe Areas for Touch Controls” (Godot 4.7 pattern: MarginContainer + margins) |
| Asset Library | Dialogue Manager listings |

## 3. Candidates (≥3)

| # | Candidate | License | Activity / fit | Godot 4.3 → 4.7.2 | Recommendation for Lunex now |
|---|-----------|---------|----------------|-------------------|------------------------------|
| 1 | **Native Godot** — `Theme` + anchors/containers + `DisplayServer.get_display_safe_area()` via `MarginContainer` | N/A (engine) | Official API; Android/iOS safe-area documented | Works on 4.3; same API on 4.7.x | **Adopt pattern in T-004** (no addon) |
| 2 | [Dialogic 2](https://hysenlabs.com/projects/dialogic-godot-dialogic) → [dialogic-godot/dialogic](https://github.com/dialogic-godot/dialogic) | MIT (+ fonts Apache) | Active; Hysen card 2026-11 | **Requires Godot ≥4.5**; alpha API churn | **Do not install** while pin is 4.3; revisit after C-GODOT upgrade |
| 3 | [DialogueNodes](https://hysenlabs.com/projects/nagidev-dialoguenodes) (nagidev) | check upstream on adopt | v1.2 (2024); graph editor | Godot 4 plugin; own data format | **Do not install** — conflicts with canon `DIA.*` JSON + server authority |
| 4 | [Dialogue Manager](https://github.com/nathanhoad/godot_dialogue_manager) (AssetLib) | MIT | Very active; v4 needs 4.6+; v3.3 line for 4.3 | v3.3 could fit 4.3, still owns dialogue format | **Do not install** for T-004; optional future balloon skin only if director OK |
| 5 | Hysen: [Godot Open RPG](https://hysenlabs.com/projects/gdquest-demos-godot-open-rpg) | demo/template | GDQuest demos; UI menus reference | Learning only | Patterns only — do not vendor |

## 4. Comparison (T-004 needs)

| Need | Native safe-area + Theme | Dialogic 2 | DialogueNodes | Dialogue Manager |
|------|--------------------------|------------|---------------|------------------|
| No third-party install | yes | no | no | no |
| Keep `client/data` `DIA.*` | yes | would replace/author new format | own graph format | own `.dialogue` format |
| Server-authoritative choices later | yes (thin UI) | client-centric VN tooling | client editor | balloon + mutations (game owns state) |
| Android safe-area | MarginContainer pattern | N/A (dialogue focus) | N/A | N/A |
| Touch ≥48 dp | scene `custom_minimum_size` | depends on balloon | depends on UI | example balloon editable |

## 5. Touch-target number

Android Material / common mobile a11y floor: **48 dp**. With Lunex viewport 1280×720 and `canvas_items` stretch, treating **48 control units** as the minimum interactive height/width matches that floor at ~1× scale and stays ≥48 when UI is not aggressively downscaled. Skill buttons already use 55×55; dialogue `Next` was 40 → raise to **48**.

## 6. Recommendation

1. **T-004 implement:** native Theme + editable `.tscn` + `SafeArea` `MarginContainer` reading `DisplayServer.get_display_safe_area()` (mobile only; desktop keeps base margin).  
2. **Do not install** Dialogic / DialogueNodes / Dialogue Manager in this task.  
3. **Future (director + owner OK):** after Godot pin moves toward 4.7.2, re-evaluate Dialogic 2 or Dialogue Manager **only** as presentation helpers that still consume server-filtered `DIA.*` lines — never as quest authority.

**Awaiting confirmation — nothing installed.**
