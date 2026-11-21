# T-030 — Inventory UI research (short)

**Depends on:** T-029 inventory architecture research.

| # | Candidate | License / note | Verdict |
|---|-----------|----------------|---------|
| 1 | Native Godot 4 Control + ItemList (same pattern as quest journal T-007) | MIT engine | **Adopt** |
| 2 | GLoot / gd-inventory-system | MIT; Godot version / client-authority risk | Reject — T-029 already rejected vendoring |
| 3 | Tetris/grid inventory addons | Extra deps | Reject — owner forbids new deps without OK |

**Recommendation:** in-house `.tscn` panel + server snapshot/delta binding. No install.
