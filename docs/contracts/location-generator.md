# Contract: location generator (EX-W01) — DRAFT

**Owner:** director · **Status:** DRAFT / not implemented · **Plan:** `docs/director/LOCATION_GENERATOR_PLAN.md`  
**Research:** `docs/research/location-procgen-isometric.md`

This file exists so roadmap links resolve. Full field schema lands in D-005 before T-032.

## 1. Output

- Target: editable Godot **`.tscn`** under `client/scenes/locations/`.
- Write only under `Generated/*` layers (`Ground`, `Paths`, `Vegetation`, `Buildings`, `Decorations` as applicable).
- Never overwrite `Manual/*` or nodes tagged manual / stable Manual IDs.
- Generator runs **editor-only** (not on Android runtime).

## 2. Seed (TBD in D-005)

Placeholder: `{ seed: int, biome_id: string, bounds: {w,h}, forbidden_masks: [] }`.

## 3. Acceptance (must hold when implemented)

1. Move Manual prop → regenerate → Manual prop unchanged.
2. Open scene in Godot 4.7.2 → move/scale/delete Generated or Manual → save persists.
3. No new third-party dependency without owner OK.
