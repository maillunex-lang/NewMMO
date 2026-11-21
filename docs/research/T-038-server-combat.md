# T-038 — server-authoritative combat: research and proposal

**Status:** research + proposal for **owner approval** (architecture decision, AGENTS.md). No code written.
**Goal:** one server-owned simulation for movement, enemy AI, skills, damage, threat, death and loot triggers, replacing the two interim kill reports (**R-KILL**, `combat.loot.request`; **R-KILL-DNG**, `dungeon.encounter.report`) and closing R-PROX (no proximity checks).
**Method note:** canon was read in full for the relevant parts; library facts come from web search on 2026-11-21 (listing/docs pages, not the repos' LICENSE files) — **licences below are unverified and must be re-checked before any adoption.** Nothing third-party is proposed for installation.

## 1. What canon already fixes (not up for debate)
| Topic | Canon | Source |
|---|---|---|
| Authority | server owns coordinates, HP, AI, threat, collisions, casts, rewards; client never sends damage/heal/crit/cooldown/position as truth | `06` §9, `05` §12 |
| Tick | **10–20 Hz** logic tick, tuned after profiling the VPS | `06` §2, `05` §12 |
| Client command | `cast_skill(request_id, skill_id, target_entity_id?, target_point?, client_tick)`; server checks class, slot, level, status, resource, GCD, cooldown, LOS, range, zone; idempotent `request_id` | `05` §12 |
| Formulas | `MaxHP, AP, SP, Armor, Resist` (`05` §3); enemy `MaxHP/AP/Armor` by level × category K (`06` §2); damage `coef × AP|SP × 100/(100+armor)`, floor 1 | `05`, `06` |
| Enemy AI | FSM `IDLE→PATROL→ALERT→CHASE→ATTACK/CAST→RECOVER`, + `STAGGER, LEASH_RETURN, DEAD`; profiles melee/ranged/caster/tank/defender/skirmish; aggro 7–9 m, leash 24 m; threat `damage + 0.6·heal + taunt`; LOS; NavMesh paths | `06` §3 |
| Telegraphs | AoE visible before damage (≥0.8 s normal, ≥1.3 s boss); hits are server-side, animation is not the source of truth | `06` §3 |
| Sync | snapshots with deltas, interest management by zone, server seed and timestamps, reconnect without duplicating NPCs | `06` §9 |
| Persistence | outcomes only (xp, gold, items, quest/dungeon/event logs); mobs are not written per second | `06` §9 |
| Autobattle | only uses allowed commands; no LOS/cooldown/range bypass | `05` §8 |

## 2. Architecture options
| # | Option | Fit | Verdict |
|---|---|---|---|
| A | **In-process Go simulation** in the existing server: one goroutine per *zone/instance*, fixed-step tick (start 10 Hz), command inbox (bounded channel) fed by WS sessions, in-memory entity store, snapshot/delta out through the existing session writer; outcomes persisted through the existing engines (quest, loot, dungeon, inventory) | matches canon, no new deps, reuses auth/WS/DB/ledger; testable with a fake clock and seeded RNG | **Recommended** |
| B | Game-server framework, e.g. **Nakama** (Heroic Labs; authoritative match handlers, Go runtime, tick 1–60 per its docs) | solves ticking/matchmaking, but brings its own accounts, sessions, storage and socket protocol — duplicating and conflicting with our auth, `newmmo.v1` protocol, contracts and PostgreSQL schema; licence not verified here | Reject for now; borrow ideas only |
| C | Separate combat microservice (gRPC/NATS between gateway and combat process) | isolates crashes, scales horizontally; but operational weight, HTTP/2-only gRPC is not usable in this environment, no need at our scale | Revisit if one process can't hold the load |
| D | Client-authoritative combat with server "sanity checks" | cheap | **Rejected — violates canon** |
| E | Go ECS library (e.g. Arche, MIT per its repo page; Ark; Donburi, licence unclear) | faster iteration over thousands of entities | Not needed: an instance holds tens of entities. Plain structs/slices first; revisit only if profiling says so |

## 3. Proposed design (option A)
**Units.** `internal/world`: `Zone` (open-world area, e.g. Dawn Valley) and `Instance` (dungeon run) share one `Sim`: `Step(dt)` every tick, `Apply(Command)` before stepping, `Snapshot()/Delta()` after. Wall-clock is injected, RNG is seeded per sim → deterministic unit tests (golden damage tables, AI traces).
**Entities.** `Player{id, pos, hp, resources, cooldowns, statuses, threat-source}`, `Enemy{id, def, spawn, pos, hp, fsm, threat table, cooldowns}`; definitions loaded from the published content build (like the loot catalog) plus an ability table.
**Movement.** Client sends *intent* (`move.input {seq, target|dir, client_tick}`), server integrates at tick rate with a speed cap and obstacle test; client **predicts its own movement** and reconciles on snapshots, and **interpolates others** with a ~100–150 ms buffer (standard snapshot-interpolation practice). Phase 1 uses world bounds + speed validation; real collision needs a server-readable export of the `.tscn` Collision/Navigation layers (shared with EX-W01 generator contract, D-005).
**Combat.** `cast_skill` validated as in canon §12; cast times, telegraph windows, cooldowns/GCD, resource costs, statuses (slow/stun with canon caps and diminishing returns); damage via one `damage` package that implements the canon formulas (shared by player skills and enemy abilities); threat table; death is terminal per spawn instance; respawn timers from `SPAWN-*`.
**Outputs.** `world.snapshot` (full, on join/reconnect) and `world.delta` (entities changed in the player's interest cell), `combat.event` (hit, heal, death, telegraph start/end) with authoritative timestamps; interest management = uniform grid per zone.
**Outcomes → existing engines.** Enemy death emits a `Kill{character_id, enemy_id}` to the loot engine (replacing `combat.loot.request`) and to the quest engine (kill objectives); dungeon rooms are `Instance` rules driven by sim events (replacing `dungeon.encounter.report`); inventory/gold/xp still go through the transactional paths already in `internal/*`.
**Persistence.** Sim state is in memory; on restart open-world mobs respawn, dungeon instances are expired/abandoned (existing idle-expiry policy) — no per-second DB writes.
**Client.** Godot keeps rendering, animation, VFX, camera, input; monster scripts become **visual puppets** driven by snapshots (their local AI/damage is removed online; kept for the offline demo); skill bar sends `cast_skill`; telegraphs drawn from `combat.event`.

## 4. Task split (each = research note + contract slice + CI-green slice)
| ID | Scope | Depends |
|----|-------|---------|
| T-038a | **Data + formulas:** extract enemy stats/abilities/AI profiles (`06`) and class skills (`05`) into the export (additive fields) or a registry; `internal/damage` implementing canon formulas with golden tests | owner OK |
| T-038b | **Sim core:** `internal/world` tick loop, entity store, movement with speed cap, snapshot/delta over WS (`world.*`), one test zone with 3 archetypes; fake-clock tests; load test at N entities | T-038a |
| T-038c | **Combat + AI:** `cast_skill`, cooldown/GCD/resources, statuses, FSM + threat, death/respawn; kill events → loot + quest engines; **retire R-KILL** | T-038b |
| T-038d | **Client:** puppets from snapshots, prediction/interpolation, skill bar → `cast_skill`, telegraphs, HUD; offline fallback kept | T-038c |
| T-038e | **Dungeon on the sim:** `Instance` rules for `DNG-01-001`, boss phases, wipe/checkpoint; **retire R-KILL-DNG** | T-038c, T-038d |
| T-038f | **Measure:** CPU/RAM per zone and per 100 sessions on the VPS-class host, Android frame cost with N puppets (also R-DEVICE) | T-038d |

## 5. Risks and mitigations
- **Scope/time:** this is the largest system so far → vertical slices, each CI-green and shippable; the interim kill reports stay until T-038c lands (flagged in contracts).
- **Network feel on mobile latency:** prediction + interpolation, tick 10 Hz start, input sequence numbers; verify with an artificial-latency test in CI (FakeTransport already supports delays).
- **Server CPU unknown (VPS size not given):** measure in T-038f; keep per-zone tick budget and an overload policy (skip non-critical deltas, never drop critical combat events).
- **Collision/nav data does not exist server-side yet:** phase 1 bounds-only; collision export shared with the location generator (D-005) — otherwise players can walk through walls on the server's view.
- **Determinism/testing:** fixed step, injected clock and seeded RNG, golden tests for formulas and AI traces.
- **Cheating surface:** command rate limits (existing WS limits), speed/teleport checks, no client-supplied outcomes; audit log of combat events for disputes.
- **Persistence on crash:** acceptable by design (outcomes already transactional); documented per system.

## 6. Decisions requested from the owner
1. **Approve option A** (in-process Go simulation, no framework, no new dependency).
2. **Start tick at 10 Hz**, configurable up to 20 after measurement.
3. **Approve the split T-038a…f** and the order (data/formulas first).
4. **Movement authority in phase 1:** server integrates movement with speed cap + world bounds now; real collision later with the generator's collision export. (Alternative: delay movement authority until collision export exists — leaves position-cheating open longer.)
