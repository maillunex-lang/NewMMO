# T-022 — Auth / character UI + REST client (EX-P04/P06)

**Date:** 2026-11-21 · **Lane:** client · **Engine:** Godot **4.7.2**  
**Contract:** `docs/contracts/auth-session.md` (D-004) · WS ticket → `session.auth` later (not this task)

## 1. Sources

| # | Source | Takeaway |
|---|--------|----------|
| 1 | Godot 4.7 docs: `HTTPRequest`, `HTTPClient`, `LineEdit.secret`, Theme/containers, `user://` | Native REST + secret fields; prefs only under `user://` |
| 2 | In-repo T-011 FakeTransport pattern + D-004 table | Injectable FakeHttp; same error shape `{"error":{"code","message_key"}}` |
| 3 | Hysen Labs catalog / GitHub login-screen demos | Scene-first login stacks; avoid vendoring auth addons |
| 4 | Official HTTP tutorial (POST JSON + headers) | `Content-Type: application/json`; one in-flight request |

## 2. Candidates (≥3)

| # | Approach | Fit | Verdict |
|---|----------|-----|---------|
| 1 | **RefCounted AuthClient + HttpTransport / FakeHttp** (mirror T-011) | Testable headless; no Node required for smoke | **Adopt** |
| 2 | Scene-owned `HTTPRequest` nodes only | Fine for UI; hard to unit-test without tree | Use only as optional live adapter later |
| 3 | Third-party Godot auth / REST addon | License + Extra-Write; not D-004 shaped | **Do not install** |
| 4 | Hardcoded hosts in `.tscn` | Forbidden by task | Reject |

## 3. Decision

- `auth_client.gd` + `http_transport.gd` + `fake_http.gd` under `client/scripts/net/`.
- Base URL: `NEWMMO_API_URL` else `http://127.0.0.1:18086`.
- No auto-retry on POST/PUT/PATCH/DELETE.
- Access token memory-only; optional remembered **login** in `user://auth_prefs.json`.
- UI: editable `.tscn` panels + `tr()`; class names from `client/config/classes.json` (canon CLS-01…05).
- Emit `session_ticket_ready` only — **no WebSocket connect**.
- Smoke: `auth_smoke.gd` → `AUTH_SMOKE_OK` wired into `tools/smoke_stage1.sh`.
