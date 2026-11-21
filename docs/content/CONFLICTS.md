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
| Status | **RESOLVED** (2026-11-21) — project is on Godot **4.7.2** |
| Decision | **Owner 2026-11-21: move to Godot 4.7.2 now («только на нём будет»).** Supersedes option B (deferred upgrade). The upgrade gate is `docs/research/godot-4-7-2-upgrade.md`; the earlier condition «device metrics first» is **waived** (no 6 GB phone) — device metrics stay a mandatory pre-release gate (R-DEVICE). Executed by T-015 (merged): `config/features` = 4.7, `.gd.uid` files committed, strict smoke + CI job `client-godot` + APK workflow run on 4.7.2. Android side effect: **minSdk 21 → 24**, targetSdk 34 → 36 (accepted). |
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
| Status | **RESOLVED** (2026-11-21) |
| Decision | **C** (owner) — chapters 1–5 have **no branching choice at `MQ-RR-20`**. The phrase «зафиксировать выбор» in the `MQ-RR-20` journal texts means «the player confirms the chapter result» (acknowledgement without a branch, no flag with alternative values, no consequence). The real branching choices remain `CHOICE-RR-09` (A/B, chapters 1–6) and the final `CHOICE-06-FINAL` (`END-01|END-02|END-03`). `08` §3 mention of `CHOICE-RR-20` is read as «confirmation of the chapter result». Canon text not edited; the owner may later upgrade to option B (define A/B per chapter) without breaking IDs. |
| Applied | Dialogue rule for batch `MQ-RR-20` (RR=01…05): a non-branching confirmation scene (`TURNIN`/`DONE`), no `CHOICE` lines; `MQ-06-20` keeps the three-way ending choice. Lint rule `DIA-CHOICE` of the T-016 spec must not require choices for `MQ-01-20…MQ-05-20`. |
| History | OPEN (T-016 audit, verified by director) → owner chose C. |

## C-LOOT-001 — «регион.коэф» in loot tables is not defined

| Field | Value |
|-------|--------|
| Status | **OPEN, interim applied** (2026-11-21, EX-P08 T-034) |
| Problem | `11_ITEMS_LOOT.md` §9 gold column is `min–max × регион.коэф`, but no region coefficient values exist anywhere in canon (`11`, `14`, `PACING.md`). Equipment drop (3rd percentage) also needs equipment instances (deferred). |
| Interim | Server config `NEWMMO_LOOT_REGION_COEF_JSON` (`{"REG-01":1.0}`), **default 1.0 for every region** (neutral reading = canon gold range as printed). Equipment roll skipped until equip UI/instances exist. No balance change made. |
| Owner decision needed | Region coefficients for REG-01…06 (balance). Until then gold is the printed canon range. |

## Other open notes

| ID | Note |
|----|------|
| C-DLG-REMNANT | One remaining `DLG-SCN-01-01` in `07_STORY.md:408` — obsolete shorthand per C-DIA-001 A; lint emits WARN `DLG_OBSOLETE`. |
| C-ITM-00 | `11` §8 consumables use `ITM-00-NNN` (region 00 = global); `14` §4 says `ITM-RR-NNN`, RR=01..06. Schema accepts `00`. |
| C-POI-001 | REG-01 stub uses gameplay POI ids (`POI-BOARD-01`, `POI-WITNESS-A`, …) not full `POI-01-NNN` registry from `03_LOCATIONS.md`. Map or alias in EX-P02. |
