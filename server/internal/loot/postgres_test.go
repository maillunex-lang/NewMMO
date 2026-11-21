package loot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
)

func TestLootClaimPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for loot integration")
	}
	for _, name := range []string{"NEWMMO_TEST_DATABASE_URL", "NEWMMO_TEST_MIGRATE_DATABASE_URL", "NEWMMO_TEST_APP_DATABASE_URL"} {
		if os.Getenv(name) == "" {
			t.Fatalf("required setting missing: %s", name)
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
	store, err := auth.Open(ctx, auth.Config{DatabaseURL: os.Getenv("NEWMMO_TEST_APP_DATABASE_URL")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	var accountID string
	if err := migrator.QueryRow(ctx, `INSERT INTO public.accounts (login, password_hash) VALUES ($1, 'fixture')
		RETURNING account_id::text`, "loot_"+hex.EncodeToString(rnd[:])).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	var nb [10]byte
	_, _ = rand.Read(nb[:])
	for i := range nb {
		nb[i] = 'a' + nb[i]%26
	}
	charStr, err := store.CreateCharacter(ctx, accountID, string(nb[:]), "CLS-01")
	if err != nil {
		t.Fatal(err)
	}
	characterID, _ := strconv.ParseInt(charStr, 10, 64)

	cat := NewCatalog()
	cat.Put(common)
	cat.Put(elite)
	cat.Put(Table{ID: "LOOT-ENM-01-301", EnemyID: "ENM-01-301", RegionID: "REG-01", Category: "босс DNG", GoldMin: 35, GoldMax: 60, Mat1Pct: 100, Mat3Pct: 70})
	cat.Put(Table{ID: "LOOT-ENM-02-001", EnemyID: "ENM-02-001", RegionID: "REG-02", Category: "обычный", GoldMin: 2, GoldMax: 5, Mat1Pct: 100, Mat3Pct: 0})

	regions := quest.RegionConfig{
		OpenAt:              map[string]time.Time{"REG-02": time.Now().Add(24 * time.Hour)},
		EarlyAccessAccounts: map[string]struct{}{},
	}
	cfg := Config{RegionCoef: map[string]float64{}, MaxPerMinute: 4, Regions: regions}
	// All rolls 0: min gold, every chance hits.
	engine := NewEngine(pool, cat, cfg).WithRoller(&seq{vals: []int64{0}})

	req1 := "550e8400-e29b-41d4-a716-446655440401"
	res, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", req1, map[string]any{"enemy_id": "ENM-01-001"})
	if err != nil || res.Gold != 2 || res.GoldTotal != 2 || len(res.Items) != 2 || len(res.Changes) != 2 || res.InventoryRevision < 2 {
		t.Fatalf("first claim: %#v err=%v", res, err)
	}

	// Replay: same request_id + payload returns the stored result and grants nothing more.
	again, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", req1, map[string]any{"enemy_id": "ENM-01-001"})
	if err != nil || again.GoldTotal != 2 || again.InventoryRevision != res.InventoryRevision {
		t.Fatalf("replay: %#v err=%v", again, err)
	}
	// Same request_id with a different payload is a conflict.
	if _, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", req1, map[string]any{"enemy_id": "ENM-01-101"}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict: %v", err)
	}

	var gold int64
	var ledgerRows int
	if err := pool.QueryRow(ctx, `SELECT gold FROM public.characters WHERE character_id=$1`, characterID).Scan(&gold); err != nil || gold != 2 {
		t.Fatalf("gold after replay=%d err=%v", gold, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.character_loot_ledger WHERE character_id=$1`, characterID).Scan(&ledgerRows); err != nil || ledgerRows != 1 {
		t.Fatalf("ledger rows=%d err=%v", ledgerRows, err)
	}
	var qty int
	if err := pool.QueryRow(ctx, `SELECT quantity FROM public.character_inventory WHERE character_id=$1 AND item_id='MAT-01-001'`, characterID).Scan(&qty); err != nil || qty != 1 {
		t.Fatalf("MAT-01-001 qty=%d err=%v", qty, err)
	}

	// Elite: gold 8, MAT-001 + MAT-003 + special MAT-01-004.
	res2, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", "550e8400-e29b-41d4-a716-446655440402", map[string]any{"enemy_id": "ENM-01-101"})
	if err != nil || res2.Gold != 8 || len(res2.Items) != 3 || res2.GoldTotal != 10 {
		t.Fatalf("elite claim: %#v err=%v", res2, err)
	}

	// Rejections do not touch state.
	for name, c := range map[string]struct {
		enemy string
		want  error
	}{
		"unknown enemy": {"ENM-01-999", ErrNotFound},
		"dungeon boss":  {"ENM-01-301", ErrUnsupported},
		"closed region": {"ENM-02-001", ErrRegionClosed},
	} {
		_, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", "550e8400-e29b-41d4-a716-4466554404"+map[string]string{"unknown enemy": "10", "dungeon boss": "11", "closed region": "12"}[name], map[string]any{"enemy_id": c.enemy})
		if !errors.Is(err, c.want) {
			t.Fatalf("%s: got %v want %v", name, err, c.want)
		}
	}
	if _, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", "not-a-uuid", map[string]any{"enemy_id": "ENM-01-001"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("bad uuid: %v", err)
	}
	if _, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", "550e8400-e29b-41d4-a716-446655440413", map[string]any{"enemy_id": "ENM-01-001", "gold": float64(9999)}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("client-authored extra field must be rejected: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT gold FROM public.characters WHERE character_id=$1`, characterID).Scan(&gold); err != nil || gold != 10 {
		t.Fatalf("gold after rejections=%d err=%v", gold, err)
	}

	// Another account's character cannot be rolled for.
	if _, err := engine.Claim(ctx, "00000000-0000-0000-0000-000000000000", characterID, "combat.loot.request", "550e8400-e29b-41d4-a716-446655440414", map[string]any{"enemy_id": "ENM-01-001"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign account: %v", err)
	}

	// Rate limit: MaxPerMinute=4 accepted claims (2 done) → 2 more succeed, the 5th is limited.
	for i, id := range []string{"550e8400-e29b-41d4-a716-446655440421", "550e8400-e29b-41d4-a716-446655440422"} {
		if _, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", id, map[string]any{"enemy_id": "ENM-01-001"}); err != nil {
			t.Fatalf("claim %d under limit: %v", i, err)
		}
	}
	if _, err := engine.Claim(ctx, accountID, characterID, "combat.loot.request", "550e8400-e29b-41d4-a716-446655440423", map[string]any{"enemy_id": "ENM-01-001"}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate limit: %v", err)
	}

	// Ledger is append-only for the app role.
	if _, err := pool.Exec(ctx, `UPDATE public.character_loot_ledger SET gold = 999999 WHERE character_id=$1`, characterID); err == nil {
		t.Fatal("app role must not update the loot ledger")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.character_loot_ledger WHERE character_id=$1`, characterID); err == nil {
		t.Fatal("app role must not delete from the loot ledger")
	}
}
