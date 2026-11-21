# Contracts (frozen interfaces)

Director-owned. Implementation lanes build against these files; they change only via a director task.

| File | Subject | Status |
|------|---------|--------|
| `content-export.md` | export bundle format (manifest, `entities.jsonl`, hashes, loader obligations) | **v1 published** (D-001) |
| `db-content.md` | PostgreSQL content registry, publish/rollback protocol, goose migration conventions | **v1 published** (D-001) |
| `toolchain-pins.md` | approved third-party pins (Go, pgx, goose, PostgreSQL) | **v1 published** |
| WebSocket protocol | client↔server messages | pending T-003 research (EX-P04) |
| POI id mapping | stub POI ids → canon `POI-01-*` | pending D-002 |

See `docs/director/DIRECTOR.md` §7.
