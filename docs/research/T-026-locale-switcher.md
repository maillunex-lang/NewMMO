# T-026 — EX-P06 locale switcher (research)

**Date:** 2026-11-21 · **Lane:** client · **Deps:** none

## Sources

| # | Source | Takeaway |
|---|--------|----------|
| 1 | Godot `TranslationServer` + `ConfigFile` | Built-in; no plugin |
| 2 | Existing `client/translations/ui.csv` + T-007 `tr()` | Already bilingual |
| 3 | Godot Asset Library locale UIs | Overkill for RU/EN OptionButton |

## Recommendation

HUD `OptionButton` in `.tscn` + `locale_settings.gd` (`user://settings.cfg`). No new deps.
