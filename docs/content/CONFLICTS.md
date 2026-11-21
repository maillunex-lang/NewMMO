# CONFLICTS — content canon discrepancies

Recorded by EX-P00 audit. **Do not silently pick a side in code.** Owner decides; then update this file with `RESOLVED` + date.

## C-DIA-001 — Dialogue ID namespace (`DIA.*` vs `DLG-*`)

| Field | Value |
|-------|--------|
| Status | **OPEN** |
| Sources | `10_DIALOGUES.md` §2 uses `DIA.<QUEST_ID>.<STAGE>.<INDEX>`; client JSON uses the same. `14_CONTENT_INTEGRATION.md` §4 table lists space `DLG-*`. `13_WORLD_EVENTS.md` cross-ref also says `DLG-*`. |
| Impact | Parsers/schemas (EX-P01/P02) and any lint that expects `DLG-` would false-fail all real dialogue IDs. |
| Runtime today | `client/data/dialogues/mq_01_01_to_05.json` — `DIA.*` only. |
| Options | **A)** Treat `DIA.*` as canonical (file 10 wins); document `DLG-*` as obsolete shorthand in 13/14. **B)** Rename all dialogues to `DLG-*` (huge, needs owner approval). **C)** Accept both as aliases in schema (`DIA.*` primary, `DLG-*` redirect). |
| Recommendation | **A or C**; do not mass-rename without approval. |

## Other open notes (not full conflicts yet)

| ID | Note |
|----|------|
| C-GODOT-001 | Integration cites Godot **4.7.2**; `client/project.godot` pins **4.3**. Needs owner pin before upgrade. |
| C-POI-001 | REG-01 stub uses gameplay POI ids (`POI-BOARD-01`, `POI-WITNESS-A`, …) not full `POI-01-NNN` registry from `03_LOCATIONS.md`. Map or alias in EX-P02. |
