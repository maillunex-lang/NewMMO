# T-017 pilot: dialogue proposals only

**Not canon.** Director must review before copying into `10_DIALOGUES.md`. Six quest sections retain original IDs and existing stages; newly added IDs use adjacent indexes.

| Quest | NPC+PLAYER lines | PROGRESS variants | NPC in scene | Questions |
|---|---:|---:|---|---|
| MQ-01-05 | 17 | 3 | NPC-802 | Should NPC-102 speak on handoff, or remain off-screen? |
| SQ-01-001 | 15 | 3 | NPC-802 | Is NPC-801 physically present or only referenced? |
| SQ-01-002 | 15 | 3 | NPC-801 | Canon defines `choose_option` but not explicit A/B labels or option IDs; confirm before integration. |
| SQ-01-003 | 15 | 3 | NPC-102 | How is the unnamed scribe voiced via existing NPC? |
| SQ-01-004 | 15 | 3 | NPC-102 | Does NPC-903 speak or remain silent during escort? |
| SQ-01-005 | 15 | 3 | NPC-906 | Confirm bridge-test trigger and whether NPC-903 appears. |

## Author checklist (§4)

| Quest | Quest/voice/story reviewed | START scene+ASK+ACCEPT | OBJ hint before / reaction after | PROGRESS 0/1/2 | TURNIN/DONE | IDs and spoiler gate |
|---|---|---|---|---|---|---|
| MQ-01-05 | yes | yes | yes | yes | yes | yes |
| SQ-01-001 | yes | yes | yes | yes | yes | yes |
| SQ-01-002 | yes | yes | yes | yes | yes | conditional choice unresolved |
| SQ-01-003 | yes | yes | yes | yes | yes | yes |
| SQ-01-004 | yes | yes | yes | yes | yes | yes |
| SQ-01-005 | yes | yes | yes | yes | yes | yes |

## Ambiguities and editorial assumptions, for director

- SQ-01-002: `09_QUESTS_SIDE.md` calls for choosing a copy, but the card does not enumerate A/B values or a `SQC-01-002` flag. The pre-existing `CHOICE.01` and `CHOICE.PLAYER` IDs are retained as neutral text, without inventing option IDs. **Block canonical integration until resolved.**
- SQ-01-003: the objective names a scribe, but there is no canonical scribe NPC ID in its card. No new speaker was created; the dialogue acknowledges the testimony through NPC-102.
- MQ-01-05: NPC-102 is the destination of OBJ3 but NPC-802 remains the quest contact. The proposal does not introduce a second mandatory conversation beyond the existing objective.
- `objectives_done` is treated as count of completed top-level objectives (0/1/2), not partial quantities inside OBJ1; runtime predicate syntax needs director confirmation.
- Original SQ-01-002 `CHOICE.PLAYER` has a nonnumeric suffix. Preserved to avoid ID breakage, pending lint/parser confirmation.
- Reactions after objective completion are narrative display only, never server progress. In a sequential implementation they may be shown on the next stage transition.
- Generic SQ narrative outcomes were inferred conservatively from the concrete three objectives because the cards do not have a separate `Сценарный итог` field.
- No optional `LATER` branch was added: START can be closed without acceptance. Confirm whether the client requires an explicit option.

## Tests

**Not run:** `python3 tools/content/dialogue_lint.py --proposal-dir docs/content/dialogue_expansion/pilot --strict`. GitHub connector allows repository file edits but does not execute repository-local Python. Director must run strict lint and all lane gates before merging. Manual inspection covers stages, original IDs, line counts, server authority, no new NPC/quest/flag IDs, and no spoilers. No claim of 0 ERROR.

**Known content-quality limitation:** progress 0/1/2 wording is still partly generic; refine after director feedback, not in canon.
