# Contract: content export bundle (D-001)

**Owner:** director · **Version:** `schema_version = 1`, contract rev **1.2** (T-010 review + T-013 review applied) · **Producer:** `python3 tools/content/export.py` · **Consumer (EX-P03):** → `server/internal/content`
Source of truth is the canon `docs/content/01…13`; the bundle is a deterministic, hash-addressed snapshot of it. Executors implement
against this file; changes go through a director task (bump `schema_version`).

## 1. Files (default `build/content/export/`, git-ignored)

| File | Content |
|------|---------|
| `manifest.json` | **canonical one-line JSON body** + LF (below). Its **raw bytes** define the build id |
| `BUILD_ID` | 64 lowercase hex + LF = `sha256(raw bytes of manifest.json)` |
| `entities.jsonl` | UTF-8, LF, one canonical JSON object per line, **sorted by `(kind, id)`**, trailing newline |
| `crossrefs.json` | ID registry (`declared_in`, `referenced_from`) from lint; informational for the server, verified by hash |

`export.py` runs the full lint first: **any ERROR ⇒ nothing is written, exit 1** (publish blocked). WARN allowed only via `content/lint_allowlist.json`.
Same canon ⇒ byte-identical files and the same `build_id` (no timestamps inside the bundle).

## 2. `manifest.json`

The file is the canonical body: keys sorted, separators `(",",":")`, UTF-8, non-ASCII unescaped, **one line + LF**. **`build_id = manifest_sha256 = SHA-256 of the exact file bytes`**
(canon `14` §13.2). The loader **never re-serializes** the manifest: it hashes the raw bytes, compares with `BUILD_ID`, then parses strictly. **Canonical form is a producer obligation** (`export.py`); the loader does **not** check canonicality — a hash-consistent, strictly valid manifest in another representation is accepted and simply yields a different `build_id` (content-addressing by exact bytes; decision on T-013 I-02, 2026-11-21). (This removes any cross-language JSON canonicalization risk — T-010 R-12.)
`manifest_sha256`/`build_id` are therefore **not** fields inside the file.

| Field | Type | Meaning |
|-------|------|---------|
| `format` | string | `"newmmo.content.export"` |
| `schema_version` | int | `1`; loader must reject unknown versions |
| `minimum_client_version` | string | lowest client that can use this build (currently `"0.0.1"`) |
| `source_files[]` | `{file, sha256}` | the 14 canon files (01–13 + 14) with raw-bytes SHA-256 |
| `entity_count` | int | number of lines in `entities.jsonl` |
| `counts` | object | entities per `kind` (sum == `entity_count`) |
| `entities_sha256` | hex64 | SHA-256 of the exact bytes of `entities.jsonl` |
| `crossrefs_sha256` | hex64 | SHA-256 of the exact bytes of `crossrefs.json` |
| `lint` | `{errors, warnings}` | `errors` is always `0` in an exported bundle |

## 3. Entity line

```json
{"id":"MQ-01-01","kind":"quest_main","payload":{…},"region_id":"REG-01","schema_version":1,"source_file":"08_QUESTS_MAIN.md","source_line":41}
```

- `id` is unique **within the build** (the loader enforces `PRIMARY KEY (build_id, entity_id)`).
- `region_id`: `REG-RR` or `null` for global entities (classes, factions, endings, items, item sets, scenes, 52 NPC/ending dialogue lines).
- `payload` is the parsed typed record for that kind (shapes = `content/schemas/<kind>.schema.json` `fields`); the loader stores it as `jsonb` **unchanged** and does not interpret gameplay.
- Typed relations live in the payload (e.g. quest `prereqs`, `npcs`, `locations`, `reward.evidence`; objective `quest_id`; enemy `spawn_refs`; dungeon `entrance/miniboss/boss`; event `world_boss`; loot table `enemy`). Use them; do not regex IDs out of text.

### Kinds in build 1 (4958 entities)
`chapter 6 · class 5 · dialogue_line 3122 · dialogue_template 1 · dungeon 12 · ending 3 · enemy 96 · equipment 270 · event 18 · faction 8 · item 8 · item_set 30 · location 48 · loot_table 96 · material 36 · npc 32 · objective 720 · poi 72 · quest_item 36 · quest_main 120 · quest_side 120 · recipe 36 · region 6 · scene 6 · settlement 18`.
(`*_ref` catalog mirrors are not exported. Settlements use `STL-RR-NN`; armor sets `SET-CC-RR` — C-SET-001.)

## 4. Loader obligations (verification before a build can be staged)

1. `sha256(raw manifest.json bytes) == BUILD_ID` (64 hex); `build_id` = that value. `manifest.json` is one line + LF.
2. Parse manifest strictly; `schema_version` supported; `lint.errors == 0`; `source_files` has exactly 14 entries.
3. SHA-256 of the exact bytes of `entities.jsonl` equals `entities_sha256`; line count equals `entity_count`; per-kind counts equal `counts`.
4. **Strict parsing (T-010 R-05):** valid UTF-8 only (no BOM); reject duplicate object keys, NaN/Infinity, non-object lines; every line has exactly the 7 keys of §3 with the right types; `id` unique; `kind ∈ counts`; lines sorted by `(kind, id)`; LF only.
5. **Bounds:** bundle ≤ 64 MiB, `entities.jsonl` line ≤ 1 MiB, JSON nesting ≤ 16, `entity_count` ≤ 100 000; streaming read (no whole-file buffering of unbounded input).
6. `crossrefs.json`: hash matches `crossrefs_sha256` **and** it is strictly valid UTF-8 JSON whose top level is an object with the keys `schema`, `entities`, `unresolved`, `soft_group_ids` (size within the bundle bound). It is informational: consumers never trust it for authorization or gameplay (T-013 I-03).
Any failure ⇒ reject with a structured error code (`db-content.md` §3), best-effort audit, active pointer untouched.

## 5. Invariants and mappings

- **Global ID namespace (T-010 R-06):** entity ids are unique across **all kinds** within a build. Enforced at the producer (lint duplicate scan across every declaration + objectives; export is blocked on violation) and by the DB key `(build_id, entity_id)`. A future kind that needs a clashing id must be renamed in canon first.
- **Canon §14 fields (T-010 R-07):** `id`, `kind`, `schema_version`, `source_file`, `source_line`, `region_id` are top-level (§3). `content_version` ≡ `build_id` (envelope of the build). `enabled`: every v1 entity is enabled (disabling is not modelled; will be added with a `schema_version` bump). `dependencies[]` ≡ typed payload relations (`prereqs`, `npcs`, `locations`, `reward.evidence`, `spawn_refs`, `entrance/miniboss/boss`, `world_boss`, `enemy`, `quest_id`). `title_key`: canon text is not localized-key based except dialogues (`dialogue.<id>` keys derive from the `DIA.*` id). Canon's sample `kind=main_quest` is illustrative; the export kind is `quest_main`.
- **Per-kind payload shapes** are the `fields` of `content/schemas/<kind>.schema.json`; those schemas are the validation source for required/optional fields.
