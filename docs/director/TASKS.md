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
| D-001 | director | Content export contract + DB contract + toolchain pins in `docs/contracts/` | EX-P02 ✔ | **DONE** — `content-export.md`, `db-content.md`, `toolchain-pins.md`; exporter `tools/content/export.py` (deterministic, blocked by lint errors); CI jobs `server-db` + export step |
| T-009 | server | EX-P03: goose migrations + loader (pgx) → `server/internal/content` + `contentctl` CLI + isolated Compose `deploy/newmmo/` | D-001 ✔, T-006 ✔ | **ISSUED** (`prompts/T-009.md`). Phase 2 (tools approved). CI `server-db` runs it |
| D-002 | director | POI id mapping contract (stub `POI-BOARD-01` … → canon `POI-01-*`, C-POI-001) | — | Needed by client EX-P06/07 |
| T-001 | research | OSS research: PostgreSQL migrations + Go DB access → `docs/research/db-migrations.md` | — | **DONE** `3fb9aa6` (branch `be9a5bb`, lane_check clean). Director spot-check of upstream `go.mod` at tags confirmed the report's Go-version warning (see ROADMAP EX-P03). Owner OK received 2026-11-21 (Go ≥1.25, goose + pgx) |
| T-002 | research | Godot 4.7.2 upgrade gate → `docs/research/godot-4-7-2-upgrade.md` | — | **DONE** (branch `387d821`, lane_check clean). Director spot-checked: smoke script `|| true` + no MQ01/stage1 runs, MQ01 marker before xp check, `include_filter=""` — all **confirmed** → T-008. Recommendation: keep 4.3 until Android toolchain (B-APK) + 4.3 baseline on 6 GB device |
| T-003 | research | OSS research: Go WebSocket libs + protocol for authoritative MMO → `docs/research/go-websocket.md` | — | **ISSUED** (`prompts/T-003.md`) |
| T-008 | client | Strict smoke script + JSON in APK export filter (Extra-Write `tools/smoke_stage1.sh`) | T-007 (same agent, sequential) | **QUEUED** (`prompts/T-008.md`) — give to Cursor after T-007 |
| T-004 | client | Compliance review of HUD/dialogue scenes + safe fixes | — | **DONE** (merge: see `git log`; branch `aa2ba71`). Director: lane_check clean, @onready paths vs `.tscn` verified statically, client-sync 0 err. **Godot is not installed on the director's machine → smokes are executor-reported**; re-run `./tools/smoke_stage1.sh` locally if in doubt |
| T-007 | client | Quest journal screen (`.tscn`) behind a data adapter + UI strings via `tr()` → `client/translations/ui.csv` | T-004 ✔ | **ISSUED** (`prompts/T-007.md`) |
| T-005 | server | Go server hardening without new deps: config via env, `/healthz` `/readyz`, graceful shutdown, stdlib structured logging, tests | — | **DONE** `b026eab` (branch `272a76f`). Director: vet + `go test -race` green, live run: healthz/readyz/X-Request-Id, SIGTERM clean stop. Follow-up: wrapper lacks Hijacker/Flusher → T-006 |
| T-006 | server | Go directive 1.25.0 + `statusWriter` Hijacker/Flusher/ResponseController (WebSocket-ready), stdlib only | T-005 ✔ | **DONE** (branch `f2516ec`). Director re-ran on Go 1.25.0: vet + `go test -race` green; go.mod only `go 1.25.0`, no requires |

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
| 2026-11-21 | T-002 reviewed and merged; T-003 issued to ChatGPT; T-008 queued for Cursor |
| 2026-11-21 | T-006 reviewed and merged; Codex idle until EX-P03 (needs owner «делай» + D-001) |
| 2026-11-21 | Owner «делай» → EX-P03 started: D-001 contracts + exporter + CI `server-db`; T-009 issued to Codex |
