# tools/content — canon lint (EX-P01)

Python 3 stdlib only; **no third-party packages** (see `docs/research/content-validation.md`).

```bash
python3 tools/content/lint.py                 # reports -> build/content/ (git-ignored)
python3 -m unittest discover -s tools/content/tests -v
```

Pipeline: `docs/content/01..13` (UTF-8, NFC, LF) → block AST (`md_ast.py`) → per-document profile
extractors (`profiles.py`, declarations only — prose mentions are ignored, code fences skipped) →
JSON Schema per kind (`content/schemas/*.schema.json`, subset validator `schema.py`) →
duplicate/golden-volume/consistency checks (`lint.py`).

Outputs (EX-P02 adds `crossrefs.json`): `validation-report.json`, `content.manifest.json` (sha256 per file + content hash,
`publishable`), `declarations.json`. **Any ERROR → exit 1 → not publishable.**

Known canon conflicts can be downgraded to WARN only via `content/lint_allowlist.json`
(entry needs `conflict`, `owner`, `expires`; expired entries become ERROR). `DLG-*` → WARN (C-DIA-001 A).

EX-P02 (`crossrefs.py`): every mention of a hard-namespace ID (FAC, STL, LOC, POI, NPC, CLS, ENM, MQ/SQ, OBJ/SQO,
DNG, EVT, END, MAT, ITM, REC, LOOT, EQ, ITEM-Q/SQ) must resolve to a declaration (ERROR `CT002_UNRESOLVED_REF`);
`crossrefs.json` holds `declared_in` / `referenced_from` per ID. Group IDs (`SPAWN`, `SPX`, `RES`, `RTE`, `TP`) are only counted.
Static checks: CT-003/004 quest unlock graph (missing prereqs, cycles/unreachable, single root, dead ends, endings),
CT-005 enemy AI/spawn-region/1:1 loot table, CT-006 dungeon entrance + miniboss/boss, CT-007 world-boss link,
CT-008 dialogue stage coverage per quest, CT-009 loot table ↔ enemy and probabilities 0..100.

Not covered (later stages): CT-009 loot *weights sum* / infinite-currency rules need balance data (EX-P08);
per-region reachability of dungeon rooms/POI graph beyond declared links (EX-P09).

Other checks: `check_client_sync.py` (client/data quests+dialogues vs canon declarations; stub POI ids are INFO per C-POI-001),
`check_docs_links.py` (repo paths referenced from docs must exist; canon specs `docs/content/NN_*` skipped — they describe planned structure).
Multi-agent gate lives in `tools/director/` (`lane_check.py`).
