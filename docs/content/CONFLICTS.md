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

## Other open notes

| ID | Note |
|----|------|
| C-POI-001 | REG-01 stub uses gameplay POI ids (`POI-BOARD-01`, `POI-WITNESS-A`, …) not full `POI-01-NNN` registry from `03_LOCATIONS.md`. Map or alias in EX-P02. |
