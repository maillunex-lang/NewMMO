# Contract: server inventory (EX-P08 / D-006)

**Owner:** director · **Version:** 1 · Implements canon `11` §1–§3 for Stage-1 bag + materials + quest keys. Protocol: `websocket-protocol.md`. Storage extends `db-gameplay.md`. No new third-party deps.

## 1. Containers
| Container | Capacity | Contents |
|-----------|----------|----------|
| `bag` | 80 slots | stackable `MAT-*` / `ITM-*`; later unique `EQ-*` instances |
| `materials` | 200 unique stacks | `MAT-*` only (canon §3 «карман материалов») |
| `quest_keys` | unbounded by slot | `ITEM-Q-*` / `ITEM-SQ-*` via existing `character_quest_items` (not movable) |

Gold/XP/reputation stay on `characters` + ledger — **not** inventory rows.

## 2. Tables (migration `00010_inventory.sql`)
```
character_inventory(
  character_id bigint REFERENCES characters,
  container text NOT NULL CHECK (container IN ('bag','materials')),
  slot int NOT NULL CHECK (slot >= 0),
  item_id text NOT NULL,
  quantity int NOT NULL CHECK (quantity > 0),
  inventory_revision bigint NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (character_id, container, slot)
)
-- UNIQUE (character_id, container, item_id) for stack merge within container when stacking rules allow.
characters.inventory_revision bigint NOT NULL DEFAULT 0  -- bump on every inventory mutation
```
Append-only audit optional later; mutations UPDATE/INSERT under character `FOR UPDATE`.

## 3. Operations (idempotent via `request_id`)
| Client | Effect | Response |
|--------|--------|----------|
| (server grant) | insert/stack MAT into `materials` then overflow to `bag` | `inventory.delta` |
| `inventory.move.request {from_container,from_slot,to_container,to_slot,quantity?}` | validate ownership/rules; swap or stack | `inventory.delta` |
| snapshot `scopes.inventory` | full bags + quest_keys list | inside `state.snapshot` |

Errors: `NOT_FOUND`, `STATE_CONFLICT`, `BAD_REQUEST`, `IDEMPOTENCY_CONFLICT`, `INTERNAL`. No client-authored quantities.

## 4. Stacking (Stage-1)
- `MAT-*`: max 999 (canon §3)
- `QUEST_KEY`: never in bag; already in `character_quest_items`
- Equipment instances: deferred until equip UI (same stage, later task)

## 5. Classes
Character creation already selects `CLS-01…05` (`auth-session.md`). EX-P08 does not re-open class change; document as done for acceptance checkbox.

## 6. Out of scope (later in EX-P08 / P09)
Loot table rolls from combat, merchants, crafting, trading, warehouse, EQ affixes, weight limits.

## 7. Acceptance
Grant MAT stacks persist; snapshot reconnect restores bag; move idempotent; quest keys listed from `character_quest_items`; `go test -race` + CI `server-db`.
