# AI Director protocol — multi-agent work on NewMMO

**Owner** (human) decides canon and priorities. **Director** (Cursor, owner-delegated 2026-11-21) plans, writes prompts,
guards canon/contracts, reviews and integrates. **Executors** do bounded tasks inside a lane.
Authority: Owner > Director > Executors. An executor never overrides canon, contracts, lanes or a task's acceptance list.

## 1. Executors and lanes

| Lane | Executor (assumed) | Writes only | Typical work |
|------|--------------------|-------------|--------------|
| `client` | Cursor | `client/**` | Godot scenes/scripts/UI/assets (editable `.tscn`) |
| `server` | ChatGPT Codex | `server/**`, `deploy/**` | Go server, SQL migrations, isolated `newmmo` compose |
| `research` | ChatGPT (chat) | `docs/research/**`, `docs/reviews/**` | OSS research reports, reviews, test-case design |
| `director` | Cursor (or a Cursor subagent acting as director) | everything | canon, schemas, `tools/**`, contracts, roadmap, CI, integration |

Machine-readable: `docs/director/lanes.json`. Protected (director-only): canon `docs/content/**`, `content/**`, `tools/**`,
`AGENTS.md`, `.github/**`, `docs/contracts/**`, `docs/director/**`, roadmap/audit docs, `client/data/**` (mirrors canon).
A director task may grant extra paths with `Extra-Write:` (listed in the task).

Owner confirmed (2026-11-21): all three executors can push to this repo, Godot and Go toolchains are available.
Lane split and git mode were delegated to the director: lanes as above; **branch-per-task** (see §4).

## 2. Hard rules for everyone (copy into every prompt)

0. **Research-first.** Before writing code/assets: search for existing, proven solutions — GitHub, **Hysen Labs directory https://hysenlabs.com/projects** (48k+ curated OSS projects: search, categories, compare, license/metadata — use it for discovery, then verify license/activity/README at the upstream GitHub repo), Godot Asset Library (client), official docs. Compare ≥3 candidates when that many exist (license, last activity, Godot 4.x/Android or Go 1.22 fit, size, risks). Record in `docs/research/T-NNN-<topic>.md` (allowed in every lane). Prefer small maintained components; say explicitly when the best answer is «use stdlib / build in-house». **Do not install, vendor, copy or `go get`/add-on anything third-party until the director relays the owner's OK**; never run unknown install scripts; never copy foreign code/assets before license review. Tasks that may adopt a dependency are **two-phase**: Phase 1 = research note + stop and report; Phase 2 (only after OK) = implement.
1. Work on **your own branch `agent/<lane>/T-NNN`** (created from fresh `origin/main`), never on `main`. Small commits; **`git pull --rebase origin main` before every push** (rebase only your own branch); never force-push `main`; never rewrite others' commits. The director opens the PR, runs gates, merges (merge commit, trailers preserved).
2. Touch only files in your lane (+ task `Extra-Write`). If you need something outside it: stop, say so in the report; do not “just fix it”.
3. Every commit message ends with trailers: `Lane: <lane>`, `Task: T-NNN` (and `Extra-Write:` if granted). CI `lane-check` fails otherwise.
4. Canon (`docs/content/01…14`) is read-only. Conflict or ambiguity → write it in your report; the director records it in `CONFLICTS.md`.
5. Contracts (`docs/contracts/**`) are frozen interfaces (protocol, DB schema, content export). Implement against them; propose changes in your report.
6. The game must stay bootable: run your lane's gates (below) before pushing; never push red tests.
7. No secrets, keystores, `.env` values. No unlicensed assets/code; no new third-party dependency without an owner-approved `docs/research/*.md`.
8. Never touch VPS, other projects, Caddy, domains, shared DBs, foreign Compose stacks.
9. Don't claim done until implemented **and** tested; report what is done / stubbed / blocked. No docs-as-feature.
10. Server is authoritative: no client trust for combat, currency, inventory, movement, quests.

## 3. Gates per lane (run before push; CI repeats)

| Lane | Gate |
|------|------|
| all | `python3 -m unittest discover -s tools/content/tests`, `python3 tools/content/lint.py`, `python3 tools/content/check_client_sync.py`, `python3 tools/content/check_docs_links.py` |
| client | `./tools/smoke_stage1.sh` (headless Godot smoke) when Godot is available; no scene geometry rebuilt in code |
| server | `cd server && go vet ./... && go test ./...` |
| research | report has ≥3 candidates, links, license, activity, Godot/Android fit, risks, recommendation; **no vendoring/installing** |

## 4. Task cycle

1. Director writes a task in `docs/director/TASKS.md` (ID, lane, depends, scope, files, acceptance, tests, out-of-scope) and a prompt `docs/director/prompts/T-NNN.md` (research-first, phases).
2. Cursor-director dispatches the prompt to an available executor/subagent, or implements the task itself with `Lane: director` and documented tests. Executor researches, (Phase 2 after OK) implements on `agent/<lane>/T-NNN`, commits with trailers, pushes the **branch**, replies with the **report format** (below).
3. Cursor-director monitors completion without requiring an owner relay. Director fetches the branch, runs gates + `lane_check`, reviews the diff against acceptance, opens the PR and merges it into `main` → task `DONE` (merge SHA); or `REWORK` (precise defects, same branch); a breaking change never reaches `main` (no revert needed).
4. Only the Cursor-director edits `TASKS.md` / roadmap (single writer → no merge conflicts there).

Parallelism rule: two tasks may run simultaneously only if their lanes differ **and** neither depends on an unfinished task's output. Director sequences dependent tasks; contracts are published first, implementations second.

## 5. Executor report format

```
TASK: T-NNN   LANE: <lane>   COMMITS: <sha…>
DONE: <bullets, verifiable>
STUBBED / NOT DONE: <bullets>
TESTS RUN: <command → result>
FILES TOUCHED: <list>
OUT-OF-LANE NEEDS / CONFLICTS / QUESTIONS: <bullets or none>
```

## 6. Integration gate and rollback

Director acceptance = gates green + lane_check clean + acceptance list met + no canon/contract drift. Only the director merges to
`main`. A branch that breaks gates or leaves its lane is sent back (`REWORK`) and not merged; if something bad lands anyway the director reverts it (`Lane: director`).
Recommended (owner, GitHub → Settings → Branches): protect `main` — require PR + the `ci` checks, restrict who can push. The director cannot set this from here.
Conflicts between two branches: whoever merges second rebases onto new `main`; the director orders merges (contract/lib tasks first).

## 7. Contracts the director must publish before implementation tasks

| Contract | Needed by | Source |
|----------|-----------|--------|
| Content export format (`declarations.json`/`crossrefs.json` → server loader tables) | EX-P03 | `tools/content`, `content/schemas` |
| DB schema / migrations naming | EX-P03 | `14` §16 adapted |
| WebSocket protocol events | EX-P04/P05 | `14` §15 adapted |
| POI id mapping (client stub ids → canon `POI-01-*`, C-POI-001) | EX-P06/P07 | director decision |

Until a contract exists in `docs/contracts/`, dependent implementation tasks are not issued.
