package quest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

func TestQuestHappyPathPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for quest integration")
	}
	for _, name := range []string{"NEWMMO_TEST_DATABASE_URL", "NEWMMO_TEST_MIGRATE_DATABASE_URL", "NEWMMO_TEST_APP_DATABASE_URL"} {
		if os.Getenv(name) == "" {
			t.Fatalf("required quest integration setting is missing: %s", name)
		}
	}

	migrator, err := pgxpool.New(ctx, os.Getenv("NEWMMO_TEST_MIGRATE_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(migrator.Close)

	pool, err := pgxpool.New(ctx, os.Getenv("NEWMMO_TEST_APP_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	cat := NewCatalog()
	path := ResolveInteractablesPath("")
	if err := cat.LoadInteractables(path); err != nil {
		t.Fatal(err)
	}
	buildID, err := cat.ActiveBuildID(ctx, pool)
	if err != nil || buildID == "" {
		t.Fatal("need published content build:", err)
	}
	if err := cat.LoadFromDB(ctx, pool, buildID); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(pool, cat, RegionConfig{})

	store, err := auth.Open(ctx, auth.Config{DatabaseURL: os.Getenv("NEWMMO_TEST_APP_DATABASE_URL")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	var accountID string
	// Fixture via migrator (avoid Register PBKDF2 + zero Limits OFFSET bug). CI DB is ephemeral;
	// append-only reward tables prevent full character cleanup after turn-in.
	if err := migrator.QueryRow(ctx, `INSERT INTO public.accounts (login, password_hash) VALUES ($1, 'fixture')
		RETURNING account_id::text`, "q_"+hex.EncodeToString(rnd[:])).Scan(&accountID); err != nil {
		t.Fatal("account fixture:", err)
	}

	var nameBytes [10]byte
	_, _ = rand.Read(nameBytes[:])
	for i := range nameBytes {
		nameBytes[i] = 'a' + nameBytes[i]%26
	}
	charIDStr, err := store.CreateCharacter(ctx, accountID, string(nameBytes[:]), "CLS-01")
	if err != nil {
		t.Fatal(err)
	}
	characterID, err := strconv.ParseInt(charIDStr, 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	requestIDs := []string{
		"550e8400-e29b-41d4-a716-446655440001",
		"550e8400-e29b-41d4-a716-446655440002",
		"550e8400-e29b-41d4-a716-446655440003",
		"550e8400-e29b-41d4-a716-446655440004",
		"550e8400-e29b-41d4-a716-446655440005",
		"550e8400-e29b-41d4-a716-446655440006",
		"550e8400-e29b-41d4-a716-446655440007",
	}

	closed := NewEngine(pool, cat, RegionConfig{
		OpenAt: map[string]time.Time{"REG-01": time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
	_, err = closed.Accept(ctx, accountID, characterID, Envelope{
		Type: "quest.accept.request", RequestID: requestIDs[6],
		Payload: map[string]any{"quest_id": "MQ-01-01", "content_build_id": buildID},
	})
	if err != ErrRegionClosed {
		t.Fatalf("want REGION_CLOSED got %v", err)
	}

	accept, err := engine.Accept(ctx, accountID, characterID, Envelope{
		Type: "quest.accept.request", RequestID: requestIDs[0],
		Payload: map[string]any{"quest_id": "MQ-01-01", "content_build_id": buildID},
	})
	if err != nil || !accept.OK || accept.State != "ACTIVE" {
		t.Fatalf("accept: %#v err=%v", accept, err)
	}
	again, err := engine.Accept(ctx, accountID, characterID, Envelope{
		Type: "quest.accept.request", RequestID: requestIDs[0],
		Payload: map[string]any{"quest_id": "MQ-01-01", "content_build_id": buildID},
	})
	if err != nil || again.Revision != accept.Revision {
		t.Fatalf("idempotent accept: %#v err=%v", again, err)
	}

	steps := []struct {
		event, target string
	}{
		{"npc_interact", "NPC-802"},
		{"world_interact", "POI-BOARD-01"},
		{"world_interact", "POI-ACCEPT-01"},
	}
	var last ProgressDelta
	for i, step := range steps {
		last, err = engine.Interact(ctx, accountID, characterID, Envelope{
			Type: "world.interact.request", RequestID: requestIDs[i+1],
			Payload: map[string]any{"event": step.event, "target_id": step.target},
		})
		if err != nil {
			t.Fatalf("interact %d: %v", i, err)
		}
	}
	if !last.Ready {
		t.Fatalf("expected ready after 3 objectives, got %#v", last)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		rid := requestIDs[4+i]
		go func(requestID string) {
			defer wg.Done()
			<-start
			_, e := engine.TurnIn(ctx, accountID, characterID, Envelope{
				Type: "quest.turnin.request", RequestID: requestID,
				Payload: map[string]any{"quest_id": "MQ-01-01"},
			})
			results <- e
		}(rid)
	}
	close(start)
	wg.Wait()
	close(results)
	var okCount, conflictCount int
	for e := range results {
		switch {
		case e == nil:
			okCount++
		case e == ErrStateConflict:
			conflictCount++
		default:
			t.Fatalf("unexpected turn-in error: %v", e)
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("concurrent turn-in ok=%d conflict=%d want 1/1", okCount, conflictCount)
	}

	var claims, ledger int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM public.reward_claims
		WHERE character_id=$1 AND source_instance_id='MQ-01-01'`, characterID).Scan(&claims); err != nil || claims != 1 {
		t.Fatalf("claims=%d err=%v", claims, err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM public.character_ledger WHERE character_id=$1`, characterID).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if ledger < 2 {
		t.Fatalf("ledger rows=%d want at least xp+gold", ledger)
	}

	var xp, gold int64
	var rep int
	if err := pool.QueryRow(ctx, `SELECT xp, gold FROM public.characters WHERE character_id=$1`, characterID).Scan(&xp, &gold); err != nil {
		t.Fatal(err)
	}
	if xp != 100 || gold != 12 {
		t.Fatalf("aggregates xp=%d gold=%d", xp, gold)
	}
	if err := pool.QueryRow(ctx, `
		SELECT value FROM public.character_reputation
		WHERE character_id=$1 AND faction_id='FAC-08'`, characterID).Scan(&rep); err != nil || rep != 20 {
		t.Fatalf("reputation=%d err=%v", rep, err)
	}

	snap, err := engine.SnapshotQuests(ctx, accountID, characterID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, id := range snap.Completed {
		if id == "MQ-01-01" {
			found = true
		}
	}
	if !found {
		t.Fatalf("snapshot missing completed MQ-01-01: %#v", snap)
	}
}

func TestLoadRegionConfigEnv(t *testing.T) {
	cfg := LoadRegionConfig(func(k string) string {
		switch k {
		case "NEWMMO_REGIONS_JSON":
			return `{"regions":{"REG-02":{"open_at":"2099-01-01T00:00:00Z"}},"early_access_accounts":["ea-1"]}`
		case "NEWMMO_INTERACTABLES_PATH":
			return "content/interactables/reg-01.json"
		default:
			return ""
		}
	})
	if _, ok := cfg.EarlyAccessAccounts["ea-1"]; !ok {
		t.Fatal("early access missing")
	}
	if cfg.OpenAt["REG-02"].Year() != 2099 {
		t.Fatal(cfg.OpenAt)
	}
}
