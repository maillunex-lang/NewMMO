# T-024 — Server dialogue engine (research)

**Date:** 2026-11-21 · **Lane:** server · **Deps:** stdlib + pgx v5.13.0 (already pinned)

## Sources

| # | Source | Takeaway |
|---|--------|----------|
| 1 | Contract `dialogue-engine.md` + `websocket-protocol.md` + `db-gameplay.md` | Open/choice, option filter, append-only choices, R-PROX deferred |
| 2 | Existing T-020 `server/internal/quest` + WS `QuestHandler` | Same transaction/idempotency pattern; optional handler on `ws.Server` |
| 3 | Hysen Labs / Godot dialogue plugins / YarnSpinner-style trees | Full tree runtimes are overkill for Stage-1 MQ flows; would add deps |
| 4 | Client `flows` + `dialogue_ui.gd` option set | ACCEPT/TURN_IN/ASK/LATER/CONTINUE — server must filter; quest accept/turn-in stay on quest messages |

## Candidates

| # | Approach | Verdict |
|---|----------|---------|
| 1 | **In-house filter + pgx + flows JSON** | **Adopt** |
| 2 | Yarn/DialogueManager server port | New dep — reject |
| 3 | Client-trusted options | Violates authority — reject |

## Recommendation

Implement `server/internal/dialogue` against `docs/contracts/dialogue-engine.md`; lines from active `dialogue_line` entities; Stage-1 modes from `content/dialogues/flows_reg01.json`. Wire `DialogueHandler` on WS like quests. No new Go modules.
