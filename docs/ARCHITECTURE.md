# GDD-12 - technical baseline (proposed)

## Components
- client/: Godot 4 2D, isometric TileMap/2D terrain, directional animated sprite actors, 2D VFX, editable .tscn UI.
- server/: Go authoritative simulation, WebSocket sessions, HTTP auth/content APIs; server validates movement, combat, loot and inventory.
- database/: PostgreSQL schema/migrations.
- admin/: React + TypeScript content and live-ops dashboard.
- tools/: offline Blender sprite export, atlas packing, validation.
- deploy/: separate development Docker Compose stack; do not touch production VPS services.

## Visual content pipeline
1. Create/obtain a licensed 3D source model (e.g. Meshy), rig and animate.
2. In Blender render fixed camera, consistent lighting and 8 directional views to transparent frame sequences.
3. Pack animations to atlas with metadata (direction, frame, duration, pivot, hit origin); optimize compression and memory.
4. Import into Godot AnimatedSprite2D or custom 2D actor scene; VFX remain separate 2D layers.
5. Keep Blender sources outside runtime APK; test frame memory on 6 GB Android.

## UI editor contract
Each full-screen panel is a dedicated .tscn with Control root. Shared buttons/cards/headers are instanced scenes; global colors/fonts in Theme resources. Use Containers for responsive flow and explicit anchors for draggable HUD elements. Godot Inspector must be sufficient for normal appearance/layout edits. Runtime custom HUD profiles are a separate user-facing feature. Automated scene checks should catch missing references.

## Network and security
Authoritative server tick and interest management per region/channel; send nearby actor deltas, not whole world. Authenticated sessions, rate limits, transactional currency/item updates, idempotent purchases, anti-cheat logs. Never expose database or admin ports publicly. Client reconnection and version compatibility must be planned.

## First vertical slice - acceptance criteria
- Godot Android landscape app opens with an editable main HUD scene.
- One test zone and one 2.5D animated character, controlled by joystick and tap/autopath.
- One NPC with non-skippable important dialogue, one quest with progress and reward.
- Three enemy archetypes; auto-combat and manual skill use; loot saved server-side.
- Two test accounts can see each other with server-validated positions.
- Go + PostgreSQL + WebSocket run in an isolated development environment.
- Reconnect preserves character, quest and inventory state.
- Runs acceptably on a 6 GB Android test device; measure actual memory, frame time and network traffic.

## Not part of first slice
Massive guild wars, economy-wide auction, monetization, all 42 screens, all 5 playable classes, production-scale world. These remain planned features.
