# T-017 pilot: dialogue proposals only

**Not canon.** Director must review before copying into `10_DIALOGUES.md`. Six quest sections retain original IDs and existing stages; newly added IDs use adjacent indexes.

| Quest | NPC+PLAYER lines | PROGRESS variants | NPC in scene | Questions |
|---|---:|---:|---|---|
| MQ-01-05 | 19 | 3 | NPC-802 | Resolved: NPC-102 speaks in OBJ3 |
| SQ-01-001 | 15 | 2 | NPC-802 | Resolved: NPC-801 speaks on journal return |
| SQ-01-002 | 15 | 2 | NPC-801 | No stateful A/B: any copy selection completes the objective. |
| SQ-01-003 | 15 | 2 | NPC-102 | Resolved: NPC-101 comments on testimony |
| SQ-01-004 | 15 | 2 | NPC-102 | Resolved: NPC-903 speaks briefly, without jokes |
| SQ-01-005 | 15 | 2 | NPC-906 | Resolved: bridge test is existing OBJ3; NPC-903 not voiced |

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

- SQ-01-002: director confirmed copy selection has no stateful A/B and no SQC; either copy completes the existing objective. Existing CHOICE keys remain.
- SQ-01-003: the unnamed scribe has no NPC ID; NPC-101 comments on the obtained testimony, without creating a speaker.
- MQ-01-05: NPC-102 speaks only at the existing OBJ3 interaction; NPC-802 remains contact for START/TURNIN/DONE.
- `objectives_done` counts fully completed top-level objectives (director confirmed), not partial quantities.
- Original SQ-01-002 `CHOICE.PLAYER` has a nonnumeric suffix; director's strict lint accepted it.
- Reactions after objective completion are narrative display only, never server progress. In a sequential implementation they may be shown on the next stage transition.
- Generic SQ narrative outcomes were inferred conservatively from the concrete three objectives because the cards do not have a separate `Сценарный итог` field.
- Every START now has explicit `LATER` with NPC reply and no flag changes.

## Tests

**Not run:** `python3 tools/content/dialogue_lint.py --proposal-dir docs/content/dialogue_expansion/pilot --strict`. GitHub connector allows repository file edits but does not execute repository-local Python. Director must run strict lint and all lane gates before merging. Manual inspection covers stages, original IDs, line counts, server authority, no new NPC/quest/flag IDs, and no spoilers. No claim of 0 ERROR.

**Revision 1:** Rewrote OBJ reactions and PROGRESS in scene-specific voices, introduced LATER and replies, NPC-102/NPC-101/NPC-903 participation as permitted, corrected SQ-01-005 DONE punctuation. Removed optional newly added lines where necessary to keep side-quest dialogue density 15. Compared all 94 substantive NPC/PLAYER utterances (19 + 5×15) by exact extracted quoted text across the six pilot files: **0 exact duplicates**. This is a manual-equivalent string comparison in the editing script, not the repository linter. Director's prior strict run (before revision) was 0 ERROR, 33 DIA-EXACT WARN; new strict result is unverified. The GitHub connector cannot run git fetch/rebase or execute Python; main was 3 commits ahead of the task branch at preflight, so no rebase was attempted and no history was rewritten. Director should synchronize branch by approved method before merging.
