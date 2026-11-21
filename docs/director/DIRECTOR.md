# AI Director protocol — multi-agent work on NewMMO

**Owner** (human) decides canon and priorities. **Director** (Claude, this session lineage) plans, writes prompts,
guards canon/contracts, reviews and integrates. **Executors** do bounded tasks inside a lane.
Authority: Owner > Director > Executors. An executor never overrides canon, contracts, lanes or a task's acceptance list.

## 1. Executors and lanes

| Lane | Executor (assumed) | Writes only | Typical work |
|------|--------------------|-------------|--------------|
| `client` | Cursor | `client/**` | Godot scenes/scripts/UI/assets (editable `.tscn`) |
| `server` | ChatGPT Codex | `server/**`, `deploy/**` | Go server, SQL migrations, isolated `newmmo` compose |
| `research` | ChatGPT (chat) | `docs/research/**`, `docs/reviews/**` | OSS research reports, reviews, test-case design |
| `director` | Claude | everything | canon, schemas, `tools/**`, contracts, roadmap, CI, integration |

Machine-readable: `docs/director/lanes.json`. Protected (director-only): canon `docs/content/**`, `content/**`, `tools/**`,
`AGENTS.md`, `.github/**`, `docs/contracts/**`, `docs/director/**`, roadmap/audit docs, `client/data/**` (mirrors canon).
A director task may grant extra paths with `Extra-Write:` (listed in the task).

Capabilities are an assumption: ChatGPT chat typically cannot push to git. If so, it delivers files as text to the owner, who
commits them (as `Lane: research`). The owner confirms each executor's real access (see TASKS.md «Open questions»).

## 2. Hard rules for everyone (copy into every prompt)

1. Work on `main` only (AGENTS.md). No branches/PRs unless the owner asks. Small commits; **`git pull --rebase origin main` before every push**; never force-push; never rewrite others' commits.
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

1. Director writes a task in `docs/director/TASKS.md` (ID, lane, depends, scope, files, acceptance, tests, out-of-scope) and a prompt `docs/director/prompts/T-NNN.md`.
2. Owner pastes the prompt to the executor. Executor works, commits with trailers, pushes, replies with the **report format** (below).
3. Owner tells the director «T-NNN готово». Director pulls, runs gates + `lane_check`, reviews the diff against acceptance, then marks the task `DONE` (with SHA) or `REWORK` (with precise defects) or reverts a breaking commit.
4. Only the director edits `TASKS.md` / roadmap (single writer → no merge conflicts there).

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

Director acceptance = gates green + lane_check clean + acceptance list met + no canon/contract drift. A commit that breaks gates
or leaves its lane is reverted by the director (revert commit, `Lane: director`) and the task returns to `REWORK`.
Recommended (owner, GitHub settings): branch protection on `main` requiring the `ci` checks, once agents use PRs; until then CI is advisory and the director enforces by revert.

## 7. Contracts the director must publish before implementation tasks

| Contract | Needed by | Source |
|----------|-----------|--------|
| Content export format (`declarations.json`/`crossrefs.json` → server loader tables) | EX-P03 | `tools/content`, `content/schemas` |
| DB schema / migrations naming | EX-P03 | `14` §16 adapted |
| WebSocket protocol events | EX-P04/P05 | `14` §15 adapted |
| POI id mapping (client stub ids → canon `POI-01-*`, C-POI-001) | EX-P06/P07 | director decision |

Until a contract exists in `docs/contracts/`, dependent implementation tasks are not issued.
