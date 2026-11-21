# T-020 — Server quest engine (research)

**Date:** 2026-11-21 · **Lane:** server · **Deps:** stdlib + pgx v5.13.0 (already pinned)

## Sources

| # | Source | Takeaway |
|---|--------|----------|
| 1 | Contract `quest-engine.md` + `db-gameplay.md` §4 | FSM, lock order, idempotency, reward ledger |
| 2 | Go `database/sql` / pgx tx patterns; Postgres `FOR UPDATE` | Per-character serialization |
| 3 | Hysen Labs / GitHub quest-FSM libs | Overkill; protocol-specific — **in-house** |
| 4 | Existing T-018/T-019 store + WS session dispatch | Extend; no new modules |

## Candidates

| # | Approach | Verdict |
|---|----------|---------|
| 1 | **In-house FSM + pgx transactions** | **Adopt** |
| 2 | Generic workflow engine (Temporal etc.) | New dep — reject |
| 3 | Client-trust progress | Violates authority — reject |

## Recommendation

Implement `server/internal/quest` against frozen contracts; load quest defs from `content_entities` (active build); interactables from `content/interactables/reg-01.json`; region gates from env config (`NEWMMO_REGIONS_JSON`). No new Go modules.

## Implementation notes (shipped)

- Package `server/internal/quest`: catalog + FSM + pgx transactions under `characters … FOR UPDATE`.
- Migration `00008_quest_gameplay.sql`: quest states, objective instances, reward claims, ledger, idempotency, append-only guards.
- WS: optional `QuestHandler` on `ws.Server` (nil keeps T-019 `UNKNOWN_TYPE`); boot via `quest_boot.go`.
- Tests: unit (region/flag/ordering/registry) + Postgres happy path, idempotent accept, `REGION_CLOSED`, concurrent turn-in (exactly one claim).
