# T-017 — Pilot research notes

1. ink uses conditional branches and explicit return paths; keep ASK inside START, without accepting a quest. Source: https://www.inklestudios.com/ink/web-tutorial/ (official documentation).
2. Yarn Spinner separates lines, choices and runtime commands; do not treat NPC prose as a server objective event. Source: https://hysenlabs.com/projects/yarnspinnertool-yarnspinner-unity (MIT, Unity integration, not adopted).
3. Dialogic 2 offers dialogue trees and characters in Godot; alpha status makes integration unnecessary for this text-only pilot. Source: https://hysenlabs.com/projects/dialogic-godot-dialogic (MIT, active, Godot 4.5+, not adopted).
4. GoNorth separates quest/NPC records and conditional dialogue nodes; state-conditioned progress should be authored against confirmed server state. Source: https://hysenlabs.com/projects/steffendx-gonorth (MIT, external planning stack, not adopted).
5. Bureaucratic sarcasm should expose an absurd rule, never mock the endangered person. One line should change one piece of knowledge; object hints must precede completion, reactions must follow confirmation. This is an original editorial rule, not third-party text.

**Decision:** keep the existing `DIA.*` format, no dependencies, no copied prose, no installation. Research metadata is discovery-only, not a compatibility test.

## Revision 1 (director feedback)

- Replaced repeated generic objective reactions and progress lines with distinct evidence, maps, signatures, escort, and bridge details.
- Added explicit LATER dialogue and NPC response to every START; no state mutation.
- Added canonical participant voices NPC-102 for MQ handoff, NPC-801 for returned journal, NPC-101 for scribe testimony, and NPC-903 for escort; no new IDs.
- SQ-01-002 copy selection is not an A/B state branch. Existing CHOICE keys remain; any selected copy completes the existing objective.
- Fixed SQ-01-005 terminal punctuation.
- Compared all NPC/PLAYER utterances by exact text across six updated files (94 lines): 0 duplicates. Repository Python strict lint not run because no command execution is available via GitHub connector.
- Preflight comparison showed main ahead by 3 commits; GitHub connector cannot perform a non-destructive git rebase. Changes committed on original task branch without rewriting history.
