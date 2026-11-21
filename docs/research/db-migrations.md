# Research: PostgreSQL migrations + Go DB access (T-001)

**Date:** 2026-11-21  
**Repo:** [maillunex-lang/NewMMO](https://github.com/maillunex-lang/NewMMO)  
**Stage:** EX-P03 (prerequisite for EX-P04)  
**Lane:** research  
**Status:** Phase 1 research only; **installed: nothing**; awaiting owner's OK. No implementation, no database access, no migrations executed.

## 1. Problem statement and scope

Select (a) versioned PostgreSQL SQL up/down migrations, (b) Go DB driver/pool and optionally typed query generation, (c) a safe content-build loader/publisher. Read-only references: [AGENTS.md](../../AGENTS.md), [director protocol](../director/DIRECTOR.md), [execution roadmap](../ROADMAP_EXECUTION.md), [content integration §§13–16](../content/14_CONTENT_INTEGRATION.md), [report example](content-validation.md).

EX-P03 requires backup/dry-run, migration rollback and content_builds loader. EX-P04 requires authoritative transactions and idempotent quest/reward mutations. Canon §16 SQL is *reference*, not permission to deploy it. Director's D-001 content-export contract must exist before loader implementation. Audit existing DB/schema first; no duplicate tables.

## 2. Search sources and verification limits (2026-11-21)

| Source | Checked / outcome |
|---|---|
| [Hysen Labs projects](https://hysenlabs.com/projects) | Opened directory (48,132 projects shown), inspected taxonomy/search interface; searched web-indexed directory for PostgreSQL/goose/Atlas/dbmate. **No reliably verified additional relevant project page surfaced**. Thus no Hysen discovery is presented as vetted; catalog is not a substitute for upstream verification. Its category/compare facilities were noted, but interactive search and comparison were not completed. |
| GitHub upstream | [goose releases](https://github.com/pressly/goose/releases), [migrate releases](https://github.com/golang-migrate/migrate/releases), [tern releases](https://github.com/jackc/tern/releases), [dbmate releases](https://github.com/amacneil/dbmate/releases), [Atlas releases](https://github.com/ariga/atlas/releases), [pgx changelog](https://github.com/jackc/pgx/blob/master/CHANGELOG.md), [sqlc releases](https://github.com/sqlc-dev/sqlc/releases). Release dates below are from upstream release pages; licenses listed by project, but **SPDX/license-file confirmation at the exact proposed tag is a Phase 2 gate**. Issues were not exhaustively audited; issue-triage remains open. |
| Official | [PostgreSQL transactions](https://www.postgresql.org/docs/current/tutorial-transactions.html), [advisory locks](https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS), [JSON types](https://www.postgresql.org/docs/current/datatype-json.html), [concurrent indexes](https://www.postgresql.org/docs/current/sql-createindex.html), [Go database/sql](https://pkg.go.dev/database/sql), [Go embed](https://pkg.go.dev/embed), [pgx docs](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool). |

**Important:** Recent upstream releases do *not* automatically support Go 1.22. Upstream pgx changelog explicitly says v5.9.0+ requires Go 1.25+, and v5.13.0 (2026-11-11) includes security fixes. Therefore pinning an old Go-1.22-compatible tag indefinitely is not a safe default. Confirm actual server go.mod/toolchain and resolve upgrade versus supported historical versions with director/owner.

## 3. Candidates (migration tools)

| Candidate | SPDX (project) | Last upstream release observed | Up/down / deployment | Assessment |
|---|---|---|---|---|
| [pressly/goose](https://github.com/pressly/goose) | MIT | **v3.30.0, 2026-11-10** ([release](https://github.com/pressly/goose/releases/tag/v3.30.0)) | SQL Up/Down sections, transaction by default, optional no-transaction, CLI, Go API and embed; PG driver selected at build | **Preferred**, conditional on Go version |
| [golang-migrate/migrate](https://github.com/golang-migrate/migrate) | MIT | **v4.20.1, 2026-09-09** ([release](https://github.com/golang-migrate/migrate/releases/tag/v4.20.1)) | Paired .up.sql/.down.sql, CLI/library, io/fs source, PG locks, dirty-version recovery | **Fallback**; explicit recovery discipline |
| [jackc/tern](https://github.com/jackc/tern) | MIT | **v2.4.3, 2026-08-23** ([release](https://github.com/jackc/tern/releases/tag/v2.4.3)) | PostgreSQL-focused SQL migrations, advisory locking, CLI/Go | Strong PG-only alternative; inspect down conventions |
| [amacneil/dbmate](https://github.com/amacneil/dbmate) | MIT | **v2.37.0, 2026-10-18** ([release](https://github.com/amacneil/dbmate/releases/tag/v2.37.0)) | Up/down in one file; CLI/container; new --migration-lock for PG | Good CI-only option; modern binary built with newer Go |
| [ariga/atlas](https://github.com/ariga/atlas) | Apache-2.0 (OSS core; inspect feature licensing) | **v1.3.0, 2026-08-02** ([release](https://github.com/ariga/atlas/releases/tag/v1.3.0)) | Versioned migrations + schema diff/inspect; feature-tier distinctions | Powerful, too much workflow/tooling for initial fixed SQL schema |
| In-house migration runner | N/A (project-owned) | N/A | Custom SQL versions, transaction, locking, embed | Possible but not first choice; see §5 |

Release date is **not** last commit date. Do not infer repository inactivity from a release date. Upstream open issues/security advisories and tag license files must be rechecked before adoption.

## 4. Go data-access and query tooling

| Candidate | SPDX | Activity / compatibility | Recommendation |
|---|---|---|---|
| [jackc/pgx/v5 + pgxpool](https://github.com/jackc/pgx) | MIT | [CHANGELOG](https://github.com/jackc/pgx/blob/master/CHANGELOG.md): **v5.13.0, 2026-11-11**; v5.9+ requires Go >=1.25 | **Preferred** for PG-native transactions, jsonb, CopyFrom, pool, explicit context; choose version after toolchain decision |
| [database/sql + lib/pq](https://github.com/lib/pq) | MIT | Historical mature driver; latest upstream activity/date **not verified** here | Fallback only; database/sql is stable, lib/pq is in maintenance mode |
| [sqlc](https://github.com/sqlc-dev/sqlc) | MIT | [v1.31.1, 2026-04-22](https://github.com/sqlc-dev/sqlc/releases/tag/v1.31.1) | Optional **build-time** SQL-to-Go generation; no runtime ORM; verify generator/target Go compatibility |
| [uptrace/bun](https://github.com/uptrace/bun) | BSD-2-Clause | Current upstream release/date not verified | Not for EX-P03: ORM/model layer and dependencies not needed |
| [ent](https://github.com/ent/ent) | Apache-2.0 | Current upstream release/date not verified | Not for EX-P03: schema/codegen abstraction and ownership overhead |
| [GORM](https://github.com/go-gorm/gorm) | MIT | Current upstream release/date not verified | Not for EX-P03: implicit behavior, migrations and extra abstraction over authoritative ledger |

Only entries with verified dated upstream releases count as activity-verified candidates; ORM entries are explicitly non-selected background comparisons.

## 5. Comparison matrix

| Criterion | goose | migrate | tern | dbmate | Atlas | stdlib in-house |
|---|---|---|---|---|---|---|
| SQL up/down | yes | yes, separate files | yes | yes | versioned; down workflow needs review | implement |
| PG transactional DDL | default tx; opt out | PG driver behavior must verify per version | PG-first | supports PG | supports PG | implement |
| Concurrent runners / locks | verify PG lock behavior at pin | PG advisory lock | advisory lock | --migration-lock (v2.37) | migration locking; verify | must design |
| Go binary / embed | Go API + embed | library + io/fs | Go integration | primarily CLI | primarily CLI | native |
| CI + isolated Compose | yes | yes | yes | yes | yes | yes |
| jsonb handling | SQL passed through | SQL passed through | SQL passed through | SQL passed through | SQL passed through | SQL passed through |
| Go 1.22 | **not established for latest** | **not established for latest** | **not established for latest** | standalone CLI avoids server Go version | standalone CLI avoids server Go version | yes |
| Runtime footprint | configurable driver imports | build tags/driver selection | PG-specific | separate CLI | larger tooling | minimal |
| Lock-in | low (SQL files) | low (SQL files) | low-medium (file conventions) | low | medium (schema workflow/features) | high (custom runner) |
| Supply chain | pin module + checksum | pin module + checksum | pin module + checksum | pin binary/image digest | pin CLI + license tier | less third-party, more self-authored risk |

**PostgreSQL caveat:** Most common DDL is transactional, but `CREATE INDEX CONCURRENTLY` cannot run inside a transaction block. Migration runner must explicitly support opt-out and test failure recovery. `jsonb` is a PG type, not a migration-tool feature. Advisory locks serialize migrators, **not** player/content transactions. For content publication use row locking, uniqueness and transaction semantics separately.

## 6. In-house ~150-line alternative: honest assessment

A ~150-line Go runner using stdlib `database/sql` + `embed` could read numbered SQL, store applied versions, run up migrations in transactions and fail closed. **It still needs a PostgreSQL driver**, so it is not truly dependency-free. Robust implementation additionally needs: inter-process advisory locking on the **same pinned connection**, atomic version bookkeeping, migration checksums/drift detection, down safety, partial/non-transactional failure recovery, cancellation/timeouts, multi-statement handling, idempotent CLI/CI behavior and tests against live PG. That exceeds a safely maintained 150-line toy. Prefer mature OSS unless dependency adoption is disallowed. If no external migration tool approved, scope in-house to strictly transactional up-only bootstrap and defer production down/repair, not pretend parity.

## 7. Recommended architecture and content publication

**Preferred:** goose (SQL-first) + pgx/v5/pgxpool (PG-native) + handwritten parameterized SQL. Add sqlc only when repeated queries warrant codegen. **Fallback:** golang-migrate + pgxpool; **Go 1.22-only contingency:** evaluate version-pinned older pgx/goose after security review, or upgrade Go before adopting current releases. No ORM for EX-P03.

Separate schema migration version from content_build_id. Proposed loader flow (requires director D-001 contract):
1. Verify export manifest SHA-256, schema_version, minimum_client_version, crossrefs and publishable lint; reject ERROR and unapproved WARN.
2. Stage a complete immutable build: insert `content_builds` status staged and all `content_entities` payload jsonb with stable IDs; verify counts, hash, references; use bounded batches and rollback on failure.
3. In a **short separate publication transaction**, lock one authoritative active-build pointer/registry row, check expected previous build, mark new published and previous retired (or retain compatibility metadata), update active pointer and write audit record, then commit. Readers resolve one active build per request/snapshot; never observe a half-published set. Exact pointer schema is a **director contract decision**, not specified in §16.
4. Rollback **content** by atomically switching active pointer to retained compatible build; do not run schema DOWN to undo content. Keep prior build for active quests/dungeons and client handshake; enforce compatibility and prevent mid-instance switching.
5. For EX-P04 quest rewards: short DB transaction, SELECT FOR UPDATE, INSERT reward_claims ON CONFLICT DO NOTHING RETURNING, ledger writes and state revision, commit before websocket delta; retry on serialization conflicts where configured. Never call CDN/network inside transaction.

**Risks:** stale client build, long-running instances, manifest mismatch, concurrent publish, DB restore mismatch, non-transactional DDL, unsafe DOWN data loss, multiple migrators at startup, sqlc generated-code drift, dependency CVEs and transitive build size. Mitigate with staged Compose tests, backup/restore drill, one migration job, immutable build IDs, version retention, checksums, RBAC/audit and pinned hashes.

## 8. Proposed migration directory (names only, no schema SQL)

```text
server/
  migrations/
    00001_content_build_registry.sql
    00002_content_entities.sql
    00003_character_quest_state.sql
    00004_reward_claim_ledger.sql
    00005_dialogue_and_instance_state.sql
```

For goose each file will contain Up/Down sections; these are **illustrative filenames**, not committed migration files. If migrate is chosen instead, each number/name becomes a paired `.up.sql` and `.down.sql`. Adapt to existing repo layout after audit.

## 9. Exact dependencies/versions proposed for Phase 2 (NOT installed)

**Candidate pins for toolchain review, not unconditional install commands:**
- `github.com/pressly/goose/v3@v3.30.0` (latest release observed 2026-11-10; **Go 1.22 compatibility unverified**).
- `github.com/jackc/pgx/v5@v5.13.0` (2026-11-11; **requires newer Go than 1.22**; upgrade approval needed).
- Optional generator `github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1` (CLI; confirm module path/toolchain at tag).
- Fallback `github.com/golang-migrate/migrate/v4@v4.20.1` (verify Go requirement, postgres source/driver imports).
- CI PostgreSQL container: pin the **existing project's major version and immutable digest**, not an unverified image tag.

Before installing: inspect current `server/go.mod` and Go toolchain; inspect each pinned `go.mod`, LICENSE/SPDX, dependency tree, release checksums, GHSA and open issues; record an approved version matrix. If Go 1.22 is immutable, **do not install these latest pins blindly**. Owner/director must choose upgrade or security-reviewed older tags.

## 10. Phase 2 after owner OK only

1. Director publishes D-001 content export + DB migration naming/active pointer contract; resolve Go toolchain/version question.
2. Server lane audits current schema, existing migrations, Compose isolation, backup/restore, and selects pinned tags/checksums; obtains explicit owner dependency OK.
3. Implement migration files and one migration execution path (CLI in isolated CI/Compose; embedded invocation only if needed), ensure no concurrent app auto-migration.
4. Implement staging, validation, atomic publish and pointer rollback in server lane, with authorization and audit.
5. Test clean up/down/up, partial failure, double-run concurrency, checksum drift, backup/restore, content publish failure, concurrent publish, compatible rollback, active-instance pinning and EX-P04 idempotent ledger.
6. Run mandated lane gates and director review; **research lane does not touch server, canon, contracts, tools or roadmap**.

## 11. Awaiting owner's OK / questions

- Approve **goose + pgxpool**, optional later sqlc? Fallback migrate.
- May server Go be upgraded beyond 1.22 to safely use current patched pgx? Otherwise require older-version security review.
- Director D-001 must define content export format and active-build pointer/compatibility semantics.
- Verify license files, latest commits/issues and exact Go requirements for proposed tags before adoption. No dependency has been installed, copied or vendored.

**Explicit final state: installed: nothing; no code, SQL schema, DB, VPS or deployment changed.**
