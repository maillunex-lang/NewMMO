# NewMMO implementation plan

**Based on:** `docs/GDD.md`, `docs/ARCHITECTURE.md`, `docs/ROADMAP.md`, `docs/WORLD_EDITOR.md`, `docs/AUDIT.md`, master build prompt  
**Rule:** Approved GDD decisions win over the prompt when they conflict. Unresolved items are documented, not invented as final law.

## Goals for the next working iterations

1. Stabilize the Godot client so it loads without critical errors.
2. Ship a **playable Stage 1 visual vertical slice** (local authority OK; server combat is Stage 2).
3. Keep World Studio + `.tscn` editing as the only supported way to rearrange props.
4. Prepare Stage 2 online foundation without blocking Stage 1 playability.

## Dependency graph (high level)

```text
Stage 0 audit/stabilization
    → Stage 1 visual slice (location, actor, input, NPC, quest, 3 enemies, local combat/loot, HUD)
        → Stage 2 Go+Postgres+WS (movement sync, server combat, persistence, reconnect)
            → Stage 3 World Studio + 3 regions + safe regen + server map export
                → Stage 4 RPG core (5 classes, gear, pets, talents)
                    → Stage 5 story/PvE (20 quests ch.1, dungeons, parties)
                        → Stage 6 MMO systems (guilds, PvP, trade, craft)
                            → Stage 7 world expansion + admin live-ops
                                → Stage 8 release hardening
```

Independent tracks that can run beside the critical path once Stage 2 exists: asset Blender pipeline, admin React shell, CI, isolated Compose deploy.

---

## Stage 0 — audit and stabilization

| ID | Task | Acceptance | Risk |
|----|------|------------|------|
| S0-1 | Code/docs audit → `docs/AUDIT.md` | File committed with verified facts | Stale docs |
| S0-2 | Implementation plan → this file | Dependencies + checklists | Over-scoping |
| S0-3 | Enable World Studio plugin; fix owner/undo | Plugin loads; add prop sets owner | Editor-only bugs |
| S0-4 | Pin Godot version note | Docs state 4.3 until 4.7.x available | Version drift |
| S0-5 | `assets/LICENSES.md` | Every shipped asset listed | Legal |

**Checklist:** [x] audit [x] plan [x] plugin fix [x] licenses [x] headless import smoke

---

## Stage 1 — visual vertical slice (current)

### Scope

- Dawn Valley as a richer **editable** location (layers preserved; more props; settlement markers).
- One **animated** Guardian stand-in (procedural 8-dir sprites → later Blender atlases).
- Camera follow; virtual joystick + tap move; basic blocking props.
- One story NPC with **important dialogue that pauses automation**.
- One starter quest with progress + reward (local).
- Three enemy archetypes: forest slime, dusk wolf, valley bandit.
- Manual skills + configurable auto-combat toggle; loot drops into local inventory UI.
- Premium-leaning dark/gold HUD theme as `.tres` + editable `.tscn`.
- World Studio: categories + status; props remain Inspector-editable.

### Explicitly out of Stage 1

- Authoritative multiplayer, PostgreSQL, WebSocket combat.
- Real Meshy/Blender atlases, APK if Android templates missing.
- All 5 classes, auction, guilds, monetization.

### Acceptance criteria

| # | Criterion | Verify |
|---|-----------|--------|
| 1 | Godot imports/opens project without critical errors | `godot --headless --path client --quit-after 1` |
| 2 | Dawn Valley `.tscn` editable: move tree, save, run | Manual / documented |
| 3 | Player animates while moving; camera follows | Manual / headless script smoke |
| 4 | Joystick and tap both move character | Manual |
| 5 | NPC dialogue opens; important flag pauses auto | Manual |
| 6 | Quest advances and grants reward | Manual |
| 7 | Three enemy types spawn, can be killed, drop loot | Manual |
| 8 | Auto-combat can be toggled; manual interrupt works | Manual |
| 9 | HUD/theme editable without rewriting layout in GDScript | Inspector |
| 10 | No secrets committed; APK status reported honestly | Review |

### Risks

- Procedural art is **interim** — must not be mistaken for final pipeline.
- Local combat will be replaced by server authority in Stage 2 (expect client prediction rewrite).
- APK may be blocked by missing export templates in CI/VM.

---

## Stage 2 — online foundation

| Task | Acceptance |
|------|------------|
| Postgres migrations: accounts, characters, inventory, quests | Migrate up/down clean |
| HTTP auth + session tokens | Integration test |
| WebSocket zone channel; interest radius | 2 clients see each other |
| Server movement validation + combat resolution | Cheat client cannot invent damage |
| Persist character/quest/inventory; reconnect | Kill WS → resume state |
| Isolated Compose (`newmmo` project name) | No conflict with other VPS stacks |

---

## Stage 3 — editor and three regions

Dawn Valley polish, Asterville, Whispering Forest; hybrid generator with Manual/Generated/Gameplay; never overwrite manual IDs; export collisions/spawns to server; World Studio brush/search/validate.

---

## Stages 4–8

Follow master prompt: RPG core → story/PvE → MMO systems → expansion → release. Do not start all at once. Monetization stays dark until GDD bonuses/caps approved.

---

## Cross-cutting standards

- Server is source of truth for money/items/combat (from Stage 2 on).
- UI geometry in `.tscn` + Theme; scripts bind data.
- Locations in `.tscn`; no runtime-only world blobs as edit source.
- Secrets via env only; `assets/LICENSES.md` mandatory.
- Each iteration ends with the master prompt report template.

## Open decisions (do not invent silently)

1. Exact Godot pin (4.3 vs future 4.7.x).
2. Combat tick rate and replication budget.
3. PvP crime/loot specifics; VIP power caps.
4. Final art direction for sprite atlas memory budget on 6 GB devices.

**Safe interim:** 4.3 + procedural 2D originals + local Stage 1 combat; document replacements.
