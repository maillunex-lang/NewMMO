# CONFLICTS — content canon discrepancies

Recorded by EX-P00 audit. **Do not silently pick a side in code.** Owner decides; then update this file with `RESOLVED` + date.

## C-DIA-001 — Dialogue ID namespace (`DIA.*` vs `DLG-*`)

| Field | Value |
|-------|--------|
| Status | **RESOLVED** (2026-11-21) |
| Decision | **A** — `DIA.*` is the sole canonical dialogue ID format. Do not rename existing IDs. Mentions of `DLG-*` in `13`/`14` are **obsolete shorthand**; parsers/schemas accept `DIA.*` only (WARN if `DLG-*` appears). |
| Sources | `10_DIALOGUES.md` §2; client JSON; owner confirmation for EX-P01 / EX-CONTENT-01. |
| History | OPEN at EX-P00 → options A/B/C → owner chose **A**. |

## C-GODOT-001 — Engine version pin

| Field | Value |
|-------|--------|
| Status | **RESOLVED** (policy) / **OPEN** (upgrade work) |
| Decision | **B** — Target Godot is **4.7.2**. **Do not upgrade the engine yet.** First verify client, plugins, export templates, and Android compatibility in a dedicated check. Current project pin remains **4.3** until that gate passes. |
| History | Noted at EX-P00 → owner chose target 4.7.2 with deferred upgrade. |

## C-SET-001 — `SET-*` ID collision (settlements vs armor sets)

| Field | Value |
|-------|--------|
| Status | **RESOLVED** (2026-11-21) |
| Decision | **A** (owner) — settlements renamed `SET-RR-NN` → `STL-RR-NN` in `03_LOCATIONS.md` §5 (18 IDs). Armor sets `SET-CC-RR` in `11_ITEMS_LOOT.md` unchanged. Names, regions, roles, services of settlements unchanged. |
| Applied | EX-P02: `03` §5 rows + `STL-XX-NN` line in §«Категории и ID»; `14` §4 registry row «Поселение». No other references existed in docs/client/server/JSON (repo-wide grep). Schema `settlement` and `content/lint_allowlist.json` (entries removed) updated. |
| Verified | `tools/content/lint.py` without allowlist: 0 duplicate declarations; unit test `test_no_id_collisions_without_allowlist`. |
| History | OPEN at EX-P01 (15 collisions: `SET-0R-0N`, R=1..5, N=1..3) → owner approved A and one-time canon edit. |

## C-ITEMQ-001 — quest evidence items `ITEM-Q-RR-SS` are used but never declared

| Field | Value |
|-------|--------|
| Status | **OPEN** — owner decision needed (found by EX-P02 crossrefs, CT-002) |
| Problem | `08_QUESTS_MAIN.md` grants 36 virtual evidence items as rewards (e.g. `ITEM-Q-01-02`, `ITEM-Q-01-05`, … `ITEM-Q-06-18`). `11_ITEMS_LOOT.md` §2 defines only the *format* `ITEM-Q-RR-SS` / `ITEM-SQ-RR-SSS`; no table declares names, types or QUEST_KEY flags. `09` references no `ITEM-SQ-*` at all. |
| Options | **A** declare the 36 `ITEM-Q-*` (name, QUEST_KEY, non-tradable) in a new table in `11` — wait for EX-P08 inventory design; **B** treat them as reward tokens defined by the quest card (no catalog), exempt from crossref; **C** remove the item grants from `08`. |
| Interim | Canon not edited. `content/lint_allowlist.json` downgrades the 36 unresolved refs to WARN (`id_pattern ^ITEM-Q-`), owner = project owner, **expires 2027-01-31**; blocks again afterwards. Recommended: A, decided before EX-P08. |

## Other open notes

| ID | Note |
|----|------|
| C-DLG-REMNANT | One remaining `DLG-SCN-01-01` in `07_STORY.md:408` — obsolete shorthand per C-DIA-001 A; lint emits WARN `DLG_OBSOLETE`. |
| C-ITM-00 | `11` §8 consumables use `ITM-00-NNN` (region 00 = global); `14` §4 says `ITM-RR-NNN`, RR=01..06. Schema accepts `00`. |
| C-POI-001 | REG-01 stub uses gameplay POI ids (`POI-BOARD-01`, `POI-WITNESS-A`, …) not full `POI-01-NNN` registry from `03_LOCATIONS.md`. Map or alias in EX-P02. |
