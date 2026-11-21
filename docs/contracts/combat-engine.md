# Contract: combat engine, slice 1 (EX-P09 / T-038c1)

**Owner:** director · **Version:** 1 · Basis: canon `05_CLASSES.md` §3/§8/§12, `06_ENEMIES.md` §2–§3; research `docs/research/T-038-server-combat.md`. Builds on `world-sim.md`. No new dependencies.

## 1. Scope
Player skills on the server simulation: `world.cast.request` with canon checks, damage/heal/shield, resources, cooldowns and GCD, server-observed enemy death with personal loot (closes **R-KILL** for the open world once the client casts through the server — client work is T-038d), enemy respawn.
**Not yet (T-038c2+):** enemy AI and attacks on players (enemies are still static targets), player death/respawn, threat, statuses (slow/stun/taunt), area/cone/line/point/ground skills, buffs/mitigation/cleanse/haste/teleport/stealth etc., party/ally targeting, quest kill objectives, dungeon on the sim (T-038e).

## 2. Command
`world.cast.request {skill_id, target_id?}` — a **mutation** (`request_id` required, UUID; 5/s mutation bucket). `target_id` (zone entity id) is required for single-target damage and optional/self for shield/heal; any other key ⇒ `BAD_REQUEST` (the server never accepts damage, crit, cooldown or position).
Validation order: joined session owner → caster alive (`DEAD`) → skill exists, belongs to the caster's class and is unlocked at the caster's level (`SKILL_LOCKED`) → effect supported (`UNSUPPORTED_SKILL`) → GCD 0.75 s (`GCD`) → per-skill cooldown (`ON_COOLDOWN`) → resource cost (`NOT_ENOUGH_RESOURCE`) → target alive enemy (`INVALID_TARGET`) → server-side range (`OUT_OF_RANGE`). Only then resources are paid and cooldowns start.
**Supported effects:** `damage` on `enemy`; `shield` on `self` (≤35% MaxHP, canon duration); `heal` on `ally|self` (solo: self; capped at missing HP). `request_id` replay inside a 32-entry window returns the stored outcome without applying it again; the same id with different content ⇒ `STATE_CONFLICT`.
Reply `world.cast.result {skill_id, target_id?, amount, crit?, target_hp?, self{…}}`; errors via `command.error` (codes above, all on the protocol list).

## 3. Rules and numbers
- **Damage:** `damage.Damage(coef, AP|SP, 0, Armor|Resist, 1, 1)` (canon formulas, one rounding), crit chance 5% (cap 35%) ×1.5 rounded, seeded RNG per sim. `scale AP` hits Armor, `scale SP` hits Resist.
- **Resources (05 §3):** Guard/Focus regen 6/s out of combat, 3/s in; Mana 5/2; Rage starts 0, −5/s out of combat, no passive gain; Light starts 30 and drifts to 30 out of combat. Skill `cost` is paid before execution; skill-text gains (`+10 стойкости`, `+12 ярости`, `+10 фокуса`, `+8 света`) are parsed into `Gain` and applied on cast. In combat for 5 s after any combat action.
- **Cooldowns/GCD** in ticks (`ceil(seconds × tick_rate)`); the HUD gets `self{resource, gcd_ms, cooldowns{skill→ms}, shield}` inside `world.delta` while it changes.
- **Range (interim, C-RANGE-001):** canon gives no skill ranges. 1 m = 32 world units; melee classes (CLS-01/02) 2.5 m, ranged classes (CLS-03/04/05) 9 m, plus 1 m latency slack. Tunables live in `world.Combat`.
- **Death and loot:** HP 0 ⇒ `death` event, enemy marked `dead`, respawns at its spawn after 30 s (interim; canon 45–90 s per spawn). Every player who dealt ≥10% of the enemy's MaxHP gets a **personal** loot roll via `loot.Engine.GrantKill` (server-observed, no per-minute cap, region gate/ledger/idempotency unchanged), delivered as `combat.loot.result` (critical push, revision = state revision).

## 4. Messages (additive, protocol rev 1.5)
`world.cast.request` C→S; `world.cast.result` S→C; `world.event {tick, events[{type: cast|damage|heal|shield|death|respawn, caster?, target?, skill?, amount?, crit?, hp}]}` S→C (droppable, only events involving entities the viewer knows); `world.delta` gains `self`; entity `View` gains `dead`.

## 5. Acceptance
Pure sim: golden damage (34 vs armor 10), crit cap and rate, GCD, cooldown, resource cost/gain (Rage), regen/decay, range (melee vs ranged), invalid/dead/locked/unsupported/wrong-class, shield cap and expiry, heal cap, contribution threshold for loot, respawn at home with full HP, dead caster. Zone/handler: request-id idempotency, conflict on changed content, events and `self` state in the tick, asynchronous kill hook with contributor/account. WebSocket: out-of-range, strict payload, missing request id, unsupported/locked codes, walk → cast → result + damage event + cooldown state, replay and GCD/cooldown refusal.
