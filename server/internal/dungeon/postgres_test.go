package dungeon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/loot"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

type zeroRoller struct{}

func (zeroRoller) Intn(int64) int64 { return 0 }

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func TestDungeonEnginePostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for dungeon integration")
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
		RETURNING account_id::text`, "dng_"+hex.EncodeToString(rnd[:])).Scan(&accountID); err != nil {
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

	defs, err := LoadDir(ResolveDir(""))
	if err != nil {
		t.Fatal(err)
	}
	cat := loot.NewCatalog()
	for enemy, tb := range map[string]loot.Table{
		"ENM-01-001": {ID: "LOOT-ENM-01-001", Category: "обычный"}, "ENM-01-002": {ID: "LOOT-ENM-01-002", Category: "обычный"},
		"ENM-01-003": {ID: "LOOT-ENM-01-003", Category: "обычный"},
		"ENM-01-201": {ID: "LOOT-ENM-01-201", Category: "мини-босс", GoldMin: 14, GoldMax: 25, Mat1Pct: 100, Mat3Pct: 45},
		"ENM-01-301": {ID: "LOOT-ENM-01-301", Category: "босс DNG", GoldMin: 35, GoldMax: 60, Mat1Pct: 100, Mat3Pct: 70},
	} {
		tb.EnemyID, tb.RegionID = enemy, "REG-01"
		if tb.GoldMax == 0 {
			tb.GoldMin, tb.GoldMax = 1, 2
		}
		cat.Put(tb)
	}
	clock := time.Now()
	cfg := Config{Regions: quest.RegionConfig{OpenAt: map[string]time.Time{}, EarlyAccessAccounts: map[string]struct{}{}}}
	engine := NewEngine(pool, defs, cat, cfg).WithRoller(zeroRoller{}).WithClock(func() time.Time { return clock })
	if err := engine.ValidateAgainstContent(); err != nil {
		t.Fatal(err)
	}

	call := func(typ string, payload map[string]any) (any, error) {
		_, res, err := engine.Handle(ctx, accountID, characterID, typ, newUUID(), payload)
		return res, err
	}
	snap := func(typ string, payload map[string]any) Snapshot {
		t.Helper()
		_, res, err := engine.Handle(ctx, accountID, characterID, typ, newUUID(), payload)
		if err != nil {
			t.Fatalf("%s %v: %v", typ, payload, err)
		}
		return res.(Snapshot)
	}

	// Level gate (character starts at level 1; entry_level is 7).
	if _, err := call("dungeon.enter.request", map[string]any{"dungeon_id": "DNG-01-001", "mode": "NORMAL"}); !errors.Is(err, ErrLevelTooLow) {
		t.Fatalf("level gate: %v", err)
	}
	if _, err := migrator.Exec(ctx, `UPDATE public.characters SET xp = 6000 WHERE character_id=$1`, characterID); err != nil {
		t.Fatal(err)
	}
	if _, err := call("dungeon.enter.request", map[string]any{"dungeon_id": "DNG-01-001", "mode": "VETERAN"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("unsupported mode: %v", err)
	}
	s1 := snap("dungeon.enter.request", map[string]any{"dungeon_id": "DNG-01-001", "mode": "NORMAL"})
	if s1.Status != StatusActive || s1.RoomID != "DNG-01-001-R01" {
		t.Fatalf("enter: %#v", s1)
	}
	// Reconnect = enter again: same instance, no second instance.
	s1b := snap("dungeon.enter.request", map[string]any{"dungeon_id": "DNG-01-001", "mode": "NORMAL"})
	if s1b.InstanceID != s1.InstanceID {
		t.Fatalf("reconnect created a new instance: %s vs %s", s1b.InstanceID, s1.InstanceID)
	}
	iid := s1.InstanceID

	// Replay of an identical request_id returns the stored snapshot; a different payload conflicts.
	reqID := newUUID()
	p := map[string]any{"instance_id": iid, "object_id": "OBJ-DNG-01-001-ENTRY"}
	_, r1, err := engine.Handle(ctx, accountID, characterID, "dungeon.interact.request", reqID, p)
	if err != nil {
		t.Fatal(err)
	}
	_, r2, err := engine.Handle(ctx, accountID, characterID, "dungeon.interact.request", reqID, p)
	if err != nil || r2.(map[string]any)["event_seq"].(float64) != float64(r1.(Snapshot).EventSeq) {
		t.Fatalf("replay: %#v err=%v", r2, err)
	}
	if _, _, err := engine.Handle(ctx, accountID, characterID, "dungeon.interact.request", reqID,
		map[string]any{"instance_id": iid, "object_id": "OBJ-DNG-01-001-A"}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("payload conflict: %v", err)
	}
	if _, err := call("dungeon.interact.request", map[string]any{"instance_id": iid, "object_id": "CP-DNG-01-001-02"}); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("object of a later room: %v", err)
	}
	if _, err := call("dungeon.interact.request", map[string]any{"instance_id": newUUID(), "object_id": "OBJ-DNG-01-001-A"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign instance id: %v", err)
	}

	current := r1.(Snapshot)
	if _, err := call("dungeon.encounter.report", map[string]any{"instance_id": iid, "outcome": "enemy_defeated", "spawn_id": "DNG-01-001-R02-E1"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("client combat outcome was accepted: %v", err)
	}
	interact := func(obj string) Snapshot {
		current = snap("dungeon.interact.request", map[string]any{"instance_id": iid, "object_id": obj})
		return current
	}
	combat := func(outcome world.EncounterOutcome) Snapshot {
		t.Helper()
		before := current
		var err error
		current, err = engine.updateEncounter(ctx, accountID, characterID, before, outcome, false)
		if err != nil {
			t.Fatal(err)
		}
		replayed, err := engine.updateEncounter(ctx, accountID, characterID, before, outcome, false)
		if err != nil || replayed.EventSeq != current.EventSeq {
			t.Fatalf("combat replay = %+v, %v; want event seq %d", replayed, err, current.EventSeq)
		}
		return current
	}
	kill := func(spawn string) Snapshot {
		return combat(world.EncounterOutcome{Kills: []world.Kill{{Spawn: spawn}}})
	}
	for _, s := range []string{"DNG-01-001-R02-E1", "DNG-01-001-R02-E2", "DNG-01-001-R02-E3"} {
		kill(s)
	}
	interact("OBJ-DNG-01-001-A")
	for _, o := range []string{"OBJ-DNG-01-001-B2", "OBJ-DNG-01-001-B3"} { // second is wrong → reset
		interact(o)
	}
	for _, o := range []string{"OBJ-DNG-01-001-B2", "OBJ-DNG-01-001-B1", "OBJ-DNG-01-001-B3"} {
		interact(o)
	}
	s := kill("DNG-01-001-R04-MINI")
	if s.RoomID != "DNG-01-001-R05" {
		t.Fatalf("after mini-boss: %s", s.RoomID)
	}
	s = interact("CP-DNG-01-001-02")
	if s.Checkpoint != "DNG-01-001-R05" || s.Status != StatusBoss {
		t.Fatalf("checkpoint: %#v", s)
	}
	// Wipe on the final boss, then retry.
	s = combat(world.EncounterOutcome{PlayerDefeated: true})
	if s.Attempts != 1 || s.RoomID != "DNG-01-001-R06" {
		t.Fatalf("wipe: %#v", s)
	}
	// Claim before completion is refused.
	if _, err := call("dungeon.claim.request", map[string]any{"instance_id": iid}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("early claim: %v", err)
	}
	for _, o := range []string{"OBJ-DNG-01-001-FINAL-1", "OBJ-DNG-01-001-FINAL-2", "OBJ-DNG-01-001-FINAL-3"} {
		interact(o)
	}
	s = kill("DNG-01-001-R06-BOSS")
	if s.Status != StatusCompleted {
		t.Fatalf("not completed: %#v", s)
	}
	// Even after idle time a COMPLETED instance stays claimable.
	clock = clock.Add(30 * time.Minute)

	// Parallel claims: exactly one succeeds, rewards are granted once.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var rewards []Reward
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, res, err := engine.Handle(ctx, accountID, characterID, "dungeon.claim.request", newUUID(), map[string]any{"instance_id": iid})
			if err == nil {
				mu.Lock()
				rewards = append(rewards, res.(Reward))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(rewards) != 1 {
		t.Fatalf("parallel claims succeeded %d times, want exactly 1", len(rewards))
	}
	rw := rewards[0]
	// zero roller: mini 14 + boss 35 gold; first clear adds MAT-01-006.
	if !rw.FirstClear || rw.Gold != 49 || rw.GoldTotal != 49 {
		t.Fatalf("reward: %#v", rw)
	}
	have := map[string]int{}
	for _, it := range rw.Items {
		have[it.ItemID] = it.Quantity
	}
	if have["MAT-01-006"] != 1 || have["MAT-01-001"] != 2 || have["MAT-01-003"] != 2 {
		t.Fatalf("items: %#v", rw.Items)
	}
	var gold int64
	var flags string
	var qty int
	if err := pool.QueryRow(ctx, `SELECT gold, flags::text FROM public.characters WHERE character_id=$1`, characterID).Scan(&gold, &flags); err != nil || gold != 49 {
		t.Fatalf("gold=%d err=%v", gold, err)
	}
	var both bool
	_ = pool.QueryRow(ctx, `SELECT flags ? 'DF-DNG-01-001-FIRST_CLEAR' AND flags ? 'DF-DNG-01-001-NORMAL_CLEAR' FROM public.characters
		WHERE character_id=$1`, characterID).Scan(&both)
	if !both {
		t.Fatalf("clear flags missing: %s", flags)
	}
	if err := pool.QueryRow(ctx, `SELECT quantity FROM public.character_inventory WHERE character_id=$1 AND item_id='MAT-01-006'`, characterID).Scan(&qty); err != nil || qty != 1 {
		t.Fatalf("MAT-01-006=%d err=%v", qty, err)
	}

	// Second run: no first clear any more; a stale instance expires after the idle window.
	s2 := snap("dungeon.enter.request", map[string]any{"dungeon_id": "DNG-01-001", "mode": "NORMAL"})
	if s2.InstanceID == iid {
		t.Fatal("closed instance must not be reused")
	}
	clock = clock.Add(11 * time.Minute)
	s3 := snap("dungeon.enter.request", map[string]any{"dungeon_id": "DNG-01-001", "mode": "NORMAL"})
	if s3.InstanceID == s2.InstanceID {
		t.Fatal("idle instance must expire and be replaced")
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM public.dungeon_instances WHERE instance_id=$1::uuid`, s2.InstanceID).Scan(&status); err != nil || status != StatusExpired {
		t.Fatalf("expired status=%q err=%v", status, err)
	}
	exerciseCombatSession(t, ctx, engine, accountID, characterID, s3.InstanceID)
	// Leave abandons an active instance; a completed one cannot be left unclaimed (checked in unit flow).
	left := snap("dungeon.leave.request", map[string]any{"instance_id": s3.InstanceID})
	if left.Status != StatusAbandoned {
		t.Fatalf("leave: %#v", left)
	}

	// Event log is append-only for the app role; one live instance per character is DB-enforced.
	if _, err := pool.Exec(ctx, `UPDATE public.dungeon_event_log SET event_type='X' WHERE instance_id=$1::uuid`, iid); err == nil {
		t.Fatal("event log must be append-only")
	}
	var clears int
	if err := pool.QueryRow(ctx, `SELECT normal_clear_count FROM public.dungeon_progress WHERE character_id=$1 AND dungeon_id='DNG-01-001'`, characterID).Scan(&clears); err != nil || clears != 1 {
		t.Fatalf("clears=%d err=%v", clears, err)
	}
}

func exerciseCombatSession(t *testing.T, ctx context.Context, engine *Engine, accountID string, characterID int64, instanceID string) {
	t.Helper()
	var pushed Snapshot
	newSession := func() *Session {
		loader := func(context.Context, string, int64) (world.PlayerInfo, error) {
			return world.PlayerInfo{ClassID: "CLS-03", Level: 7}, nil
		}
		s := engine.NewSession(ctx, roomTestCatalog(), loader, accountID, characterID,
			func(string, any) bool { return true }, func(typ string, _ int64, payload any) bool {
				if typ != "dungeon.snapshot" {
					t.Errorf("unexpected critical push: %s", typ)
				}
				pushed = payload.(Snapshot)
				return true
			}, slog.Default())
		s.stop = func() {} // Advance the real zone ticks from the test.
		return s
	}
	call := func(s *Session, typ string, payload map[string]any) Snapshot {
		t.Helper()
		_, result, err := s.Handle(ctx, typ, newUUID(), payload)
		if err != nil {
			t.Fatal(err)
		}
		return result.(Snapshot)
	}
	enter := func(s *Session) Snapshot {
		t.Helper()
		return call(s, "dungeon.enter.request", map[string]any{"dungeon_id": "DNG-01-001", "mode": "NORMAL"})
	}
	s := newSession()
	if snapshot := enter(s); snapshot.InstanceID != instanceID || snapshot.Combat == nil {
		t.Fatalf("live instance did not start combat stream: %+v", snapshot)
	}
	wave := call(s, "dungeon.interact.request", map[string]any{"instance_id": instanceID, "object_id": "OBJ-DNG-01-001-ENTRY"})
	cast := func(s *Session, encounter string, enemy world.EntityID) error {
		t.Helper()
		_, _, handled, err := s.HandleWorld(ctx, "world.cast.request", newUUID(), map[string]any{
			"encounter_id": encounter, "skill_id": "CLS-03-SK01", "target_id": float64(enemy),
		})
		if !handled {
			t.Fatal("dungeon cast escaped to the overworld")
		}
		return err
	}
	tick := func(s *Session, count int) {
		t.Helper()
		for range count {
			s.pending = s.zone.TickEncounter(s.ref)
			if err := s.flushCombat(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	approach := func(s *Session) {
		t.Helper()
		_, _, handled, err := s.HandleWorld(ctx, "world.move.request", "", map[string]any{
			"encounter_id": s.encounter, "seq": float64(1), "x": float64(530), "y": float64(260),
		})
		if !handled || err != nil {
			t.Fatalf("move handled=%v, err=%v", handled, err)
		}
		tick(s, 28)
	}
	approach(s)
	if err := cast(s, wave.Combat.Encounter, wave.Combat.Entities[0].ID); err != nil {
		t.Fatal(err)
	}
	tick(s, 1)
	if !pushed.Rooms[wave.RoomID].Spawns["DNG-01-001-R02-E1"] {
		t.Fatal("server cast did not persist a defeated spawn")
	}
	s.clear()
	s = newSession()
	resumed := enter(s)
	if resumed.RoomID != wave.RoomID || len(resumed.Combat.Entities) != 3 {
		t.Fatalf("reconnect did not restore remaining spawns: %+v", resumed)
	}
	if err := cast(s, wave.Combat.Encounter, resumed.Combat.Entities[0].ID); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("old encounter cast: %v", err)
	}
	call(s, "dungeon.interact.request", map[string]any{"instance_id": instanceID, "object_id": "OBJ-DNG-01-001-A"})
	approach(s)
	for _, enemy := range resumed.Combat.Entities[:2] {
		if err := cast(s, resumed.Combat.Encounter, enemy.ID); err != nil {
			t.Fatal(err)
		}
		tick(s, 8)
	}
	if pushed.RoomID != "DNG-01-001-R03" || pushed.Combat == nil || pushed.Combat.Encounter == resumed.Combat.Encounter {
		t.Fatalf("last server kill did not open a fresh stream for R03: %+v", pushed)
	}
}
