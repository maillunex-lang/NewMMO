# Prompt template (director fills in; owner pastes to the executor)

You are the **<EXECUTOR>** executor for the Lunex MMORPG repo `maillunex-lang/NewMMO` (branch `main` only).
Your lane: **<lane>** — you may write only: <globs> <+ Extra-Write>.
Task: **T-NNN — <title>**.

## Read first (in order)
`AGENTS.md` → `docs/director/DIRECTOR.md` (hard rules §2 apply) → `docs/ROADMAP_EXECUTION.md` → <task-specific docs/contracts>.

## Research-first (mandatory, before any implementation)
Search for existing, proven solutions: GitHub, **Hysen Labs directory https://hysenlabs.com/projects** (search/categories/compare; then verify the upstream repo), Godot Asset Library (client work), official docs. Compare ≥3 candidates when that many exist. Write `docs/research/T-NNN-<topic>.md`: links, license, last activity, Godot 4.x/Android or Go 1.22 fit, risks, recommendation (including «use stdlib/in-house» if best). **Do not install/vendor/copy any third-party code or asset.** <Phase rule: either «STOP after the note and report» (Phase 1) or «continue with stdlib/in-repo only».>

## Goal
<one paragraph>

## Scope (do)
- …

## Out of scope (do NOT)
- Anything outside your lane; changing canon `docs/content/**`, contracts, schemas, CI; new dependencies; other tasks.

## Acceptance (all must hold)
- [ ] …

## Tests / gates to run before pushing
<commands>

## Git
`git fetch origin && git checkout -b agent/<lane>/T-NNN origin/main` → commit(s) → `git pull --rebase origin main` → `git push -u origin agent/<lane>/T-NNN`.
**Never push to `main`.** Do not open/merge the PR — the director does.
Each commit message must end with:
```
Lane: <lane>
Task: T-NNN
```

## Final report (mandatory format)
TASK / LANE / COMMITS / DONE / STUBBED-NOT DONE / TESTS RUN / FILES TOUCHED / OUT-OF-LANE NEEDS-CONFLICTS-QUESTIONS
