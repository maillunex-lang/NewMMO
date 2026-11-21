package content

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBuildNotFound   = errors.New("content build not found")
	ErrAlreadyActive   = errors.New("content build is already active")
	ErrActiveChanged   = errors.New("active build differs from expected previous build")
	ErrBuildState      = errors.New("content build has an invalid status for this operation")
	ErrIncompleteBuild = errors.New("database entity counts do not match the content build")
	ErrInvalidBuildID  = errors.New("build ID must be 64 lowercase hexadecimal characters")
)

type Store struct {
	pool  *pgxpool.Pool
	actor string
}

// Open validates the URL and opens a lazy pool. Stage verifies its files
// before the first database operation.
func Open(ctx context.Context, databaseURL, actor string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("content database URL is required")
	}
	if actor == "" {
		return nil, errors.New("content audit actor is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid content database URL")
	}
	config.MinConns = 0
	config.MinIdleConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, safeDBError("open content database", err)
	}
	return &Store{pool: pool, actor: actor}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Stage(ctx context.Context, dir string) (Summary, error) {
	b, err := loadBundle(dir)
	if err != nil {
		return Summary{}, s.rejectStage(ctx, "", "verification_failed", err)
	}

	err = pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		m := b.manifest
		tag, err := tx.Exec(ctx, `
			INSERT INTO public.content_builds
				(build_id, schema_version, minimum_client_version, manifest_sha256, entity_count, manifest, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'staged')
			ON CONFLICT (build_id) DO NOTHING`,
			m.BuildID, m.SchemaVersion, m.MinimumClientVersion, m.ManifestSHA256, m.EntityCount, b.manifestJSON)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return s.audit(ctx, tx, "stage", m.BuildID, "", map[string]any{"already_exists": true})
		}

		written, err := tx.CopyFrom(ctx, pgx.Identifier{"public", "content_entities"},
			[]string{"build_id", "entity_id", "kind", "region_id", "source_file", "source_line", "payload"},
			pgx.CopyFromSlice(len(b.entities), func(i int) ([]any, error) {
				e := b.entities[i]
				return []any{m.BuildID, e.ID, e.Kind, e.RegionID, e.Source, e.Line, e.Payload}, nil
			}))
		if err != nil {
			return err
		}
		if written != int64(m.EntityCount) {
			return ErrIncompleteBuild
		}
		if err := checkCounts(ctx, tx, m.BuildID, m.EntityCount, m.Counts); err != nil {
			return err
		}
		return s.audit(ctx, tx, "stage", m.BuildID, "", map[string]any{"entity_count": m.EntityCount})
	})
	if err != nil {
		return Summary{}, s.rejectStage(ctx, b.manifest.BuildID, "stage_transaction_failed", safeDBError("stage content", err))
	}
	return b.summary(), nil
}

func (s *Store) rejectStage(ctx context.Context, buildID, reason string, failure error) error {
	// The rejected build has no committed transaction. Audit independently
	// even when the original request was cancelled.
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.audit(auditCtx, s.pool, "reject", buildID, "", map[string]any{"reason": reason}); err != nil {
		return errors.Join(failure, safeDBError("write reject audit", err))
	}
	return failure
}

func (s *Store) Publish(ctx context.Context, buildID, expectedPrevious string) error {
	if expectedPrevious != "" && !hashPattern.MatchString(expectedPrevious) {
		return ErrInvalidBuildID
	}
	return s.switchBuild(ctx, "publish", buildID, expectedPrevious)
}

func (s *Store) Rollback(ctx context.Context, buildID string) error {
	return s.switchBuild(ctx, "rollback", buildID, "")
}

func (s *Store) switchBuild(ctx context.Context, action, buildID, expectedPrevious string) error {
	if !hashPattern.MatchString(buildID) {
		return ErrInvalidBuildID
	}
	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		var current string
		err := tx.QueryRow(ctx, `SELECT build_id FROM public.content_active WHERE id = 1 FOR UPDATE`).Scan(&current)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if current == buildID {
			return ErrAlreadyActive
		}
		if action == "publish" && current != expectedPrevious {
			return ErrActiveChanged
		}

		var targetStatus string
		var entityCount int
		var counts map[string]int
		err = tx.QueryRow(ctx, `
			SELECT status, entity_count, manifest -> 'counts'
			FROM public.content_builds WHERE build_id = $1 FOR NO KEY UPDATE`, buildID).
			Scan(&targetStatus, &entityCount, &counts)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBuildNotFound
		}
		if err != nil {
			return err
		}

		if current == "" {
			// There is no row to lock on the first switch. Competing inserts
			// contend on the singleton key and only the winner may commit.
			tag, err := tx.Exec(ctx, `
				INSERT INTO public.content_active (id, build_id, switched_by)
				VALUES (1, $1, $2) ON CONFLICT (id) DO NOTHING`, buildID, s.actor)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return ErrActiveChanged
			}
		}

		if action == "rollback" {
			if targetStatus != "retired" && targetStatus != "published" {
				return ErrBuildState
			}
			if err := checkCounts(ctx, tx, buildID, entityCount, counts); err != nil {
				return err
			}
		} else if targetStatus != "staged" && targetStatus != "retired" {
			return ErrBuildState
		}

		if current != "" {
			if _, err := tx.Exec(ctx, `UPDATE public.content_builds SET status = 'retired', retired_at = now()
				WHERE build_id = $1`, current); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.content_builds SET status = 'published', published_at = now()
			WHERE build_id = $1`, buildID); err != nil {
			return err
		}
		if current != "" {
			if _, err := tx.Exec(ctx, `UPDATE public.content_active
				SET build_id = $1, previous_build_id = $2, switched_at = now(), switched_by = $3 WHERE id = 1`,
				buildID, current, s.actor); err != nil {
				return err
			}
		}
		return s.audit(ctx, tx, action, buildID, current, map[string]any{})
	})
	return safeDBError(action+" content", err)
}

func checkCounts(ctx context.Context, tx pgx.Tx, buildID string, wantTotal int, want map[string]int) error {
	rows, err := tx.Query(ctx, `SELECT kind, count(*) FROM public.content_entities WHERE build_id = $1 GROUP BY kind`, buildID)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts := make(map[string]int, len(want))
	var total int
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return err
		}
		counts[kind] = count
		total += count
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if total != wantTotal || !maps.Equal(counts, want) {
		return ErrIncompleteBuild
	}
	return nil
}

type auditExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (s *Store) audit(ctx context.Context, db auditExecutor, action, buildID, previous string, details map[string]any) error {
	_, err := db.Exec(ctx, `
		INSERT INTO public.content_audit (action, build_id, previous_build_id, actor, details)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, $5)`, action, buildID, previous, s.actor, details)
	return err
}

func (s *Store) ActiveBuildID(ctx context.Context) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT build_id FROM public.content_active WHERE id = 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, safeDBError("read active build", err)
}

func (s *Store) Entities(ctx context.Context, buildID string) ([]Entity, error) {
	if !hashPattern.MatchString(buildID) {
		return nil, ErrInvalidBuildID
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
	for _, safe := range []error{ErrBuildNotFound, ErrAlreadyActive, ErrActiveChanged, ErrBuildState, ErrIncompleteBuild,
		context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, safe) {
			return fmt.Errorf("%s: %w", op, safe)
		}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return fmt.Errorf("%s: database operation failed (SQLSTATE %s)", op, pgErr.Code)
	}
	return fmt.Errorf("%s: database operation failed", op)
}
