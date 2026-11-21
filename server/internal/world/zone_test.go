package world

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
)

func testCatalog(t *testing.T) *combatdata.Catalog {
	t.Helper()
	dir := t.TempDir()
	lines := `{"id":"ENM-01-001","kind":"enemy","payload":{"name":"Трактовый грабитель","level":1,"threat":"обычный","ai":"melee"},"region_id":"REG-01"}
{"id":"AB-ENM-01-001-A","kind":"ability","payload":{"enemy":"ENM-01-001","slot":"A","coefficient":0.6,"scale":"AP","cooldown_s":2,"text":"a"},"region_id":"REG-01"}
{"id":"AB-ENM-01-001-B","kind":"ability","payload":{"enemy":"ENM-01-001","slot":"B","coefficient":1.05,"scale":"AP","cooldown_s":14,"text":"b"},"region_id":"REG-01"}
{"id":"CLS-01-SK01","kind":"skill","payload":{"class_id":"CLS-01","name":"Удар","level":1,"target":"enemy","cost":0,"cooldown_s":5,"effect_raw":"1.10*AP","effect_kind":"damage","coefficient":1.1,"scale":"AP","rule":"Базовый удар; +10 стойкости"},"region_id":null}
`
	if err := os.WriteFile(filepath.Join(dir, "entities.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := combatdata.LoadExport(dir)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func testDef() ZoneDef {
	d := ZoneDef{ZoneID: "ZONE-T", RegionID: "REG-01", TickRate: 10, PlayerSpeed: 180, InterestRadius: 600}
	d.Bounds.MaxX, d.Bounds.MaxY = 2048, 1400
	d.PlayerSpawn.X, d.PlayerSpawn.Y = 100, 100
	d.Enemies = append(d.Enemies, struct {
		SpawnID string  `json:"spawn_id"`
		EnemyID string  `json:"enemy_id"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
	}{"ZS-1", "ENM-01-001", 300, 100})
	return d
}

type sink struct {
	msgs []any
	open bool
}

func (s *sink) push(typ string, payload any) bool {
	if !s.open {
		return false
	}
	s.msgs = append(s.msgs, payload)
	return true
}

func TestZoneJoinMoveDeltaLeave(t *testing.T) {
	z, err := NewZone(testDef(), testCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	owner := new(int)
	s := &sink{open: true}
	snap, err := z.Join("7", owner, PlayerInfo{Name: "Hero", ClassID: "CLS-01", Level: 1}, s.push, nil)
	if err != nil {
		t.Fatal(err)
	}
	var me, enemy *View
	for i := range snap.Entities {
		v := &snap.Entities[i]
		if v.ID == snap.You {
			me = v
		} else if v.Kind == KindEnemy {
			enemy = v
		}
	}
	if me == nil || me.MaxHP != 400 || me.Name != "Hero" || enemy == nil || enemy.MaxHP != 150 || enemy.Ref != "ENM-01-001" {
		t.Fatalf("snapshot views: me=%+v enemy=%+v", me, enemy)
	}
	if err := z.Move("7", owner, 1, Vec{400, 100}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		z.Tick()
	}
	if len(s.msgs) != 3 {
		t.Fatalf("expected a delta per moving tick, got %d", len(s.msgs))
	}
	d := s.msgs[2].(Delta)
	if d.AckSeq != 1 || len(d.Upsert) != 1 || d.Upsert[0].X < 150 {
		t.Fatalf("delta: %+v", d)
	}
	// A different owner (older/newer session) cannot steer the character; leaving needs the right owner.
	if err := z.Move("7", new(int), 2, Vec{0, 0}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("foreign owner: %v", err)
	}
	z.Leave("7", new(int))
	if z.PlayerCount() != 1 {
		t.Fatal("foreign Leave must not remove the player")
	}
	z.Leave("7", owner)
	if z.PlayerCount() != 0 {
		t.Fatal("Leave with the right owner removes the player")
	}
}

func TestZoneDropsSessionWhenPushFails(t *testing.T) {
	z, _ := NewZone(testDef(), testCatalog(t))
	owner := new(int)
	s := &sink{open: true}
	z.Join("7", owner, PlayerInfo{Name: "H", ClassID: "CLS-02", Level: 3}, s.push, nil)
	z.Move("7", owner, 1, Vec{500, 500})
	s.open = false // slow consumer / closed session
	z.Tick()
	if z.PlayerCount() != 0 {
		t.Fatal("a session whose push fails must be dropped")
	}
}

func TestHandlerCommandsAndValidation(t *testing.T) {
	z, _ := NewZone(testDef(), testCatalog(t))
	h := NewHandler([]*Zone{z}, func(ctx context.Context, account string, id int64) (PlayerInfo, error) {
		if id == 404 {
			return PlayerInfo{}, errors.New("missing")
		}
		return PlayerInfo{Name: "Hero", ClassID: "CLS-03", Level: 5}, nil
	})
	owner := new(int)
	s := &sink{open: true}
	ctx := context.Background()
	if _, _, err := h.Handle(ctx, "a", 7, "world.move.request", "", map[string]any{"seq": 1.0, "x": 5.0, "y": 5.0}, owner, s.push, nil); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("move before join: %v", err)
	}
	if _, _, err := h.Handle(ctx, "a", 7, "world.join.request", "", map[string]any{"zone_id": "NOPE"}, owner, s.push, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown zone: %v", err)
	}
	if _, _, err := h.Handle(ctx, "a", 404, "world.join.request", "", map[string]any{"zone_id": "ZONE-T"}, owner, s.push, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown character: %v", err)
	}
	if _, _, err := h.Handle(ctx, "a", 7, "world.join.request", "", map[string]any{"zone_id": "ZONE-T", "x": 1.0}, owner, s.push, nil); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("extra keys: %v", err)
	}
	typ, payload, err := h.Handle(ctx, "a", 7, "world.join.request", "", map[string]any{"zone_id": "ZONE-T"}, owner, s.push, nil)
	if err != nil || typ != "world.snapshot" || payload.(Snapshot).You == 0 {
		t.Fatalf("join: %v %v %v", typ, payload, err)
	}
	bad := []map[string]any{
		{"seq": 0.0, "x": 1.0, "y": 1.0}, {"seq": 1.5, "x": 1.0, "y": 1.0}, {"seq": 1.0, "x": "1", "y": 1.0},
		{"seq": 1.0, "x": 1.0}, {"seq": 1.0, "x": 1.0, "y": 1.0, "speed": 999.0}, {"seq": 1e30, "x": 1.0, "y": 1.0},
	}
	for _, p := range bad {
		if _, _, err := h.Handle(ctx, "a", 7, "world.move.request", "", p, owner, s.push, nil); !errors.Is(err, ErrBadRequest) {
			t.Fatalf("payload %v must be BAD_REQUEST, got %v", p, err)
		}
	}
	if _, _, err := h.Handle(ctx, "a", 7, "world.move.request", "", map[string]any{"seq": 1.0, "x": 900.0, "y": 100.0}, owner, s.push, nil); err != nil {
		t.Fatal(err)
	}
	// Newer session of the same character takes over; the old owner's Disconnect must not remove it.
	newer := new(int)
	s2 := &sink{open: true}
	if _, _, err := h.Handle(ctx, "a", 7, "world.join.request", "", map[string]any{"zone_id": "ZONE-T"}, newer, s2.push, nil); err != nil {
		t.Fatal(err)
	}
	h.Disconnect(7, owner)
	if z.PlayerCount() != 1 {
		t.Fatal("old session's disconnect removed the new session")
	}
	h.Disconnect(7, newer)
	if z.PlayerCount() != 0 {
		t.Fatal("disconnect of the current session must remove the player")
	}
	if _, _, err := h.Handle(ctx, "a", 7, "world.fly.request", "", nil, newer, s.push, nil); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("unknown type: %v", err)
	}
}

func TestRealZoneDefinitionLoads(t *testing.T) {
	defs, err := LoadZoneDefs(ResolveZonesDir(""))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := defs["ZONE-01-DAWN-VALLEY"]
	if !ok || len(d.Enemies) != 7 || d.TickRate != 10 {
		t.Fatalf("zone def: %+v", d)
	}
}

func TestZoneCastIdempotencyEventsAndKillHook(t *testing.T) {
	def := testDef()
	z, _ := NewZone(def, testCatalog(t))
	owner := new(int)
	s := &sink{open: true}
	var critMsgs []any
	crit := func(typ string, rev int64, payload any) bool { critMsgs = append(critMsgs, payload); return true }
	snap, _ := z.Join("7", owner, PlayerInfo{Name: "Hero", ClassID: "CLS-01", Level: 1, AccountID: "acc", Character: 7}, s.push, crit)
	var enemy EntityID
	for _, v := range snap.Entities {
		if v.Kind == KindEnemy {
			enemy = v.ID
		}
	}
	// Enemy at (300,100), player at (100,100): 200 units is out of melee (3.5 m*32 = 112).
	if _, err := z.Cast("7", owner, "r1", "CLS-01-SK01", enemy); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("far cast: %v", err)
	}
	z.Move("7", owner, 1, Vec{250, 100})
	for i := 0; i < 10; i++ {
		z.Tick()
	}
	out, err := z.Cast("7", owner, "r2", "CLS-01-SK01", enemy)
	if err != nil || out.Amount == 0 {
		t.Fatalf("cast: %+v %v", out, err)
	}
	// Same request id again: stored outcome, not applied twice (HP unchanged, no cooldown error).
	again, err := z.Cast("7", owner, "r2", "CLS-01-SK01", enemy)
	if err != nil || again.TargetHP != out.TargetHP {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := z.Cast("7", owner, "r2", "CLS-01-SK01", enemy+1); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("same id, different content: %v", err)
	}
	s.msgs = nil
	z.Tick()
	sawEvent, sawSelf := false, false
	for _, m := range s.msgs {
		switch v := m.(type) {
		case map[string]any:
			if evs, ok := v["events"].([]Event); ok && len(evs) > 0 {
				sawEvent = true
			}
		case Delta:
			if v.Self != nil && v.Self.Cooldowns["CLS-01-SK01"] > 0 {
				sawSelf = true
			}
		}
	}
	if !sawEvent || !sawSelf {
		t.Fatalf("event=%v self-state=%v in %v", sawEvent, sawSelf, s.msgs)
	}
	// Kill hook: kill the enemy directly and expect one asynchronous call with the contributor.
	got := make(chan KillEvent, 1)
	z.SetKillHook(func(k KillEvent) { got <- k })
	z.mu.Lock()
	e := z.sim.entities[enemy]
	e.contrib["7"] = e.MaxHP
	e.HP = 1
	z.mu.Unlock()
	for i := 0; i < 60; i++ {
		z.mu.Lock()
		z.sim.entities[z.sim.byRef["7"]].cooldowns = map[string]uint64{}
		z.mu.Unlock()
		if _, err := z.Cast("7", owner, "k"+string(rune('a'+i%26))+string(rune('a'+i/26)), "CLS-01-SK01", enemy); err == nil {
			break
		}
		z.Tick()
	}
	z.Tick()
	select {
	case k := <-got:
		if k.EnemyID != "ENM-01-001" || len(k.Contributors) != 1 || k.Contributors[0].Character != 7 || k.Contributors[0].AccountID != "acc" || k.Contributors[0].Push == nil {
			t.Fatalf("kill event: %+v", k)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("kill hook not called")
	}
}
