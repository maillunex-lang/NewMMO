# Contract: PostgreSQL content registry + migrations (D-001 / EX-P03)

**Owner:** director · **rev 1.1** (applies T-010 review R-01…R-13) · Adapts `14_CONTENT_INTEGRATION.md` §13.2 and §16 (reference SQL; gameplay tables come in EX-P04). Server lane writes the SQL files
and Go code against this contract; the SQL below is the **required shape**, not a script to paste blindly.

## 1. Scope of EX-P03
In scope: content build registry, entity store, active-build pointer, audit log, loader (stage → verify → publish → rollback), migration job, CI integration test.
**Out of scope (EX-P04+):** `character_quest_states`, `reward_claims`, `character_dialogue_choices`, `dungeon_instances`, `world_event_instances` (§16 reference stays valid; they are created later, `content_build_id` on instances references `content_builds`).

## 2. Tables (all in schema `public`; names are final)

```
content_builds(
  build_id text PRIMARY KEY CHECK (build_id ~ '^[0-9a-f]{64}$'),
  schema_version integer NOT NULL CHECK (schema_version > 0),
  minimum_client_version text NOT NULL,
  manifest_sha256 text NOT NULL CHECK (manifest_sha256 ~ '^[0-9a-f]{64}$' AND manifest_sha256 = build_id),
  entity_count integer NOT NULL CHECK (entity_count > 0),
  ingest_digest text NOT NULL CHECK (ingest_digest ~ '^[0-9a-f]{64}$'),   -- see §3 Stage
  manifest jsonb NOT NULL CHECK (jsonb_typeof(manifest) = 'object'),
  status text NOT NULL CHECK (status IN ('staged','published','retired')),
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz,
  retired_at timestamptz )

content_entities(
  build_id text NOT NULL REFERENCES content_builds(build_id),
  entity_id text NOT NULL,
  kind text NOT NULL,
  region_id text CHECK (region_id IS NULL OR region_id ~ '^REG-0[1-6]$'),
  source_file text NOT NULL, source_line integer NOT NULL CHECK (source_line > 0),
  line_sha256 text NOT NULL CHECK (line_sha256 ~ '^[0-9a-f]{64}$'),      -- sha256 of the exact JSONL line bytes (without LF)
  payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
  PRIMARY KEY (build_id, entity_id) )      -- + indexes (build_id, kind), (build_id, region_id)

content_active(                              -- singleton row, SEEDED BY THE MIGRATION with build_id NULL ("nothing published yet")
  id smallint PRIMARY KEY CHECK (id = 1),
  build_id text REFERENCES content_builds(build_id),
  previous_build_id text REFERENCES content_builds(build_id),
  switched_at timestamptz NOT NULL DEFAULT now(),
  switched_by text NOT NULL )

content_audit(
  audit_id bigserial PRIMARY KEY,
  at timestamptz NOT NULL DEFAULT now(),
  action text NOT NULL CHECK (action IN ('stage','reject','publish','rollback')),
  build_id text, previous_build_id text,
  actor text NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb )     -- structured error code/class; never secrets
```

### Integrity enforcement (T-010 R-02, R-09)
1. **Row guards (triggers):** `content_entities`: `UPDATE`/`DELETE` allowed only while the parent build is `staged`. `content_builds`: `DELETE` only while `staged`; `UPDATE` may change **only** `status`, `published_at`, `retired_at`, and **only** along the allowed transitions `staged→published`, `published→retired`, `retired→published`; never anything→`staged`; `build_id`, `manifest*`, `entity_count`, `ingest_digest`, `schema_version`, `created_at` are immutable after insert.
2. **`TRUNCATE`** on `content_builds`, `content_entities`, `content_active`, `content_audit` is rejected by statement-level triggers (defense in depth) and not granted.
3. **Pointer guard:** a `BEFORE UPDATE` trigger on `content_active` requires `NEW.build_id` to reference a build whose `status` is `published` (set in the same transaction before the pointer flips) and requires `build_id`/`previous_build_id` consistency (`previous_build_id` = old `build_id`); `content_audit` rows are append-only (no UPDATE/DELETE).
4. **Roles (privileges are the real boundary; triggers are defense in depth):** `newmmo_migrator` owns the objects; `newmmo_content_publisher` has the DML needed by Stage/Publish/Rollback (used by `contentctl`/admin publish); `newmmo_app` has **`SELECT` only** on all four tables. The application role is never a table owner. DB owner/superuser remains the trusted boundary.

## 3. Operations

All pointer mutations follow **one lock order (T-010 R-03):** (1) `SELECT … FROM content_active WHERE id=1 FOR UPDATE` (the singleton always exists), then (2) the affected `content_builds` rows `FOR UPDATE` in ascending `build_id` order. Stage never locks the pointer and never mutates a non-staged build. Deadlock (`40P01`) / serialization (`40001`) ⇒ retry up to 3 times with bounded backoff, then fail with `CONFLICT`. Default isolation `READ COMMITTED` **plus** the explicit locks above (sufficient only with this protocol).

| Operation | Rules |
|-----------|-------|
| **Stage(bundle)** | Verify the bundle first (`content-export.md` §4). Take `pg_advisory_xact_lock(hashtext(build_id))` (single writer per build). If the build exists: `published`/`retired` ⇒ **no-op** (audit note) after checking `manifest_sha256`; `staged` ⇒ recompute `ingest_digest` from stored rows (below): equal ⇒ no-op, different ⇒ delete the staged rows and re-ingest inside the same tx. New build: insert `content_builds` (`staged`) + all entities (`COPY`/batched, benchmark both; bounded memory), compute `line_sha256` per line and `ingest_digest = sha256( concat(line_sha256 in (kind, entity_id) order) )`, recount in SQL (`count(*)`, per-kind), audit `stage`. Staged builds older than **7 days** with no activity may be removed by an audited cleanup job (never a non-staged build). |
| **Publish(build_id, expected_current)** | Tx: lock pointer; require `content_active.build_id IS NOT DISTINCT FROM expected_current` (`NULL` = nothing published) else `CONFLICT`; lock target build; require target `staged` or `retired`; for `staged` **recompute `ingest_digest` and per-kind counts from the DB rows and compare** with the stored values (fails ⇒ reject, audit `reject`); set target `published` (+`published_at`), previous active → `retired` (+`retired_at`), update pointer (`previous_build_id`, `switched_by`), audit `publish`. Two simultaneous *first* publishes: both lock the seeded pointer row ⇒ serialized; the second sees `build_id` ≠ its `expected_current` (`NULL`) ⇒ `CONFLICT` (not success). |
| **Rollback(to_build_id, expected_current)** | Same lock order and `expected_current` check (no blind overwrite of a newer publish); target must be `retired` with complete entities; swap statuses, pointer, audit `rollback`. **Content rollback = pointer switch; never run schema DOWN to undo content.** Before the flip, target `schema_version` must be supported by the running server. |
| **Read** | One query for the active `build_id`, then every query filters by that explicit `build_id`; running instances keep their `content_build_id` (no mid-instance switch). Runtime uses `newmmo_app` (SELECT only). |
| **Retention / GC (T-010 R-08)** | Never delete the active and previous builds. Every persistent EX-P04+ row that depends on content (`dungeon_instances`, `world_event_instances`, quest/dialogue state that pins a build) carries `content_build_id` **with an FK to `content_builds`**; builds referenced by any such row are never deleted. (Canon §16 `world_event_instances` lacks the column: EX-P04 adds it — recorded as a director decision, not a canon change to the entity model.) GC is out of scope in EX-P03. |
| **Errors (T-010 R-13)** | Loader errors carry a stable code: `BUNDLE_INVALID`, `BUNDLE_TOO_LARGE`, `HASH_MISMATCH`, `SCHEMA_UNSUPPORTED`, `COUNT_MISMATCH`, `CONFLICT`, `NOT_FOUND`, `INVALID_STATE`, `DB_ERROR`. The `reject` audit row is best-effort in a separate tx: if it fails, the original error is still returned and a structured log line is written (no secrets, no payloads). |

No network/CDN calls inside transactions.

## 4. Migrations (goose)

- Directory `server/migrations/`, files `NNNNN_snake_name.sql` (5-digit, sequential from `00001`), SQL format with `-- +goose Up` / `-- +goose Down` (+ `-- +goose StatementBegin/End` for functions/triggers).
- Planned files for EX-P03: `00001_content_builds.sql`, `00002_content_entities.sql`, `00003_content_active_audit.sql` (incl. singleton seed), `00004_content_integrity_guards.sql` (row/transition/TRUNCATE/pointer triggers + grants); the server lane may merge/split but keeps this order.
- **Seed:** migration `00003…` inserts `content_active (id=1, build_id NULL, switched_by 'migration')`; roles/grants and triggers (§2) are created by migrations (role creation may be done by the compose init/CI setup if the migrator lacks `CREATEROLE`; document which).
- **One migration path:** the migration job/CLI. The server process **never** auto-migrates. DDL role ≠ application role (recommended: `newmmo_migrator` owns objects, `newmmo_app` has DML only).
- `Down` is implemented for development/CI (clean up→down→up must pass); **production is forward-only with backup + dry-run** (§16 canon warning); `Down` is never run against a database that holds published content (the CI/dev runner uses an isolated empty DB).
- Only **one migration process** may run (single job; use the tool's locking; concurrent-runner behaviour is tested, not assumed). goose build tool: Go toolchain pinned separately from the server (`toolchain-pins.md`).
- No `CREATE INDEX CONCURRENTLY` in these migrations (tables start empty); if one is ever needed use `-- +goose NO TRANSACTION`.

## 5. Configuration and secrets
`NEWMMO_DATABASE_URL` (app), `NEWMMO_MIGRATE_DATABASE_URL` (migration job), `NEWMMO_TEST_DATABASE_URL` (integration tests; tests **skip** when unset). Only environment/secret store; never committed; never logged.
Isolated Compose stack for local/dev lives under `deploy/newmmo/` (own project name, own network/volume, loopback-bound port, no shared volumes/DBs/Caddy).

## 6. Acceptance for the loader (server lane, T-009)
Clean `up → down → up` (isolated empty DB); stage the real bundle (4925 entities) and read back counts per kind; reject tampered manifest/entities/crossrefs, duplicate JSON keys, invalid UTF-8, oversize line, wrong counts, missing/extra entity; publish; double-publish rejected; **two simultaneous first publishes ⇒ exactly one wins, the other gets `CONFLICT`**; publish vs rollback and stage vs publish races (no deadlock, bounded retries); rollback with stale `expected_current` rejected; rollback restores previous pointer; **forbidden direct DML as `newmmo_app`** (UPDATE/DELETE/TRUNCATE/pointer to a staged build) fails; parent transition guard (published→staged rejected, immutable columns rejected); idempotent re-stage; tampered **staged** rows detected at publish by `ingest_digest`; failure mid-stage leaves no partial build visible; two simultaneous migration runners do not corrupt the schema.

## 7. Review log (T-010, `docs/reviews/T-010-contracts-review.md`)
R-01 seeded singleton + `expected_current` (§2, §3) · R-02 transition/immutable-column guards, TRUNCATE triggers, roles (§2) · R-03 lock order + retries (§3) · R-04 `ingest_digest`, advisory lock, staged TTL (§2–3) ·
R-05 strict parsing and bounds (`content-export.md` §4) · R-06 global-namespace invariant (`content-export.md` §5) · R-07 canon §14 mapping (`content-export.md` §5) · R-08 FK/retention rule (§3) · R-09 pointer guard (§2) ·
R-10 text+CHECK hashes, jsonb type checks (§2); COPY-vs-INSERT left to a benchmark in T-009 · R-11 separate goose toolchain, single migration job, no Down on live data (§4) · R-12 manifest hashed as raw bytes — no cross-language canonicalization (`content-export.md` §2) · R-13 error codes + best-effort audit (§3).
