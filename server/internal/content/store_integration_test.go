package content

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestContentRegistryPostgres(t *testing.T) {
	strictRoles := os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") == "1"
	if strictRoles {
		for _, name := range []string{"NEWMMO_TEST_DATABASE_URL", "NEWMMO_TEST_MIGRATE_DATABASE_URL", "NEWMMO_TEST_APP_DATABASE_URL"} {
			if os.Getenv(name) == "" {
				t.Fatalf("%s is required when NEWMMO_REQUIRE_ROLE_URLS=1", name)
			}
		}
	}
	url := os.Getenv("NEWMMO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEWMMO_TEST_DATABASE_URL is unset")
	}
	dir := os.Getenv("NEWMMO_CONTENT_EXPORT_DIR")
	if dir == "" {
		if strictRoles {
			t.Fatal("NEWMMO_CONTENT_EXPORT_DIR is required when NEWMMO_REQUIRE_ROLE_URLS=1")
		}
		t.Skip("NEWMMO_CONTENT_EXPORT_DIR is unset")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	token := rand.Text()
	store, err := Open(ctx, url, "integration-"+token)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner := openOptionalTestStore(t, ctx, "NEWMMO_TEST_MIGRATE_DATABASE_URL", store)
	app := openOptionalTestStore(t, ctx, "NEWMMO_TEST_APP_DATABASE_URL", store)
	if strictRoles {
		assertCurrentUser(t, ctx, store, "newmmo_content_publisher")
		assertCurrentUser(t, ctx, owner, "newmmo_migrator")
		assertCurrentUser(t, ctx, app, "newmmo_app")
	}

	t.Run("transaction_retry_limits", func(t *testing.T) {
		for _, sqlState := range []string{"40P01", "40001"} {
			attempts := 0
			err := store.transact(ctx, func(_ pgx.Tx) error {
				attempts++
				if attempts < 3 {
					return &pgconn.PgError{Code: sqlState}
				}
				return nil
			})
			if err != nil || attempts != 3 {
				t.Fatalf("SQLSTATE %s recovered after %d attempts with %v", sqlState, attempts, err)
			}
		}
		attempts := 0
		err := store.transact(ctx, func(_ pgx.Tx) error {
			attempts++
			return &pgconn.PgError{Code: "40001"}
		})
		if ErrorCode(err) != Conflict || attempts != 4 {
			t.Fatalf("retry limit returned %v after %d attempts", err, attempts)
		}
	})

	real, err := loadBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if real.manifest.EntityCount != 4958 {
		t.Fatalf("real export has %d entities; want 4958", real.manifest.EntityCount)
	}

	original, err := store.ActiveBuildID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	realSummary := stageExport(t, ctx, store, dir)
	if realSummary != real.summary() {
		t.Fatalf("staged summary = %+v, want %+v", realSummary, real.summary())
	}
	rows, err := store.Entities(ctx, realSummary.BuildID)
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	for _, row := range rows {
		counts[row.Kind]++
	}
	if len(rows) != 4958 || !maps.Equal(counts, real.manifest.Counts) {
		t.Fatalf("read back %d rows with counts %v; want %v", len(rows), counts, real.manifest.Counts)
	}
	var samePayload bool
	if err := store.pool.QueryRow(ctx, `SELECT payload = $3::jsonb FROM public.content_entities
		WHERE build_id = $1 AND entity_id = $2`, realSummary.BuildID, real.entities[0].ID, real.entities[0].Payload).
		Scan(&samePayload); err != nil || !samePayload {
		t.Fatalf("staged payload comparison = %v, %v", samePayload, err)
	}

	var previousAuditID int64
	if err := store.pool.QueryRow(ctx, `SELECT max(audit_id) FROM public.content_audit WHERE actor = $1`, store.actor).
		Scan(&previousAuditID); err != nil {
		t.Fatal(err)
	}
	if summary := stageExport(t, ctx, store, dir); summary != realSummary {
		t.Fatalf("re-stage changed summary: %+v", summary)
	}
	assertAuditCount(t, ctx, store, `action = 'stage' AND build_id = $2 AND audit_id > $3 AND details @> '{"already_exists":true}'`, 1, realSummary.BuildID, previousAuditID)

	entities := readFile(t, dir, "entities.jsonl")
	crossrefs := readFile(t, dir, "crossrefs.json")
	for _, file := range []string{"manifest.json", "entities.jsonl", "crossrefs.json"} {
		t.Run("reject_tampered_"+file, func(t *testing.T) {
			corrupt, _ := writeFixture(t, real.manifest, entities, crossrefs)
			raw := readFile(t, corrupt, file)
			switch file {
			case "manifest.json":
				raw = bytes.Replace(raw, []byte(real.manifest.MinimumClientVersion), []byte("0.0.2"), 1)
			case "entities.jsonl":
				raw = bytes.Replace(raw, []byte(`"payload":{`), []byte(`"payload":{"tampered":true,`), 1)
			default:
				raw = append(raw, ' ')
			}
			writeFile(t, corrupt, file, raw)
			if _, err := store.Stage(ctx, corrupt); ErrorCode(err) != HashMismatch {
				t.Fatalf("tampered export returned %v", err)
			}
			assertActive(t, ctx, store, original)
		})
	}
	assertAuditCount(t, ctx, store, `action = 'reject' AND details @> '{"code":"HASH_MISMATCH","operation":"stage"}'`, 3)

	t.Run("copy_failure_leaves_no_build", func(t *testing.T) {
		lines := bytes.Split(bytes.TrimSuffix(entities, []byte("\n")), []byte("\n"))
		last, err := objectFields(lines[len(lines)-1])
		if err != nil {
			t.Fatal(err)
		}
		// PostgreSQL jsonb rejects the JSON null character. Fail at the
		// end of COPY, after processing previous entity rows.
		last["payload"] = json.RawMessage(`{"invalid":"\u0000"}`)
		lines[len(lines)-1], err = json.Marshal(last)
		if err != nil {
			t.Fatal(err)
		}
		corrupt, summary := writeFixture(t, real.manifest, append(bytes.Join(lines, []byte("\n")), '\n'), crossrefs)
		if _, err := Verify(corrupt); err != nil {
			t.Fatalf("fixture must pass preflight: %v", err)
		}
		if _, err := store.Stage(ctx, corrupt); err == nil || !strings.Contains(err.Error(), "SQLSTATE 22P05") {
			t.Fatalf("expected JSONB COPY failure, got %v", err)
		}
		var builds, rows int
		err = store.pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM public.content_builds WHERE build_id = $1),
			(SELECT count(*) FROM public.content_entities WHERE build_id = $1)`, summary.BuildID).Scan(&builds, &rows)
		if builds != 0 || rows != 0 || err != nil {
			t.Fatalf("failed stage left %d builds, %d rows; error %v", builds, rows, err)
		}
		assertAuditCount(t, ctx, store, `action = 'reject' AND build_id = $2 AND details @> '{"code":"DB_ERROR","operation":"stage"}'`, 1, summary.BuildID)
		assertActive(t, ctx, store, original)
	})

	variant := func(name string) string {
		m := real.manifest
		m.MinimumClientVersion = "0.0.1+" + token + "." + name
		path, info := writeFixture(t, m, entities, crossrefs)
		if staged := stageExport(t, ctx, store, path); staged != info {
			t.Fatalf("variant staged summary = %+v, want %+v", staged, info)
		}
		return info.BuildID
	}

	current := original
	if current == "" {
		t.Run("two_first_publishes", func(t *testing.T) {
			current = racePublish(t, ctx, store, "", realSummary.BuildID, variant("initial"))
			assertActive(t, ctx, store, current)
		})
		if current == "" {
			t.Fatal("first-publish test left no active build")
		}
	}

	base := variant("base")
	if err := store.Rollback(ctx, base, current); ErrorCode(err) != InvalidState {
		t.Fatalf("rollback to staged build returned %v", err)
	}
	if err := store.Publish(ctx, base, current); err != nil {
		t.Fatal(err)
	}
	assertActive(t, ctx, store, base)
	if err := store.Publish(ctx, base, base); ErrorCode(err) != InvalidState {
		t.Fatalf("repeat publish returned %v", err)
	}
	assertAuditCount(t, ctx, store, `action = 'publish' AND build_id = $2 AND previous_build_id = $3`, 1, base, current)

	winner := racePublish(t, ctx, store, base, variant("first"), variant("second"))
	assertActive(t, ctx, store, winner)
	assertAuditCount(t, ctx, store, `action = 'publish' AND previous_build_id = $2`, 1, base)
	if err := store.Rollback(ctx, base, current); ErrorCode(err) != Conflict {
		t.Fatalf("rollback with stale expected current returned %v", err)
	}
	if err := store.Rollback(ctx, base, winner); err != nil {
		t.Fatal(err)
	}
	assertActive(t, ctx, store, base)
	assertAuditCount(t, ctx, store, `action = 'rollback' AND build_id = $2 AND previous_build_id = $3`, 1, base, winner)

	status, err := store.Status(ctx)
	if err != nil || status.ActiveBuildID == nil || *status.ActiveBuildID != base ||
		status.PreviousBuildID == nil || *status.PreviousBuildID != winner {
		t.Fatalf("unexpected registry status after rollback: %+v, %v", status, err)
	}
	states := make(map[string]string)
	for _, build := range status.Builds {
		states[build.BuildID] = build.Status
	}
	if states[base] != "published" || states[winner] != "retired" {
		t.Fatalf("rollback left incorrect build states: %v", states)
	}

	for _, id := range []string{base, winner} {
		for _, sql := range []string{
			`UPDATE public.content_entities SET payload = '{}' WHERE build_id = $1`,
			`DELETE FROM public.content_entities WHERE build_id = $1`,
			`INSERT INTO public.content_entities (build_id, entity_id, kind, source_file, source_line, payload)
				VALUES ($1, 'INTEGRATION-TEST', 'class', 'test.md', 1, '{}')`,
			`DELETE FROM public.content_builds WHERE build_id = $1`,
			`UPDATE public.content_builds SET status = 'staged' WHERE build_id = $1`,
			`UPDATE public.content_builds SET entity_count = entity_count + 1 WHERE build_id = $1`,
		} {
			_, err := owner.pool.Exec(ctx, sql, id)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55000" {
				t.Fatalf("immutability query %s returned %v", sql, err)
			}
		}
	}

	t.Run("publish_vs_rollback", func(t *testing.T) {
		next := variant("publish-race")
		results := simultaneous(
			func() error { return store.Publish(ctx, next, base) },
			func() error { return store.Rollback(ctx, winner, base) },
		)
		var published string
		for i, id := range []string{next, winner} {
			if results[i] == nil {
				if published != "" {
					t.Fatal("both pointer switches succeeded")
				}
				published = id
			} else if ErrorCode(results[i]) != Conflict {
				t.Fatalf("pointer race returned %v", results[i])
			}
		}
		if published == "" {
			t.Fatal("neither pointer switch succeeded")
		}
		assertActive(t, ctx, store, published)
		if err := store.Rollback(ctx, base, published); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("tampered_payload_and_stage_vs_publish", func(t *testing.T) {
		m := real.manifest
		m.MinimumClientVersion = "0.0.1+" + token + ".tamper"
		path, summary := writeFixture(t, m, entities, crossrefs)
		stageExport(t, ctx, store, path)
		tag, err := owner.pool.Exec(ctx, `UPDATE public.content_entities SET payload = payload || '{"tampered":true}'
			WHERE build_id = $1 AND entity_id = $2`, summary.BuildID, real.entities[0].ID)
		if err != nil || tag.RowsAffected() != 1 {
			t.Fatalf("could not change one payload: %s %v", tag, err)
		}
		var count int
		err = owner.pool.QueryRow(ctx, `SELECT count(*) FROM public.content_entities WHERE build_id = $1`, summary.BuildID).Scan(&count)
		if err != nil || count != summary.EntityCount {
			t.Fatalf("payload tampering changed the count to %d: %v", count, err)
		}
		if err := store.Publish(ctx, summary.BuildID, base); ErrorCode(err) != HashMismatch {
			t.Fatalf("publish accepted a changed staged payload: %v", err)
		}
		assertActive(t, ctx, store, base)
		assertAuditCount(t, ctx, store, `action = 'reject' AND build_id = $2 AND details @> '{"code":"HASH_MISMATCH","operation":"publish"}'`, 1, summary.BuildID)
		stageExport(t, ctx, store, path)
		assertAuditCount(t, ctx, store, `action = 'stage' AND build_id = $2 AND details @> '{"reingested":true}'`, 1, summary.BuildID)
		results := simultaneous(
			func() error { _, err := store.Stage(ctx, path); return err },
			func() error { return store.Publish(ctx, summary.BuildID, base) },
		)
		for _, err := range results {
			if err != nil {
				t.Fatalf("stage/publish race returned %v", err)
			}
		}
		assertActive(t, ctx, store, summary.BuildID)
		if err := store.Rollback(ctx, base, summary.BuildID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("publisher_direct_transition_checks_integrity", func(t *testing.T) {
		// Soft local mode may use an admin URL. Execute the writes as the
		// publisher even in that mode.
		asPublisher := func(operation func(pgx.Tx) error) error {
			return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, `SET LOCAL ROLE newmmo_content_publisher`); err != nil {
					return err
				}
				return operation(tx)
			})
		}
		stage := func(label string) string {
			m := fixtureManifest()
			m.MinimumClientVersion = "0.0.1+" + token + "." + label
			path, _ := writeFixture(t, m, []byte(fixtureEntities), []byte(fixtureCrossrefs))
			return stageExport(t, ctx, store, path).BuildID
		}
		updateStatus := func(tx pgx.Tx, id string) error {
			_, err := tx.Exec(ctx, `UPDATE public.content_builds SET status = 'published' WHERE build_id = $1`, id)
			return err
		}
		pointTo := func(tx pgx.Tx, id string) error {
			_, err := tx.Exec(ctx, `UPDATE public.content_active SET build_id = $1,
				previous_build_id = build_id, switched_by = 'integration' WHERE id = 1`, id)
			return err
		}

		tampered := stage("publisher-tampered")
		if err := asPublisher(func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, `WITH removed AS (
				DELETE FROM public.content_entities WHERE build_id = $1 AND entity_id = 'CLS-01' RETURNING *
			)
			INSERT INTO public.content_entities (build_id, entity_id, kind, region_id, source_file, source_line, payload)
			SELECT build_id, entity_id, kind, region_id, source_file, source_line, payload || '{"tampered":true}'
			FROM removed`, tampered)
			if err == nil && tag.RowsAffected() != 1 {
				return errors.New("expected one replaced payload")
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		assertSQLState(t, asPublisher(func(tx pgx.Tx) error { return updateStatus(tx, tampered) }), "55000")
		assertSQLState(t, asPublisher(func(tx pgx.Tx) error { return pointTo(tx, tampered) }), "55000")
		assertActive(t, ctx, store, base)

		short := stage("publisher-short")
		if err := asPublisher(func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `DELETE FROM public.content_entities WHERE build_id = $1 AND entity_id = 'CLS-01'`, short)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		assertSQLState(t, asPublisher(func(tx pgx.Tx) error { return updateStatus(tx, short) }), "55000")

		intact := stage("publisher-intact")
		assertSQLState(t, asPublisher(func(tx pgx.Tx) error { return pointTo(tx, intact) }), "55000")
		// A trusted publisher may perform the legal SQL transition. Roll it
		// back, then exercise normal publication and pointer movement.
		probeDone := errors.New("rollback successful direct transition")
		if err := asPublisher(func(tx pgx.Tx) error {
			if err := updateStatus(tx, intact); err != nil {
				return err
			}
			return probeDone
		}); !errors.Is(err, probeDone) {
			t.Fatalf("valid direct transition returned %v", err)
		}
		if err := store.Publish(ctx, intact, base); err != nil {
			t.Fatal(err)
		}
		assertActive(t, ctx, store, intact)
		if err := store.Rollback(ctx, base, intact); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unsupported_rollback_schema", func(t *testing.T) {
		unsupported := digest([]byte(token + "unsupported"))
		if err := pgx.BeginFunc(ctx, owner.pool, func(tx pgx.Tx) error {
			for _, sql := range []string{
				`INSERT INTO public.content_builds (build_id, schema_version, minimum_client_version, manifest_sha256,
					entity_count, ingest_digest, manifest, status)
				SELECT $1, 2, minimum_client_version, $1, entity_count, ingest_digest, manifest, 'staged'
				FROM public.content_builds WHERE build_id = $2`,
				`INSERT INTO public.content_entities (build_id, entity_id, kind, region_id, source_file, source_line, payload)
				SELECT $1, entity_id, kind, region_id, source_file, source_line, payload
				FROM public.content_entities WHERE build_id = $2`,
			} {
				if _, err := tx.Exec(ctx, sql, unsupported, base); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE public.content_builds SET status = 'published' WHERE build_id = $1`, unsupported); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE public.content_builds SET status = 'retired' WHERE build_id = $1`, unsupported)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.Rollback(ctx, unsupported, base); ErrorCode(err) != SchemaUnsupported {
			t.Fatalf("unsupported rollback returned %v", err)
		}
		assertActive(t, ctx, store, base)
	})

	t.Run("reader_permissions_and_database_guards", func(t *testing.T) {
		staged := variant("guard")
		if err := asReader(ctx, app, store, func(tx pgx.Tx) error {
			var readable bool
			if err := tx.QueryRow(ctx, `SELECT current_user = 'newmmo_app' AND
				EXISTS(SELECT 1 FROM public.content_entities WHERE build_id = $1)`, base).Scan(&readable); err != nil {
				return err
			}
			if !readable {
				return errors.New("expected reader role and readable active content")
			}
			return nil
		}); err != nil {
			t.Fatalf("reader cannot select: %v", err)
		}
		for _, sql := range []string{
			`UPDATE public.content_entities SET payload = '{}' WHERE build_id = $1`,
			`DELETE FROM public.content_entities WHERE build_id = $1`,
			`UPDATE public.content_active SET build_id = $1 WHERE id = 1`,
		} {
			err := asReader(ctx, app, store, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, sql, staged)
				return err
			})
			assertSQLState(t, err, "42501")
		}
		err := asReader(ctx, app, store, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `TRUNCATE public.content_entities`)
			return err
		})
		assertSQLState(t, err, "42501")
		_, err = owner.pool.Exec(ctx, `UPDATE public.content_active
			SET build_id = $1, previous_build_id = $2 WHERE id = 1`, staged, base)
		assertSQLState(t, err, "55000")
		for _, sql := range []string{
			`TRUNCATE public.content_builds, public.content_entities, public.content_active CASCADE`,
			`TRUNCATE public.content_entities`,
			`TRUNCATE public.content_active`,
			`TRUNCATE public.content_audit`,
			`UPDATE public.content_audit SET actor = 'rewritten'`,
			`DELETE FROM public.content_audit`,
		} {
			_, err := owner.pool.Exec(ctx, sql)
			assertSQLState(t, err, "55000")
		}
		assertActive(t, ctx, store, base)
	})

	t.Run("golden_ingest_digest", func(t *testing.T) {
		testGoldenIngestDigest(t, ctx, store, token)
	})
	t.Run("status_requires_singleton", func(t *testing.T) {
		testStatusRequiresSingleton(t, ctx, store, owner)
	})

}

func stageExport(t *testing.T, ctx context.Context, store *Store, dir string) Summary {
	t.Helper()
	summary, err := store.Stage(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

func assertActive(t *testing.T, ctx context.Context, store *Store, want string) {
	t.Helper()
	active, err := store.ActiveBuildID(ctx)
	if err != nil || active != want {
		t.Fatalf("active build = %q, %v; want %q", active, err, want)
	}
}

func assertAuditCount(t *testing.T, ctx context.Context, store *Store, predicate string, want int, args ...any) {
	t.Helper()
	var count int
	err := store.pool.QueryRow(ctx, `SELECT count(*) FROM public.content_audit WHERE actor = $1 AND `+predicate,
		append([]any{store.actor}, args...)...).Scan(&count)
	if err != nil || count != want {
		t.Fatalf("audit count for %s = %d, %v; want %d", predicate, count, err, want)
	}
}

func racePublish(t *testing.T, ctx context.Context, store *Store, previous string, ids ...string) string {
	t.Helper()
	type result struct {
		id  string
		err error
	}
	start := make(chan struct{})
	results := make(chan result, len(ids))
	for _, id := range ids {
		go func() {
			<-start
			results <- result{id, store.Publish(ctx, id, previous)}
		}()
	}
	close(start)
	var winner string
	for range ids {
		res := <-results
		if res.err == nil {
			if winner != "" {
				t.Fatalf("both concurrent publishes won: %q and %q", winner, res.id)
			}
			winner = res.id
		} else if ErrorCode(res.err) != Conflict {
			t.Fatalf("unexpected concurrent publish error: %v", res.err)
		}
	}
	if winner == "" {
		t.Fatal("neither concurrent publish won")
	}
	return winner
}

func assertCurrentUser(t *testing.T, ctx context.Context, store *Store, expected string) {
	t.Helper()
	var current string
	if err := store.pool.QueryRow(ctx, `SELECT current_user`).Scan(&current); err != nil {
		t.Fatalf("cannot check current_user for %s: %v", expected, safeDBError("check test database role", err))
	}
	if current != expected {
		t.Fatalf("current_user = %q, want %q", current, expected)
	}
	t.Logf("current_user = %s", current)
}

func openOptionalTestStore(t *testing.T, ctx context.Context, env string, fallback *Store) *Store {
	t.Helper()
	if url := os.Getenv(env); url != "" {
		store, err := Open(ctx, url, fallback.actor)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(store.Close)
		return store
	}
	return fallback
}

func simultaneous(operations ...func() error) []error {
	start := make(chan struct{})
	results := make(chan struct {
		index int
		err   error
	}, len(operations))
	for i, operation := range operations {
		go func() {
			<-start
			results <- struct {
				index int
				err   error
			}{i, operation()}
		}()
	}
	close(start)
	errs := make([]error, len(operations))
	for range operations {
		result := <-results
		errs[result.index] = result.err
	}
	return errs
}

func asReader(ctx context.Context, app, fallback *Store, operation func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, app.pool, func(tx pgx.Tx) error {
		if app == fallback {
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE newmmo_app`); err != nil {
				return err
			}
		}
		return operation(tx)
	})
}

func assertSQLState(t *testing.T, err error, want string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != want {
		t.Fatalf("want SQLSTATE %s, got %v", want, err)
	}
}
