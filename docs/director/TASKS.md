# Task board (director is the only writer)

Status: `PROPOSED` (not issued) · `ISSUED` (prompt given to executor) · `REVIEW` · `DONE <sha>` · `REWORK` · `BLOCKED`.
Prompts live in `docs/director/prompts/T-NNN.md`. Protocol: `DIRECTOR.md`.

## Owner decisions (2026-11-21)

**Temporary handoff (2026-11-21):** director + executor role passes from Claude to **ChatGPT Codex** — see `prompts/CODEX_HANDOFF.md`. State at handoff: main contains T-038d, T-032/T-033, D-005; next queue T-038e → T-038f → T-039 → daily loop T-041…45 → EX-W01 tails.

**Latest delegation (2026-11-21):** **Claude** is the sole autonomous director **and** lead implementer. Claude owns task issuance, acceptance, CI verification, PR merge, roadmap updates, and may implement personally (`Lane: director` or task lane). Cursor/Codex/ChatGPT remain optional executors. Owner approval remains required for canon, new dependencies, major architecture, deployment, destructive changes. **Next:** EX-P08 combat loot / equip (issue T-034+), then EX-P09. **Scheduled (not started):** EX-W01 — `LOCATION_GENERATOR_PLAN.md`; draft contract `docs/contracts/location-generator.md` exists; OSS install still needs owner OK. Handoff: `docs/director/prompts/CLAUDE_HANDOFF.md`.

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
| T-012 | client | Android debug APK baseline on Godot 4.3 | T-008 ✔ | **DONE** (branch `e10c507`). Diff has no secrets (env var names only); scripts pass `bash -n`; executor built + verified APK locally (pkg com.lunex.newmmo, minSdk 21, targetSdk 34, arm64-v8a, 23.5 MiB, JSON inside, apksigner OK); **no phone attached → no device metrics (honest)**. Director added workflow `android-debug.yml` (manual) so the owner can download the APK as a GitHub artifact |
| T-013 | research | Independent review of the EX-P03 implementation → `docs/reviews/T-013-content-loader-review.md` | T-009 ✔ | **DONE** (branch `538fe9a`; 11 findings, 5 major). Triage: I-04 (CI pipefail, true: GitHub default shell has no pipefail) fixed; I-10 two-runner step added to CI; I-02 resolved by relaxing contract (raw-byte addressing); I-01 → trust-boundary decision + DB-level digest check; I-03/I-05/I-07/I-08/I-11 → T-014. Reviewer could not run code (Go 1.23, no Docker) — static only |
| T-016 | research | Dialogue quality audit + style guide + lint spec → `docs/reviews/T-016-dialogue-audit.md`, `docs/research/T-016-dialogue-style-guide.md` | EX-CONTENT-01 plan ✔ | **DONE** (branch `3729c2c`, lane_check clean). Numbers confirmed (3021 quest lines, 729 unique = 24.1%; same 12 lines ×120). Found: PROGRESS has one variant everywhere; MQ-0x-20 `CHOICE` gap → **C-CHOICE-001 (OPEN)**. Reviewer did not run unit tests (static). Director implemented the spec as `tools/content/dialogue_lint.py` (report mode, not CI) |
| D-002 | director | Interaction registry `content/interactables/reg-01.json` (MQ-01-01…05 objectives → event/target/instances → canon POI/LOC) + sync check | — | **DONE** (checked by `check_client_sync.py`; R-OBJ-SPAM solved via distinct `instances`) |
| D-004 | director | EX-P04 contracts: `auth-session.md`, `db-gameplay.md`, `quest-engine.md`, WS protocol rev 1.1 (`world.interact.request`), `min_level` in quest payload, govulncheck pin + CI | D-001, D-003 | **DONE** |
| T-018 | server | EX-P04 1/3: accounts, login, characters, WS tickets (REST), migrations 00006…, stdlib PBKDF2 | D-004 ✔ | **DONE** — merged `3e25b43`; lane_check clean, CI green incl. server-db + govulncheck (Go 1.26.11) |
| T-022 | client | Auth UI: login/register/characters screens + REST client, FakeHttp tests, no gameplay wiring | D-004 ✔ | **DONE** — `dec6b51` merged `dc183e2`; lane_check clean, client-sync 0 err, CI green (`9a7b7d3`); live login E2E waits for T-018 (D-004) |
| T-019 | server | EX-P04 2/3: WebSocket endpoint (coder/websocket v1.8.15): ticket auth, session lifecycle, heartbeat, limits, shutdown, snapshot skeleton | T-018 ✔ | **DONE 2026-11-21** (merged PR #2; merge `324bd2d`; CI #155 green) |
| T-020 | server | EX-P04 3/3: quest engine + reward ledger + idempotency (migrations 00008…), `world.interact.request`, accept/turn-in, snapshots, **region-open gating (`REGION_CLOSED`, config `regions.*`)** | T-019 | **DONE 2026-11-21** — PR #3 merge `4461aa7`; CI green (server-db concurrent turn-in + REGION_CLOSED); commits `22e531a`/`0707363` |
| T-021 | research | Security review of EX-P04 auth + tickets + ledger | T-018 ✔, T-020 ✔ | **DONE** — `docs/reviews/T-021-ex-p04-security.md`; S-01/S-03/S-06 mitigated in follow-up; S-02 (R-PROX) deferred to EX-P08/P09 |
| T-023 | client | Wire net client + journal to the real server (login → ticket → WS → snapshot → accept/interact/turn-in); instance ids for count>1 objectives | T-019 ✔, T-020 ✔, T-022 ✔ | **DONE 2026-11-21** — merge `de26d83`; Fake* `ONLINE_SMOKE_OK`; live E2E when `NEWMMO_ONLINE=1` |
| T-024 | server | EX-P05: dialogue engine — option filter, `dialogue.open/choice`, `character_dialogue_choices` (00009), flows_reg01, WS DialogueHandler, idempotency; R-PROX deferred | T-020 ✔, T-023 ✔ | **DONE 2026-11-21** — PR #6 merge `a24e3d6`; CI green incl. server-db |
| T-025 | client | Wire Godot dialogue UI to server open/choice (server options, choice mutation, keep offline mq01) | T-024 ✔ | **DONE 2026-11-21** — PR #7 merge `222f68e`; ONLINE_SMOKE covers open→choice→accept |
| T-026 | client | EX-P06: RU/EN locale switcher on HUD (`.tscn` OptionButton) + `user://settings.cfg` | T-004 ✔, T-007 ✔ | **DONE 2026-11-21** — PR #8 merge `38791dc` |
| T-027 | server | EX-P07 start: SQ flag names, EX-P07 interact events, SQ-01-001 registry + PG happy path + flows | T-020 ✔, T-024 ✔ | **DONE 2026-11-21** — PR #8 merge `38791dc` |
| T-028 | server | EX-P07: SQ-01-002…005 registry + flows + table-driven PG tests | T-027 ✔ | **DONE 2026-11-21** — PR #9 merge `0057a66` |
| T-029 | server | EX-P08 start: inventory research + contract + `character_inventory` + grant/move/snapshot | T-028 ✔ | **DONE 2026-11-21** — PR #10 merge `820aa39` |
| T-030 | client | EX-P08: inventory bag UI (`.tscn`) + HUD + snapshot/delta bind + `inventory.move.request` | T-029 ✔ | **DONE 2026-11-21** — PR #11 merge `23fe4cc` |
| D-005 | director | EX-W01: complete draft `docs/contracts/location-generator.md` (seed schema, regen API; stub already on main) | owner schedule ✔ | **DONE** (v1 contract) |
| T-034 | director (server+client) | EX-P08: combat loot — server-rolled drops from canon `LOOT-ENM-*` (`combat.loot.request/result`, ledger `00011`, `internal/loot`), exporter fields, contract `loot-engine.md` + WS rev 1.2, client request + HUD feedback | T-030 ✔ | **DONE 2026-11-21** — merge `b63d903`; CI green (server-db incl. migration 00011, client-godot NET_SMOKE loot case); live PG16 tests green. Equipment roll deferred; R-KILL + C-LOOT-001 recorded |
| T-035 | director (client) | Playtest UX fixes (owner notes): canon NPC names instead of codes (`client/data/names.json` + sync check), quest id removed from tracker/journal, interactable labels only near the player, hint instead of silence when «ГОВОРИТЬ» has no target/business | T-034 ✔ | **DONE 2026-11-21** — merge `29834e7`; CI green incl. Godot smokes. Not included: desktop layout of mobile UI |
| T-040 | director (client) | Real art placeholders (owner OK 2026-11-21): hero/NPC/monster sheets from OpenGameArt «Isometric Hero and Creatures» (CC-BY 3.0, repacked by `tools/spritegen/import_flare_pack.py`), dungeon props from Kenney (CC0), new monster scenes goblin/zombie/skeleton/ogre used in the dungeon; provenance in `client/assets/LICENSES.md` + `licenses/` | T-037 ✔ | **DONE 2026-11-21** — art batch 1 merged `0e3e85d` (CI green). batch 2 (valley ground/trees/cottage/rocks/fence/lantern/crystal, dungeon stone floors) + credits screen (`credits.tscn`, CI check that CC-BY sources are credited) merged `6c9c664` (CI green). **Open:** confirm original author of the CC-BY pack before release; art only eyeballed from single assets (no full-scene render here) |
| T-036 | director (server) | EX-P09 1/4: dungeon engine core — data-driven state machine, doors, server-checked puzzle, checkpoints, personal reward claim (first clear), reconnect, idle expiry; registry `content/dungeons/dng-01-001.json` + canon sync check; migration `00012`; WS rev 1.3; contract `dungeon-engine.md` | T-034 ✔ | **DONE 2026-11-21** — merge `e690ec6`; CI green (server-db incl. 00012, dungeon registry check). Interim R-KILL-DNG (client-reported kills) |
| T-037 | director (client) | EX-P09 2/4: dungeon client — editable scene `dng_01_001.tscn` (R01…R07, ids = registry, checked by `check_dungeon_registry.py`), `DungeonController` (enter/interact/report/claim/leave, room hints), entrance in Dawn Valley (`POI-01-006`), `DUNGEON_SMOKE_OK` | T-036 ✔ | **DONE 2026-11-21** — merge `379e699`; CI green incl. DUNGEON_SMOKE_OK (smoke caught a room-spawn ordering bug). Placeholder art (ColorRect floors, stand-in monsters) |
| T-038 | server (+client) | EX-P09 3/4: server combat simulation — research `docs/research/T-038-server-combat.md`; **owner approved 2026-11-21: option A (in-process Go sim, no framework), 10 Hz, split a…f, phase-1 movement authority with speed cap + bounds** | T-036 ✔ | **APPROVED — in progress** (T-038a…f below) |
| T-038a | director (server+tools) | Combat data + formulas: exporter kinds `skill` (60) and `ability` (258), schemas, `CT_COMBAT_*` checks, `internal/damage` (canon formulas, golden tests), `internal/combatdata` loader; entity-count constants removed from tests | owner OK ✔ | **DONE 2026-11-21** — merge `ddd133c`; CI green |
| T-038b | server | Sim core: `internal/world` (pure Sim + Zone + WS handler), server-owned movement, interest snapshot/delta `world.*` (protocol rev 1.4), zone def `content/zones/dawn-valley.json` + canon check, `combatdata.LoadFromDB`, contract `world-sim.md` | T-038a ✔ | **DONE 2026-11-21** — merge `0906421`; CI green |
| T-038c1 | server | Combat slice 1: `world.cast.request` (damage/shield/heal), resources, cooldown/GCD, range, server-observed enemy death → personal loot (`GrantKill`), respawn; contract `combat-engine.md`, protocol rev 1.5, C-RANGE-001 | T-038b ✔ | **DONE 2026-11-21** — merged with c2 `01f1713`; CI green |
| T-038c2 | server | Enemy AI FSM (aggro/chase/attack/leash) + abilities A/B with telegraphs, threat, damage to players, player death/respawn (contract `combat-engine.md` §6). Statuses and quest kill objectives remain | T-038c1 | **DONE 2026-11-21** — merge `01f1713`; CI green. Statuses and quest kill objectives remain |
| T-038d | client | Puppets from snapshots, prediction/interpolation, skill bar → `cast_skill`, telegraphs | T-038c | PROPOSED |
| T-038e | server | Dungeon on the sim; retire R-KILL-DNG | T-038c, T-038d | PROPOSED |
| T-038f | server+client | Measure CPU/RAM per zone, Android cost | T-038d | PROPOSED |
| T-038d | client | World client: `WorldController` (join, puppets from snapshots/deltas, movement intent + prediction/reconciliation, HUD skills → `world.cast.request`, damage numbers, telegraphs, cooldown/resource state, death freeze), `data/skills.json` (+ canon sync check), `WORLD_SMOKE_OK` | T-038c ✔ | **DONE** — merged to main (e921bb3), CI green |
| T-041 | server | Daily board engine (`docs/DAILY_LOOP.md` §8): templates, 03:00 UTC reset, deterministic rotation by day parity, progress from server-observed kills/interactions, ledger rewards, chest, weekly stamps | T-038c, T-020 ✔ | PROPOSED — design in `docs/DAILY_LOOP.md` (owner delegated parameters 2026-11-21) |
| T-042 | client | Daily board + weekly book UI, tracker, optional event notifications | T-041 | PROPOSED |
| T-043 | server | Event scheduler (4 windows/day per region, `EVT-*` instances, contribution, anti-AFK) — aligns with EX-P10 | T-041, EX-P10 | PROPOSED |
| T-044 | content | Daily/event texts (new narrative, owner «делай» needed), RU/EN | owner | PROPOSED |
| T-045 | server+tools | Retention telemetry (D1/D7, daily completion funnel) + parameter panel | T-041 | PROPOSED |
| T-039 | server+client | EX-P09 4/4: equipment drops + class RARE first-clear reward, reward mail, pity, `STORY_SOLO`, party/queue | T-038, equip UI | PROPOSED |
| T-031 | research | EX-W01: re-verify Gaea/WFC/in-house on Godot 4.7.2; recommend one | D-005 | **DONE** — in-house chosen, no deps |
| T-032 | client | EX-W01: Lunex Generate editor dock → `Generated/*` only | T-031 ✔, owner OK if deps | **DONE** — `client/addons/lunex_gen` (CLI; editor dock deferred), no-quest-dead-end verify+repair, CI smoke |
| T-033 | client | EX-W01: Manual-survive regen test + Dawn Valley generate pass | T-032 | **DONE** — covered by `mapgen_smoke.gd` + Dawn Valley generated (seed 7) |
| T-017 | research(+content) | EX-CONTENT-01 pilot: MQ-01-05 + SQ-01-001…005 as drop-in proposals | plan approved ✔, T-016 ✔ | **DONE — integrated into canon 2026-11-21** (owner «переноси пилот в канон»; canon `e69dd80`, client `987945f`; 33 new DIA IDs, none removed; entity count 4925→4958). Before integration: branch `41e9e38` merged into `main` as proposals only (`docs/content/dialogue_expansion/pilot/`); `10_DIALOGUES.md` untouched. Director: lane_check clean (17 commits); strict lint on overlay **0 ERROR / 0 WARN**; MQ-01-05 19 lines / PROGRESS×3, SQ ×15 / PROGRESS×2; owner's decisions applied (Ilra speaks at OBJ3, no A/B in SQ-01-002, LATER added). Quality note: MQ-01-05 strong; SQ voices still somewhat flat — owner judges |
| T-015 | client | Upgrade the client to Godot 4.7.2 (owner decision) | T-012 ✔ | **DONE** (branch `61eedda`). Director: lane_check clean, `project.godot` + scripts diff reviewed, no secrets; executor reports strict smoke green on 4.7.2, frames before/after match, APK builds (29.2 MB, minSdk 24, targetSdk 36). Director added CI job `client-godot` (Godot 4.7.2, checksum-verified) and moved `android-debug` workflow to 4.7.2; smoke script now refuses non-4.7 unless `ALLOW_GODOT_43=1`. World Studio GUI click-test not done (no Vulkan in executor VM) |
| T-014 | server | EX-P03 hardening per T-013 (migration 00005 DB-level digest check, strict crossrefs, strict role-URL tests, golden digest vector, Status singleton, audit deadline) | T-013 ✔ | **DONE** (branch `14d5d9d`, 3 commits). Director: lane_check clean, go.mod/go.sum unchanged, migration 00005 read (fingerprint function + `staged→published` trigger), branch CI run 99 green on GitHub (server-db with live PG 17, no skips) |
| T-009 | server | EX-P03: goose migrations + content loader (pgx) + `contentctl` + isolated Compose | D-001 ✔, T-006 ✔ | **DONE** (branch `aaeac7b`, 7 commits). Contract rev 1.1 + amendments implemented; executor ran full DB integration (no SKIP), goose up/reset/up, COPY 46.8 ms vs INSERT 87.1 ms (4925 rows). Director: lane_check clean, go.mod = pgx v5.13.0 + 5 listed indirects only, vet/test green without DB, CI `server-db` fixed (roles init before goose, three role URLs, fail on SKIP) |
| D-002b | director | POI id mapping contract (stub `POI-BOARD-01` … → canon `POI-01-*`, C-POI-001) | — | **OPEN** (C-POI-001); interactables registry was separate D-002 DONE |
| T-001 | research | OSS research: PostgreSQL migrations + Go DB access → `docs/research/db-migrations.md` | — | **DONE** `3fb9aa6` (branch `be9a5bb`, lane_check clean). Director spot-check of upstream `go.mod` at tags confirmed the report's Go-version warning (see ROADMAP EX-P03). Owner OK received 2026-11-21 (Go ≥1.25, goose + pgx) |
| T-002 | research | Godot 4.7.2 upgrade gate → `docs/research/godot-4-7-2-upgrade.md` | — | **DONE** (branch `387d821`, lane_check clean). Director spot-checked: smoke script `|| true` + no MQ01/stage1 runs, MQ01 marker before xp check, `include_filter=""` — all **confirmed** → T-008. Recommendation: keep 4.3 until Android toolchain (B-APK) + 4.3 baseline on 6 GB device |
| T-003 | research | Go WebSocket libs + protocol → `docs/research/go-websocket.md` | — | **DONE** (branch `d21b375`; go.mod at tags re-verified by director: coder/websocket v1.8.15 → go 1.23; gorilla v1.5.3 → 1.12; gobwas/ws v1.5.0 and centrifuge v0.40.1 → 1.26). Recommendation: coder/websocket v1.8.15 (+ gorilla fallback), JSON v1, no compression, ticket auth, request_id idempotency. Note: report predates T-006 (statusWriter now implements Hijacker/Flusher). Needs **owner OK** for the dependency + director WS contract (D-003) before EX-P04/P05 |
| T-010 | research | Independent review of EX-P03 contracts + exporter → `docs/reviews/T-010-contracts-review.md` | D-001 ✔ | **DONE** (branch `dd2177d`; 13 findings, 2 blockers). All applied by director in contracts **rev 1.1** + exporter change (BUILD_ID/raw-bytes manifest); Codex gets `prompts/T-009-amendment-1.md`. Reviewer could not run tests (no checkout) — director ran them |
| T-008 | client | Strict smoke script + JSON in APK export filter | T-007 ✔ | **DONE** (branch `a80d863`). Diff reviewed: import exit code enforced, stage1+MQ01 scripts run with exit 0 + marker, MQ01 marker after xp check, `include_filter` added; executor verified artificial failures and `--export-pack` contents (APK/keystore dry-run not done) |
| T-011 | client | WebSocket client library per D-003 (transport abstraction, envelope validation, session/backoff, revisions, idempotent resend; FakeTransport tests; not wired into gameplay) | D-003 ✔ | **DONE** (branch `30c093b`). Director: lane_check clean; envelope/validation code read against contract; `NET_SMOKE_OK` hooked into strict smoke (executor-reported run). Known limits: Godot JSON parser cannot reject duplicate keys (server-side check remains); regexes compiled per call (perf nit); no live-server handshake yet |
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
| 2026-11-21 | T-008 merged; T-011 issued to Cursor |
| 2026-11-21 | T-011 merged; Cursor idle until EX-P04 server exists (candidate: Android 4.3 baseline needs owner OK for SDK installs) |
| 2026-11-21 | Owner approved Android baseline toolchain install; T-012 issued to Cursor |
| 2026-11-21 | T-009 merged; CI server-db updated for role bootstrap; ChatGPT gets code review task T-013 |
| 2026-11-21 | T-013 merged; contracts rev 1.2; CI: pipefail + two-goose-runners + REQUIRE_ROLE_URLS; T-014 issued to Codex |
| 2026-11-21 | T-012 merged; android-debug workflow (manual dispatch) added so the owner gets an APK artifact; device measurements pending the owner's phone |
| 2026-11-21 | Owner: no 6 GB phone → Android baseline accepted provisionally; device metrics are a mandatory pre-release gate (R-DEVICE in ROADMAP, EX-P13) |
| 2026-11-21 | Owner ordered the Godot 4.7.2 upgrade (waives device-metrics-first); T-015 issued; lane_check now accepts comma-separated Extra-Write |
| 2026-11-21 | Owner has a second ChatGPT → T-016 (dialogue audit + style guide) issued; evidence: 3122 lines, 760 unique (24%), 8 lines repeated 120×, ~12 lines/MQ |
| 2026-11-21 | Owner approved `CONTENT_EXPANSION_PLAN.md`; pilot start awaits «делай» (T-016 audit/style guide first is recommended) |
| 2026-11-21 | T-014 merged — EX-P03 complete (implementation + two independent reviews applied) |
| 2026-11-21 | T-016 merged; dialogue_lint.py baseline: 240/240 quests have findings (3270 errors: 2070 exact-dup, 1200 stamps; 240 PROGRESS warnings); C-CHOICE-001 opened |
| 2026-11-21 | T-015 merged: project pin = Godot 4.7.2; AGENTS.md/CONFLICTS updated; minSdk 24 accepted |
| 2026-11-21 | Owner decided C-CHOICE-001 = C (no branching at MQ-01…05-20; confirmation only); batch MQ-01-20 unblocked |
| 2026-11-21 | Owner: «делай пилот» → T-017 issued; dialogue_lint.py got --proposal-dir overlay mode |
| 2026-11-21 | Owner: «делай EX-P04» → contracts D-004/D-002 published (auth-session, db-gameplay, quest-engine, interaction registry); T-018 (Codex) and T-022 (Cursor) issued; T-019…T-021, T-023 queued |
| 2026-11-21 | Owner pacing decision (regions one by one, ~1 month to L60, rested XP allowed) → `docs/PACING.md`; quest-engine contract gets region-open gating + `REGION_CLOSED` + `rested_xp` field |
| 2026-11-21 | T-017 pilot reviewed: lint 0 ERROR / 33 WARN (cross-quest duplicates); sent back (rework-1); canon unchanged |
| 2026-11-21 | govulncheck CI step failed legitimately: 9 reachable stdlib vulns on Go 1.25.14 (fixed only in 1.26.9+) → server moved to Go 1.26.11 (go.mod + CI 1.26.x); clean; executors on open branches must rebase (amendment prompt) |
| 2026-11-21 | T-017 reworked proposals merged (as proposals); owner approval needed before canon integration (10_DIALOGUES.md + client JSON) |
| 2026-11-21 | Owner: «переноси пилот в канон» → `10_DIALOGUES.md` (6 sections) + client JSON + `progress` variants in quest_manager.gd; entity count 4958 (tests/contracts updated; Go tests fixed `9a7b7d3` after red CI) |
| 2026-11-21 | T-022 (Cursor) reviewed and merged `dc183e2`: auth/character UI + REST client; CI green |
| 2026-11-21 | T-020 reviewed and merged PR #3 (`4461aa7`): quest FSM + ledger + idempotency + REGION_CLOSED; CI server-db green; T-021 issued |
| 2026-11-21 | T-021 security review DONE; S-01 pinned-build rewards + S-03 idempotency prune + S-06 fail-closed claim path fixed; T-023 issued |
| 2026-11-21 | T-023 merged `de26d83`: Godot online session + quest wiring; ONLINE_SMOKE_OK; T-024 (EX-P05 dialogue) issued to Cursor-director |
| 2026-11-21 | T-024 merged PR #6 (`a24e3d6`): dialogue engine + 00009 + WS; T-025 client dialogue wiring issued |
| 2026-11-21 | T-025 merged PR #7 (`222f68e`): client dialogue open/choice; EX-P05 closed; T-026 locale + T-027 SQ-01-001 issued |
| 2026-11-21 | T-026/T-027 merged PR #8 (`38791dc`); T-028 SQ-01-002…005 issued |
| 2026-11-21 | T-028 merged PR #9 (`0057a66`); EX-P07 SQ pilot closed on server; T-029 inventory issued |
| 2026-11-21 | T-029 merged PR #10 (`820aa39`); T-030 inventory bag UI issued |
| 2026-11-21 | T-030 merged PR #11 (`23fe4cc`); EX-P08 bag UI done; paused per owner (loot/equip next) |
| 2026-11-21 | Owner: schedule location generator with Godot-editable output → EX-W01 plan + D-005/T-031…T-033 PROPOSED |
| 2026-11-21 | EX-W01 plan merged PR #13 (`06d5751`); draft `location-generator.md` for docs-links |
| 2026-11-21 | Owner hands director+executor role to Claude; Cursor handoff closed; `CLAUDE_HANDOFF.md` published |
| 2026-11-21 | Owner: Claude is the autonomous director+lead dev; T-034 loot implemented on `agent/director/T-034` (server `internal/loot`, migration 00011, client request + HUD feedback) |
| 2026-11-21 | Owner: «Продолжай, EX-P09» → T-036 dungeon engine core on `agent/director/T-036`; T-037…T-039 queued (client scenes, server combat, equipment/mail/pity/party) |
| 2026-11-21 | Owner: «Да на все» → T-038 approved (A, 10 Hz, split, movement authority); T-038a started |
| 2026-11-21 | Owner: daily/event loop to retain players — «сделай как больше нравится игрокам» → `docs/DAILY_LOOP.md` (proposal), T-041…T-045 PROPOSED |
