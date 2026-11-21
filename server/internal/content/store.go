package content

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool        *pgxpool.Pool
	actor       string
	auditLogger *slog.Logger
}

// Open validates the URL and opens a lazy pool. Stage verifies its files
// before the first database operation.
func Open(ctx context.Context, databaseURL, actor string) (*Store, error) {
	if databaseURL == "" {
		return nil, failure(DBError, "content database URL is required")
	}
	if actor == "" {
		return nil, failure(DBError, "content audit actor is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, failure(DBError, "invalid content database URL")
	}
	config.MinConns = 0
	config.MinIdleConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, safeDBError("open content database", err)
	}
	return &Store{pool: pool, actor: actor, auditLogger: slog.New(slog.NewJSONHandler(os.Stderr, nil))}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Stage(ctx context.Context, dir string) (Summary, error) {
	b, err := loadBundle(dir)
	if err != nil {
		return Summary{}, s.reject(ctx, "stage", "", err)
	}

	m := b.manifest
	err = s.transact(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, m.BuildID); err != nil {
			return err
		}
		existing, err := lockBuilds(ctx, tx, m.BuildID)
		if err != nil {
			return err
		}
		target, exists := existing[m.BuildID]
		if exists {
			if target.manifestHash != m.ManifestSHA256 {
				return failure(HashMismatch, "existing build manifest digest differs")
			}
			if target.status != "staged" {
				return s.audit(ctx, tx, "stage", m.BuildID, "", map[string]any{"already_exists": true})
			}
			state, err := readStoredEntities(ctx, tx, m.BuildID)
			if err != nil {
				return err
			}
			if state.digest == target.digest && state.matches(m.EntityCount, m.Counts) {
				return s.audit(ctx, tx, "stage", m.BuildID, "", map[string]any{"already_exists": true})
			}
			if _, err := tx.Exec(ctx, `DELETE FROM public.content_entities WHERE build_id = $1`, m.BuildID); err != nil {
				return err
			}
		} else {
			// Store the immutable digest with the first parent INSERT. The
			// deferred FK still verifies that the parent exists at commit.
			if _, err := tx.Exec(ctx, `SET CONSTRAINTS public.content_entities_build_fk DEFERRED`); err != nil {
				return err
			}
		}

		written, err := tx.CopyFrom(ctx, pgx.Identifier{"public", "content_entities"}, entityColumns,
			pgx.CopyFromSlice(len(b.entities), func(i int) ([]any, error) {
				e := b.entities[i]
				return []any{m.BuildID, e.ID, e.Kind, e.RegionID, e.Source, e.Line, e.Payload}, nil
			}))
		if err != nil {
			return err
		}
		state, err := readStoredEntities(ctx, tx, m.BuildID)
		if err != nil {
			return err
		}
		if written != int64(m.EntityCount) || !state.matches(m.EntityCount, m.Counts) {
			return failure(CountMismatch, "stored entity counts differ from the bundle")
		}
		if exists {
			if state.digest != target.digest {
				return failure(HashMismatch, "re-ingested entities differ from the recorded digest")
			}
		} else if _, err := tx.Exec(ctx, `
			INSERT INTO public.content_builds
				(build_id, schema_version, minimum_client_version, manifest_sha256, entity_count, ingest_digest, manifest, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'staged')`,
			m.BuildID, m.SchemaVersion, m.MinimumClientVersion, m.ManifestSHA256, m.EntityCount, state.digest, b.manifestJSON); err != nil {
			return err
		}
		return s.audit(ctx, tx, "stage", m.BuildID, "", map[string]any{"entity_count": m.EntityCount, "reingested": exists})
	})
	if err != nil {
		return Summary{}, s.reject(ctx, "stage", m.BuildID, safeDBError("stage content", err))
	}
	return b.summary(), nil
}

var entityColumns = []string{"build_id", "entity_id", "kind", "region_id", "source_file", "source_line", "payload"}

func (s *Store) reject(ctx context.Context, action, buildID string, original error) error {
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	code := ErrorCode(original)
	details := map[string]any{"code": code, "operation": action}
	if err := s.audit(auditCtx, s.pool, "reject", buildID, "", details); err != nil {
		s.auditLogger.ErrorContext(auditCtx, "content reject audit failed", "operation", action,
			"build_id", buildID, "code", code, "audit_code", DBError)
	}
	return original
}

func (s *Store) Publish(ctx context.Context, buildID, expectedCurrent string) error {
	return s.switchBuild(ctx, "publish", buildID, expectedCurrent)
}

func (s *Store) Rollback(ctx context.Context, buildID, expectedCurrent string) error {
	return s.switchBuild(ctx, "rollback", buildID, expectedCurrent)
}

func (s *Store) switchBuild(ctx context.Context, action, buildID, expectedCurrent string) error {
	if !hashPattern.MatchString(buildID) || (expectedCurrent != "" && !hashPattern.MatchString(expectedCurrent)) {
		return s.reject(ctx, action, "", failure(BundleInvalid, "build IDs must be 64 lowercase hexadecimal characters"))
	}
	err := s.transact(ctx, func(tx pgx.Tx) error {
		var current string
		if err := tx.QueryRow(ctx, `SELECT coalesce(build_id, '') FROM public.content_active WHERE id = 1 FOR UPDATE`).Scan(&current); err != nil {
			return err
		}
		if current != expectedCurrent {
			return failure(Conflict, "active build differs from expected current build")
		}
		if current == buildID {
			return failure(InvalidState, "content build is already active")
		}

		builds, err := lockBuilds(ctx, tx, buildID, current)
		if err != nil {
			return err
		}
		target, ok := builds[buildID]
		if !ok {
			return failure(NotFound, "content build not found")
		}
		if action == "rollback" && target.version != schemaVersion {
			return failure(SchemaUnsupported, "rollback target uses an unsupported schema version")
		}
		if target.status != "retired" && (action == "rollback" || target.status != "staged") {
			return failure(InvalidState, "target build has an invalid status for this operation")
		}

		state, err := readStoredEntities(ctx, tx, buildID)
		if err != nil {
			return err
		}
		if !state.matches(target.count, target.counts) {
			return failure(CountMismatch, "stored entity counts differ from the manifest")
		}
		if state.digest != target.digest {
			return failure(HashMismatch, "stored entity digest differs from the staged build")
		}

		if _, err := tx.Exec(ctx, `UPDATE public.content_builds SET status = 'published', published_at = now()
			WHERE build_id = $1`, buildID); err != nil {
			return err
		}
		if current != "" {
			if _, err := tx.Exec(ctx, `UPDATE public.content_builds SET status = 'retired', retired_at = now()
				WHERE build_id = $1`, current); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.content_active
			SET build_id = $1, previous_build_id = NULLIF($2, ''), switched_at = now(), switched_by = $3 WHERE id = 1`,
			buildID, current, s.actor); err != nil {
			return err
		}
		return s.audit(ctx, tx, action, buildID, current, nil)
	})
	if err != nil {
		return s.reject(ctx, action, buildID, safeDBError(action+" content", err))
	}
	return nil
}

type lockedBuild struct {
	status       string
	manifestHash string
	digest       string
	version      int
	count        int
	counts       map[string]int
}

func lockBuilds(ctx context.Context, tx pgx.Tx, ids ...string) (map[string]lockedBuild, error) {
	rows, err := tx.Query(ctx, `SELECT build_id, status, manifest_sha256, ingest_digest, schema_version,
			entity_count, manifest -> 'counts'
		FROM public.content_builds WHERE build_id = ANY($1) ORDER BY build_id FOR UPDATE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	builds := make(map[string]lockedBuild, len(ids))
	for rows.Next() {
		var id string
		var b lockedBuild
		if err := rows.Scan(&id, &b.status, &b.manifestHash, &b.digest, &b.version, &b.count, &b.counts); err != nil {
			return nil, err
		}
		builds[id] = b
	}
	return builds, rows.Err()
}

type storedEntities struct {
	count  int
	counts map[string]int
	digest string
}

func (s storedEntities) matches(count int, byKind map[string]int) bool {
	return s.count == count && maps.Equal(s.counts, byKind)
}

func readStoredEntities(ctx context.Context, tx pgx.Tx, buildID string) (storedEntities, error) {
	var state storedEntities
	err := tx.QueryRow(ctx, `SELECT entity_count, counts, ingest_digest
		FROM public.content_entity_fingerprint($1)`, buildID).Scan(&state.count, &state.counts, &state.digest)
	return state, err
}

func (s *Store) transact(ctx context.Context, operation func(pgx.Tx) error) error {
	for retry := 0; ; retry++ {
		err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, operation)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "40P01" && pgErr.Code != "40001") {
			return err
		}
		if retry == 3 {
			return failure(Conflict, "transaction retries exhausted")
		}
		delay := time.NewTimer((10 << retry) * time.Millisecond)
		select {
		case <-ctx.Done():
			delay.Stop()
			return ctx.Err()
		case <-delay.C:
		}
	}
}

type auditExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (s *Store) audit(ctx context.Context, db auditExecutor, action, buildID, previous string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	_, err := db.Exec(ctx, `
		INSERT INTO public.content_audit (action, build_id, previous_build_id, actor, details)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, $5)`, action, buildID, previous, s.actor, details)
	return err
}

func (s *Store) ActiveBuildID(ctx context.Context) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT coalesce(build_id, '') FROM public.content_active WHERE id = 1`).Scan(&id)
	return id, safeDBError("read active build", err)
}

func (s *Store) Entities(ctx context.Context, buildID string) ([]Entity, error) {
	if !hashPattern.MatchString(buildID) {
		return nil, failure(BundleInvalid, "build ID must be 64 lowercase hexadecimal characters")
	}
	rows, err := s.pool.Query(ctx, `SELECT entity_id, kind, region_id, source_file, source_line, payload
		FROM public.content_entities WHERE build_id = $1 ORDER BY kind, entity_id`, buildID)
	if err != nil {
		return nil, safeDBError("read entities", err)
	}
	entities, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Entity, error) {
		var e Entity
		err := row.Scan(&e.ID, &e.Kind, &e.RegionID, &e.Source, &e.Line, &e.Payload)
		return e, err
	})
	return entities, safeDBError("read entities", err)
}

type BuildStatus struct {
	Summary
	Status string `json:"status"`
}

type RegistryStatus struct {
	ActiveBuildID   *string       `json:"active_build_id"`
	PreviousBuildID *string       `json:"previous_build_id"`
	Builds          []BuildStatus `json:"builds"`
}

func (s *Store) Status(ctx context.Context) (RegistryStatus, error) {
	result := RegistryStatus{Builds: []BuildStatus{}}
	rows, err := s.pool.Query(ctx, `
		SELECT b.build_id, b.entity_count, b.status, a.build_id, a.previous_build_id
		FROM public.content_builds b
		LEFT JOIN public.content_active a ON a.id = 1 AND a.build_id = b.build_id
		ORDER BY b.created_at, b.build_id`)
	if err != nil {
		return RegistryStatus{}, safeDBError("read content status", err)
	}
	defer rows.Close()
	for rows.Next() {
		var build BuildStatus
		var active, previous *string
		if err := rows.Scan(&build.BuildID, &build.EntityCount, &build.Status, &active, &previous); err != nil {
			return RegistryStatus{}, safeDBError("read content status", err)
		}
		if active != nil {
			result.ActiveBuildID = active
			result.PreviousBuildID = previous
		}
		result.Builds = append(result.Builds, build)
	}
	return result, safeDBError("read content status", rows.Err())
}

// Do not return raw pgx errors to callers that may log them. Connection
// errors can contain parts of the URL, and SQL errors can contain row data.
func safeDBError(op string, err error) error {
	if err == nil {
		return nil
	}
	var known *RegistryError
	if errors.As(err, &known) {
		return known
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return failure(DBError, fmt.Sprintf("%s: database operation failed (SQLSTATE %s)", op, pgErr.Code))
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return failure(DBError, op+": cancelled or deadline exceeded")
	}
	return failure(DBError, op+": database operation failed")
}
