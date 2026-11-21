# AGENTS.md — Lunex MMORPG / NewMMO

Standing instructions for Cursor agents and human collaborators working in **https://github.com/maillunex-lang/NewMMO** only.

## Temporary delegation → ChatGPT Codex (2026-11-21)

The owner **temporarily** hands the director + lead-implementer role to **ChatGPT Codex** (prompt: `docs/director/prompts/CODEX_HANDOFF.md`). The same rules below apply to Codex in full (branches, green CI before merge, owner-approval gates, canon read-only). Claude resumes the role when the owner says so.

## Owner delegation: Claude as autonomous director + lead implementer (2026-11-21)

The owner has delegated day-to-day planning, task prioritization, reviews, roadmap maintenance, **and implementation** to **Claude** as the sole director and lead developer. This supersedes the earlier Cursor-as-sole-director handoff. Claude may issue tasks, implement work **using `Lane: director`** (or the appropriate lane when acting as an executor), run gates, open PRs and merge after review. Optional executors (Cursor/Codex/ChatGPT) may still be used under Claude’s direction. Owner approval remains mandatory for canon changes, new third-party dependencies/assets, production deployment, destructive operations, paid services, and major architecture/engine changes. Keep the repository bootable and never claim untested features done. Do not operate on VPS or other projects without explicit authorization. Proceed autonomously through safe, already-approved roadmap work, one small vertical slice at a time. Handoff prompt: `docs/director/prompts/CLAUDE_HANDOFF.md`.

## Scope lock

- Work **only** in this repository and its isolated NewMMO deploy paths.
- Do **not** modify other projects, VPS services, Caddy configs, domains, certificates, shared databases, or unrelated Compose stacks.
- Do not claim a feature complete until it is implemented **and** tested.

## Canon and design authority

1. **Orchestrator:** `docs/content/14_CONTENT_INTEGRATION.md` links all content specs.
2. **Execution map:** `docs/ROADMAP_EXECUTION.md` — stage order, checkboxes, acceptance tests. After finishing a stage, mark `[x]` there with commit SHA and test notes.
3. Approved design also lives in `docs/GDD.md`, `docs/ARCHITECTURE.md`, `docs/ROADMAP.md`, `docs/WORLD_EDITOR.md`.
4. **`docs/content/01_*.md`…`14_*.md` is game canon.** Do **not** invent or rewrite without owner approval. Conflicts → `docs/content/CONFLICTS.md`, then wait.
5. If requirements conflict, prefer GDD/content canon, document the conflict, propose a safe interim, and continue independent work.

## Engine and architecture invariants

- Client: Godot **3D** world, visual direction **«Lunex Fantasy 3D»** (owner decision 2026-11-21, supersedes the earlier 2D/prerendered-sprite rule): bright saturated stylised 3D characters, buildings, trees and monsters, soft shadows, sun rays, expressive magic VFX, camera pitch ~40–50° with adjustable zoom, landscape UI, graphics presets Low → Ultra. The server stays 2D-coordinate authoritative (x,y → ground plane). UI remains editable `.tscn` Control scenes. 3D assets come from Meshy (owner has a premium account; API key stored as an environment network secret) or other licensed sources, with provenance in `client/assets/LICENSES.md`. The 2D world layer stays bootable as a fallback until the 3D slice passes the device gate (`docs/research/3d-migration.md`).
- Project pin: Godot **4.7.2** (`client/project.godot` features `4.7`, GL Compatibility; owner decision 2026-11-21, migration T-015). Rollback to 4.3 only deliberately (`ALLOW_GODOT_43=1`, see `docs/research/T-015-godot-4-7-2-migration.md`). Any further engine upgrade needs an explicit owner decision.
- Server: authoritative **Go + PostgreSQL + WebSocket**. Never trust the client for combat, currency, inventory, or movement authority.
- UI: major screens are editable `.tscn` Control scenes + Theme resources; scripts bind data, they do not rebuild full layout in code.
- Locations: editable `.tscn` with layer contract (Ground, Paths, Vegetation, Buildings, Decorations, NPCSpawns, MonsterSpawns, Navigation, Effects). Manual edits must survive regeneration (`Manual` / stable IDs).
- Secrets: environment / secret store only — never commit keys, keystores, or `.env` values.
- Assets: track provenance in `client/assets/LICENSES.md` / `assets/LICENSES.md`. No unlicensed third-party art or code.

## Open-source research (mandatory before large systems)

Before implementing a large subsystem (world gen, networking libs, combat frameworks, UI kits, etc.):

1. Search GitHub, Hysen Labs, Godot Asset Library, and official docs.
2. Compare **at least three** suitable candidates when that many exist.
3. Write `docs/research/<system-name>.md` with links, license, activity, Godot/Android fit, risks, and a recommendation.
4. **Do not install, vend, or integrate** a found project until the owner confirms the research report.
5. Prefer small, maintained components over replacing Godot or the Go server.
6. Never copy foreign code/assets before license review; never run unknown install scripts.

Detailed checklist: `.cursor/rules/open-source-research.mdc`.

## World generation special rule

Any location generator must:

- Output or update **editable `.tscn`** scenes (not opaque runtime-only blobs as the edit source).
- Preserve **manual** placements across regen (stable IDs, Manual layer, preview/diff).
- Export validated gameplay data (collision, spawns, navigation) for the server separately from raw visuals.

## Delivery loop

1. **Work only on `main`** (director and owner). **Exception (owner-delegated 2026-11-21):** parallel AI executors work on one branch per task `agent/<lane>/T-NNN`, never on `main`; the director reviews, opens the PR and merges. No other branches/PRs unless the owner asks.
2. Implement the smallest working slice; keep the game bootable.
3. Test (headless smoke, `go test`, manual checks as applicable).
4. Commit and push directly to `main` (director/owner; executors follow the branch exception above); report what is done / stubbed / blocked (APK, VPS, etc.).
5. Do not mix “docs written” with “feature shipped.”

## Execution priorities

Follow **`docs/ROADMAP_EXECUTION.md`** (EX-P00 → EX-P13; plus scheduled EX-W01). Do not start all stages at once. Do not re-implement completed checklist items. Default next step: Claude-director autonomously selects the next unblocked approved task, respecting owner-approval gates.

## Multi-agent work

When several AI executors work in parallel, follow `docs/director/DIRECTOR.md`: write **lanes** (`docs/director/lanes.json`),
mandatory commit trailers (`Lane:`, `Task:`), branch-per-task `agent/<lane>/T-NNN` + `git pull --rebase origin main` before push, tasks issued by the director in `docs/director/TASKS.md`. **Research-first:** every task starts with a search of existing solutions (GitHub, Hysen Labs directory, Godot Asset Library, official docs) recorded in `docs/research/T-NNN-*.md`; nothing third-party is installed/vendored until the owner approves.
Canon, contracts, tools, CI and roadmap are director-only. No force-push.
