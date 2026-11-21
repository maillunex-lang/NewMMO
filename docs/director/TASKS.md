# Task board (director is the only writer)

Status: `PROPOSED` (not issued) · `ISSUED` (prompt given to executor) · `REVIEW` · `DONE <sha>` · `REWORK` · `BLOCKED`.
Prompts live in `docs/director/prompts/T-NNN.md`. Protocol: `DIRECTOR.md`.

## Open questions for the owner (answer once; director then issues prompts)

1. **Real access per executor.** Which of Cursor / ChatGPT / Codex can `git push` to `main` of this repo? (If ChatGPT chat cannot — owner commits its files with `Lane: research`.)
2. **Godot** availability on the owner's/Cursor machine (headless smoke `./tools/smoke_stage1.sh`)? Go toolchain (1.22+) for Codex?
3. Confirm lane split in `docs/director/lanes.json` (client=Cursor, server=Codex, research=ChatGPT) or reassign.
4. Confirm: executors push straight to `main` (per AGENTS.md) — or switch to PR-per-task + branch protection (safer for 3 parallel agents; needs owner to change AGENTS.md rule).

## Backlog (all PROPOSED; none issued)

| ID | Lane | Title | Depends | Notes |
|----|------|-------|---------|-------|
| D-001 | director | Content export contract (`declarations.json`/`crossrefs.json` → loader tables) in `docs/contracts/` | EX-P02 ✔ | Unblocks T-005…; director does it |
| D-002 | director | POI id mapping contract (stub `POI-BOARD-01` … → canon `POI-01-*`, C-POI-001) | — | Needed by client EX-P06/07 |
| T-001 | research | OSS research: PostgreSQL migrations + Go DB access (goose / golang-migrate / atlas / sqlc / pgx) → `docs/research/db-migrations.md` | — | Required by EX-P03; **no install** until owner OK |
| T-002 | research | Godot 4.7.2 upgrade gate (C-GODOT-001 B): compat checklist, export templates, Android → `docs/research/godot-4-7-2-upgrade.md` | — | Report only; engine NOT upgraded |
| T-003 | research | OSS research: Go WebSocket libs + protocol patterns for authoritative MMO → `docs/research/go-websocket.md` | — | For EX-P04/P05 |
| T-004 | client | Compliance review of existing HUD/dialogue/quest-log scenes (editable `.tscn`, Theme, landscape safe-area) + fix violations | — | Cursor; may not touch `client/data/**` |
| T-005 | server | Go server hardening without new deps: config via env, `/healthz` `/readyz`, graceful shutdown, stdlib structured logging, tests | — | Codex; no Postgres yet (EX-P03 first) |

Suggested first parallel batch (different lanes, no dependencies): **T-001 + T-004 + T-005**, then T-002/T-003; D-001/D-002 by director meanwhile.

## Journal

| Date | Event |
|------|-------|
| 2026-11-21 | Protocol, lanes, lane_check, CI `lane-check` created; backlog proposed |
