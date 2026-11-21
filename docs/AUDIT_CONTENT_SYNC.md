# Audit: docs/content canon vs client (2026-11-21)

**Branch:** `main`  
**Scope:** approved `docs/content/*.md` vs NewMMO client  
**Superseded for readiness claims by:** `docs/AUDIT_BASELINE.md` (EX-P00)

## Canon inventory

| File | Role | Used this iteration |
|------|------|---------------------|
| 01_WORLD.md | World, Seal, 6 regions | REG-01 context |
| 02_FACTIONS.md | Factions | FAC-08 reputation IDs only |
| 03_LOCATIONS.md | LOC/POI registry | LOC-01-001…003 markers in one scene |
| 04_CHARACTERS.md | NPCs | NPC-802 Фейра Нисс, NPC-102 Ильра Венн |
| 05_CLASSES.md | Classes | Not implemented (Guardian stand-in remains) |
| 06_ENEMIES.md | Enemies | Existing slime/wolf/bandit kept as interim; not remapped to ENM IDs yet |
| 07_STORY.md | Chapter beats | Cross-check MQ order |
| 08_QUESTS_MAIN.md | 120 MQ contracts | **MQ-01-01…05 only** |
| 09_QUESTS_SIDE.md | Side quests | Deferred |
| 10_DIALOGUES.md | Dialogue IDs | DIA.MQ-01-01…05 + NPC greetings |
| 11_ITEMS_LOOT.md | Items / loot / materials | Synced 2026-11-21; not in client runtime (quest evidence IDs referenced) |
| 12_DUNGEONS.md | 12 instances | Synced 2026-11-21; DNG-01-001/002 not built |
| 13_WORLD_EVENTS.md | 18 public events | Synced 2026-11-21; not implemented |
| 14_CONTENT_INTEGRATION.md | Integration + P00–P13 plan | Synced 2026-11-21; drives `ROADMAP_EXECUTION.md` |

## Gaps before this iteration

| Area | Canon | Was in code |
|------|-------|-------------|
| Prologue quest | MQ-01-01…20 | Single stub “Clear the Path” / Elder Mira |
| NPCs | NPC-802, NPC-102, … | `elder_mira` (non-canon) |
| Dialogues | DIA.* data files | Hardcoded Russian lines in scene |
| Locations | LOC-01-001 city + open zone + Field Ford | One placeholder `dawn_valley.tscn` |
| Server quest ops | Postgres + idempotent events | Local-only quest_manager |
| 240 quests | 120 main + side | 0 canon quests |

## Conflicts / safe decisions

1. **Elder Mira → removed** as quest giver; replaced by canon NPC-802 / NPC-102 (no lore rewrite).
2. **Single playable scene** still hosts Asterville plaza + valley + Field Ford **markers** (LOC IDs as metadata). Full separate `.tscn` per LOC deferred.
3. **Authority:** Stage 2 server not ready; client quest state is an **explicit local stub** mirroring server event names (`npc_interact`, `world_interact`, `escort_checkpoint`) for later swap.
4. **Witnesses A/B** have no approved NPC IDs → interactable POIs (not new named characters).
5. **OSS:** extending thin in-house data-driven quest layer; no third-party quest framework installed (decision D spirit: no unsolicited generator/framework churn).

## This iteration goal

Playable **MQ-01-01 … MQ-01-05** with canon titles, journal text, dialogue IDs, objectives, local rewards (100 XP / 12 gold / +20 FAC-08), preserving movement, combat, HUD, World Studio.
