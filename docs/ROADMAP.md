# Lunex MMORPG roadmap

Status legend: [ ] planned, [x] completed (implemented **and** tested).

## Phase 0 - project foundation

- [x] GDD-01 through GDD-11 decisions collected in conversation and summarized in docs/GDD.md.
- [x] GDD-12 initial technical architecture baseline documented.
- [x] Technical audit (`docs/AUDIT.md`) and implementation plan (`docs/IMPLEMENTATION_PLAN.md`).
- [ ] Confirm unresolved architecture decisions and production budgets.
- [x] Editable Godot 4 project skeleton with Stage 1 HUD theme (component library still thin).
- [ ] Create Go server skeleton beyond `/healthz`, PostgreSQL migrations, isolated Compose dev environment.
- [x] Add lint/build/test CI (server tests + file presence checks).

## Phase 1 - playable vertical slice

- [x] One zone with camera and collision (Dawn Valley Stage 1).
- [x] One 8-direction character sheet + 2D interim art (procedural; Blender pipeline pending).
- [x] Joystick, tap movement (autopath to quests still basic tap-to-point).
- [x] Editable HUD and quest dialogue scenes.
- [x] Three enemy types and **local** combat/loot (server authority = Stage 2).
- [ ] Two online test clients, persistence and reconnect.
- [ ] Android APK test on 6 GB device (export templates / SDK not available in current CI image).

## Phase 2 - game systems

- [ ] Five classes and specializations.
- [ ] Parties of five, role-aware matching, dungeons.
- [ ] Inventory, crafting professions, auction and player trade.
- [ ] Guilds, crime/PvP, dynamic world events.
- [ ] Extended story campaign and regions.

## Phase 3 - live operations

- [ ] Territory ownership and wars.
- [ ] VIP/monetization rules with balance tests.
- [ ] Admin content editor, telemetry, moderation, backups.
- [ ] Load/security/performance testing and staged release.

No task is considered complete until implemented and tested, not merely documented.
