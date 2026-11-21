# Lunex MMORPG roadmap

Status legend: [ ] planned, [x] completed.

## Phase 0 - project foundation
- [x] GDD-01 through GDD-11 decisions collected in conversation and summarized in docs/GDD.md.
- [x] GDD-12 initial technical architecture baseline documented.
- [ ] Confirm unresolved architecture decisions and production budgets.
- [ ] Create editable Godot 4 project skeleton and UI component library.
- [ ] Create Go server skeleton, PostgreSQL migrations, isolated Compose dev environment.
- [ ] Add lint/build/test CI.

## Phase 1 - playable vertical slice
- [ ] One zone with camera and collision.
- [ ] One 8-direction prerendered character and 2D VFX.
- [ ] Joystick, tap movement and autopath.
- [ ] Editable HUD and quest dialogue scenes.
- [ ] Three enemy types and authoritative combat/loot.
- [ ] Two online test clients, persistence and reconnect.
- [ ] Android APK test on 6 GB device.

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
