# Contract: WebSocket protocol v1 (D-003)

**Owner:** director · **Version:** `protocol_version = 1`, contract rev **1.5** (EX-P04 + EX-P08 loot + EX-P09 dungeon + world simulation additions, all additive) · Owner approvals (2026-11-21): **`coder/websocket v1.8.15`**, **JSON text messages**.
Basis: canon `14_CONTENT_INTEGRATION.md` §15 (event names and required fields are kept), research `docs/research/go-websocket.md`.
Implementers (server lane: Go; client lane: Godot `WebSocketPeer`) build against this file; changes only via a director task. Machine-readable envelope: `websocket-envelope.schema.json`.

## 1. Transport
| Item | Rule |
|------|------|
| Endpoint | `GET /v1/ws` upgraded to WebSocket; `wss://` in any non-loopback environment (cert + hostname validated by the client); `ws://127.0.0.1` for local dev only |
| Subprotocol | `Sec-WebSocket-Protocol: newmmo.v1` (server rejects others) |
| Frames | **text only**, UTF-8 JSON, **one JSON object per WebSocket message**; binary frames ⇒ close `1003` |
| Compression | **off** (no permessage-deflate) until measured |
| Size limits | client→server ≤ **16 KiB** per message (over ⇒ close `1009`); server→client ≤ 256 KiB |
| JSON hygiene | max nesting depth 8, unknown top-level fields ignored, duplicate keys rejected, numbers: see §3 |
| Auth transport | **never** in URL/query/logs. A short-lived one-time **ticket** is issued over HTTPS (`POST /v1/session/ticket`, defined with the auth task); the first WS message is `session.auth` |
| Library | server: `github.com/coder/websocket v1.8.15` (see `toolchain-pins.md`); client: built-in Godot `WebSocketPeer` (call `poll()` every frame) |
| HTTP timeouts | `http.Server` Read/Write timeouts apply to the handshake only; after upgrade the handler owns deadlines (clear connection deadlines via `ResponseController` / library API) and uses a **session context**, not `r.Context()`, as the session lifetime |

## 2. Envelope
Every message is a JSON object. Direction-specific required fields (canon §15.2 says each message carries `protocol_version`, `session_id`, `server_time`, `revision`, `trace_id`, and that `session_id` comes from authorization and must not be trusted from the client; this is how the contract applies it):

| Field | C→S | S→C | Notes |
|-------|-----|-----|-------|
| `protocol_version` | required (`1`) | required | integer |
| `type` | required | required | event name (§5) |
| `request_id` | required for **mutations** | echoed on every response/error to a request | UUID v4 string, lowercase |
| `payload` | required (object, may be `{}`) | required | event-specific |
| `session_id` | ignored if sent | required | assigned by the server at `session.ready` |
| `server_time` | ignored if sent | required | RFC 3339 UTC, ms precision |
| `revision` | ignored if sent | required | integer, **character-state revision** (§6) |
| `trace_id` | optional (server may adopt a sanitized value) | required | opaque string ≤ 64 chars |

Anything the client sends in a server-owned field is ignored and logged at debug level (never trusted).

## 3. Value rules
- JSON numbers are doubles in Godot: integers must stay **< 2^53**; revisions, counts, ticks are JSON integers. **64-bit ids (`character_id`, claim ids, ledger ids) are sent as strings.**
- Entity ids use canon forms (`MQ-01-01`, `NPC-802`, `DIA.MQ-01-01.START.01`, …); `content_build_id` is the 64-hex build id from `content-export.md`.
- Times: RFC 3339 UTC. Durations: integer milliseconds (`*_ms`).
- Text for players is **never** sent as free text from the server when a canon/localization key exists: send ids/keys; the client resolves text from its content data / `tr()`.

## 4. Session lifecycle
1. Client opens WS (subprotocol `newmmo.v1`). Server starts a **5 s pre-auth timer** and a per-IP pre-auth limit; before auth only `session.auth` is accepted.
2. C→S `session.auth` `{ticket, client_version, protocol_versions:[1], content_build_id?}`. Server validates ticket (one-time, unexpired, bound to account/character), assigns `session_id`.
3. S→C `session.ready` `{session_id, character_id(str), server_time, heartbeat_ms, content_build_id}` then S→C `content.version` `{content_build_id, compatibility: "ok"|"update_required"|"incompatible"}` then S→C `state.snapshot` (full character state: quests, inventory revision, …).
4. Steady state: commands (C→S) → results/deltas (S→C). Every S→C message after `session.ready` carries the envelope of §2.
5. **Heartbeat:** server sends WebSocket **Ping every 25 s**, closes if no Pong within **15 s**. The client does not need an app-level ping.
6. **Reconnect (no resume token in v1):** client reconnects with exponential backoff + jitter (0.5, 1, 2, 4, 8, 15 s cap), gets a **new ticket**, authenticates again and receives a fresh `state.snapshot`. Unacknowledged mutations are re-sent with the **same `request_id`** (idempotent, §7). Mobile background/foreground is a normal reconnect, not a penalty.
7. **Server shutdown:** session registry; on SIGTERM stop accepting, send close `1001`, drain ≤ 10 s, then close (hijacked connections are not closed by `http.Server.Shutdown`).
8. **Multiple sessions per character:** the newest authenticated session wins; the older one is closed with `4005`.

### Close codes
`1000` normal · `1001` going away · `1003` unsupported data · `1008` policy · `1009` too big · `1011` server error · `1013` try again later (slow consumer/overload) · app: `4001` auth failed/expired ticket · `4002` unsupported `protocol_version` · `4003` rate limited · `4004` content incompatible · `4005` replaced by newer session.

## 5. Message catalogue (v1)

Canon events keep their names and required fields (§15.2); added fields are marked ➕. Direction: C→S / S→C.

| Type | Dir | Payload (required) | Notes |
|------|-----|--------------------|-------|
| `session.auth` ➕ | C→S | `ticket, client_version, protocol_versions[]`, opt `content_build_id` | only message allowed before auth |
| `session.ready` ➕ | S→C | `session_id, character_id, heartbeat_ms, content_build_id` | |
| `content.version` | S→C | `content_build_id, compatibility` | canon |
| `state.snapshot.request` ➕ | C→S | `{}` (opt `scopes[]`) | on revision gap / after reconnect (also sent automatically by the server after auth) |
| `state.snapshot` ➕ | S→C | `scopes: {character, quests, …}`, `revision` | full state; replaces client state; EX-P04 scope shapes in `quest-engine.md` §5 |
| `quest.accept.request` | C→S | `quest_id, content_build_id` (+envelope `request_id`) | canon; prerequisites + owner checked |
| `quest.accept.result` | S→C | `quest_id, state, revision` (+`request_id`) | canon; ➕ `ok:boolean`, on failure `error` is sent as `command.error` |
| `quest.progress.delta` | S→C | `quest_id, objective_id, count, revision` | canon; server-authoritative only; **never** C→S; rev 1.1 additive fields `need`, `ready` (true when the quest became `READY_TO_TURN_IN`) |
| `world.interact.request` ➕ (rev 1.1) | C→S | `event, target_id`, opt `instance_id` | mutation; server-validated interaction (`quest-engine.md` §3); answered by `quest.progress.delta` or `command.error` |
| `quest.turnin.request` | C→S | `quest_id` | canon; requires `READY_TO_TURN_IN` |
| `quest.reward.granted` | S→C | `quest_id, reward_claim_id(str), inventory_revision` | canon; exactly-once via ledger; rev 1.1 additive fields `xp_total, gold_total, level, reputation, evidence[]` (`quest-engine.md` §4) |
| `dialogue.open.request` | C→S | `npc_id, dialogue_id` | canon; proximity + predicates |
| `dialogue.node.state` | S→C | `node_id, options[], revision` | canon; options filtered by the server |
| `dialogue.choice.request` | C→S | `node_id, option_id` | canon; allowed transition only |
| `dungeon.enter.request` | C→S | `dungeon_id, mode, party_id` | canon; ➕ `request_id` |
| `dungeon.state.delta` | S→C | `instance_id, phase, revision` | canon |
| `event.snapshot` | S→C | `event_instance_id, phase, remaining_ms` | canon |
| `event.contribution` | S→C | `event_instance_id, contribution_score` | canon; server-computed only |
| `inventory.move.request` ➕ (EX-P08) | C→S | `from_container, from_slot, to_container, to_slot`, opt `quantity` | mutation; server-validated (`inventory-engine.md`); answered by `inventory.delta` or `command.error` |
| `combat.loot.request` ➕ (EX-P08, rev 1.2) | C→S | `enemy_id` (only; no amounts, no extra keys) | mutation; **interim kill report** (`loot-engine.md` §1); answered by `combat.loot.result` (includes the bag `changes[]`; no separate `inventory.delta`) or `command.error` |
| `combat.loot.result` ➕ | S→C | `enemy_id, loot_table_id, gold, items[{item_id,quantity}], gold_total, inventory_revision, changes[]` | server-rolled drop; idempotent replay returns the same result |
| `dungeon.interact.request` ➕ (EX-P09, rev 1.3) | C→S | `instance_id, object_id` | mutation; `dungeon-engine.md` §4; answered by `dungeon.snapshot` |
| `dungeon.encounter.report` ➕ | C→S | `instance_id, outcome, spawn_id?` | mutation; **interim kill report** (R-KILL-DNG) until server combat |
| `dungeon.claim.request` ➕ / `dungeon.leave.request` ➕ | C→S | `instance_id` | mutations; answered by `dungeon.reward` / `dungeon.snapshot` |
| `dungeon.snapshot` ➕ | S→C | `instance_id, dungeon_id, mode, status, room_id, checkpoint_room_id, event_seq, rooms{…}, attempts` | answer to enter/interact/report/leave |
| `dungeon.reward` ➕ | S→C | `instance_id, gold, items[], first_clear, gold_total, inventory_revision, changes[]` | personal reward (single message incl. bag changes, like loot) |
| `world.join.request` ➕ (EX-P09/T-038b, rev 1.4) | C→S | `zone_id` | joins the server simulation of a zone; answered by `world.snapshot`; not a mutation (no `request_id` needed) |
| `world.move.request` ➕ | C→S | `seq` (≥1, strictly increasing), `x`, `y` | movement **target**; no speed field exists (extra keys ⇒ `BAD_REQUEST`); no direct reply — see `world.delta.ack_seq` |
| `world.stop.request` ➕ / `world.leave.request` ➕ | C→S | `seq` / — | stop moving / leave the zone |
| `world.snapshot` ➕ | S→C | `tick, tick_ms, you, bounds{}, entities[{id,kind,ref,name?,level?,x,y,facing,moving,hp,max_hp}]` | full interest-filtered state on join |
| `world.delta` ➕ | S→C | `tick, ack_seq, upsert[…], remove[ids]` | per tick, only when something changed; **droppable** under backpressure (the next delta supersedes it); envelope `revision` is 0 and is not part of character-revision tracking |
| `world.cast.request` ➕ (rev 1.5) | C→S | `skill_id`, opt `target_id` | **mutation** (`request_id`); canon checks, see `combat-engine.md`; answered by `world.cast.result` or `command.error` |
| `world.cast.result` ➕ / `world.event` ➕ | S→C | `skill_id, target_id?, amount, crit?, target_hp?, self{}` / `tick, events[]` | result of a cast / droppable combat events (cast, damage, heal, shield, death, respawn) |
| `inventory.delta` | S→C | `inventory_revision, changes[]` | canon; never client-authored |
| `command.error` ➕ | S→C | `request_id, code, message_key`, opt `retry_after_ms` | correlated error for any rejected C→S message |

**Reserved, not defined in v1** (separate contracts when EX-P08/P09 start): `movement.input`, `world.snapshot`, `world.delta`, `combat.ability.request`, `combat.result`, chat. Servers must reject unknown C→S types with `command.error UNKNOWN_TYPE` (no disconnect) and clients must ignore unknown S→C types (log only).

### Error codes (`command.error.code`)
`BAD_REQUEST`, `UNAUTHENTICATED`, `UNKNOWN_TYPE`, `PROTOCOL_UNSUPPORTED`, `RATE_LIMITED` (+`retry_after_ms`), `NOT_FOUND`, `STATE_CONFLICT` (e.g. quest not READY), `PREREQUISITE_NOT_MET`, `OUT_OF_RANGE` (proximity), `CONTENT_MISMATCH`, `IDEMPOTENCY_CONFLICT`, `REGION_CLOSED` (rev 1.1), combat (rev 1.5): `ON_COOLDOWN`, `GCD`, `NOT_ENOUGH_RESOURCE`, `SKILL_LOCKED`, `UNSUPPORTED_SKILL`, `DEAD`, `INVALID_TARGET`, `INTERNAL`. `message_key` is a localization key (`error.<code>`), never free text.

## 6. Revisions and snapshots
- Envelope `revision` = the **character-state revision**: a per-character monotonic integer incremented by the server for every state change it pushes (quest, inventory, dialogue-choice flags). It is persisted with the state (EX-P04 `revision` columns).
- Deltas apply strictly in order. The client keeps `last_revision`; on `revision != last+1` for a delta, or after reconnect, it sends `state.snapshot.request` and **drops deltas until the snapshot arrives**; after the snapshot, deltas with `revision > snapshot.revision` apply.
- World/combat streams (future) use their own `world_revision`/`server_tick` in the payload; they never share the character revision.

## 7. Idempotency
- Every mutation carries `request_id` (UUID v4). Server dedup key: `(character_id, request_id)`; **truth lives in the database** (unique constraints / `reward_claims` ledger, EX-P04), never only in session memory. Retention ≥ 24 h.
- Same `request_id` + same payload → the **original result is returned again** (no second effect). Same `request_id` + different payload → `command.error IDEMPOTENCY_CONFLICT`.
- `quest.turnin.request` ≠ "claim": the server checks `READY_TO_TURN_IN` and completes the reward in one DB transaction (`SELECT … FOR UPDATE`, `INSERT reward_claims … ON CONFLICT DO NOTHING RETURNING`, state/ledger writes, commit) **before** sending `quest.reward.granted`.

## 8. Limits and backpressure (initial values; tunable by config, re-measured later)
| Limit | Value |
|-------|-------|
| Handshake | ≤ 10 per minute per IP; pre-auth connections ≤ 3 per IP |
| Commands | token bucket 20 msg/s, burst 40 per session; quest/dialogue/dungeon mutations 5/s |
| Outbound queue | one bounded queue per session (256 messages) and one writer loop; critical messages (`quest.*.result`, `quest.reward.granted`, `inventory.delta`, `command.error`) are never dropped: on overflow close `1013` and rely on reconnect + snapshot; replaceable future position deltas may be coalesced |
| Write timeout | 10 s per message, then close |
| Violations | `RATE_LIMITED`; repeated abuse ⇒ close `4003` and audit log |

## 9. Security rules
No secrets/tickets/tokens in logs (log ids, sizes, codes only). The client is never authoritative for position, damage, quest state, currency, inventory or time. `character_id` is bound to the ticket, never taken from messages. `Origin` is not authentication. All request fields validated for type/size/range before use; unknown fields ignored.

## 10. Acceptance for the first implementation (EX-P04 server lane)
Upgrade works through the logging wrapper (HTTP 101, `Hijack`); wrong subprotocol/binary/oversize/malformed JSON/duplicate keys rejected; pre-auth timeout; ticket one-time; heartbeat ping/pong timeout; slow consumer closed with `1013` without blocking others; shutdown closes all sessions with `1001`; duplicate `request_id` returns the original result without a second reward (**T-020**, quest/reward engine); revision-gap flow (server snapshot request/response in **T-019**, client revision detection in **T-023**); `go test -race`, `govulncheck` clean.
