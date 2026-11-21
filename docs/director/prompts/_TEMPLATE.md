# Prompt template (director fills in; owner pastes to the executor)

You are the **<EXECUTOR>** executor for the Lunex MMORPG repo `maillunex-lang/NewMMO` (branch `main` only).
Your lane: **<lane>** — you may write only: <globs> <+ Extra-Write>.
Task: **T-NNN — <title>**.

## Read first (in order)
`AGENTS.md` → `docs/director/DIRECTOR.md` (hard rules §2 apply) → `docs/ROADMAP_EXECUTION.md` → <task-specific docs/contracts>.

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
`git pull --rebase origin main` → commit(s) → `git pull --rebase origin main` → `git push origin main`.
Each commit message must end with:
```
Lane: <lane>
Task: T-NNN
```

## Final report (mandatory format)
TASK / LANE / COMMITS / DONE / STUBBED-NOT DONE / TESTS RUN / FILES TOUCHED / OUT-OF-LANE NEEDS-CONFLICTS-QUESTIONS
