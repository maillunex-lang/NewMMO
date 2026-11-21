# Research: content validation (Markdown → AST → JSON Schema)

**Date:** 2026-11-21  
**Repo:** `maillunex-lang/NewMMO`  
**Stage:** EX-P01  
**Status:** Research complete — **nothing third-party installed or vendored**  
**Awaiting confirmation** for any future dependency adoption.

## 1. Problem statement

Canon lives in `docs/content/01_*.md`…`14_*.md` (~13k lines, heterogeneous tables/headings/lists). Need:

- Profile parsers per document (declarations ≠ prose mentions)
- Normalized JSON + schema checks (required fields, types, ID format, duplicate declarations)
- CT-001 / CT-002 (static); ERROR blocks publish
- No VPS/gameplay changes; no Godot bump

## 2. Search sources (2026-11-21)

| Source | What was checked |
|--------|------------------|
| GitHub | mistune / wenmode, markdown-it-py, python-jsonschema, loopstack markdown-parser, mdsl, md-models |
| Hysen Labs | https://hysenlabs.com/projects — Satteri (MD/MDX Rust→JS); MDA Open Spec (agent MD + JSON Schema); directory has no Lunex-specific content lint |
| Official docs | JSON Schema Draft 2020-12 validation; CommonMark / GFM table conventions |
| Prior NewMMO research | `docs/research/location-procgen-isometric.md` (D) — pattern: research → wait |

## 3. Candidates (≥3)

| # | Candidate | License | Role | Activity / fit | Score |
|---|-----------|---------|------|----------------|-------|
| 1 | **In-house stdlib Python** (block AST + profile extractors + minimal schema subset) | N/A (ours) | Full control; zero deps | Fits EX-P01 gate “no unapproved installs” | **Best now** |
| 2 | [lepture/wenmode](https://github.com/lepture/wenmode) / Mistune lineage | BSD-like (verify SPDX on adopt) | CommonMark → mdast AST | Strong AST; still needs Lunex profiles on top | Strong later |
| 3 | [executablebooks/markdown-it-py](https://github.com/executablebooks/markdown-it-py) | MIT | Token stream (not tree-first) | Mature; tables via plugins | OK later |
| 4 | [python-jsonschema/jsonschema](https://github.com/python-jsonschema/jsonschema) | MIT | Full Draft 2020-12 | Gold standard schema engine | Strong later |
| 5 | [loopstack-ai/markdown-parser](https://github.com/loopstack-ai/markdown-parser) | MIT | MD→object via schema | Tiny/young (0★); heading-mapped docs | Weak |
| 6 | [RGHenderson/mdsl](https://github.com/rghenderson/mdsl) | check on adopt | Typed MD document schemas | Needs Zod/npm; wrong stack for Go/Godot monorepo | Weak for us |
| 7 | Hysen: [Satteri](https://hysenlabs.com/projects/bruits-satteri) | check upstream | Fast MD/MDX (Rust+JS) | JS-centric; overkill for ID registries | Not for P01 |

## 4. Comparison matrix

| Need | Stdlib in-house | wenmode/mistune | markdown-it-py | jsonschema pkg | mdsl / loopstack |
|------|-----------------|-----------------|----------------|----------------|------------------|
| No install for P01 | yes | no | no | no | no |
| Profile declarations (not global ID scrape) | yes (custom) | needs custom | needs custom | N/A (post-AST) | partial |
| Line/column diagnostics | yes | yes | yes | via instance path | yes |
| JSON Schema depth | subset enough for P01 | N/A | N/A | full | via zod/schema |
| Go/Android game impact | none | none | none | none | none |
| Maintainability | owned | dependency risk | dependency risk | low risk later | JS toolchain |

## 5. Recommendation

**Build EX-P01 in-house with Python 3 stdlib only** under `tools/content/` + schemas in `content/schemas/`.

Rationale:

1. Owner rule: do not install without confirmation.
2. Canon formats are custom (tables with ID columns, `#### \`MQ-01-01\``, ``- `DIA.*` ``) — any MD library still needs profile extractors.
3. Minimal schema subset (`type`, `required`, `properties`, `pattern`, `enum`, `minItems`) covers CT-001/002 declaration checks.
4. Later (after owner OK): optionally adopt `jsonschema` and/or `wenmode` behind the same CLI — adapters stay thin.

**Explicit:** nothing installed from this research.

## 6. Proposed architecture (implemented in EX-P01)

```
MD (UTF-8, NFC) → block AST (heading/paragraph/list/table + line)
  → profile parser (per doc 01–13) → declaration records
  → per-kind JSON Schema checks
  → duplicate-declaration scan (CT-002 partial; crossrefs = EX-P02)
  → validation-report.json + content.manifest.json (sha256)
  → exit 1 on any ERROR
```

Document 14 is the orchestrator: presence checked; it does not own entity declarations.

## 7. Decisions recorded elsewhere

- **C-DIA-001 A:** `DIA.*` canonical; `DLG-*` obsolete — `docs/content/CONFLICTS.md`
- **C-GODOT-001 B:** target 4.7.2; do not upgrade engine in this stage — same file
