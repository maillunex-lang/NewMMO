# Contract: equipment instances (EX-P08 / T-039)

**Owner:** director · **Version:** 1 (slice a: data, drops, bag placement) · Implements canon `11_ITEMS_LOOT.md` §3, §4, §6 and §9 (equipment roll). Storage: migration `00013_equipment.sql`. No new third-party dependencies. Package: `server/internal/equipment`.

## 1. Model
- **Template** = canon `EQ-CC-RR-SS-NN` (270, exported as `kind=equipment`): class (from the id), region, slot (`MH OH HD CH HA LG FT RG AM`), base level, name.
- **Instance** = one rolled unique item, table `character_equipment_items(instance_id uuid, character_id, template_id, rarity, item_level, required_level, stats jsonb, bind_state, source)`. Never created from client input.
- **Location:** in the bag as a `character_inventory` row with `instance_id` set (quantity 1, never stacks, never in `materials`), or worn in `character_equipped(character_id, slot) → instance_id` (slice b).
- **Stats:** canon `P = 8 + 2.4·L + 0.045·L²`, `budget = round(P·rarity_mult·slot_mult)` split by the class table of `11` §4 (VIT/DEF/STR … ); remainder to the primary stat. Conversion to AP/SP/HP/armor stays in the single class model (`05_CLASSES.md`) and is **not** done here. Affixes (§7), enhancement and bind-on-equip handling are not implemented yet.

## 2. Drop (loot-engine.md §2 step 4, now implemented)
One personal roll per kill: chance `equip_pct` of the loot table (exported with `equip_rarity`); on a hit a slot is chosen uniformly among the 9 and a template **of the character's own class** from the table's region. Same transaction as gold/materials (`FOR UPDATE` on the character, idempotency, ledger). If the bag is full, only the equipment part is lost (`equipment_lost: true`, savepoint) — gold and materials of that kill are still granted. `item_level`/`required_level` interim rule: conflict **C-EQUIP-001**.

## 3. Inventory integration
`inventory.Stack` gains `instance_id` (omitted for plain stacks). `inventory.move.request` may move gear only bag→bag onto an **empty** slot (`STATE_CONFLICT` otherwise). The plain-stack unique index is partial (`WHERE instance_id IS NULL`).

## 4. Slice b (implemented, T-039b): equip / unequip
`inventory.equip.request {instance_id}` / `inventory.unequip.request {slot}`: ownership → class → `required_level` → slot swap (previous item returns to the freed bag slot) → `inventory_revision++` → idempotent result + equipment snapshot scope; stat totals (`gear_score = Σ budget`, informational per canon) in the snapshot. Snapshot scope `inventory` now also carries `equipped[]`, `items{instance_id → details}`, `gear_score`. Next: client UI (slice c) and feeding the totals into combat (`combat-engine.md`) after the owner approves the stat→AP/SP model use.

## 5. Acceptance (slice a)
Unit: budget/distribution/required level; roll class filter and bp thresholds. DB (CI `server-db`): drop creates instance + bag row with stats summing to the budget, replay is idempotent, two instances of one template coexist, gear move rules, full bag loses only the equipment, published build yields 5×6×9 templates.
