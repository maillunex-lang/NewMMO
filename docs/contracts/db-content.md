# Contract: PostgreSQL content registry + migrations (D-001 / EX-P03)

**Owner:** director · Adapts `14_CONTENT_INTEGRATION.md` §13.2 and §16 (reference SQL; gameplay tables come in EX-P04). Server lane writes the SQL files
and Go code against this contract; the SQL below is the **required shape**, not a script to paste blindly.

## 1. Scope of EX-P03
In scope: content build registry, entity store, active-build pointer, audit log, loader (stage → verify → publish → rollback), migration job, CI integration test.
**Out of scope (EX-P04+):** `character_quest_states`, `reward_claims`, `character_dialogue_choices`, `dungeon_instances`, `world_event_instances` (§16 reference stays valid; they are created later, `content_build_id` on instances references `content_builds`).

## 2. Tables (all in schema `public`; names are final)

```
content_builds(
  build_id text PRIMARY KEY CHECK (build_id ~ '^[0-9a-f]{64}$'),
  schema_version integer NOT NULL,
  minimum_client_version text NOT NULL,
  manifest_sha256 char(64) NOT NULL CHECK (manifest_sha256 = build_id),
  entity_count integer NOT NULL CHECK (entity_count > 0),
  manifest jsonb NOT NULL,
  status text NOT NULL CHECK (status IN ('staged','published','retired')),
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz,
  retired_at timestamptz )

content_entities(
  build_id text NOT NULL REFERENCES content_builds(build_id),
  entity_id text NOT NULL,
  kind text NOT NULL,
  region_id text,
  source_file text NOT NULL, source_line integer NOT NULL,
  payload jsonb NOT NULL,
  PRIMARY KEY (build_id, entity_id) )      -- + indexes (build_id, kind), (build_id, region_id)

content_active(                              -- singleton row
  id smallint PRIMARY KEY CHECK (id = 1),
  build_id text NOT NULL REFERENCES content_builds(build_id),
  previous_build_id text REFERENCES content_builds(build_id),
  switched_at timestamptz NOT NULL DEFAULT now(),
  switched_by text NOT NULL )

content_audit(
  audit_id bigserial PRIMARY KEY,
  at timestamptz NOT NULL DEFAULT now(),
  action text NOT NULL CHECK (action IN ('stage','reject','publish','rollback')),
  build_id text, previous_build_id text,
  actor text NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb )
```
**Immutability:** a trigger forbids `UPDATE`/`DELETE` on `content_entities` rows whose build is not `staged`, and forbids `DELETE` on non-staged `content_builds`. Content of a published/retired build never changes.

## 3. Operations (each is one short transaction unless stated)

| Operation | Rules |
|-----------|-------|
| **Stage** | Verify bundle (`content-export.md` §4) **before** touching the DB. In one transaction insert the `content_builds` row (`staged`) and all entities (batched/`COPY`), recount in SQL (`count(*) = entity_count`, per-kind counts), audit `stage`. Re-staging an existing `build_id` is idempotent (no-op, audit note). Failure ⇒ rollback + audit `reject` (separate tx). |
| **Publish(build_id, expected_previous)** | Tx: `SELECT … FROM content_active WHERE id=1 FOR UPDATE`; require `build_id` current == `expected_previous` (else reject: concurrent publish); require target `staged` or `retired` (not already active); set target `published` (+`published_at`), previous active → `retired` (+`retired_at`), update `content_active` (previous_build_id, switched_by), audit `publish`. First ever publish: no row ⇒ insert id=1 (guard against double insert). Readers never see a half-published set because the pointer flips atomically. |
| **Rollback(to_build_id)** | Same transaction shape; target must exist and be `retired`/`published` with all entities; audit `rollback`. **Content rollback = pointer switch; never run schema DOWN to undo content.** |
| **Read** | Every query filters by an explicit `build_id` obtained once per request/instance from `content_active`; running instances keep their `content_build_id` (no mid-instance switch). |
| **Retention** | Keep at least the previous published build; never delete non-staged builds in EX-P03. |

No network/CDN calls inside transactions. Default isolation `READ COMMITTED` with the explicit row lock above.

## 4. Migrations (goose)

- Directory `server/migrations/`, files `NNNNN_snake_name.sql` (5-digit, sequential from `00001`), SQL format with `-- +goose Up` / `-- +goose Down` (+ `-- +goose StatementBegin/End` for functions/triggers).
- Planned files for EX-P03: `00001_content_builds.sql`, `00002_content_entities.sql`, `00003_content_active_audit.sql`, `00004_content_immutability_triggers.sql` (the server lane may merge/split but keeps order and idempotent re-run via goose version table).
- **One migration path:** the migration job/CLI. The server process **never** auto-migrates. DDL role ≠ application role (recommended: `newmmo_migrator` owns objects, `newmmo_app` has DML only).
- `Down` is implemented for development/CI (clean up→down→up must pass); **production is forward-only with backup + dry-run** (§16 canon warning).
- No `CREATE INDEX CONCURRENTLY` in these migrations (tables start empty); if one is ever needed use `-- +goose NO TRANSACTION`.

## 5. Configuration and secrets
`NEWMMO_DATABASE_URL` (app), `NEWMMO_MIGRATE_DATABASE_URL` (migration job), `NEWMMO_TEST_DATABASE_URL` (integration tests; tests **skip** when unset). Only environment/secret store; never committed; never logged.
Isolated Compose stack for local/dev lives under `deploy/newmmo/` (own project name, own network/volume, loopback-bound port, no shared volumes/DBs/Caddy).

## 6. Acceptance for the loader (server lane, T-009)
Clean `up → down → up`; stage the real export bundle (4925 entities) and read back counts per kind; reject tampered manifest/entities/crossrefs; publish, double-publish rejected, concurrent publish (two goroutines) → exactly one wins; rollback restores the previous pointer; immutability triggers reject updates/deletes; idempotent re-stage; failure mid-stage leaves no partial build visible.
