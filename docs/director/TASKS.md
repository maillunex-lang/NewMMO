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
| D-003 | director | WebSocket protocol contract v1 (`docs/contracts/websocket-protocol.md` + envelope schema) | T-003 ✔, owner OK ✔ | **DONE** — canon §15.2 events/directions kept (tested), added session/state/error messages, idempotency, limits, close codes; pins updated (coder/websocket v1.8.15) |
| T-009 | server | EX-P03: goose migrations + loader (pgx) → `server/internal/content` + `contentctl` CLI + isolated Compose `deploy/newmmo/` | D-001 ✔, T-006 ✔ | **ISSUED** (`prompts/T-009.md`). Phase 2 (tools approved). CI `server-db` runs it |
| D-002 | director | POI id mapping contract (stub `POI-BOARD-01` … → canon `POI-01-*`, C-POI-001) | — | Needed by client EX-P06/07 |
| T-001 | research | OSS research: PostgreSQL migrations + Go DB access → `docs/research/db-migrations.md` | — | **DONE** `3fb9aa6` (branch `be9a5bb`, lane_check clean). Director spot-check of upstream `go.mod` at tags confirmed the report's Go-version warning (see ROADMAP EX-P03). Owner OK received 2026-11-21 (Go ≥1.25, goose + pgx) |
| T-002 | research | Godot 4.7.2 upgrade gate → `docs/research/godot-4-7-2-upgrade.md` | — | **DONE** (branch `387d821`, lane_check clean). Director spot-checked: smoke script `|| true` + no MQ01/stage1 runs, MQ01 marker before xp check, `include_filter=""` — all **confirmed** → T-008. Recommendation: keep 4.3 until Android toolchain (B-APK) + 4.3 baseline on 6 GB device |
| T-003 | research | Go WebSocket libs + protocol → `docs/research/go-websocket.md` | — | **DONE** (branch `d21b375`; go.mod at tags re-verified by director: coder/websocket v1.8.15 → go 1.23; gorilla v1.5.3 → 1.12; gobwas/ws v1.5.0 and centrifuge v0.40.1 → 1.26). Recommendation: coder/websocket v1.8.15 (+ gorilla fallback), JSON v1, no compression, ticket auth, request_id idempotency. Note: report predates T-006 (statusWriter now implements Hijacker/Flusher). Needs **owner OK** for the dependency + director WS contract (D-003) before EX-P04/P05 |
| T-010 | research | Independent review of EX-P03 contracts + exporter → `docs/reviews/T-010-contracts-review.md` | D-001 ✔ | **DONE** (branch `dd2177d`; 13 findings, 2 blockers). All applied by director in contracts **rev 1.1** + exporter change (BUILD_ID/raw-bytes manifest); Codex gets `prompts/T-009-amendment-1.md`. Reviewer could not run tests (no checkout) — director ran them |
| T-008 | client | Strict smoke script + JSON in APK export filter (Extra-Write `tools/smoke_stage1.sh`) | T-007 ✔ | **ISSUED** (`prompts/T-008.md`) to Cursor |
| T-004 | client | Compliance review of HUD/dialogue scenes + safe fixes | — | **DONE** (merge: see `git log`; branch `aa2ba71`). Director: lane_check clean, @onready paths vs `.tscn` verified statically, client-sync 0 err. **Godot is not installed on the director's machine → smokes are executor-reported**; re-run `./tools/smoke_stage1.sh` locally if in doubt |
| T-007 | client | Quest journal screen (`.tscn`) behind a read-only data adapter + UI strings via `tr()` → `client/translations/ui.csv` | T-004 ✔ | **DONE** (branch `f543e32`). Director: lane_check clean; node paths vs `.tscn` OK; adapter read-only; `project.godot` only `[internationalization]`; client-sync 0 err; smokes executor-reported |
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
| 2026-11-21 | T-003 reviewed and merged; T-010 (contracts review) issued to ChatGPT; D-003 WS contract awaits owner OK on coder/websocket + JSON v1 |
| 2026-11-21 | T-007 reviewed and merged; T-008 issued to Cursor |
| 2026-11-21 | Owner approved coder/websocket v1.8.15 + JSON; D-003 contract published (EX-P04 prerequisite) |
| 2026-11-21 | T-010 merged; contracts rev 1.1 (R-01…R-13), exporter manifest = raw bytes + BUILD_ID; T-009 amendment issued |
| 2026-11-21 | Codex reported T-009 blocker (R-04: line_sha256 digest bypass) → contract decision: SQL-computed `ingest_digest`, no `line_sha256`; docs-links generated names fixed; T-009 amendment 2 (finish order, push WIP) |
