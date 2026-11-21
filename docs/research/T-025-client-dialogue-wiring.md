# T-025 — Client dialogue live wiring (research)

**Date:** 2026-11-21 · **Lane:** client · **Deps:** none new (uses T-011 NetClient + T-024 server)

## Sources

| # | Source | Takeaway |
|---|--------|----------|
| 1 | `docs/contracts/dialogue-engine.md` + WS protocol | open → `dialogue.node.state`; choice → next node; options server-filtered |
| 2 | T-023 `quest_manager.enable_online` + FakeTransport smoke | Same pattern: request mutation, inject S2C, assert outbound |
| 3 | Godot Dialogue Manager / similar plugins | UI kits add deps; existing `.tscn` dialogue UI is enough |

## Candidates

| # | Approach | Verdict |
|---|----------|---------|
| 1 | **Extend quest_manager + dialogue_ui for server nodes** | **Adopt** |
| 2 | Third-party dialogue runtime | New dep — reject |
| 3 | Keep client-only option filter online | Violates authority — reject |

## Recommendation

Online path: `on_npc_talk` → `dialogue.open.request`; UI shows `open_server_node`; choices → `dialogue.choice.request` then existing quest accept/turn-in. Offline `mq01_smoke` unchanged.
