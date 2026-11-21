# T-007 — Quest journal UI + `tr()` research (no install)

**Date:** 2026-11-21  
**Lane:** client · **Task:** T-007  
**Engine pin:** Godot **4.3** (do not upgrade)  
**Status:** Research only for third-party — **nothing installed**

## 1. Need

Editable landscape quest journal (active quest + objectives n/m + reward + completed list), Theme + safe-area, data behind a thin QuestManager-facing API (swap to server later). UI chrome via CSV/`tr()`; canon/dialogue/quest text stays in `client/data`.

## 2. Sources

| Source | Used |
|--------|------|
| Hysen Labs | [GLoot](https://hysenlabs.com/projects/peter-kish-gloot), [Godot Open RPG](https://hysenlabs.com/projects/gdquest-demos-godot-open-rpg), [GoNorth](https://hysenlabs.com/projects/steffendx-gonorth), Dialogic/Dialogue Manager (T-004) |
| Godot 4 docs | CSV localization, `tr()`, ItemList/ScrollContainer/Theme |
| T-004 research | SafeArea `MarginContainer` pattern already in-repo |

## 3. Candidates (≥3)

| # | Option | License / activity | Fit for 4.3 → 4.7.2 | Risks | Verdict |
|---|--------|--------------------|---------------------|-------|---------|
| 1 | **Native Controls** — Panel + VBox + ItemList/ScrollContainer + Theme + existing `safe_area_margins.gd` + CSV/`tr()` | Engine | Full | Own layout work | **Adopt for T-007** |
| 2 | [GLoot](https://hysenlabs.com/projects/peter-kish-gloot) inventory UI (CtrlInventory → ItemList) | MIT; Godot **4.4+** | Too new for pin 4.3; inventory not quest log | Wrong domain + version | **Do not install** |
| 3 | [Godot Open RPG](https://hysenlabs.com/projects/gdquest-demos-godot-open-rpg) menus | Demo | Learning only | Would vendor demo UI | Patterns only |
| 4 | Dialogic 2 / Dialogue Manager | MIT | Dialogue balloons, not journal; Dialogic needs ≥4.5 | Format conflict with `DIA.*` | **Do not install** (same as T-004) |
| 5 | GoNorth (quest planning) | ASP.NET tool | Offline planning/export | Not a Godot HUD | Out of scope |

## 4. Localization approach

Official Godot CSV: `keys,ru,en` → import → `ui.ru.translation` / `ui.en.translation` → register only under `[internationalization]` in `project.godot`. Scripts call `tr("ui.*")`. **Do not** put quest titles/dialogue in CSV.

## 5. Recommendation

Build **native** `quest_journal.tscn` + binder script; ItemList for objectives/completed (data fill, no `Control.new()` layout). Extend QuestManager with **read-only** snapshot methods. Wire HUD open (≥48). No addons.

**Awaiting confirmation for any third-party — none requested.**

## 6. How to verify (manual)

1. `godot --path client`
2. Click **Журнал** (under the top-right tracker; ≥48×48).
3. Confirm overlay: title, active MQ title/journal from data, objectives `n/m`, reward chrome via `tr()`, empty completed at start.
4. Esc / **Закрыть** / dim tap closes.
5. Artifact screenshot (this run): `/opt/cursor/artifacts/t007-quest-journal.png`

