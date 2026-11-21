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
| Status | **RESOLVED — upgrade in progress** (T-015) |
| Decision | **Owner 2026-11-21: move to Godot 4.7.2 now («только на нём будет»).** Supersedes option B (deferred upgrade). The upgrade gate is `docs/research/godot-4-7-2-upgrade.md`; the earlier condition «device metrics first» is **waived** (no 6 GB phone) — device metrics stay a mandatory pre-release gate (R-DEVICE). The project pin remains **4.3 on `main` until T-015 is accepted**, then becomes **4.7.2**. |
| History | Noted at EX-P00 → option B (target 4.7.2, deferred) → owner ordered the upgrade (T-015). |

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
| Status | **RESOLVED** (2026-11-21) |
| Decision | **A** (owner) — the 36 `ITEM-Q-*` granted as rewards in `08_QUESTS_MAIN.md` are registered in `11_ITEMS_LOOT.md` §12.1 (type `quest_evidence`, `QUEST_KEY`, no trade value) — exactly what `08` §1 already required. |
| Applied | New table in `11` §12.1; names derived mechanically from quest titles (`Улика: «<название квеста>»`) — **working names, owner approves final in-game names**. No `ITEM-SQ-*` registered (no SQ grants any). Schema `quest_item` + check «declared set == granted set» (CT009_EVIDENCE_*). Allowlist emptied. |
| History | OPEN at EX-P02 (36 unresolved refs) → owner chose A. |

## C-CHOICE-001 — chapter-end choices `CHOICE-RR-20`: mentioned but not specified

| Field | Value |
|-------|--------|
| Status | **OPEN** — owner decision needed before dialogue batch «REG-01 MQ-01-20» (found by T-016 audit, verified by director) |
| Problem | `08_QUESTS_MAIN.md` §3 says branches are `CHOICE-RR-09` **and `CHOICE-RR-20`** (A/B). Quest cards `MQ-01-09`…`MQ-06-09` define the choice (e.g. `CHOICE-01-09`), and `MQ-06-20` defines the final three-way ending, but the cards `MQ-01-20`…`MQ-05-20` contain **no «Выбор» definition** (their journal text only says «зафиксировать выбор»), and `10_DIALOGUES.md` has no `CHOICE` lines for them. |
| Options | **A** `CHOICE-RR-20` does not exist for chapters 1–5 (fix the sentence in `08` §3); **B** it exists: owner defines the A/B options, effects and flags per chapter (then `08` cards and `10` get them); **C** treat the «20» choice as the chapter-summary acknowledgement without gameplay branching. |
| Interim | Nothing implemented; dialogue pilot (MQ-01-05, SQ-01-001…005) is unaffected. Batch `MQ-01-20` waits for this decision. |

## Other open notes

| ID | Note |
|----|------|
| C-DLG-REMNANT | One remaining `DLG-SCN-01-01` in `07_STORY.md:408` — obsolete shorthand per C-DIA-001 A; lint emits WARN `DLG_OBSOLETE`. |
| C-ITM-00 | `11` §8 consumables use `ITM-00-NNN` (region 00 = global); `14` §4 says `ITM-RR-NNN`, RR=01..06. Schema accepts `00`. |
| C-POI-001 | REG-01 stub uses gameplay POI ids (`POI-BOARD-01`, `POI-WITNESS-A`, …) not full `POI-01-NNN` registry from `03_LOCATIONS.md`. Map or alias in EX-P02. |
