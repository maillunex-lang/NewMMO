package world

import (
	"math"
	"testing"
)

func testCfg() Config {
	return Config{TickRate: 10, PlayerSpeed: 180, Bounds: Rect{0, 0, 2048, 1400}, InterestRadius: 500, PlayerSpawn: Vec{100, 100}}
}

func TestMovementUsesServerSpeedOnly(t *testing.T) {
	s := New(testCfg())
	p := s.AddPlayer("1", "Hero", 1, 400)
	if !s.Move(p, 1, Vec{1000, 100}) {
		t.Fatal("move rejected")
	}
	for i := 0; i < 10; i++ { // 1 second = 180 units at 180 u/s
		s.Step()
	}
	e, _ := s.Entity(p)
	if math.Abs(e.Pos.X-280) > 1e-6 || e.Pos.Y != 100 || !e.Moving {
		t.Fatalf("after 1s: %+v", e.Pos)
	}
	// No matter how far the requested target is, one tick never moves more than speed×dt = 18.
	before := e.Pos
	s.Move(p, 2, Vec{2048, 1400})
	s.Step()
	e, _ = s.Entity(p)
	if d := dist(before, e.Pos); d > 18+1e-6 {
		t.Fatalf("moved %.3f in one tick (cap 18)", d)
	}
}

func TestArrivalStopsAndFacing(t *testing.T) {
	s := New(testCfg())
	p := s.AddPlayer("1", "Hero", 1, 400)
	s.Move(p, 1, Vec{110, 100}) // 10 units east: reached within one tick
	s.Step()
	e, _ := s.Entity(p)
	if e.Pos != (Vec{110, 100}) || e.Moving || e.Facing != 0 {
		t.Fatalf("arrival: %+v moving=%v facing=%d", e.Pos, e.Moving, e.Facing)
	}
	for _, c := range []struct {
		to   Vec
		want int
	}{{Vec{110, 300}, 2}, {Vec{0, 100}, 4}, {Vec{110, 0}, 6}, {Vec{300, 300}, 1}} {
		s := New(testCfg())
		p := s.AddPlayer("1", "H", 1, 1)
		s.Move(p, 1, c.to)
		s.Step()
		e, _ := s.Entity(p)
		start := Vec{100, 100}
		if got := facing(c.to.X-start.X, c.to.Y-start.Y, 2); got != c.want || e.Facing != c.want {
			t.Fatalf("facing to %+v = %d/%d, want %d", c.to, got, e.Facing, c.want)
		}
	}
}

func TestBoundsAndInvalidTargets(t *testing.T) {
	s := New(testCfg())
	p := s.AddPlayer("1", "Hero", 1, 400)
	s.Move(p, 1, Vec{-5000, 99999})
	for i := 0; i < 400; i++ {
		s.Step()
	}
	e, _ := s.Entity(p)
	if e.Pos != (Vec{0, 1400}) {
		t.Fatalf("clamped target: %+v", e.Pos)
	}
	if s.Move(p, 2, Vec{math.NaN(), 5}) || s.Move(p, 2, Vec{math.Inf(1), 5}) {
		t.Fatal("NaN/Inf targets must be rejected")
	}
}

func TestSequenceOrdering(t *testing.T) {
	s := New(testCfg())
	p := s.AddPlayer("1", "Hero", 1, 400)
	if !s.Move(p, 5, Vec{300, 100}) {
		t.Fatal("first move")
	}
	if s.Move(p, 4, Vec{900, 900}) || s.Move(p, 5, Vec{900, 900}) {
		t.Fatal("older/equal seq must be ignored")
	}
	if s.AckSeq(p) != 5 {
		t.Fatalf("ack %d", s.AckSeq(p))
	}
	if !s.Stop(p, 6) {
		t.Fatal("stop")
	}
	s.Step()
	e, _ := s.Entity(p)
	if e.Pos != (Vec{100, 100}) {
		t.Fatalf("stopped player moved: %+v", e.Pos)
	}
}

func TestEnemiesAreStaticAndNotMovable(t *testing.T) {
	s := New(testCfg())
	en := s.AddEnemy("ZS-1", "ENM-01-001", "Грабитель", 1, 150, Vec{300, 300})
	if s.Move(en, 1, Vec{900, 900}) {
		t.Fatal("clients must not move enemies")
	}
	s.Step()
	e, _ := s.Entity(en)
	if e.Pos != (Vec{300, 300}) || e.HP != 150 {
		t.Fatalf("enemy: %+v", e)
	}
}

func TestSnapshotAndInterestDelta(t *testing.T) {
	cfg := testCfg()
	s := New(cfg)
	near := s.AddEnemy("ZS-1", "ENM-01-001", "Near", 1, 150, Vec{300, 100}) // 200 away
	far := s.AddEnemy("ZS-2", "ENM-01-001", "Far", 1, 150, Vec{1500, 1000}) // out of interest
	p := s.AddPlayer("1", "Hero", 1, 400)
	snap, ok := s.SnapshotFor(p)
	if !ok || snap.You != p || snap.TickMS != 100 {
		t.Fatalf("snapshot: %+v", snap)
	}
	ids := map[EntityID]bool{}
	for _, v := range snap.Entities {
		ids[v.ID] = true
	}
	if !ids[p] || !ids[near] || ids[far] {
		t.Fatalf("interest filter: %v", ids)
	}
	// A quiet tick sends nothing.
	s.Step()
	if _, send := s.DeltaFor(p); send {
		t.Fatal("no change, no delta")
	}
	// Walking toward the far enemy brings it into view (upsert) and a far-behind enemy out of view (remove).
	s.Move(p, 1, Vec{1200, 900})
	var sawFar, sawNearRemoved bool
	for i := 0; i < 80; i++ {
		s.Step()
		if d, send := s.DeltaFor(p); send {
			for _, v := range d.Upsert {
				if v.ID == far {
					sawFar = true
				}
			}
			for _, r := range d.Remove {
				if r == near {
					sawNearRemoved = true
				}
			}
			if d.AckSeq != 1 {
				t.Fatalf("ack_seq %d", d.AckSeq)
			}
		}
	}
	if !sawFar || !sawNearRemoved {
		t.Fatalf("far entered=%v near left=%v", sawFar, sawNearRemoved)
	}
}

func TestRemovedPlayerIsAnnouncedToOthers(t *testing.T) {
	s := New(testCfg())
	a := s.AddPlayer("1", "A", 1, 400)
	b := s.AddPlayer("2", "B", 1, 400)
	s.SnapshotFor(a)
	s.SnapshotFor(b)
	s.RemovePlayer(b)
	s.Step()
	d, send := s.DeltaFor(a)
	if !send || len(d.Remove) != 1 || d.Remove[0] != b {
		t.Fatalf("removal delta: %+v send=%v", d, send)
	}
	if s.AddPlayer("1", "A", 1, 400) != a {
		t.Fatal("re-adding the same character must return the same entity")
	}
}

func TestDeterministicTraces(t *testing.T) {
	run := func() []Vec {
		s := New(testCfg())
		p := s.AddPlayer("1", "H", 1, 400)
		var out []Vec
		cmds := []Vec{{500, 400}, {200, 900}, {1500, 300}}
		for i := 0; i < 90; i++ {
			if i%30 == 0 {
				s.Move(p, uint64(i/30+1), cmds[i/30])
			}
			s.Step()
			e, _ := s.Entity(p)
			out = append(out, e.Pos)
		}
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("tick %d differs: %v vs %v", i, a[i], b[i])
		}
	}
}
