# Contract: combat loot (EX-P08 / T-034)

**Owner:** director · **Version:** 1 · Implements canon `11_ITEMS_LOOT.md` §9 (tables `LOOT-ENM-*`, units basis points, 10 000 = 100%) for **open-world kills**. Protocol: `websocket-protocol.md` rev 1.2. Storage: migration `00011_loot.sql`. No new third-party dependencies.

## 1. Authority and the interim (R-KILL)
Drops are **rolled by the server** (`crypto/rand`); the client never sends amounts. Because there is **no server combat or position yet** (EX-P09), the server cannot verify that the kill happened. The client reports `combat.loot.request {enemy_id}` and the server:
1. validates `enemy_id` against the published content build (`loot_table` for that enemy; unknown ⇒ `NOT_FOUND`);
2. accepts only categories `обычный`, `элита`, `мини-босс` (dungeon/world bosses ⇒ `UNSUPPORTED_EVENT`, they need instance/weekly rules in EX-P09/P10);
3. applies region gating (`REGION_CLOSED`, same config as quests);
4. rate-limits accepted claims per character (default **30 per rolling minute**, `NEWMMO_LOOT_MAX_PER_MINUTE`) ⇒ `RATE_LIMITED`.
This is a **bounded-exploit interim, not anti-cheat**: a modified client can farm up to the rate cap. It is closed when the server owns combat (EX-P09). Gold/XP from loot is small relative to quest rewards (PACING.md §quests give most XP; loot gives no XP).

## 2. Roll (per claim, in this order)
1. **Gold:** uniform integer in `[gold_min, gold_max]`, × region coefficient (config, default 1.0 — conflict **C-LOOT-001**), floored. Always granted (canon: guaranteed gold).
2. `MAT-RR-001` with probability `mat1_pct`; `MAT-RR-003` with `mat3_pct` (independent rolls, 1 unit each).
3. Special item (`special_item`, `special_pct`, e.g. elite `MAT-01-004` 8%) — independent roll.
4. Equipment roll (3rd percentage) is **deferred** until equipment instances exist (inventory-engine §4); not rolled, not granted.
Per-player personal rolls (canon: no race for the first click) hold by construction: each character claims its own roll.

## 3. Transaction (single DB transaction, character row `FOR UPDATE`)
idempotency check (`idempotency_keys`: same `request_id` + payload ⇒ stored result, different payload ⇒ `IDEMPOTENCY_CONFLICT`) → rate-limit count → roll → grant materials via `inventory.GrantMaterialTx` (stack/overflow rules of inventory-engine) → `characters.gold += gold`, `state_revision++` → append `character_loot_ledger` → store idempotent result → commit. A failure at any step rolls back everything (no gold without items, no ledger without grant).

## 4. Data
`character_loot_ledger(ledger_id, character_id, request_id uuid, enemy_id, loot_table_id, gold, items jsonb, gold_after, created_at)`, `UNIQUE(character_id, request_id)`, **append-only** (triggers reject UPDATE/DELETE/TRUNCATE; app role has `SELECT, INSERT`).
Exporter additions (`tools/content/profiles.py`, additive fields on `loot_table` payload): `category, gold_min, gold_max, mat1_pct, mat3_pct, special_item?, special_pct?`.

## 5. Messages
`combat.loot.request {enemy_id}` → `combat.loot.result {enemy_id, loot_table_id, gold, items[], gold_total, inventory_revision, changes[]}` then, if stacks changed, `inventory.delta {inventory_revision, changes[]}` (same `request_id`). Errors: `BAD_REQUEST` (extra keys, bad uuid), `NOT_FOUND`, `UNSUPPORTED_EVENT`, `REGION_CLOSED`, `RATE_LIMITED`, `IDEMPOTENCY_CONFLICT`, `INTERNAL`.

## 6. Out of scope
Equipment drops/equip UI, dungeon & world-boss loot, loot shared among a party, vendor sell, drop pity, server-verified kills (EX-P09).

## 7. Acceptance
Unit: roll bounds, basis-point thresholds, special item, coefficient, claimable categories. DB (CI `server-db`): grant + ledger + idempotent replay + payload conflict + rejections leave state untouched + foreign account + rate limit + ledger append-only for the app role. Client: `NET_SMOKE_OK` covers request/result/delta.
