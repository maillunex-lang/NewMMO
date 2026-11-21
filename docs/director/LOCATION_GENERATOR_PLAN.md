# Plan: hybrid location generator (editable in Godot)

**Status:** SCHEDULED (owner 2026-11-21) · **Not started** · Research: `docs/research/location-procgen-isometric.md`  
**Invariant:** generator output is always a normal editable Godot `.tscn`. You open the project on your PC, move/scale/delete props in the 2D editor / World Studio, save — changes stick. Regeneration must **not** overwrite `Manual/*` or nodes you marked manual.

## 1. Why this exists

Stage-1 Dawn Valley is placeholder art + hand placement. We need a **hybrid** tool: seed fills terrain/vegetation/paths into `Generated/*`, then you polish details in Godot. This was deferred as decision **D**; now scheduled as stage **EX-W01** (does not replace EX-P08…P13 gameplay order).

## 2. What you will do on your computer

1. Open `client/project.godot` in Godot **4.7.2**.
2. Open `client/scenes/locations/<loc>.tscn`.
3. Run **Lunex Generate** (editor dock / menu — to be built) with seed + biome params → fills `Generated/Ground|Paths|Vegetation|…`.
4. Edit freely: move trees, place cottages, fix paths under `Manual/` or by converting a generated node to Manual.
5. Save `.tscn`. Re-run generate → Manual placements stay; only Generated rebuilds (diff/preview before apply).
6. (Later) export collision/spawns/nav for the Go server — separate gate, not required for visual editing.

## 3. Non-negotiables

| Rule | Meaning |
|------|---------|
| Editable `.tscn` | No opaque runtime-only blob as the edit source |
| Manual survives regen | Stable IDs; acceptance test already in `WORLD_EDITOR.md` |
| Editor-only gen | No heavy WFC/Gaea on Android at runtime |
| 2D isometric | No runtime 3D world |
| No silent OSS install | Spike may recommend Gaea / WFC / in-house; **owner OK** before vendoring |

## 4. Phased delivery (EX-W01)

| Phase | Deliverable | Owner gate? |
|-------|-------------|-------------|
| **W1** | Director contract `docs/contracts/location-generator.md` — layers, seed schema, Manual rules, regen API | No |
| **W2** | Tool choice confirmed (in-house vs WFC vs Gaea on 4.7.2) + short spike report | **Yes** if new dependency |
| **W3** | Editor generate → `Generated/*` for one LOC (Dawn Valley / LOC-01-001 scale) | No (if in-house); Yes if dep |
| **W4** | Regen-safe Manual test + World Studio polish pass documented for owner PC | No |
| **W5** | Optional: map-data export stub for server (collision/spawns) | Separate from look |

Recommended start: **after EX-P08 loot/equip is playable enough**, or on owner «делай генератор» anytime — gameplay and world-gen can proceed on separate lanes if files don’t collide.

## 5. Task stubs (PROPOSED — not issued while paused)

| ID | Lane | Title |
|----|------|-------|
| D-005 | director | Contract `location-generator.md` + ROADMAP checkbox EX-W01 |
| T-031 | research | Re-verify Gaea / WFC / in-house on Godot **4.7.2**; recommend one |
| T-032 | client | Implement Lunex Generate dock (Generated layers only) |
| T-033 | client | Manual-survive regen test + Dawn Valley generate pass |
| T-034 | client(+server later) | Map export stub (optional, after look is OK) |

## 6. Out of scope for EX-W01

Final Blender 8-dir art bible, full REG-02…06 mass gen, runtime streaming of generated chunks, installing third-party code before owner OK.
