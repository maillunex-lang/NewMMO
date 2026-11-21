# Game content canon

**Orchestrator:** start with [`14_CONTENT_INTEGRATION.md`](14_CONTENT_INTEGRATION.md).  
**Execution roadmap:** [`../ROADMAP_EXECUTION.md`](../ROADMAP_EXECUTION.md).

Files in `docs/content/` are the **approved narrative and content canon** for Lunex MMORPG.

## Rules

- Do **not** change plot, characters, geography, or IDs without explicit owner permission.
- Conflicts between docs → `CONFLICTS.md` + owner decision (see integration §1).
- Implementation progress is tracked in `docs/ROADMAP_EXECUTION.md`, not by rewriting these MD files.

## Approved documents

| File | Topic |
|------|--------|
| `01_WORLD.md` | World of Eliar, Seal, six regions |
| `02_FACTIONS.md` | Factions |
| `03_LOCATIONS.md` | Locations / POI |
| `04_CHARACTERS.md` | Characters |
| `05_CLASSES.md` | Classes |
| `06_ENEMIES.md` | Enemies |
| `07_STORY.md` | Story chapters |
| `08_QUESTS_MAIN.md` | Main quests (120) |
| `09_QUESTS_SIDE.md` | Side quests (120) |
| `10_DIALOGUES.md` | Dialogues |
| `11_ITEMS_LOOT.md` | Items / loot |
| `12_DUNGEONS.md` | Dungeons (12) |
| `13_WORLD_EVENTS.md` | World events (18) |
| `14_CONTENT_INTEGRATION.md` | Integration contract + Cursor plan |

## Implementation snapshot

- **Baseline audit:** [`../AUDIT_BASELINE.md`](../AUDIT_BASELINE.md) (EX-P00).
- **Machine checks (EX-P01/P02):** `python3 tools/content/lint.py` — parsers, JSON Schema (`content/schemas/`), ID registry, crossrefs, quest/enemy/dungeon/event/dialogue/loot static checks; `check_client_sync.py` — client data vs canon. Canon currently: **0 ERROR**, 1 WARN (`DLG-SCN-01-01` in `07`).
- **Conflicts:** [`CONFLICTS.md`](CONFLICTS.md) — C-DIA-001 (A), C-GODOT-001 (B), C-SET-001 (A: `STL-*`), C-ITEMQ-001 (A: `11` §12.1) resolved; notes C-POI-001, C-DLG-REMNANT, C-ITM-00 open.
- **Volumes verified:** 8 FAC, 6 REG, 48 LOC, 72 POI, 18 STL, 32 NPC, 5 CLS, 96 ENM, 120 MQ, 120 SQ, 12 DNG, 18 EVT, 3 END, 270 EQ, 30 SET, 36 MAT, 96 LOOT, 36 ITEM-Q, 3122 dialogue lines.
- **Done (partial):** REG-01 MQ-01-01…05 local client stub; Stage 1 visual Dawn Valley.
- **Not done:** authoritative Go/Postgres quests, SQ, DNG, EVT, full 240 quests, Android metrics.
