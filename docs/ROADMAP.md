# Lunex MMORPG roadmap

Status legend: [ ] planned, [x] completed (implemented **and** tested).

## Phase 0 - project foundation

- [x] GDD-01 through GDD-11 decisions collected in conversation and summarized in docs/GDD.md.
- [x] GDD-12 initial technical architecture baseline documented.
- [x] Technical audit (`docs/AUDIT.md`) and implementation plan (`docs/IMPLEMENTATION_PLAN.md`).
- [ ] Confirm unresolved architecture decisions and production budgets.
- [x] Editable Godot 4 project skeleton with Stage 1 HUD theme (component library still thin).
- [x] Go server skeleton, PostgreSQL migrations, isolated Compose dev environment and REST auth/WebSocket transport (EX-P03, T-018, T-019); quest engine still pending.
- [x] Add lint/build/test CI (server tests + file presence checks).

## Phase 1 - playable vertical slice

- [x] One zone with camera and collision (Dawn Valley Stage 1).
- [x] One 8-direction character sheet + 2D interim art (procedural; Blender pipeline pending).
- [x] Joystick, tap movement (autopath to quests still basic tap-to-point).
- [x] Editable HUD and quest dialogue scenes.
- [x] Three enemy types and **local** combat/loot (server authority = Stage 2).
- [ ] Two online test clients, quest persistence and reconnect (T-020/T-023 pending; WebSocket transport T-019 complete).
- [ ] Android APK test on 6 GB device (APK build tooling exists; real-device performance metrics pending).

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

## Current execution status (2026-11-21)

- [x] EX-P00–P03: audit, content tooling/registry, DB loader/migrations.
- [x] T-018 REST accounts/tickets; T-019 WebSocket server (PR #2, merge `324bd2d`, CI #155); T-022 login/characters UI.
- [~] EX-P04: T-020 authoritative quests/reward ledger is next, followed by T-021 security review and T-023 live client wiring.
- [ ] EX-P05–P13: not completed; see `docs/ROADMAP_EXECUTION.md`.
- **Director:** Cursor, delegated by owner 2026-11-21, subject to owner-approval gates in `AGENTS.md`.
