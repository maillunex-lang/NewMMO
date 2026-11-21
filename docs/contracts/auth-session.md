# Contract: accounts, authentication, characters, WebSocket tickets (D-004)

**Owner:** director · **Version:** 1 · Scope: vertical-slice-grade identity for EX-P04 (no e-mail, no OAuth, no admin RBAC — those come with EX-P13). Implementers: server lane (Go, stdlib + approved pins only), client lane (login/character UI).
Security posture: assume hostile clients; never log secrets; constant-time comparisons; generic errors (no user enumeration).

## 1. Credentials
| Item | Rule |
|------|------|
| `login` | `^[a-z0-9_]{3,24}$`, stored lowercase, unique |
| password | 10–128 Unicode characters (NFC-normalized before hashing); never stored, never logged |
| hash | **PBKDF2-HMAC-SHA256** from Go stdlib `crypto/pbkdf2` (Go ≥ 1.24), **600 000** iterations, 16-byte random salt, 32-byte key; stored as `pbkdf2-sha256$600000$<salt_b64url>$<key_b64url>`; verification re-reads the iteration count (allows future upgrade, rehash on login when lower than current) |
| compare | `crypto/subtle.ConstantTimeCompare`; for an unknown login hash a dummy password (equalized timing) |
| brute force | ≤ **5 failed logins per 15 min per (login, source IP)** and ≤ 20 per IP; then `429 RATE_LIMITED` with `retry_after_ms` (limits tunable by config) |

## 2. Tokens
- **Access token:** 32 random bytes (`crypto/rand`), base64url, returned once; the DB stores **only SHA-256(token)**; TTL **12 h**; no refresh in EX-P04; `POST /v1/auth/logout` revokes it. Sent as `Authorization: Bearer <token>`.
- **WebSocket ticket:** 32 random bytes, base64url; DB stores SHA-256 only; TTL **30 s**; **one-time**: consumed atomically by `UPDATE ws_tickets SET used_at=now() WHERE ticket_hash=$1 AND used_at IS NULL AND expires_at>now() RETURNING account_id, character_id` (0 rows ⇒ `4001`). Bound to `(account_id, character_id)`; never in URLs or logs. A background job (or opportunistic delete) removes tickets older than 1 h.

## 3. REST API (`/v1`, JSON UTF-8, body ≤ 4 KiB, `Content-Type: application/json`, no CORS)
| Method / path | Auth | Request | Success | Notes |
|---------------|------|---------|---------|-------|
| `POST /v1/accounts` | none | `{login, password}` | `201 {account_id}` | duplicate login ⇒ `409 LOGIN_TAKEN`; per-IP creation limit 5/h |
| `POST /v1/auth/login` | none | `{login, password}` | `200 {access_token, expires_at, account_id}` | wrong login **or** password ⇒ identical `401 INVALID_CREDENTIALS` |
| `POST /v1/auth/logout` | Bearer | `{}` | `204` | revokes the presented token |
| `GET /v1/characters` | Bearer | — | `200 {characters:[{character_id(str), name, class_id, level}]}` | |
| `POST /v1/characters` | Bearer | `{name, class_id}` | `201 {character_id(str)}` | `name` `^[A-Za-zА-Яа-яЁё]{3,16}$` unique case-insensitively; `class_id` ∈ `CLS-01…CLS-05` (canon `05`); ≤ **3 characters per account** |
| `POST /v1/session/ticket` | Bearer | `{character_id}` | `200 {ticket, expires_in_ms, ws_url}` | character must belong to the account |
| `GET /healthz`, `/readyz` | none | — | existing | unchanged |

Errors: `{"error":{"code":"<CODE>","message_key":"error.<code>"}}` with HTTP status: `400 BAD_REQUEST`, `401 UNAUTHENTICATED|INVALID_CREDENTIALS`, `403 FORBIDDEN`, `404 NOT_FOUND`, `409 LOGIN_TAKEN|NAME_TAKEN|LIMIT_REACHED`, `429 RATE_LIMITED`, `500 INTERNAL`. 64-bit ids are strings. All handlers validate type/size/range before DB access; request bodies with unknown fields are rejected (`400`).

## 4. Character creation (initial state)
`xp = 0`, `gold = 0`, level derived from xp (`05` §6: `XP_to_next(L) = round(100·L^1.6)`, cap 60), `state_revision = 0`, **flag `TUTORIAL-COMPLETE` set** (there is no tutorial in the slice; this is a documented stub so that `MQ-01-01` can become AVAILABLE — canon `08`). Creation also records `content_build_id` = active build (diagnostic only).

## 5. WebSocket binding (see `websocket-protocol.md` §4)
`ws_url` = configured public WS URL (`ws://127.0.0.1:<port>/v1/ws` in dev, `wss://…` otherwise). `session.auth` carries the ticket; on success the server loads the character, sets `session_id`, and sends `session.ready`. A second session for the same character closes the older one with `4005`. Token/ticket values are never echoed in logs or errors.

## 6. Storage (shapes; SQL by the server lane, names final) — see `db-gameplay.md` §2
`accounts`, `auth_tokens`, `characters`, `ws_tickets`.

## 7. Out of scope (later)
E-mail verification/reset, MFA, social logins, refresh tokens, admin accounts/RBAC, account deletion (GDPR flow), device binding. The slice's accounts are **not** production identity; an owner decision on the production identity model is required before public release.

## 8. Acceptance
Registration/login/logout/ticket flows; password hash format and re-hash; login rate limiting and equalized timing; token/ticket stored hashed only; one-time ticket (second use rejected; expired rejected; wrong character rejected); character limits and validation; no secrets in logs (test asserts); `go vet`, `go test -race`, `govulncheck` clean.
