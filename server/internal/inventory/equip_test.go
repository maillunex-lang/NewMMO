package inventory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

func TestEquipUnequipPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for inventory integration")
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

	mkChar := func(class string) (string, int64) {
		var rnd [8]byte
		_, _ = rand.Read(rnd[:])
		var acc string
		if err := migrator.QueryRow(ctx, `INSERT INTO public.accounts (login, password_hash) VALUES ($1, 'fixture')
			RETURNING account_id::text`, "eqp_"+hex.EncodeToString(rnd[:])).Scan(&acc); err != nil {
			t.Fatal(err)
		}
		var nb [10]byte
		_, _ = rand.Read(nb[:])
		for i := range nb {
			nb[i] = 'a' + nb[i]%26
		}
		cs, err := store.CreateCharacter(ctx, acc, string(nb[:]), class)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := strconv.ParseInt(cs, 10, 64)
		return acc, id
	}
	acc, ch := mkChar("CLS-01")
	otherAcc, otherCh := mkChar("CLS-01")
	_ = otherAcc

	n := 0
	give := func(character int64, tpl, rarity string, level, required, bagSlot int) string {
		n++
		id := fmt.Sprintf("7b1d0000-0000-4000-8000-%012d", time.Now().UnixNano()%1e11*10+int64(n))
		if _, err := migrator.Exec(ctx, `INSERT INTO public.character_equipment_items
			(instance_id, character_id, template_id, rarity, item_level, required_level, stats, source)
			VALUES ($1,$2,$3,$4,$5,$6,'{"VIT":10,"DEF":5,"STR":2}','test')`, id, character, tpl, rarity, level, required); err != nil {
			t.Fatal(err)
		}
		if _, err := migrator.Exec(ctx, `INSERT INTO public.character_inventory
			(character_id, container, slot, item_id, quantity, instance_id) VALUES ($1,'bag',$2,$3,1,$4)`,
			character, bagSlot, tpl, id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	helmA := give(ch, "EQ-01-01-HD-01", "UNCOMMON", 8, 5, 0)
	helmB := give(ch, "EQ-01-01-HD-01", "RARE", 8, 5, 1)
	wrong := give(ch, "EQ-02-01-MH-01", "UNCOMMON", 8, 1, 2)
	foreign := give(otherCh, "EQ-01-01-CH-01", "UNCOMMON", 8, 1, 0)

	e := NewEngine(pool)
	reqN := 0
	env := func(typ string, payload map[string]any) Envelope {
		reqN++
		return Envelope{Type: typ, RequestID: fmt.Sprintf("550e8400-e29b-41d4-a716-4466554409%02d", reqN), Payload: payload}
	}
	equip := func(id string) (Delta, error) {
		return e.Equip(ctx, acc, ch, env("inventory.equip.request", map[string]any{"instance_id": id}))
	}

	if _, err := equip(wrong); !errors.Is(err, ErrWrongClass) {
		t.Fatalf("other class gear: %v", err)
	}
	if _, err := equip(helmA); !errors.Is(err, ErrLevelTooLow) {
		t.Fatalf("level 1 wearing a level-5 item: %v", err)
	}
	if _, err := equip(foreign); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another character's item must look absent: %v", err)
	}
	if _, err := e.Equip(ctx, acc, ch, env("inventory.equip.request", map[string]any{"instance_id": helmA, "slot": "HD"})); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("extra payload key: %v", err)
	}
	if _, err := equip("not-a-uuid"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("bad uuid: %v", err)
	}

	// Level up (test fixture writes xp as the migrator role).
	if _, err := migrator.Exec(ctx, `UPDATE public.characters SET xp = 5000 WHERE character_id=$1`, ch); err != nil {
		t.Fatal(err)
	}
	d, err := equip(helmA)
	if err != nil || len(d.Changes) != 1 || d.Changes[0].Quantity != 0 || len(d.Equipped) != 1 || d.Equipped[0].Slot != "HD" ||
		d.Equipped[0].InstanceID != helmA || d.GearScore == nil || *d.GearScore != 17 || d.Items[helmA].Rarity != "UNCOMMON" {
		t.Fatalf("equip: %#v err=%v", d, err)
	}
	// Replay (same request id) returns the stored result and does not move anything.
	again, err := e.Equip(ctx, acc, ch, Envelope{Type: "inventory.equip.request", RequestID: fmt.Sprintf("550e8400-e29b-41d4-a716-4466554409%02d", reqN),
		Payload: map[string]any{"instance_id": helmA}})
	if err != nil || again.InventoryRevision != d.InventoryRevision {
		t.Fatalf("replay: %#v err=%v", again, err)
	}

	// Swap: helmB replaces helmA, helmA returns to helmB's bag slot (1).
	d, err = equip(helmB)
	if err != nil || len(d.Changes) != 1 || d.Changes[0].Slot != 1 || d.Changes[0].InstanceID != helmA || d.Equipped[0].InstanceID != helmB {
		t.Fatalf("swap: %#v err=%v", d, err)
	}

	snap, err := e.Snapshot(ctx, acc, ch)
	if err != nil || len(snap.Equipped) != 1 || snap.Equipped[0].InstanceID != helmB || snap.GearScore != 17 ||
		len(snap.Items) != 3 || len(snap.Bag) != 2 {
		t.Fatalf("snapshot: %#v err=%v", snap, err)
	}

	// Equipped gear cannot be equipped again / is no longer a bag row.
	if _, err := equip(helmB); !errors.Is(err, ErrNotFound) {
		t.Fatalf("re-equip worn item: %v", err)
	}

	// Unequip: bag full ⇒ STATE_CONFLICT and nothing changes; with room ⇒ item returns.
	for i := 3; i < 80; i++ {
		if _, err := migrator.Exec(ctx, `INSERT INTO public.character_inventory (character_id, container, slot, item_id, quantity)
			VALUES ($1,'bag',$2,$3,1)`, ch, i, fmt.Sprintf("ITM-FILL-%02d", i)); err != nil {
			t.Fatal(err)
		}
	}
	// slots 0 (empty), 1 (helmA), 2 (wrong) ⇒ one free slot (0). Take it too.
	if _, err := migrator.Exec(ctx, `INSERT INTO public.character_inventory (character_id, container, slot, item_id, quantity)
		VALUES ($1,'bag',0,'ITM-FILL-00',1)`, ch); err != nil {
		t.Fatal(err)
	}
	unequip := func() (Delta, error) {
		return e.Unequip(ctx, acc, ch, env("inventory.unequip.request", map[string]any{"slot": "HD"}))
	}
	if _, err := unequip(); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("unequip with a full bag: %v", err)
	}
	var worn int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.character_equipped WHERE character_id=$1`, ch).Scan(&worn); err != nil || worn != 1 {
		t.Fatalf("worn after failed unequip = %d err=%v", worn, err)
	}
	if _, err := migrator.Exec(ctx, `DELETE FROM public.character_inventory WHERE character_id=$1 AND slot=40`, ch); err != nil {
		t.Fatal(err)
	}
	d, err = unequip()
	if err != nil || len(d.Changes) != 1 || d.Changes[0].Slot != 40 || d.Changes[0].InstanceID != helmB ||
		len(d.Equipped) != 1 || d.Equipped[0].InstanceID != "" || d.GearScore == nil || *d.GearScore != 0 {
		t.Fatalf("unequip: %#v err=%v", d, err)
	}
	if _, err := unequip(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unequip empty slot: %v", err)
	}
	if _, err := e.Unequip(ctx, acc, ch, env("inventory.unequip.request", map[string]any{"slot": "ZZ"})); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("bad slot: %v", err)
	}
}
