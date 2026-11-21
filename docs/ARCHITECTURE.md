# GDD-12 - technical baseline (proposed)

## Components
- client/: Godot 4 **3D** world (Lunex Fantasy 3D, owner decision 2026-11-21): 3D terrain/props/characters, 3D camera rig, GPU particle VFX, quality presets; editable .tscn UI. The previous 2D isometric layer is kept as a fallback until the device gate passes (docs/research/3d-migration.md).
- server/: Go authoritative simulation, WebSocket sessions, HTTP auth/content APIs; server validates movement, combat, loot and inventory.
- database/: PostgreSQL schema/migrations.
- admin/: React + TypeScript content and live-ops dashboard.
- tools/: offline Blender sprite export, atlas packing, validation.
- deploy/: separate development Docker Compose stack; do not touch production VPS services.

## Visual content pipeline (Lunex Fantasy 3D)
1. Generate/obtain licensed 3D models (Meshy text/image-to-3D → refine/PBR → auto-rig → animation library); record prompt, task id, date and plan in `client/assets/LICENSES.md`.
2. Import GLB into `client/assets/models/**`; budgets (mobile): hero ≤ 12k tris, monster 4–8k, building 3–6k, tree/prop 0.3–2k; textures 512–1024 px, ASTC/ETC2; shared materials.
3. Character/monster scenes `.tscn` (Skeleton3D + AnimationTree) bound to server entities by a *view adapter* (world_controller → puppet view); gameplay never reads the 3D nodes.
4. Terrain/props are editable `.tscn` with the same layer contract as before (Ground, Paths, Vegetation, Buildings, Decorations, NPCSpawns, MonsterSpawns, Navigation, Effects); server positions map x→X, y→Z.
5. VFX: GPUParticles3D with per-preset budgets; sun rays as cheap fake volumes; presets Low/Medium/High/Ultra change render scale, shadow quality/distance, particle budget, MSAA and vegetation density (`client/scripts/gfx/quality.gd`).
6. Device gate: `tools/android/measure_device.sh` on the owner's phone (iQOO 15R, 12 GB) and a mid-range reference before the 2D fallback is removed.

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
