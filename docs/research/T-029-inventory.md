# T-029 — Inventory / loot / classes (EX-P08 research)

**Date:** 2026-11-21 · **Lane:** research(+director) · **Deps:** none new until owner OK

## Sources

| # | Source | License / notes | Takeaway |
|---|--------|-----------------|----------|
| 1 | Canon `11_ITEMS_LOOT.md` §3 | project | 80 bag slots; materials pocket 200 stacks; QUEST_KEY separate; server authoritative |
| 2 | Existing `character_quest_items` + ledger (T-020) | in-repo | QUEST_KEY already append-only; gold/xp not bag items |
| 3 | Server inventory architecture (SourceGame #44) | article | def vs instance; stackable rows; FOR UPDATE transactions; never trust client quantities |
| 4 | [gloot](https://github.com/peter-kish/gloot) | MIT, Godot 4.4+ | Rich client UI; would be a **new dep** — reject until owner OK |
| 5 | [Wyvernbox](https://godotengine.org/asset-library/asset/1919) | MIT | Action-RPG loot/equip; heavy for Stage-1 |
| 6 | [gd-inventory-system](https://github.com/arnoldblake/gd-inventory-system) | — | Bag-bar UI; client-only authority risk |
| 7 | DrBlury `inventory_management_tetris` (Go+PG) | OSS | Generic warehouse API — overkill, new module |

## Candidates

| # | Approach | Verdict |
|---|----------|---------|
| 1 | **In-house Go+PG stacks + thin Godot `.tscn` UI** | **Adopt** for EX-P08 |
| 2 | Vendor gloot/Wyvernbox | New dep — **reject** until owner research OK |
| 3 | Client-authoritative bag | Violates architecture — reject |

## Recommendation

Ship director contract `inventory-engine.md`: stackable `character_inventory` for `MAT-*`/`ITM-*`/`EQ-*` instances later; keep `ITEM-Q-*`/`ITEM-SQ-*` in `character_quest_items`; `inventory_revision`; WS `inventory.delta` + snapshot scope. Client: read-only bag panel bound to server state (editable `.tscn`), no third-party addon. Combat loot tables wait until enemies/combat authority (still EX-P08 scope, next task after bag+MAT grant path).
