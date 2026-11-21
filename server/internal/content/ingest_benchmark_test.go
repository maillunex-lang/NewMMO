package content

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func BenchmarkRegistryIngest(b *testing.B) {
	url, dir := os.Getenv("NEWMMO_TEST_DATABASE_URL"), os.Getenv("NEWMMO_CONTENT_EXPORT_DIR")
	if url == "" || dir == "" {
		b.Skip("benchmark requires NEWMMO_TEST_DATABASE_URL and NEWMMO_CONTENT_EXPORT_DIR")
	}
	snapshot, err := loadBundle(dir)
	if err != nil {
		b.Fatal(err)
	}
	if len(snapshot.entities) < 1000 || len(snapshot.entities) != snapshot.manifest.EntityCount {
		b.Fatalf("bundle has %d entities, manifest says %d", len(snapshot.entities), snapshot.manifest.EntityCount)
	}
	store, err := Open(b.Context(), url, "benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	values := make([][]any, 0, len(snapshot.entities))
	for _, e := range snapshot.entities {
		values = append(values, []any{snapshot.manifest.BuildID, e.ID, e.Kind, e.RegionID, e.Source, e.Line, e.Payload})
	}
	for _, tc := range []struct {
		name   string
		insert func(context.Context, pgx.Tx, [][]any) error
	}{
		{"copy", benchmarkCopy},
		{"insert_batch_250", benchmarkInsertBatch},
	} {
		b.Run(tc.name, func(b *testing.B) {
			ctx := b.Context()
			conn, err := store.pool.Acquire(ctx)
			if err != nil {
				b.Fatal(err)
			}
			defer conn.Release()
			if _, err := conn.Exec(ctx, `CREATE TEMP TABLE benchmark_entities
				(LIKE public.content_entities INCLUDING ALL)`); err != nil {
				b.Fatal(err)
			}
			defer conn.Exec(context.WithoutCancel(ctx), `DROP TABLE pg_temp.benchmark_entities`)

			for b.Loop() {
				tx, err := conn.Begin(ctx)
				if err != nil {
					b.Fatal(err)
				}
				insertErr := tc.insert(ctx, tx, values)
				rollbackErr := tx.Rollback(ctx)
				if insertErr != nil || rollbackErr != nil {
					b.Fatalf("insert: %v; rollback: %v", insertErr, rollbackErr)
				}
			}
			b.ReportMetric(float64(b.N*len(values))/b.Elapsed().Seconds(), "rows/s")
		})
	}
}

func benchmarkCopy(ctx context.Context, tx pgx.Tx, values [][]any) error {
	written, err := tx.CopyFrom(ctx, pgx.Identifier{"pg_temp", "benchmark_entities"}, entityColumns, pgx.CopyFromRows(values))
	if err == nil && written != int64(len(values)) {
		return fmt.Errorf("COPY wrote %d of %d rows", written, len(values))
	}
	return err
}

func benchmarkInsertBatch(ctx context.Context, tx pgx.Tx, values [][]any) error {
	const batchSize = 250

	batch := &pgx.Batch{}
	for start := 0; start < len(values); start += batchSize {
		chunk := values[start:min(start+batchSize, len(values))]
		args := make([]any, 0, len(chunk)*len(entityColumns))
		var sql strings.Builder
		sql.WriteString("INSERT INTO pg_temp.benchmark_entities (")
		sql.WriteString(strings.Join(entityColumns, ","))
		sql.WriteString(") VALUES ")
		for i, row := range chunk {
			if i != 0 {
				sql.WriteByte(',')
			}
			sql.WriteByte('(')
			for j, value := range row {
				if j != 0 {
					sql.WriteByte(',')
				}
				args = append(args, value)
				fmt.Fprintf(&sql, "$%d", len(args))
			}
			sql.WriteByte(')')
		}
		batch.Queue(sql.String(), args...)
	}

	results := tx.SendBatch(ctx, batch)
	defer results.Close()
	var written int64
	for range batch.Len() {
		tag, err := results.Exec()
		if err != nil {
			return err
		}
		written += tag.RowsAffected()
	}
	if written != int64(len(values)) {
		return fmt.Errorf("INSERT wrote %d of %d rows", written, len(values))
	}
	return results.Close()
}
