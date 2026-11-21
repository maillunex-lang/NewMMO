# T-023 — Wire Godot client to live server (research)

**Date:** 2026-11-21 · **Lane:** client · **Deps:** Godot 4.7.2 built-ins only (WebSocketPeer, HTTPClient)

## Sources

| # | Source | Takeaway |
|---|--------|----------|
| 1 | Contracts `auth-session` + `websocket-protocol` + `quest-engine` | login→ticket→session.auth→snapshot→accept/interact/turn-in |
| 2 | Existing `AuthClient` + `NetClient` + `Fake*` (T-011/T-022) | Reuse; do not rewrite transports |
| 3 | Godot 4.7 WebSocketPeer / HTTPClient docs | Peer recreate on reconnect already in WebSocketTransport |
| 4 | Hysen Labs / AssetLib net kits | Reject — protocol-specific; no new addons |

## Candidates

| # | Approach | Verdict |
|---|----------|---------|
| 1 | **OnlineSession Node + quest_manager online mode** (signal-driven) | **Adopt** |
| 2 | Replace quest_manager with pure server mirror | Too invasive for Stage-1 UI dialogues |
| 3 | Third-party multiplayer addon | New dep — reject |

## Recommendation

Keep local dialogue/journal presentation; route accept / `world.interact` / turn-in through `NetClient` when `OnlineSession` is active. Default boot stays offline so `mq01_smoke` remains green. Headless `online_smoke.gd` proves the full chain with FakeHttp+FakeTransport (no live server required in CI). Instance IDs for `count>1` come from `content/interactables/reg-01.json` + optional `@export instance_id` on interactables.
