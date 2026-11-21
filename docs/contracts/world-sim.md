# Contract: world simulation (EX-P09 / T-038b)

**Owner:** director · **Version:** 1 · Basis: `docs/research/T-038-server-combat.md` (owner-approved 2026-11-21: in-process Go simulation, 10 Hz, phase-1 movement authority with speed cap + bounds). Protocol: `websocket-protocol.md` rev 1.4. No new dependencies.

## 1. Scope of this slice
Players and **static** enemies in a zone, movement owned by the server, interest-filtered snapshots/deltas. **Not yet** (T-038c…f): casting, damage, AI/threat, death, loot hook, client rendering, collision against level geometry (phase 1 = world bounds only), persistence of positions.

## 2. Model
- `internal/world.Sim` is pure and deterministic: `Step()` advances exactly one tick (`dt = 1/tick_rate`); no wall clock, no goroutines. `Zone` wraps one Sim with a mutex (commands and ticks serialised) and a `Run` loop at `tick_rate`; tests call `Tick()` themselves.
- **Movement:** the client sends a *target* (`world.move.request {seq,x,y}`); the server moves the entity toward it at `player_speed` (zone config, units/second) — **no speed or position is ever read from the client**, so speed hacking and teleporting are impossible by construction. Targets are clamped into the bounds; NaN/Inf rejected; `seq` must strictly increase per character (stale/duplicate commands are ignored, reordering-safe). Arrival stops the entity. Facing uses the game's 8 rows (0=E,1=SE,2=S,3=SW,4=W,5=NW,6=N,7=NE).
- **Entities:** ids are per-zone integers; players `ref` = character id, enemies `ref` = canon `ENM-*` (HP from `damage.EnemyStats`, name/level from the content build). Enemies are not movable by clients.
- **Interest:** an entity is sent to a viewer while within `interest_radius` of it (brute-force radius test; a grid replaces it if profiling asks). Entering ⇒ `upsert`, leaving/disappearing ⇒ `remove`.
- **Ownership:** each joined character is bound to the WebSocket session (`owner`) that joined. A newer session of the same character takes over (old socket closed with `4005` by the existing rule); the old session's disconnect never removes the entity of the new one. A failed `push` (closed session) drops the player from the zone.

## 3. Messages
`world.join.request {zone_id}` → `world.snapshot`; `world.move.request {seq,x,y}`, `world.stop.request {seq}`, `world.leave.request {}` → no direct reply (effects arrive as `world.delta`). Errors: `BAD_REQUEST` (extra keys, bad seq/coordinates), `NOT_FOUND` (unknown zone or character), `STATE_CONFLICT` (command before join / not the owning session), `RATE_LIMITED` (general command bucket, 20/s burst 40).
`world.delta {tick, ack_seq, upsert[], remove[]}`: `ack_seq` is the last movement sequence the server applied for the receiving player (input for client reconciliation in T-038d). Deltas are **droppable**: if the outbound queue is full the delta is skipped, never the session closed (critical messages keep the existing rule).

## 4. Data
`content/zones/*.json` (`zone_id, region_id, tick_rate 5–30, player_speed, interest_radius, bounds, player_spawn, enemies[{spawn_id, enemy_id, x, y}]`), checked against canon by `tools/content/check_zone_defs.py` (CI). The first zone `ZONE-01-DAWN-VALLEY` mirrors the stage-1 client scene (units = client pixels; stand-ins bandit→ENM-01-001, wolf→ENM-01-003, slime→ENM-01-004).
Combat numbers come from the published build through `internal/combatdata.LoadFromDB` (kinds `enemy`, `ability`, `skill`).

## 5. Acceptance
Pure sim: server-only speed, per-tick step cap, arrival/facing, bounds and invalid targets, sequence ordering, static enemies, interest enter/leave, removal announcement, deterministic traces. Zone/handler: join/move/delta/leave, foreign owner rejected, failed push drops the session, strict payload validation, newer session takeover. WebSocket integration: join → snapshot, `speed` field rejected, deltas with `ack_seq`, UNKNOWN_TYPE for unknown world types, takeover (`4005`), removal on disconnect, no world payload in logs.
