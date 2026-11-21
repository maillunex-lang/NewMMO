# Contract: Location generator (EX-W01 / D-005)

Status: **v1 (in-house, no third-party dependency)**. Code: `client/addons/lunex_gen/` (`lunex_gen.gd`, `cli.gd`).

## 1. Purpose
Fill an ordinary, hand-editable location `.tscn` with trails, trees/pines and rocks — **without ever creating a quest dead end**.

## 2. Manual vs Generated
- Generated content lives only under `<Layer>/Generated` (`Paths`, `Vegetation`, `Decorations`), names `Gen_*`, meta `gen_id`.
- **Everything else is Manual** (NPCs, interactables, buildings, hand-placed props). The generator never edits, moves or deletes it; it only treats it as an obstacle/anchor.
- Regeneration clears and rebuilds only `Generated`. Same seed + same Manual content ⇒ identical placement (node `unique_id`s may differ in the file).
- To keep a generated object permanently: drag it out of `Generated` (becomes Manual).

## 3. Scene metadata (root)
`lunex_bounds=[x,y,w,h]`, `lunex_spawn=[x,y]` (player spawn, mirrors the server zone), `lunex_gen={seed,trees,pine_ratio,rocks,forest_threshold,trails}`.

## 4. No-dead-end guarantee
Anchors: player spawn, every `NPCActor` (`npc_id`), every `Interactable` (`interact_id`, incl. dungeon entrances), monster spawns.
1. Occupancy grid (16 px cells) from all blocking `StaticBody2D` shapes inflated by player clearance (22 px).
2. Props are never placed within 110 px of an anchor or 56 px of a trail; trails (Prim MST over anchors, A* around Manual blockers) connect all anchors.
3. **Repair:** after generation `verify` floods from the spawn; while any anchor is cut off, the nearest *generated* blocker is removed. If the cut-off is caused by Manual objects only, generation fails (exit 1) and reports the anchors.
4. `verify` additionally fails when a quest objective target in `res://data/quests/*.json` has no node in the scene.
5. CI runs `mapgen_smoke.gd` (8 seeds at 3× density, determinism, Manual untouched, Manual wall detected) and `verify` on Dawn Valley.

## 5. CLI
`godot --headless --path client -s res://addons/lunex_gen/cli.gd -- <verify|generate|dry-run|export> <scene> [seed=N] [out=path]`
`verify` exit 1 on dead end / missing target; `generate` rewrites the scene; `dry-run` reports only; `export` writes `{circles,rects,bounds,player_clearance}` obstacle data for server movement (server consumption = later task).

## 6. Out of scope (v1)
Biome-aware ground textures, water/cliffs, buildings, in-editor menu plugin (use CLI), server obstacle enforcement.
