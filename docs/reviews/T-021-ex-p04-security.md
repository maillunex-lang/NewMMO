# T-021 — Security review: EX-P04 auth, tickets, quest ledger

**2026-11-21 · lane research (Cursor-director) · static + contract cross-check of `main` @ `4461aa7`.** Scope: T-018 auth/REST, T-019 WebSocket tickets/sessions, T-020 quest FSM + rewards. **No VPS, no new dependencies, no canon edits.** Severity reflects exploitability or acceptance failure for EX-P04 online authority.

## Research-first

1. [OWASP ASVS V2/V3](https://owasp.org/www-project-application-security-verification-standard/) / [Cheat Sheet Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html): PBKDF2 ≥ 600k iterations acceptable for EX-P04; Argon2 preferred later — not a new dep without owner OK.
2. [PostgreSQL 17 locking](https://www.postgresql.org/docs/17/explicit-locking.html): `FOR UPDATE` on character row matches `db-gameplay.md` §4 serialization.
3. [RFC 6455](https://datatracker.ietf.org/doc/html/rfc6455) / coder/websocket v1.8.15: ticket-in-first-message pattern; Origin checks are deployment-dependent.
4. [Hysen Labs projects](https://hysenlabs.com/projects): directory scanned as discovery; **no verified drop-in MMO auth/quest ledger** suitable under current dependency freeze — in-house remains correct.
5. Contracts: `auth-session.md`, `db-gameplay.md`, `quest-engine.md`, WS protocol rev 1.1.

## Findings

| ID | Severity | Where | Problem | Scenario | Proposal |
|---|---|---|---|---|---|
| **S-01** | **major** | `quest/engine.go` TurnIn/Interact; catalog holds **active** build only | Contract requires rewards/objectives from the **pinned** `content_build_id`. Code loads defs from the active catalog (`_ = pinnedBuild`). Publishing a new build with changed XP/gold/objectives mid-quest changes rewards or progress rules. | Accept MQ-01-01 on build A; publish build B with `xp: 999`; turn-in grants 999 instead of pinned 100. Static; not executed against live publish race. | Load quest payload (and objective registry merge) from `content_entities` for the row's pinned build on Interact/TurnIn; add regression test: stage+publish altered reward, turn-in still uses pinned numbers. |
| **S-02** | **major** (known interim) | `quest-engine.md` §3 R-PROX | No server position/visibility. Hostile client can `world.interact.request` any registered target while ACTIVE. | Spoof interact sequence without visiting POIs → READY → rewards. Documented owner-visible interim until EX-P08/P09. | Keep EX-P04 acceptance honest; **block “online-safe” claims**; prioritize movement authority before public playtests. |
| **S-03** | **minor** | `auth/store.go` `Prune`; `main.go` | `idempotency_keys` retention (≥24h) has **no prune job**; only login_attempts + ws_tickets pruned. | Long-lived shard accumulates JSON results per request_id; disk/index growth. | Extend `Prune` to `DELETE … WHERE created_at < now()-interval '48 hours'` (or contract window); CI size smoke optional. |
| **S-04** | **minor** | `ws` upgrade path | No explicit Origin/Host allowlist in server code. Fine behind trusted reverse proxy; open raw WS is CSRF-adjacent for browser clients. | Browser on evil origin connects if ticket leaked into JS. Tickets are short-lived/one-time, mitigating. | Document deploy requirement (Caddy Origin checks) or optional `NEWMMO_WS_ORIGINS` when browser clients appear. |
| **S-05** | **minor** | `00008` grants; quest tests | Append-only triggers exist, but quest package **does not** assert `newmmo_app` cannot UPDATE/DELETE `reward_claims`/`character_ledger` (auth/content tests cover other guards). | Regression removing triggers could ship green. | Add negative privilege tests as `newmmo_app` in `quest/postgres_test.go`. |
| **S-06** | **minor** | `quest/engine.go` TurnIn ON CONFLICT path | If a claim row exists while state is still `READY_TO_TURN_IN` (crash mid-tx historically impossible under single tx; manual DML), second turn-in returns reconstructed reward without completing state. | Operator/manual INSERT into `reward_claims` then client turn-in → inconsistent state. | On conflict: if state ≠ COMPLETED, fail closed (`STATE_CONFLICT`/`INTERNAL`) or complete state in same tx when claim already matches. |
| **S-07** | **info** | `auth-session.md` §7 | Slice accounts are not production identity (no email/MFA/refresh). | Public launch with slice auth only. | Owner decision before EX-P13; do not market as production IAM. |
| **S-08** | **info** | `auth/password.go` | PBKDF2-SHA256 600k + constant-time compare + dummy hash on unknown login; tokens/tickets stored as SHA-256 only. Matches contract. Timing equalization tested in CI. | — | Keep; consider Argon2id later with owner OK. |

## What looks correct (static)

- **Passwords:** NFC normalize, 10–128 runes, PBKDF2 600k, `subtle.ConstantTimeCompare`, unknown-login dummy KDF, rehash on legacy iterations (`auth/password.go`, `store.go` Login).
- **Tokens/tickets:** 32-byte secrets, DB stores hash only, ticket one-time `UPDATE … RETURNING`, foreign character → FORBIDDEN, concurrent ticket consume tested.
- **Rate limits:** registration/login IP+pair windows; WS admission + per-session token bucket; no secrets in structured logs (integration asserts).
- **Quest mutations:** character `FOR UPDATE`; idempotency under lock; `reward_claims` UNIQUE; concurrent turn-in CI → one claim; `REGION_CLOSED` gated; client cannot supply reward numbers.
- **Roles:** `newmmo_app` no DDL; ledger/claims append-only triggers; COMPLETED immutability trigger.

## EX-P04 acceptance impact

| Gate | Verdict |
|------|---------|
| Auth/tickets/no secrets in logs | **Pass** (T-018/T-019 CI) |
| WS session lifecycle | **Pass** |
| Quest FSM + idempotency + one reward | **Pass** for single active build |
| Build pinning of rewards | **Fail until S-01 fixed** |
| Online anti-cheat proximity | **Deferred (S-02)** — not a silent pass |
| Client live MQ-01-01…05 | **Open (T-023)** |

## Recommendation

1. **Fix S-01 before claiming EX-P04 complete** (small server follow-up; no new deps).
2. Merge this review; track S-03…S-06 as hardening (may ride with T-023 or a short T-024).
3. Proceed to **T-023** in parallel with S-01 fix — client wiring does not depend on prune/Origin.
4. Do **not** waive S-02 in public/playtest messaging.

## Closure

Review complete at `4461aa7`. Findings are static unless noted.

**Mitigations landed with this acceptance:**
- **S-01:** `Catalog.QuestForBuild` — Interact/TurnIn use pinned `content_build_id` payloads.
- **S-03:** `Store.Prune` deletes `idempotency_keys` older than 48h.
- **S-06:** claim conflict with non-COMPLETED quest state → `STATE_CONFLICT`.

**Still open:** S-02 (R-PROX), S-04 (Origin docs), S-05 (append-only negative tests), S-07/S-08 informational.
