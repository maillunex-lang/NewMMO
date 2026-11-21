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
| Status | **OPEN** — owner decision needed (found by EX-P01 lint, CT-002) |
| Problem | `03_LOCATIONS.md` §5 declares settlements as `SET-RR-NN` (e.g. `SET-01-01` Астервиль). `11_ITEMS_LOOT.md` §2/§6 declares armor sets as `SET-CC-RR` (e.g. `SET-01-01` «Страж Заря»). 15 strings collide (`SET-0R-0N`, R=1..5, N=1..3). `14` §4 lists `SET-*` only for items. |
| Options | **A** rename settlements to a new prefix (e.g. `STL-RR-NN`) in `03`; **B** rename item sets (e.g. `ISET-CC-RR`) in `11`; **C** keep both, namespaced by typed field (`kind`+`id`) — parsers already carry `kind`, but bare-string ID registry in `14` §4 stays ambiguous. |
| Interim | Not edited (canon is owner-approved only). `content/lint_allowlist.json` downgrades the 15 duplicates to WARN, owner = project owner, **expires 2026-12-31**, then lint fails again. |

## Other open notes

| ID | Note |
|----|------|
| C-DLG-REMNANT | One remaining `DLG-SCN-01-01` in `07_STORY.md:408` — obsolete shorthand per C-DIA-001 A; lint emits WARN `DLG_OBSOLETE`. |
| C-ITM-00 | `11` §8 consumables use `ITM-00-NNN` (region 00 = global); `14` §4 says `ITM-RR-NNN`, RR=01..06. Schema accepts `00`. |
| C-POI-001 | REG-01 stub uses gameplay POI ids (`POI-BOARD-01`, `POI-WITNESS-A`, …) not full `POI-01-NNN` registry from `03_LOCATIONS.md`. Map or alias in EX-P02. |
