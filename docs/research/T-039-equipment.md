# T-039 — Equipment, drops and rewards (EX-P08 research)

**Date:** 2026-11-21 · **Lane:** director · **Deps:** none new

## Existing solutions (searched; see T-029 for the inventory survey)
| # | Source | License | Fit |
|---|--------|---------|-----|
| 1 | [gloot](https://github.com/peter-kish/gloot) (Godot inventory) | MIT | Client-side item/equip data model; would put authority on the client and add a dependency — reject |
| 2 | [Wyvernbox](https://godotengine.org/asset-library/asset/1919) | MIT | Action-RPG loot + equipment UI; heavy, client-authoritative — reject for now |
| 3 | [Nakama](https://github.com/heroiclabs/nakama) storage/economy | Apache-2.0 | Generic server; replaces the Go server — out of scope (architecture invariant) |
| 4 | Canon `11_ITEMS_LOOT.md` §4/§6/§7/§9/§14 | project | Fully specifies templates, budget formula, class split, drop table, server order of operations |

## Decision
In-house Go + PostgreSQL (same pattern as inventory/loot), data from the canon export, thin `.tscn` UI later. No new dependency, no new asset. Contract: `docs/contracts/equipment-engine.md`. Open canon gap: **C-EQUIP-001** (drop item level / wear requirement).
