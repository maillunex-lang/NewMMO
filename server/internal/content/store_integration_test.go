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

	"github.com/jackc/pgx/v5/pgconn"
)

func TestContentRegistryPostgres(t *testing.T) {
	url := os.Getenv("NEWMMO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEWMMO_TEST_DATABASE_URL is unset")
	}
	dir := os.Getenv("NEWMMO_CONTENT_EXPORT_DIR")
	if dir == "" {
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

	real, err := loadBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if real.manifest.EntityCount != 4925 {
		t.Fatalf("real export has %d entities; want 4925", real.manifest.EntityCount)
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
	if len(rows) != 4925 || !maps.Equal(counts, real.manifest.Counts) {
		t.Fatalf("read back %d rows with counts %v; want %v", len(rows), counts, real.manifest.Counts)
	}
	var samePayload bool
	if err := store.pool.QueryRow(ctx, `SELECT payload = $3::jsonb FROM public.content_entities
		WHERE build_id = $1 AND entity_id = $2`, realSummary.BuildID, real.entities[0].ID, real.entities[0].Payload).
		Scan(&samePayload); err != nil || !samePayload {
		t.Fatalf("staged payload comparison = %v, %v", samePayload, err)
	}

	if summary := stageExport(t, ctx, store, dir); summary != realSummary {
		t.Fatalf("re-stage changed summary: %+v", summary)
	}
	assertAuditCount(t, ctx, store, `action = 'stage' AND build_id = $2 AND details @> '{"already_exists":true}'`, 1, realSummary.BuildID)

	entities := readFile(t, dir, "entities.jsonl")
	crossrefs := readFile(t, dir, "crossrefs.json")
	for _, file := range []string{"manifest.json", "entities.jsonl", "crossrefs.json"} {
		t.Run("reject_tampered_"+file, func(t *testing.T) {
			corrupt, _ := writeFixture(t, real.manifest, entities, crossrefs)
			raw := readFile(t, corrupt, file)
			if file == "manifest.json" {
				raw = bytes.Replace(raw, []byte(real.manifest.MinimumClientVersion), []byte("0.0.2"), 1)
			} else {
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
		// end of COPY, after the build insert and previous entity rows.
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
		current = racePublish(t, ctx, store, "", realSummary.BuildID, variant("initial"))
		assertActive(t, ctx, store, current)
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
		} {
			_, err := store.pool.Exec(ctx, sql, id)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55000" {
				t.Fatalf("immutability query %s returned %v", sql, err)
			}
		}
	}
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
