# Lunex MMORPG (NewMMO)

Landscape Android online fantasy MMORPG with a 2.5D isometric world, prerendered animated characters, and editable Godot 4 UI.

## Status
Preproduction completed through GDD-11. GDD-12 technical architecture is a baseline proposal. Development starts with a small vertical slice, not the full MMO.

## Non-negotiable constraints
- Godot 4 2D client; no runtime 3D characters.
- Animated directional spritesheets exported offline from Blender (source models may come from Meshy).
- All major UI screens built as editable Godot Control scenes (.tscn), with reusable components and Theme resources. Do not generate the primary UI entirely in GDScript.
- Android landscape, responsive safe areas and customizable HUD, target 6 GB RAM devices.
- Authoritative Go server, PostgreSQL, WebSocket, React/TypeScript admin.
- Never trust the client for combat outcomes, currency, inventory or movement authorization.
- Keep development assets, credentials and production services separate.

## Documentation
- [Approved design decisions](docs/GDD.md)
- [Technical architecture and vertical slice](docs/ARCHITECTURE.md)
- [Implementation roadmap](docs/ROADMAP.md)

## First milestone
One region, one playable class, movement, one NPC, quests, a few enemies, skill combat, loot and live server synchronization. Other classes and MMO systems are planned, not cut from the game.

## License
No license has been selected. Public source visibility does not by itself grant reuse rights.
