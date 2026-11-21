# Contract: server dialogue filter + choices (D-005 / EX-P05)

**Owner:** director · **Version:** 1 · Implements canon `14` §15.2 dialogue events and `10_DIALOGUES` presentation for **EX-P05**. Storage: `character_dialogue_choices` (`db-gameplay.md`); protocol: `websocket-protocol.md`. Proximity remains the known interim **R-PROX** (same as quests): not verified until movement authority (EX-P08/P09).

## 1. Identifiers
- `dialogue_id` = quest id for Stage-1 flows (`MQ-01-01` …) — matches client `flows` keys.
- `node_id` = dialogue line id (`DIA.MQ-01-01.START.01`) or synthetic stage node (`FLOW.<quest>.<mode>`).
- `option_id` ∈ `CONTINUE` | `ACCEPT` | `TURN_IN` | `ASK` | `LATER` | canon choice ids (`CHOICE-…-A/B`) when present.

## 2. Operations (idempotent via `request_id`)
| Client | Preconditions | Effects | Response |
|--------|----------------|---------|----------|
| `dialogue.open.request {npc_id, dialogue_id}` | character owns session; dialogue known; predicates OK (quest/flags); **R-PROX skipped in EX-P05** | none (read) or create cursor row | `dialogue.node.state {node_id, speaker, text, options:[{option_id, label_key}], revision}` |
| `dialogue.choice.request {node_id, option_id}` | option was offered on last open for this dialogue (or still legal by predicates); idempotent | insert `character_dialogue_choices`; may set choice flags; `state_revision++` | `dialogue.node.state` (next) or `command.error` |

Errors: `NOT_FOUND`, `PREREQUISITE_NOT_MET`, `STATE_CONFLICT`, `OUT_OF_RANGE` (reserved for proximity), `BAD_REQUEST`, `IDEMPOTENCY_CONFLICT`, `INTERNAL`.

## 3. Option filtering (server authoritative)
Offer an option only if:
- `ACCEPT` ⇒ quest `AVAILABLE` and `npc_id` is contact;
- `TURN_IN` ⇒ quest `READY_TO_TURN_IN` and contact NPC;
- `CONTINUE` / `ASK` / `LATER` ⇒ quest `ACTIVE` or presentation allows;
- `CHOICE-*` ⇒ quest active and choice not already committed for that group.

Hidden/illegal `option_id` on choice ⇒ `PREREQUISITE_NOT_MET` (CT-014).

## 4. Persistence
```
character_dialogue_choices(character_id, dialogue_id, node_id, option_id, content_build_id, chosen_at)
  PRIMARY KEY (character_id, dialogue_id, node_id)
```
One committed choice per node; replay returns stored option. Append-only preferred; UPDATE forbidden for `option_id`.

## 5. Content source
Active content build `dialogue_line` entities for text/speaker; Stage-1 flow modes from exported quest dialogue graph file `content/dialogues/flows_reg01.json` (director-published, mirrors client flows for MQ-01-01…05). No new third-party deps.

## 6. Out of scope
Full localization pipelines, cinematic skip flags, proximity, party shared dialogue, EX-CONTENT-01 full 240 rewrite.

## 7. Acceptance
Open → filtered options; illegal choice rejected; legal choice persisted once; reconnect snapshot may include `scopes.dialogue` later (additive); `go test -race`, CI `server-db`.
