package loot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/equipment"
	"github.com/maillunex-lang/NewMMO/server/internal/inventory"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
)

func TestEquipmentDropPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for loot integration")
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
		RETURNING account_id::text`, "eq_"+hex.EncodeToString(rnd[:])).Scan(&accountID); err != nil {
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
	cat.Put(Table{ID: "LOOT-ENM-01-001", EnemyID: "ENM-01-001", RegionID: "REG-01", Category: "обычный",
		GoldMin: 2, GoldMax: 5, Mat1Pct: 0, Mat3Pct: 0, EquipPct: 100, EquipRarity: "RARE"})
	cfg := Config{RegionCoef: map[string]float64{}, MaxPerMinute: 100,
		Regions: quest.RegionConfig{OpenAt: map[string]time.Time{}, EarlyAccessAccounts: map[string]struct{}{}}}
	engine := NewEngine(pool, cat, cfg).WithRoller(&seq{vals: []int64{0}}).WithEquipment(eqCatalog())
	claim := func(id string) (Result, error) {
		return engine.Claim(ctx, accountID, characterID, "combat.loot.request",
			"550e8400-e29b-41d4-a716-4466554407"+id, map[string]any{"enemy_id": "ENM-01-001"})
	}

	res, err := claim("01")
	if err != nil || len(res.Items) != 1 || res.Items[0].InstanceID == "" || res.Items[0].ItemID != "EQ-01-01-MH-01" ||
		res.Items[0].Rarity != "RARE" || res.EquipmentLost || len(res.Changes) != 1 || res.Changes[0].InstanceID != res.Items[0].InstanceID {
		t.Fatalf("equipment claim: %#v err=%v", res, err)
	}
	inst := res.Items[0].InstanceID
	if d, ok := res.GearItems[inst]; !ok || d.Rarity != "RARE" || d.RequiredLevel != 5 || d.Stats["VIT"] == 0 {
		t.Fatalf("gear_items detail missing/wrong: %#v", res.GearItems)
	}
	var budgetSum int
	var stats map[string]int
	var req int
	if err := pool.QueryRow(ctx, `SELECT stats, required_level FROM public.character_equipment_items
		WHERE instance_id=$1 AND character_id=$2`, inst, characterID).Scan(&stats, &req); err != nil {
		t.Fatal(err)
	}
	for _, v := range stats {
		budgetSum += v
	}
	if budgetSum != equipment.StatBudget(8, "RARE", "MH") || req != 5 || stats["VIT"] == 0 || stats["DEF"] == 0 {
		t.Fatalf("stats %v sum=%d required=%d", stats, budgetSum, req)
	}

	// Replay: same request id returns the stored result, creates no second instance.
	if _, err := claim("01"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.character_equipment_items WHERE character_id=$1`, characterID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("instances after replay = %d err=%v", n, err)
	}

	// Two instances of the same template coexist in the bag (no stack merge).
	if _, err := claim("02"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.character_inventory
		WHERE character_id=$1 AND item_id='EQ-01-01-MH-01' AND instance_id IS NOT NULL`, characterID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("bag gear rows = %d err=%v", n, err)
	}

	// Gear can be repositioned inside the bag, never into the materials pocket, never onto an occupied slot.
	inv := inventory.NewEngine(pool)
	mv := func(id string, to string, slot int) (inventory.Delta, error) {
		return inv.Move(ctx, accountID, characterID, inventory.Envelope{Type: "inventory.move.request",
			RequestID: "550e8400-e29b-41d4-a716-4466554408" + id,
			Payload:   map[string]any{"from_container": "bag", "from_slot": 0, "to_container": to, "to_slot": slot}})
	}
	if _, err := mv("01", "materials", 3); err == nil {
		t.Fatal("gear into the materials pocket must fail")
	}
	if _, err := mv("02", "bag", 1); err == nil {
		t.Fatal("gear onto an occupied slot must fail")
	}
	d, err := mv("03", "bag", 5)
	if err != nil || len(d.Changes) != 2 || d.Changes[1].InstanceID == "" || d.Changes[1].Slot != 5 {
		t.Fatalf("gear move: %#v err=%v", d, err)
	}
	snap, err := inv.Snapshot(ctx, accountID, characterID)
	if err != nil || len(snap.Bag) != 2 || snap.Bag[0].Slot != 1 || snap.Bag[1].Slot != 5 || snap.Bag[1].InstanceID != res.Items[0].InstanceID {
		t.Fatalf("snapshot after move: %#v err=%v", snap.Bag, err)
	}
	// Fill the bag: the equipment drop is lost, gold from the same kill is still granted.
	var free []int
	for sl := 0; sl < 80; sl++ {
		if sl != 1 && sl != 5 {
			free = append(free, sl)
		}
	}
	fillSlot := func(i int) int { return free[i] }
	for i := 0; i < 78; i++ {
		if _, err := pool.Exec(ctx, `INSERT INTO public.character_inventory (character_id, container, slot, item_id, quantity)
			VALUES ($1,'bag',$2,$3,1)`, characterID, fillSlot(i), fmt.Sprintf("ITM-FILL-%02d", i)); err != nil {
			t.Fatal(err)
		}
	}
	var goldBefore int64
	_ = pool.QueryRow(ctx, `SELECT gold FROM public.characters WHERE character_id=$1`, characterID).Scan(&goldBefore)
	full, err := claim("03")
	if err != nil || !full.EquipmentLost || len(full.Items) != 0 || full.Gold == 0 || full.GoldTotal != goldBefore+full.Gold {
		t.Fatalf("full bag claim: %#v err=%v", full, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.character_equipment_items WHERE character_id=$1`, characterID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("orphan instance after full bag: %d err=%v", n, err)
	}
}
