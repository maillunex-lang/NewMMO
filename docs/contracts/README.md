# Contracts (frozen interfaces)

Director-owned. Implementation lanes build against these files; they change only via a director task.

| File | Subject | Status |
|------|---------|--------|
| `content-export.md` | export bundle format (manifest, `entities.jsonl`, hashes, loader obligations) | **v1 published** (D-001) |
| `db-content.md` | PostgreSQL content registry, publish/rollback protocol, goose migration conventions | **v1 published** (D-001) |
| `toolchain-pins.md` | approved third-party pins (Go, pgx, goose, PostgreSQL) | **v1 published** |
| `websocket-protocol.md` + `websocket-envelope.schema.json` | WebSocket v1: transport, envelope, lifecycle, messages, idempotency, limits | **v1 published** (D-003) |
| `auth-session.md` | accounts, login, tokens, characters, one-time WS tickets (REST) | **v1 published** (D-004) |
| `db-gameplay.md` | gameplay tables: characters, quests, reward ledger, idempotency | **v1 published** (D-004) |
| `quest-engine.md` | quest states, interactions, exactly-once rewards, snapshots | **v1 published** (D-004) |
| `content/interactables/reg-01.json` | interaction registry: objective → event/target/instances → canon POI/LOC (stub ids kept) | **v1 published** (D-002), checked by `check_client_sync.py` |
| `loot-engine.md` | combat loot: server-rolled drops, kill-report interim (R-KILL), ledger, messages | **v1 published** (T-034) |

See `docs/director/DIRECTOR.md` §7.
