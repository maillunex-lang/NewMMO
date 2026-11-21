package content

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func testGoldenIngestDigest(t *testing.T, ctx context.Context, store *Store, token string) {
	t.Helper()
	const entities = `{"id":"TXT-01","kind":"dialogue","payload":{"text":"Север\u001eюг\u001f\"\\\n","n":7},"region_id":null,"schema_version":1,"source_file":"01_TEST.md","source_line":7}
{"id":"TXT-02","kind":"item","payload":{"text":"Ключ\t«Луна»","ok":true},"region_id":"REG-02","schema_version":1,"source_file":"02_TEST.md","source_line":12}
`
	// Independently derived from the contract in docs/research/T-014-hardening.md.
	const wantDigest = "c5b22a62109e54e095a349d17539878090a821ade7fba6b883801423de2cbba5"
	manifest := fixtureManifest()
	manifest.Counts = map[string]int{"dialogue": 1, "item": 1}
	manifest.MinimumClientVersion = "0.0.1+" + token + ".golden"
	dir, _ := writeFixture(t, manifest, []byte(entities), []byte(fixtureCrossrefs))
	summary := stageExport(t, ctx, store, dir)
	var stored, computed string
	err := store.pool.QueryRow(ctx, `
		SELECT b.ingest_digest, f.ingest_digest
		FROM public.content_builds b
		CROSS JOIN LATERAL public.content_entity_fingerprint(b.build_id) f
		WHERE b.build_id = $1`, summary.BuildID).Scan(&stored, &computed)
	if err != nil {
		t.Fatal(err)
	}
	if stored != wantDigest || computed != wantDigest {
		t.Fatalf("golden digest: stored %s, recomputed %s, want %s", stored, computed, wantDigest)
	}
	t.Logf("golden digest = %s", computed)
}

func testStatusRequiresSingleton(t *testing.T, ctx context.Context, store, owner *Store) {
	t.Helper()
	var buildID, previousBuildID *string
	var switchedAt time.Time
	var switchedBy string
	// This fixture simulates privileged corruption of an isolated test DB.
	// Restore the row before the next test or CI run.
	err := pgx.BeginFunc(ctx, owner.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `ALTER TABLE public.content_active DISABLE TRIGGER content_active_no_delete`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `DELETE FROM public.content_active WHERE id = 1
			RETURNING build_id, previous_build_id, switched_at, switched_by`).
			Scan(&buildID, &previousBuildID, &switchedAt, &switchedBy); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `ALTER TABLE public.content_active ENABLE TRIGGER content_active_no_delete`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, err := owner.pool.Exec(restoreCtx, `
			INSERT INTO public.content_active (id, build_id, previous_build_id, switched_at, switched_by)
			VALUES (1, $1, $2, $3, $4)`, buildID, previousBuildID, switchedAt, switchedBy)
		if err != nil {
			t.Errorf("restore content_active: %v", err)
		}
	})
	if _, err := store.Status(ctx); err == nil || ErrorCode(err) != DBError ||
		!strings.Contains(err.Error(), "content_active singleton is missing") {
		t.Fatalf("Status with absent singleton returned %v", err)
	}
}
