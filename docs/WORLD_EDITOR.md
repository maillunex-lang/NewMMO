# WORLD-EDITOR-001 C - visual location editing

Approved: each location is a Godot `.tscn` scene; editable decorations are reusable scenes and can be placed, moved, scaled, and deleted in the Godot 2D editor without GDScript changes.

## Open the Stage 1 Dawn Valley

1. Open `client/project.godot` in Godot **4.3** (project feature pin; master prompt 4.7.2 not available yet).
2. Open `client/scenes/locations/dawn_valley.tscn`.
3. In the Scene dock, select `Vegetation/Oak01`, move with the 2D move tool and save.
4. Drag props from `scenes/props/` (`tree`, `rock`, `crystal`, `cottage`, `lantern`, `fence`) into `Vegetation`, `Buildings`, or `Decorations`.
5. Enable **Project → Project Settings → Plugins → Lunex World Studio** (enabled by default in `project.godot`). Use the dock search + **Add to selected layer**.
6. Stage 1 sprites are **original procedural PNGs** (see `client/assets/LICENSES.md`), interim stand-ins for the future Blender 8-dir pipeline.

## Layer contract

`Ground`, `Paths`, `Vegetation`, `Buildings`, `Decorations`, `NPCSpawns`, `MonsterSpawns`, `Navigation`, `Effects`, plus `Manual` / `Generated` markers for the future hybrid generator.

## Server safety

Moving visual objects does not automatically change authoritative collision, navigation or spawn logic. A validated map-data exporter (Stage 3) must synchronize gameplay-relevant coordinates to the server before publishing.

## Acceptance checks (editors)

1. Move a tree in `dawn_valley.tscn`, save, run main scene — tree stays moved.
2. Add a decoration via World Studio library.
3. Change HUD button text/color in `scenes/ui/hud.tscn` without editing GDScript layout code.

## Next iterations

TileMapLayer terrain painting, reusable collision footprints, navigation baking, sprite asset catalog thumbnails, multi-place brush, map validation, controlled server export, Manual-safe regeneration tests.
