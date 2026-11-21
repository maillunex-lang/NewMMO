# AGENTS.md — Lunex MMORPG / NewMMO

Standing instructions for Cursor agents and human collaborators working in **https://github.com/maillunex-lang/NewMMO** only.

## Scope lock

- Work **only** in this repository and its isolated NewMMO deploy paths.
- Do **not** modify other projects, VPS services, Caddy configs, domains, certificates, shared databases, or unrelated Compose stacks.
- Do not claim a feature complete until it is implemented **and** tested.

## Canon and design authority

1. Approved design lives in `docs/GDD.md`, `docs/ARCHITECTURE.md`, `docs/ROADMAP.md`, `docs/WORLD_EDITOR.md`.
2. **`docs/content/*.md` is game canon** for story, characters, geography, quest IDs, and lore naming. Do **not** invent or rewrite those without explicit owner approval. If a content file is missing, document the gap; do not silently invent canon.
3. If requirements conflict, prefer GDD/content canon, document the conflict, propose a safe interim, and continue independent work.

## Engine and architecture invariants

- Client: Godot **2D** isometric 2.5D (prerendered sprites). **No runtime 3D character/world rendering.**
- Project pin today: Godot **4.3** (`client/project.godot`). Target mentioned in planning docs may be newer (e.g. 4.7.x) — pin must be explicit before upgrades.
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

1. Branch from the agreed base (`cursor/<name>-6dd6` when using Cloud Agent branch policy).
2. Implement the smallest working slice; keep the game bootable.
3. Test (headless smoke, `go test`, manual checks as applicable).
4. Commit, push, update PR; report what is done / stubbed / blocked (APK, VPS, etc.).
5. Do not mix “docs written” with “feature shipped.”

## First vertical priorities (context)

Stage order: audit → visual slice → online foundation → editor/three regions → RPG → story/PvE → MMO systems → expansion → release hardening. Do not start all stages at once.
