# Task board (director is the only writer)

Status: `PROPOSED` (not issued) · `ISSUED` (prompt given to executor) · `REVIEW` · `DONE <sha>` · `REWORK` · `BLOCKED`.
Prompts live in `docs/director/prompts/T-NNN.md`. Protocol: `DIRECTOR.md`.

## Owner decisions (2026-11-21)

1. All three executors can push to this repo. Godot and Go toolchains are available.
2. Lane split delegated to director → as in `lanes.json` (Cursor=client, Codex=server, ChatGPT=research).
3. Git mode delegated to director → **branch-per-task `agent/<lane>/T-NNN`; only the director opens PRs and merges to `main`** (AGENTS.md updated).
4. Executors must **search existing solutions first** (GitHub, Hysen Labs https://hysenlabs.com/projects, Godot Asset Library, official docs) and record it in `docs/research/T-NNN-*.md`; nothing third-party is installed before owner OK.
5. Pending for the owner (not blocking): GitHub → Settings → Branches → protect `main` (require PR + `ci` checks).

## Backlog (all PROPOSED; none issued)

| ID | Lane | Title | Depends | Notes |
|----|------|-------|---------|-------|
| D-001 | director | Content export contract (`declarations.json`/`crossrefs.json` → loader tables) in `docs/contracts/` | EX-P02 ✔ | Unblocks T-005…; director does it |
| D-002 | director | POI id mapping contract (stub `POI-BOARD-01` … → canon `POI-01-*`, C-POI-001) | — | Needed by client EX-P06/07 |
| T-001 | research | OSS research: PostgreSQL migrations + Go DB access → `docs/research/db-migrations.md` | — | **DONE** `3fb9aa6` (branch `be9a5bb`, lane_check clean). Director spot-check of upstream `go.mod` at tags confirmed the report's Go-version warning (see ROADMAP EX-P03). Awaiting owner OK on tool choice + Go upgrade |
| T-002 | research | Godot 4.7.2 upgrade gate (C-GODOT-001 B): compat checklist, export templates, Android → `docs/research/godot-4-7-2-upgrade.md` | — | **ISSUED** (`prompts/T-002.md`). Report only; engine NOT upgraded |
| T-003 | research | OSS research: Go WebSocket libs + protocol patterns for authoritative MMO → `docs/research/go-websocket.md` | — | For EX-P04/P05 |
| T-004 | client | Compliance review of existing HUD/dialogue/quest-log scenes (editable `.tscn`, Theme, landscape safe-area) + fix violations | — | **ISSUED** (`prompts/T-004.md`). Cursor; may not touch `client/data/**`; research note first |
| T-005 | server | Go server hardening without new deps: config via env, `/healthz` `/readyz`, graceful shutdown, stdlib structured logging, tests | — | **ISSUED** (`prompts/T-005.md`). Codex; stdlib only; no Postgres yet |

First parallel batch (different lanes, no dependencies, disjoint files): **T-001 + T-004 + T-005** — issued. Next: T-002/T-003 (research) when a research slot frees; D-001/D-002 by director meanwhile. Merge order when all return: T-001 (docs) → T-005 (server) → T-004 (client); each gated by CI + lane_check.

## Journal

| Date | Event |
|------|-------|
| 2026-11-21 | Protocol, lanes, lane_check, CI `lane-check` created; backlog proposed |
| 2026-11-21 | Owner delegated lanes/git mode; branch-per-task adopted; research-first (+Hysen Labs) made rule 0; T-001/T-004/T-005 prompts issued |
| 2026-11-21 | T-001 reviewed and merged (`3fb9aa6`); T-002 issued to ChatGPT |
