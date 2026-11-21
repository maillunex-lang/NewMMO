# WORLD-EDITOR-001 C - visual location editing

Approved: each location is a Godot .tscn scene; editable decorations are reusable scenes and can be placed, moved, scaled, and deleted in the Godot 2D editor without GDScript changes.

## Open the prototype
1. Open client/project.godot in Godot 4.7.2.
2. Open client/scenes/locations/dawn_valley.tscn.
3. In Scene dock, select Vegetation/Oak01, move with the 2D move tool and save.
4. To add objects manually, drag tree.tscn, rock.tscn or crystal.tscn from scenes/props into Vegetation or Decorations.
5. Optionally enable Project > Project Settings > Plugins > Lunex World Studio, select a layer and use its dock to add objects.
6. Props are provisional vector placeholders, not final prerendered sprite assets.

## Layer contract
Ground, Paths, Vegetation, Buildings, Decorations, NPCSpawns, MonsterSpawns, Navigation, Effects.

## Server safety
Moving visual objects does not automatically change authoritative collision, navigation or spawn logic. A future validated map-data exporter must synchronize gameplay-relevant coordinates to the server before publishing.

## Next iterations
TileMapLayer terrain painting, reusable collision footprints, navigation baking, sprite asset catalog, search/filter, multi-place brush, map validation and controlled server export.
