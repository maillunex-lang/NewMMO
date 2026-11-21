# T-027 — EX-P07 REG-01 side quests start (research)

**Date:** 2026-11-21 · **Lane:** server(+director content interactables) · **Deps:** none new

## Sources

| # | Source | Takeaway |
|---|--------|----------|
| 1 | Canon `09_QUESTS_SIDE.md` SQ-01-001…005 | Events: inspect_object, operate_mechanism, … |
| 2 | T-020 quest engine + D-002 registry | Extend allowlist; map objectives in `reg-01.json` |
| 3 | Flag canon `SQF-01-001-*` / `SQR-01-001-PAID` | Fix FlagName (was `SQF-SQ-…`) |

## Recommendation

In-house: allow EX-P07 events on same interact path; registry rows for SQ-01-001; FlagName aligned to canon. No new modules.
