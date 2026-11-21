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
| T-001 | research | OSS research: PostgreSQL migrations + Go DB access → `docs/research/db-migrations.md` | — | **DONE** `3fb9aa6` (branch `be9a5bb`, lane_check clean). Director spot-check of upstream `go.mod` at tags confirmed the report's Go-version warning (see ROADMAP EX-P03). Owner OK received 2026-11-21 (Go ≥1.25, goose + pgx) |
| T-002 | research | Godot 4.7.2 upgrade gate (C-GODOT-001 B): compat checklist, export templates, Android → `docs/research/godot-4-7-2-upgrade.md` | — | **ISSUED** (`prompts/T-002.md`). Report only; engine NOT upgraded |
| T-003 | research | OSS research: Go WebSocket libs + protocol patterns for authoritative MMO → `docs/research/go-websocket.md` | — | For EX-P04/P05 |
| T-004 | client | Compliance review of HUD/dialogue scenes + safe fixes | — | **DONE** (merge: see `git log`; branch `aa2ba71`). Director: lane_check clean, @onready paths vs `.tscn` verified statically, client-sync 0 err. **Godot is not installed on the director's machine → smokes are executor-reported**; re-run `./tools/smoke_stage1.sh` locally if in doubt |
| T-007 | client | Quest journal screen (`.tscn`) behind a data adapter + UI strings via `tr()` → `client/translations/ui.csv` | T-004 ✔ | **ISSUED** (`prompts/T-007.md`) |
| T-005 | server | Go server hardening without new deps: config via env, `/healthz` `/readyz`, graceful shutdown, stdlib structured logging, tests | — | **DONE** `b026eab` (branch `272a76f`). Director: vet + `go test -race` green, live run: healthz/readyz/X-Request-Id, SIGTERM clean stop. Follow-up: wrapper lacks Hijacker/Flusher → T-006 |
| T-006 | server | Go directive 1.25.0 + `statusWriter` Hijacker/Flusher/ResponseController (WebSocket-ready), stdlib only | T-005 ✔ | **ISSUED** (`prompts/T-006.md`) |

First parallel batch (different lanes, no dependencies, disjoint files): **T-001 + T-004 + T-005** — issued. Next: T-002/T-003 (research) when a research slot frees; D-001/D-002 by director meanwhile. Merge order when all return: T-001 (docs) → T-005 (server) → T-004 (client); each gated by CI + lane_check.

## Journal

| Date | Event |
|------|-------|
| 2026-11-21 | Protocol, lanes, lane_check, CI `lane-check` created; backlog proposed |
| 2026-11-21 | Owner delegated lanes/git mode; branch-per-task adopted; research-first (+Hysen Labs) made rule 0; T-001/T-004/T-005 prompts issued |
| 2026-11-21 | T-001 reviewed and merged (`3fb9aa6`); T-002 issued to ChatGPT |
| 2026-11-21 | Owner approved Go ≥1.25, goose (CLI) + pgx/v5; CI setup-go → 1.25.x; `server/go.mod` bump deferred to P03 server task (avoid conflict with T-005) |
| 2026-11-21 | T-005 reviewed and merged (`b026eab`); T-006 issued to Codex |
| 2026-11-21 | T-004 reviewed and merged; T-007 issued to Cursor |
