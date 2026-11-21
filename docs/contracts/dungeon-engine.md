# Contract: dungeon engine (EX-P09 / T-036)

**Owner:** director · **Version:** 1 (solo vertical slice of `DNG-01-001`) · Canon: `12_DUNGEONS.md` §2–§4.1, §7, §9–§10. Data: `content/dungeons/dng-01-001.json`. Protocol: `websocket-protocol.md` rev 1.3. No new dependencies.

## 1. Scope and honest interims
Implemented now: **solo `NORMAL`**, data-driven state machine (reused for the other 11 dungeons by adding definitions), linear doors, server-checked puzzle sequence, checkpoints, personal reward claim, first-clear, reconnect (idempotent re-enter), idle expiry.
**Not yet (recorded, not hidden):**
- **R-KILL-DNG:** no server combat simulation → enemy deaths are *reported* by the client (`dungeon.encounter.report`). The server owns the **encounter composition** (it only accepts a death for a `spawn_id` that exists in the current room, once, and ignores reports for other rooms), but cannot verify the fight itself. Closed by the server combat engine (planned T-038); until then the dungeon is **not anti-cheat** and must not gate paid/competitive rewards.
- No parties/queue/ready-check (no party system yet) → `max_party=1`; `STORY_SOLO`, `VETERAN`, pity (8 dry clears), class RARE equipment drop, reward mail: deferred (equipment instances/mail do not exist).
- Mini-boss 50% HP interaction (`OBJ-…-MINI`) is optional; ritual-window timer for the final fires: replaced by "all three final objects interacted".

## 2. States (canon §3)
Instance: `ACTIVE → BOSS_ACTIVE → COMPLETED → CLOSED`, plus `EXPIRED`, `ABANDONED`. Room: `LOCKED → AVAILABLE → IN_PROGRESS → CLEARED`. Every transition is atomic, appends `dungeon_event_log` with a monotonic `event_seq`, and never skips a step. A door opens only when the server-evaluated `clear` condition of the room holds (`objects` all interacted; `spawns_all` all non-optional spawns reported dead; `puzzle` solved).
Checkpoints: `R01`, `R05`. Defeat → back to the last checkpoint room (R01 before the mini-boss is cleared, R05 after); spawns killed in an earlier-cleared room stay dead, the current room's progress resets, killed bosses are never re-rolled for loot.

## 3. Puzzle (R03)
`puzzle.order` is the required interaction order of `OBJ-…-B1/B2/B3` (canon: 2→1→3). A wrong next object **resets only the current sequence** (never locks), logs `PUZZLE_RESET`. Solved only when the full order was interacted. Server-side only; the client never sends "solved".

## 4. Commands (all mutations take `request_id`; repeats are safe)
| C→S | payload | Result S→C |
|-----|---------|------------|
| `dungeon.enter.request` | `dungeon_id, mode` | `dungeon.snapshot` — creates the instance, or **returns the existing active one** (reconnect). Checks level (`entry_level`), region open, one active instance per character, idle expiry (default 10 min) |
| `dungeon.interact.request` | `instance_id, object_id` | `dungeon.snapshot` — object must belong to the current room (`INVALID_TARGET` otherwise) |
| `dungeon.encounter.report` | `instance_id, outcome: enemy_defeated\|player_defeated, spawn_id?` | `dungeon.snapshot` (**interim R-KILL-DNG**) |
| `dungeon.claim.request` | `instance_id` | `dungeon.reward` — only in `COMPLETED`; rolls `rewards.tables` (canon `11` §9) + first-clear `MAT-RR-006`; UNIQUE `(character_id, instance_id, 'FINAL')` |
| `dungeon.leave.request` | `instance_id` | `dungeon.snapshot` (status `ABANDONED`; progress of that instance is lost) |
Errors: `BAD_REQUEST, NOT_FOUND, STATE_CONFLICT, INVALID_TARGET, LEVEL_TOO_LOW, REGION_CLOSED, IDEMPOTENCY_CONFLICT, RATE_LIMITED, INTERNAL`.
`dungeon.snapshot` payload: `instance_id, dungeon_id, mode, status, room_id, checkpoint_room_id, event_seq, rooms{room_id→{state, objects{id→bool}, spawns{id→bool}, puzzle_progress?}}, attempts`. Envelope `revision` = the character's `state_revision` (unchanged except on claim).

## 4.1 Reward (claim, one transaction)
character `FOR UPDATE` → instance `COMPLETED` & unclaimed → roll each table with `loot.Roll` (bypassing the open-world `Claimable` limit: dungeon bosses are claimed only through this path) → grant `MAT-*` via `inventory.GrantMaterialTx` → gold to `characters.gold`, `state_revision++` → first clear (`dungeon_progress.first_clear_at IS NULL`) adds `first_clear_material_id` ×1 and sets `DF-…-FIRST_CLEAR` → always `DF-…-NORMAL_CLEAR`, `normal_clear_count++` → insert claim row → instance `CLOSED`. **Inventory full ⇒ `STATE_CONFLICT` with nothing changed, claim retryable** (mail system is deferred, nothing is lost).

## 5. Data
Migration `00012_dungeons.sql`: `dungeon_instances` (one active instance per character via partial unique index), `dungeon_event_log` (append-only), `dungeon_progress`, `dungeon_reward_claims`. Definitions are loaded from `content/dungeons/*.json` and validated at startup against the published content build (enemies, loot tables exist).

## 5.1 Room simulation geometry (T-038e)

The registry stores each room's floor width and height, entry point, and each spawn's position. Coordinates are pixels in the editable dungeon scene. The registry gate checks the room, enemy ID and position of each spawn against the scene. Room simulations use the existing 10 Hz combat rules and 180 px/s movement. Automatic respawns are disabled in rooms, with persisted defeat flags used when rebuilding a room. These simulation rules are ready for the dungeon session integration.

## 6. Acceptance
Closed door until clear condition; wrong object in room ⇒ `INVALID_TARGET`; puzzle reset/solved; wipe → checkpoint rules; reconnect returns same state; idle expiry; double `claim`/`report` never doubles rewards (parallel claims test); first clear once; inventory-full claim retryable; one active instance per character; event log append-only. CI `server-db`.
