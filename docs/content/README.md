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

- **Done (partial):** REG-01 MQ-01-01…05 local client stub; Stage 1 visual Dawn Valley.
- **Not done:** authoritative Go/Postgres quests, SQ, DNG, EVT, full 240 quests, Android metrics.
