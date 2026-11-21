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

Outputs: `validation-report.json`, `content.manifest.json` (sha256 per file + content hash,
`publishable`), `declarations.json`. **Any ERROR → exit 1 → not publishable.**

Known canon conflicts can be downgraded to WARN only via `content/lint_allowlist.json`
(entry needs `conflict`, `owner`, `expires`; expired entries become ERROR). `DLG-*` → WARN (C-DIA-001 A).

Out of scope here (EX-P02): full crossref resolution/policy, `crossrefs.json`, region/graph checks (CT-003…009).
