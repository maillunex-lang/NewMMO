# Contract: server quest engine, interactions and rewards (D-004 / D-002)

**Owner:** director · **Version:** 1 · Implements canon `14` §15.1 (state machine), `08`/`09` (cards), `11` §12 (evidence items) for **EX-P04/P07**. Data comes from the active content build (`content-export.md`): quest payload fields `title, objectives[], prereqs[], min_level, reward{xp,gold,reputation[],evidence[]}, flags[], npcs[], locations[]`. Interaction targets come from `content/interactables/reg-01.json` (registry, D-002). Storage: `db-gameplay.md`; protocol: `websocket-protocol.md` (+ rev 1.1 additions).

## 1. States (per character)
`LOCKED → AVAILABLE → ACTIVE → READY_TO_TURN_IN → COMPLETED` (canon §15.1). **Only `ACTIVE`, `READY_TO_TURN_IN`, `COMPLETED` are stored** (`character_quest_states`); `LOCKED`/`AVAILABLE` are **derived** from content + the character's rows + flags:
- `AVAILABLE` ⇔ not stored **and** every quest in `prereqs` is `COMPLETED` (an empty list ⇒ gated only by the flag `TUTORIAL-COMPLETE` for `MQ-01-01`) **and** `level(xp) ≥ min_level` **and** (main quests) no other main quest is `ACTIVE`/`READY_TO_TURN_IN` **and** (side quests) ≤ **5** side quests active.
- Otherwise `LOCKED`. `COMPLETED` never returns to `ACTIVE` (one-shot MQ/SQ). `FAILED/ABANDONED` are **not** implemented in EX-P04.

## 2. Operations (all idempotent via `request_id`, `db-gameplay.md` §4)
| Client message | Preconditions | Effects (one transaction) | Responses |
|----------------|---------------|---------------------------|-----------|
| `quest.accept.request {quest_id, content_build_id}` | quest `AVAILABLE`; `content_build_id` equals the server's active build (else `CONTENT_MISMATCH`) | insert state `ACTIVE`, `content_build_id` pinned, empty progress; `QF-…-START`/`SQF-…-ACCEPTED` flag; `state_revision++` | `quest.accept.result {quest_id,state:"ACTIVE",revision,ok:true}` (no per-objective deltas at accept; the client derives zero progress from the result and the snapshot) |
| `world.interact.request {event, target_id, instance_id?}` (rev 1.1) | exactly one `ACTIVE` quest has the **current** objective matching (§3); `quest` content = the pinned build | record progress (and `quest_objective_instances`); when all objectives complete ⇒ state `READY_TO_TURN_IN` and flag `…-DONE`; `state_revision++` | `quest.progress.delta {quest_id,objective_id,count,revision}`; when ready also `quest.accept.result`-like **`quest.state.changed`** is NOT used — the delta carries `ready:true` (additive field) |
| `quest.turnin.request {quest_id}` | state `READY_TO_TURN_IN` | **reward transaction (§4)** ⇒ `COMPLETED` | `quest.reward.granted {quest_id, reward_claim_id, inventory_revision, xp_total, gold_total, level, reputation:{FAC-08:n}, evidence:["ITEM-Q-…"]}` (extra fields are additive) |
Errors (`command.error`): `NOT_FOUND` (unknown quest), `STATE_CONFLICT` (wrong state, e.g. turn-in before READY, accept when not AVAILABLE), `PREREQUISITE_NOT_MET`, `LEVEL_TOO_LOW`, `UNSUPPORTED_EVENT`, `INVALID_TARGET`, `CONTENT_MISMATCH`, `IDEMPOTENCY_CONFLICT`, `RATE_LIMITED`.

## 3. Objective rules (server authoritative)
- Objectives are **ordered**: objective N is current only after N−1 is complete (matches canon `START→OBJ1→OBJ2→OBJ3→TURNIN` and the SQ rule «по порядку»).
- A `world.interact.request` counts only if the registry entry of the **current** objective has the same `event` and `target_id`, and — when `count > 1` — `instance_id` is one of the entry's `instances` **not yet recorded** for this character+objective (`quest_objective_instances` unique key). Same instance twice ⇒ `INVALID_TARGET` (no progress).
- Supported `event` values in EX-P04: `npc_interact`, `world_interact`, `escort_checkpoint`; anything else ⇒ `UNSUPPORTED_EVENT` (others — `inspect_object`, `operate_mechanism`, `collect_quest_evidence`, kills, dungeon/event events — arrive with their systems).
- **Known interim gap (R-PROX, owner-visible):** there is no server-side movement/position yet, so **proximity/visibility are not verified** in EX-P04. The server still verifies state, ordering, target registry, distinctness and rate limits. Proximity and anti-teleport checks are mandatory when movement authority exists (EX-P08/P09) — until then the slice is not safe against a cheating client **moving** (not against granting rewards without the quest flow).
- No client message can complete an objective directly or claim a reward; `READY_TO_TURN_IN` is reached only by server-evaluated objectives.

## 4. Reward transaction (turn-in) — exactly-once
Under the character row lock: verify state `READY_TO_TURN_IN` and the pinned build; `INSERT reward_claims(claim_id=uuid, source_kind='quest', source_instance_id=<quest_id>, reward_bundle_id='REWARD-<quest_id>', content_build_id) ON CONFLICT (character_id, source_kind, source_instance_id) DO NOTHING RETURNING claim_id` — no row ⇒ already claimed ⇒ return the stored result of the first claim (no second effect). Then in the same transaction: append `character_ledger` rows (xp, gold, one per reputation entry, one per evidence item), update `characters.xp/gold` (+ `balance_after`), upsert `character_reputation` (value clamped to the canon range, if any, otherwise unbounded in EX-P04), insert `character_quest_items` for evidence (`ITEM-Q-*`), set quest `COMPLETED` (+`completed_at`) and flags `…-REWARD`, `state_revision++`, store the idempotency result. Rewards come **only from the pinned build's payload** (`08/09` numbers; never from the client, never from loot tables). Commit; **then** push `quest.reward.granted` and (if the unlock changed) a fresh `state.snapshot` scope `quests`.
Level is derived: `level = max L (1…60) with Σ_{k=1..L-1} round(100·k^1.6) ≤ xp` (canon `05` §6). Level-ups need no separate message in EX-P04 (clients read `level`).

## 5. Snapshot (`state.snapshot` scopes, additive to protocol v1)
```
scopes.character = {character_id(str), name, class_id, level, xp, gold, reputation:{"FAC-08":20,…}}
scopes.quests    = {active:[{quest_id,state,objectives:[{objective_id,have,need}],content_build_id}],
                    completed:[quest_id,…], available:[quest_id,…]}
```
`revision` = `characters.state_revision`. Sent after `session.ready`, on `state.snapshot.request`, and after unlock changes.

## 6. Out of scope (later stages)
Dialogue trees and server-filtered options (EX-P05), inventory/loot/equipment (EX-P08), dungeons/events (EX-P09/P10), failure/abandon, daily/repeatable quests, party/shared progress, combat objectives (kill counts), proximity (EX-P08/P09).

## 7. Acceptance (server lane)
Accept → interact×N → turn-in happy path for `MQ-01-01…05` against the real exported content; ordering/distinct-instance/duplicate-request rules; state conflicts; level/prereq gating; idempotent replay of every mutation returns the original result; **concurrent double turn-in ⇒ exactly one claim and one set of ledger rows**; reward numbers equal canon (`08`) for each of the 5 quests; ledger sums = aggregates; build pinning (publishing a new build mid-quest does not change the reward); snapshot reflects state after reconnect; `go test -race`, CI `server-db`.
