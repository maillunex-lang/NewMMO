# NewMMO technical audit

**Date:** 2026-11-21  
**Repo:** `maillunex-lang/NewMMO` @ `main` (pre-Stage-1 baseline)  
**Auditor:** Cursor Cloud Agent  
**Godot used for checks:** 4.3.stable (project `config/features` = 4.3; master prompt cites 4.7.2 — treat as target/typo until a newer binary is pinned)

## Executive summary

The repository is an early **preproduction skeleton**, not a playable MMORPG. Documentation (GDD, architecture, roadmap, world-editor notes) is ahead of code. The client opens a tap-to-move polygon “player” on a polygon ground with a few placeholder props and a non-wired HUD. The Go server only exposes `/healthz`. There is no database, admin app, CI, asset pipeline, combat, quests, networking beyond health, or Android export toolchain in this environment.

## Inventory

| Area | Present? | Real status |
|------|----------|-------------|
| `docs/GDD.md` | Yes | Approved decisions recorded |
| `docs/ARCHITECTURE.md` | Yes | Proposed baseline |
| `docs/ROADMAP.md` | Yes | Phase 0 mostly unchecked beyond GDD text |
| `docs/WORLD_EDITOR.md` | Yes | Matches current prop/plugin intent |
| Godot client project | Yes | Minimal; features 4.3 / GL Compatibility |
| Location `.tscn` (Dawn Valley) | Yes | Layers exist; content is Polygon2D placeholders |
| Reusable props | Yes | Tree / rock / crystal polygons + `placeable_prop.gd` |
| Lunex World Studio plugin | Yes | Dock adds 3 props; no search/brush/validation |
| HUD `.tscn` | Yes | Layout only; no scripts / theme / joystick logic |
| Player / camera / movement | Partial | Tap/mouse move only; diamond Polygon2D |
| Characters / animations | No | No spritesheets, no AnimatedSprite2D |
| NPC / dialogue / quests | No | Empty spawn layers |
| Combat / monsters / loot | No | Absent |
| Autopath / auto-combat | No | Absent |
| Go server | Stub | `/healthz` JSON only on `127.0.0.1:18086` |
| PostgreSQL / migrations | No | `database/` missing |
| WebSocket game session | No | Absent |
| Admin React app | No | `admin/` missing |
| Tools (Blender/atlas) | No | `tools/` missing |
| Deploy / Compose | No | `deploy/` missing |
| CI workflows | No | `.github/workflows/` missing |
| `assets/LICENSES.md` | No | Missing |
| Android APK | Config only | `export_presets.cfg` present; no templates/SDK here |

## Client findings

### What works (with caveats)

1. **Project boots** with `run/main_scene = res://scenes/world/main.tscn`.
2. **Dawn Valley** has the required layer names: Ground, Paths, Vegetation, Buildings, Decorations, NPCSpawns, MonsterSpawns, Navigation, Effects.
3. **Props** are editable PackedScenes with Inspector exports (`blocks_movement`, `server_object_id`, `display_name`).
4. **Tap / left-click** moves a Node2D toward a world point (`scripts/world.gd`).
5. **HUD** is a Control scene (editable in Godot), not built purely in code.
6. **World Studio** can instance tree/rock/crystal onto a selected Node2D with undo.

### Stubs / placeholders

- All visuals are **Polygon2D** shapes — explicitly provisional per `WORLD_EDITOR.md`.
- HUD buttons (skills, MOVE) have **no behavior**.
- Joystick is a labeled Button, not a virtual stick.
- Plugin is **not enabled** in `project.godot` (`[editor_plugins]` missing).
- No collision shapes, Y-sort, navigation baking, or TileMap terrain.
- No character class, stats, inventory, or save data on client.

### Broken / risky

- Plugin `set_owner` via undo-redo `add_do_method` is fragile; owner may not stick after undo/redo cycles.
- `game_world.gd` (formerly world.gd) uses `get_global_mouse_position()` for touch — OK for single pointer, but conflicts with future UI mouse_filter handling.
- Godot feature string is **4.3**, not 4.7.2; export/Android not verified.
- No `.godot` import cache committed (correct); first open needs editor import.

## Server findings

- Module path: `github.com/maillunex-lang/NewMMO/server`, Go 1.24 in `go.mod` (environment has Go 1.22.2 — version mismatch risk).
- Single endpoint `/healthz`; binds loopback only (good for local stub).
- No auth, WebSocket, persistence, combat validation, or rate limits.

## Missing packages / tooling (this Cloud VM)

- Godot was **not** preinstalled; 4.3.stable installed under `$HOME/godot` for this run.
- No Android SDK / export templates observed → **debug APK cannot be produced here** until toolchain is added.
- No Blender / Meshy assets → Stage 1 uses **procedural original 2D sprites** as interim art (see `assets/LICENSES.md`), not final prerendered Blender atlases.
- Pillow installed for asset generation tooling.

## VPS / deploy

- No deploy manifests in repo. Master prompt requires isolated NewMMO paths on VPS Lunex — **not audited against live VPS** (no access assumed; do not touch other services).

## Contradictions / open questions

1. **Godot version:** prompt 4.7.2 vs project 4.3 — recommend pinning an installable stable and updating docs.
2. **Go version:** `go.mod` 1.24 vs VM 1.22.2 — lower `go` directive or upgrade toolchain.
3. **Monetization / PvP crime rules** still unresolved in GDD — correctly deferred.
4. Vertical slice in `ARCHITECTURE.md` includes online sync; master prompt Stage 1 is visual-first, Stage 2 online — **follow stage order**.

## Baseline metrics (pre-Stage-1)

| Metric | Value |
|--------|-------|
| Tracked source-ish files | ~15 (gd/tscn/go/md) |
| Client GDScript LOC | ~80 |
| Server Go LOC | ~21 |
| Locations | 1 (placeholder) |
| Playable classes | 0 |
| Automated tests | 0 |
| APK | Not built |

## Verdict

**Stage 0 incomplete at audit time.** Proceed to document an implementation plan, then deliver **Stage 1**: one visually coherent editable Dawn Valley, animated player, joystick/camera, NPC+quest, three monsters, local combat/loot, themed HUD — with honest stubs called out where online authority is still Stage 2.

## Post-iteration note (2026-11-21)

Stage 1 client slice landed on branch `cursor/stage1-vertical-slice-6dd6`: Dawn Valley sprites + props, Guardian 8-dir sheet, joystick/HUD/dialogue, Elder Mira prologue quest, slime/wolf/bandit local combat, World Studio search palette, CI + headless smoke. Still stubs: authoritative multiplayer, Postgres, APK toolchain, Blender atlas pipeline, admin app.
