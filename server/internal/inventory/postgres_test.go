package inventory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

func TestInventoryGrantMovePostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for inventory integration")
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
		RETURNING account_id::text`, "inv_"+hex.EncodeToString(rnd[:])).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	var nameBytes [10]byte
	_, _ = rand.Read(nameBytes[:])
	for i := range nameBytes {
		nameBytes[i] = 'a' + nameBytes[i]%26
	}
	charIDStr, err := store.CreateCharacter(ctx, accountID, string(nameBytes[:]), "CLS-02")
	if err != nil {
		t.Fatal(err)
	}
	characterID, err := strconv.ParseInt(charIDStr, 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(pool)
	delta, err := engine.GrantMaterial(ctx, accountID, characterID, "MAT-01-001", 5)
	if err != nil || delta.InventoryRevision < 1 || len(delta.Changes) != 1 {
		t.Fatalf("grant: %#v err=%v", delta, err)
	}
	if delta.Changes[0].Container != "materials" || delta.Changes[0].Quantity != 5 {
		t.Fatalf("materials stack: %#v", delta.Changes[0])
	}

	snap, err := engine.Snapshot(ctx, accountID, characterID)
	if err != nil || len(snap.Materials) != 1 || snap.Materials[0].ItemID != "MAT-01-001" {
		t.Fatalf("snapshot: %#v err=%v", snap, err)
	}

	move, err := engine.Move(ctx, accountID, characterID, Envelope{
		Type:      "inventory.move.request",
		RequestID: "550e8400-e29b-41d4-a716-446655440301",
		Payload: map[string]any{
			"from_container": "materials", "from_slot": float64(snap.Materials[0].Slot),
			"to_container": "bag", "to_slot": float64(0), "quantity": float64(2),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	again, err := engine.Move(ctx, accountID, characterID, Envelope{
		Type:      "inventory.move.request",
		RequestID: "550e8400-e29b-41d4-a716-446655440301",
		Payload: map[string]any{
			"from_container": "materials", "from_slot": float64(snap.Materials[0].Slot),
			"to_container": "bag", "to_slot": float64(0), "quantity": float64(2),
		},
	})
	if err != nil || again.InventoryRevision != move.InventoryRevision {
		t.Fatalf("idempotent move: %#v err=%v", again, err)
	}

	snap2, err := engine.Snapshot(ctx, accountID, characterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap2.Bag) != 1 || snap2.Bag[0].Quantity != 2 {
		t.Fatalf("bag after move: %#v", snap2.Bag)
	}
	if len(snap2.Materials) != 1 || snap2.Materials[0].Quantity != 3 {
		t.Fatalf("materials after move: %#v", snap2.Materials)
	}
	// CLS selectable at create (acceptance): character was CLS-02.
	var classID string
	if err := pool.QueryRow(ctx, `SELECT class_id FROM public.characters WHERE character_id=$1`, characterID).Scan(&classID); err != nil || classID != "CLS-02" {
		t.Fatalf("class=%s err=%v", classID, err)
	}
}
