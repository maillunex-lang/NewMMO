# Contract: dungeon engine (EX-P09 / T-036)

**Owner:** director · **Version:** 2 (solo server combat in `DNG-01-001`) · Canon: `12_DUNGEONS.md` §2–§4.1, §7, §9–§10. Data: `content/dungeons/dng-01-001.json`. Protocol: `websocket-protocol.md` rev 1.9. No new dependencies.

## 1. Scope and honest interims
Implemented now: **solo `NORMAL`**, data-driven state machine (reused for the other 11 dungeons by adding definitions), linear doors, server-checked puzzle sequence, checkpoints, personal reward claim, first-clear, server-observed combat, reconnect (idempotent re-enter), idle expiry.
**Not yet (recorded, not hidden):**
- No parties/queue/ready-check (no party system yet) → `max_party=1`; `STORY_SOLO`, `VETERAN`, pity (8 dry clears), class RARE equipment drop, reward mail: deferred (equipment instances/mail do not exist).
- The mini-boss interaction remains optional. The owner confirmed the three final flame interactions as the clear condition for the first server-combat slice (C-DNG-001).

## 2. States (canon §3)
Instance: `ACTIVE → BOSS_ACTIVE → COMPLETED → CLOSED`, plus `EXPIRED`, `ABANDONED`. Room: `LOCKED → AVAILABLE → IN_PROGRESS → CLEARED`. Every transition is atomic, appends `dungeon_event_log` with a monotonic `event_seq`, and never skips a step. A door opens only when the server-evaluated `clear` condition of the room holds (`objects` all interacted; `spawns_all` all non-optional spawns defeated in server combat; `puzzle` solved).
Checkpoints: `R01`, `R05`. Defeat resumes from the last checkpoint and reapplies its automatic room progression. The rooms beyond the checkpoint reset. Final-arena wipes preserve the earlier mini-boss clear and never roll a reward.

## 3. Puzzle (R03)
`puzzle.order` is the required interaction order of `OBJ-…-B1/B2/B3` (canon: 2→1→3). A wrong next object **resets only the current sequence** (never locks), logs `PUZZLE_RESET`. Solved only when the full order was interacted. Server-side only; the client never sends "solved".

## 4. Commands (all mutations take `request_id`; repeats are safe)
| C→S | payload | Result S→C |
|-----|---------|------------|
| `dungeon.enter.request` | `dungeon_id, mode` | `dungeon.snapshot` — creates the instance, or **returns the existing active one** (reconnect). Checks level (`entry_level`), region open, one active instance per character, idle expiry (default 10 min) |
| `dungeon.interact.request` | `instance_id, object_id` | `dungeon.snapshot` — object must belong to the current room (`INVALID_TARGET` otherwise) |
| `dungeon.claim.request` | `instance_id` | `dungeon.reward` — only in `COMPLETED`; rolls `rewards.tables` (canon `11` §9) + first-clear `MAT-RR-006`; UNIQUE `(character_id, instance_id, 'FINAL')` |
| `dungeon.leave.request` | `instance_id` | `dungeon.snapshot` (status `ABANDONED`; progress of that instance is lost) |
Errors: `BAD_REQUEST, NOT_FOUND, STATE_CONFLICT, INVALID_TARGET, LEVEL_TOO_LOW, REGION_CLOSED, IDEMPOTENCY_CONFLICT, RATE_LIMITED, INTERNAL`.
`dungeon.snapshot` payload: `instance_id, dungeon_id, mode, status, room_id, checkpoint_room_id, event_seq, rooms{room_id→{state, objects{id→bool}, spawns{id→bool}, puzzle_progress?}}, attempts, combat?`. Envelope `revision` = the character's `state_revision` (unchanged except on claim).

## 4.1 Reward (claim, one transaction)
character `FOR UPDATE` → instance `COMPLETED` & unclaimed → roll each table with `loot.Roll` (bypassing the open-world `Claimable` limit: dungeon bosses are claimed only through this path) → grant `MAT-*` via `inventory.GrantMaterialTx` → gold to `characters.gold`, `state_revision++` → first clear (`dungeon_progress.first_clear_at IS NULL`) adds `first_clear_material_id` ×1 and sets `DF-…-FIRST_CLEAR` → always `DF-…-NORMAL_CLEAR`, `normal_clear_count++` → insert claim row → instance `CLOSED`. **Inventory full ⇒ `STATE_CONFLICT` with nothing changed, claim retryable** (mail system is deferred, nothing is lost).

## 5. Data
Migration `00012_dungeons.sql`: `dungeon_instances` (one active instance per character via partial unique index), `dungeon_event_log` (append-only), `dungeon_progress`, `dungeon_reward_claims`. Definitions are loaded from `content/dungeons/*.json` and validated at startup against the published content build (enemies, loot tables exist).

## 5.1 Room simulation geometry (T-038e)

The registry stores each room's floor width and height, entry point, and each spawn's position. Coordinates are pixels in the editable dungeon scene. The registry gate checks the room, enemy ID and position of each spawn against the scene. Room simulations use the existing 10 Hz combat rules and 180 px/s movement. Automatic respawns are disabled in rooms, with persisted defeat flags used when rebuilding a room. On enter, room changes and checkpoint resets, `dungeon.snapshot.combat` contains a world snapshot and a new opaque `encounter_id`. Room commands (`world.move.request`, `world.stop.request`, `world.cast.request`) include that ID, as do the room's `world.delta` and `world.event` messages. An old stream ID is rejected. Leaving or claiming ends the private stream. Re-entering rebuilds combat from persisted room outcomes, with live HP and positions reset.

The server applies kills and wipes atomically against the expected `event_seq` and pushes `dungeon.snapshot` when progression changes. A player defeat wins over a concurrent kill in that tick. The retired `dungeon.encounter.report` returns `UNKNOWN_TYPE`. Private kills never call the open-world loot hook. Pre-existing instance progress remains readable.

## 5.2 Broken Circle (final arena, above 70% HP)

The server locks two opposite quarter circles at the start of `AB-ENM-01-301-B`. One faces the target. The radius reuses the shared boss signature range. Godot renders the same polygon vertices used for the server hit check after 1.5 s. The 18 s cooldown starts with the warning. In the current unobstructed arena the path to a point outside the attack is at most 257 px, within the 270 px movement allowance.

## 5.3 Double Witness (final arena, 35–70% HP)

The server starts `AB-ENM-01-301-C` after aggro, during phase 2. Each cast marks two diagonal quadrants safe for 2.2 s and damages only players outside them on the resolution tick. The 25 s cooldown begins at cast start. One pair of regular regional enemies joins on the first cast. These temporary summons never grant loot or room progress.

Both arena crystals can be broken once in the same encounter during a cast. A successful break requires the current `encounter_id`, a living player and authoritative distance at most 100 px. Breaking the second crystal interrupts the attack. The registry verifies both crystal placements against the scene. Wipes and reconnects reset the live fight, including crystals. The three flames remain required for progression.

## 5.4 Last Countdown (final arena, below 35% HP)

The server previews all three numbered sectors for 4 s. The first sector faces the target at the start of preparation. The remaining sectors rotate by 120° each and use the same quarter-circle range as Broken Circle. An individual warning leads each 1.1 s strike. A safe route remains open throughout the sequence, and Godot draws the exact polygons used for the server hit checks.

After the third strike the boss takes 15% extra damage for 3 s. The basic attack can run during this window. The next cycle can begin after the window expires. A boss defeat or a room reset clears any pending strikes.

## 6. Acceptance
Closed door until clear condition; wrong object in room ⇒ `INVALID_TARGET`; puzzle reset/solved; wipe → checkpoint rules; reconnect returns same state; idle expiry; replayed server outcomes never advance twice, and parallel claims never double rewards; first clear once; inventory-full claim retryable; one active instance per character; event log append-only. CI `server-db`.
