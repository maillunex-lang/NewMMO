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

## C-RANGE-001 — skill ranges and unit scale are not defined

| Field | Value |
|-------|--------|
| Status | **OPEN, interim applied** (2026-11-21, T-038c1) |
| Problem | `05_CLASSES.md` skill tables give target type, cost, cooldown and coefficient but **no range**; `06` uses metres (aggro 7–9 m, leash 24 m) while the client works in pixels. |
| Interim | `world.Combat`: 1 m = 32 world units; single-target range 2.5 m for CLS-01/02 and 9 m for CLS-03/04/05, +1 m latency slack; enemy respawn 30 s. Tunables only; no balance claim. |
| Owner decision needed | Real per-skill ranges (or per-class), unit scale, respawn timers per spawn. |

## C-DNG-001 — ritual window for DNG-01-001

| Field | Value |
|-------|--------|
| Status | **OPEN, interim approved** (2026-11-21, T-038e) |
| Problem | `12_DUNGEONS.md` §4.1 requires linking three flames within a ritual window. It gives neither the duration nor the triggering event. The 2.2 s in `06_ENEMIES.md` applies to taking cover during `AB-ENM-01-301-C`. |
| Interim | The owner confirmed that the first authoritative combat slice keeps the existing three flame interactions without a deadline. |
| Owner decision needed | Specify the duration and trigger when introducing the ritual timer. |

## C-DNG-002 — phase-2 crystals and safe zones for DNG-01-001

| Field | Value |
|-------|--------|
| Status | **RESOLVED** (2026-11-21, T-038e) |
| Owner decision | Use two separate interactable crystals with existing art. Place safe zones inside the current R06 arena. |
| Applied | The server highlights one of the two diagonal pairs of room quadrants for the canon 2.2 s. Both crystals can be destroyed once per encounter to interrupt the cast. The three ritual flames keep their existing progression role. |

## C-EQUIP-001 — wear requirement and drop item level of equipment

| Field | Value |
|-------|--------|
| Status | **OPEN, interim** (2026-11-21, T-039a) — not a canon change |
| Problem | `11` §4 says `required_level<=item_level` with a gap of at most 3, and §6 gives one base level per region model (R1 = 8) while the loot tables (§9) do not say at which `item_level` a drop is rolled. Class stats of the base model (e.g. 8 for a level-1 player) would make a level-1 drop meaningless or too strong. |
| Interim | Drops use the template's base level as `item_level`; `required_level = max(1, item_level − 3)` (R1 gear wearable from level 5). Affixes (`11` §7) are not rolled yet; stat budget (`11` §4 formulas, class split) is the only stat source. |
| Owner decision needed | Per-drop `item_level` scaling inside a region (§6 "варианты уровней") and the real wear requirement. |

## C-MAGE-001 — a level-1 Mage cannot kill a single level-4 enemy

| Field | Value |
|-------|--------|
| Status | **RESOLVED 2026-11-21 by the owner: option (a) + (d)** (found by the live playtest bot, `client/scripts/dev/playtest.gd`) |
| Problem | `05_CLASSES.md`: all classes except the Mage have a free level-1 basic attack (cost 0, builds their resource: Guard/Rage/Focus/Light). `CLS-03-SK01` «Искра Узора» costs **20 mana**, CD 3 s, `1.05*SP`; mana is 100 with regen **5/s out of combat, 2/s in combat** (`05` §3). Region-1 open-world enemies are level 4: 258 HP vs ~36 damage per cast ⇒ 7 casts = 140 mana, but a mage has 100 and regains 2/s. Measured over 60 s with auto-combat: Mage dies (HP 288→0, enemy left with 6 HP); Guardian/Berserker/Ranger/Oracle survive and win. |
| Options | (a) `CLS-03-SK01` cost 0 and +mana on cast like the others; (b) cost 8–10; (c) in-combat regen ≥ 5/s; (d) level-1 open-world enemies of level 1–2 (also fixes the first-fight difficulty of every class). |
| Applied | (a) `05_CLASSES.md` `CLS-03-SK01`: cost 0, rule «Базовый снаряд; +10 маны» (same pattern as the other classes' basic attacks). (d) Dawn Valley open-world spawns (`content/zones/dawn-valley.json`, not canon text) now use the canon level-1/2 enemies (`ENM-01-001`, `ENM-01-002`) nearest the spawn, level 3–4 further away; no change to `06_ENEMIES.md` levels. |

## C-REGEN-001 — out-of-combat HP regeneration (new mechanic, interim)

| Field | Value |
|-------|--------|
| Status | **OPEN, interim** (2026-11-21, playtest PLAYTEST-1) — not in canon |
| Problem | Canon defines HP recovery only through heals/consumables (`11` §8); with no passive recovery a player who survives a fight is stuck near 0 HP and auto-combat keeps pulling. |
| Interim | Server config `Combat.HPRegenOutPct = 2% of MaxHP per second`, only while out of combat (5 s without combat actions). Client auto-combat rests below 35% HP (no enemy within 8 m) until 80%. |
| Owner decision needed | Whether to keep passive regeneration, its rate, and whether the consumables `ITM-00-001…005` stay the intended HP source. |

## Other open notes

| ID | Note |
|----|------|
| C-DLG-REMNANT | One remaining `DLG-SCN-01-01` in `07_STORY.md:408` — obsolete shorthand per C-DIA-001 A; lint emits WARN `DLG_OBSOLETE`. |
| C-ITM-00 | `11` §8 consumables use `ITM-00-NNN` (region 00 = global); `14` §4 says `ITM-RR-NNN`, RR=01..06. Schema accepts `00`. |
| C-POI-001 | REG-01 stub uses gameplay POI ids (`POI-BOARD-01`, `POI-WITNESS-A`, …) not full `POI-01-NNN` registry from `03_LOCATIONS.md`. Map or alias in EX-P02. |
