# T-034 — combat loot: research note

**Question:** how to roll `LOOT-ENM-*` drops (canon `11` §9) authoritatively on the Go server, with a client reward feedback.
**Method note:** compared from established knowledge of the ecosystem and the canon's own rules; I did **not** open live GitHub / Hysen Labs pages in this pass, so licences/activity below are not freshly verified. Nothing third-party was installed — the chosen option adds no dependency.

| # | Option | Fit | Verdict |
|---|--------|-----|---------|
| 1 | **In-house roll from canon rows** (independent basis-point rolls, `crypto/rand`, one DB tx) | Canon already specifies the exact model: guaranteed gold range, independent MAT-001/003 rolls, special item roll, units 10 000 = 100%. ~60 lines, fully unit-testable with an injected roller. | **Chosen** |
| 2 | Go weighted-pick libraries (e.g. `weightedrand`-style packages) | They pick **one** item from weights. Canon uses **independent** per-item probabilities that do not sum to 100%, so the model does not match; new dependency for no gain. | Rejected |
| 3 | Godot loot-table addons (Asset Library) | Client-side rolls violate "server authoritative"; would also need review before any install (owner rule). | Rejected |
| 4 | Pre-rolled drop tables / seeds per spawn | Needs server spawns + position (EX-P09); today's server has neither. | Revisit in EX-P09 |

**Risks recorded:** R-KILL (server cannot verify kills until EX-P09; mitigated by validation + per-minute cap, see `docs/contracts/loot-engine.md` §1); canon gap C-LOOT-001 (region coefficient undefined → default 1.0).
