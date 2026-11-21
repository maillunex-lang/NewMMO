# T-011 — Client WebSocket protocol layer (no game wiring)

**Date:** 2026-11-21 · **Lane:** client · **Engine:** Godot **4.3**  
**Status:** Research → native only; **nothing installed**

## 1. Need

Isolated `client/scripts/net/` implementing D-003 (`docs/contracts/websocket-protocol.md`): text JSON, subprotocol `newmmo.v1`, no compression, `poll()` every frame, FakeTransport tests, no hook into `quest_manager` / scenes.

## 2. Sources

| Source | Used |
|--------|------|
| Contract | `websocket-protocol.md`, `websocket-envelope.schema.json` |
| Research | `docs/research/go-websocket.md` §4–§6 |
| Godot docs | [WebSocketPeer](https://docs.godotengine.org/en/4.3/classes/class_websocketpeer.html), websocket tutorial |
| Hysen Labs | discovery for networking wrappers (centrifuge-related server tools; client remains engine-native) |
| Demos | godot-demo-projects `WebSocketClient.gd` pattern |

## 3. Candidates (≥3)

| # | Option | License / fit | Risks | Verdict |
|---|--------|---------------|-------|---------|
| 1 | **Native `WebSocketPeer` + thin GDScript session** | Built-in 4.3 | Must recreate peer on reconnect (4.3) | **Adopt** |
| 2 | Godot demo `WebSocketClient` Node wrapper | MIT demo | Couples to Node/`_process`; still need protocol | Pattern only — copy ideas, not vendor |
| 3 | Third-party reconnect / net addons (AssetLib) | varies | Addon policy; license; may not match D-003 | **Do not install** |
| 4 | Centrifuge/Centrifugo client | MIT | Own protocol ≠ D-003 | Reject for v1 |

## 4. Test approach

`FakeTransport` implements the same interface as `WebSocketTransport` so `NetClient` is fully exercised headless without a server: inject S→C text, assert C→S queue, simulate close codes, advance time for backoff with a seeded RNG for jitter.

## 5. Recommendation

Build in-house RefCounted modules under `client/scripts/net/`; wire only `net_smoke.gd` into `tools/smoke_stage1.sh`. No addons. Do not attach to game loop until EX-P04.

**Contract note:** Godot `JSON.parse_string` does not reliably surface duplicate keys; we validate schema fields/types/depth and document that duplicate-key rejection depends on engine JSON (report if blocking).
