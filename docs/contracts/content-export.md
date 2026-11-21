# Contract: content export bundle (D-001)

**Owner:** director · **Version:** `schema_version = 1` · **Producer:** `python3 tools/content/export.py` · **Consumer (EX-P03):** → `server/internal/content`
Source of truth is the canon `docs/content/01…13`; the bundle is a deterministic, hash-addressed snapshot of it. Executors implement
against this file; changes go through a director task (bump `schema_version`).

## 1. Files (default `build/content/export/`, git-ignored)

| File | Content |
|------|---------|
| `manifest.json` | metadata + hashes (below) |
| `entities.jsonl` | UTF-8, LF, one canonical JSON object per line, **sorted by `(kind, id)`**, trailing newline |
| `crossrefs.json` | ID registry (`declared_in`, `referenced_from`) from lint; informational for the server, verified by hash |

`export.py` runs the full lint first: **any ERROR ⇒ nothing is written, exit 1** (publish blocked). WARN allowed only via `content/lint_allowlist.json`.
Same canon ⇒ byte-identical files and the same `build_id` (no timestamps inside the bundle).

## 2. `manifest.json`

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
| `manifest_sha256` | hex64 | SHA-256 of the **canonical JSON** (sorted keys, `(",",":")` separators, UTF-8, non-ASCII unescaped) of the manifest **without** `manifest_sha256` and `build_id` |
| `build_id` | hex64 | `== manifest_sha256` (canon `14` §13.2: content build id = SHA-256 of the normalized manifest) |

## 3. Entity line

```json
{"id":"MQ-01-01","kind":"quest_main","payload":{…},"region_id":"REG-01","schema_version":1,"source_file":"08_QUESTS_MAIN.md","source_line":41}
```

- `id` is unique **within the build** (the loader enforces `PRIMARY KEY (build_id, entity_id)`).
- `region_id`: `REG-RR` or `null` for global entities (classes, factions, endings, items, item sets, scenes, 52 NPC/ending dialogue lines).
- `payload` is the parsed typed record for that kind (shapes = `content/schemas/<kind>.schema.json` `fields`); the loader stores it as `jsonb` **unchanged** and does not interpret gameplay.
- Typed relations live in the payload (e.g. quest `prereqs`, `npcs`, `locations`, `reward.evidence`; objective `quest_id`; enemy `spawn_refs`; dungeon `entrance/miniboss/boss`; event `world_boss`; loot table `enemy`). Use them; do not regex IDs out of text.

### Kinds in build 1 (4925 entities)
`chapter 6 · class 5 · dialogue_line 3122 · dialogue_template 1 · dungeon 12 · ending 3 · enemy 96 · equipment 270 · event 18 · faction 8 · item 8 · item_set 30 · location 48 · loot_table 96 · material 36 · npc 32 · objective 720 · poi 72 · quest_item 36 · quest_main 120 · quest_side 120 · recipe 36 · region 6 · scene 6 · settlement 18`.
(`*_ref` catalog mirrors are not exported. Settlements use `STL-RR-NN`; armor sets `SET-CC-RR` — C-SET-001.)

## 4. Loader obligations (verification before a build can become `staged`-complete)

1. `manifest_sha256` recomputed from the manifest body equals the stored value; `build_id == manifest_sha256`.
2. `schema_version` supported; `lint.errors == 0`.
3. SHA-256 of `entities.jsonl` bytes equals `entities_sha256`; line count equals `entity_count`; per-kind counts equal `counts`.
4. Every line has exactly the 7 keys of §3; ids unique; `kind` ∈ `counts` keys.
5. `crossrefs.json` hash matches `crossrefs_sha256`.
Any failure ⇒ reject, write an audit row, leave the active pointer untouched.
