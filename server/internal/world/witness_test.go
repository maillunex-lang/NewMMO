package world

import (
	"errors"
	"testing"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/damage"
)

func witnessFixture(t *testing.T) (*Sim, *Entity, *Entity) {
	t.Helper()
	cfg := Config{TickRate: 10, PlayerSpeed: 180, Bounds: Rect{0, 0, 760, 560}, InterestRadius: 1000, NoRespawn: true}
	s := New(cfg)
	s.SetCombat(DefaultCombat())
	p := s.entities[s.AddPlayer("7", "Hero", 10, 1000)]
	s.SetPlayerCombat(p.ID, "CLS-01", damage.Stats{MaxHP: 1000})
	boss := s.entities[s.AddEnemy("boss", "ENM-01-301", "Boss", 10, 1000, Vec{420, 260})]
	boss.estats.SP = 100
	boss.ai.state = stChase
	s.addThreat(boss, p.ID, 1)
	catalog := &combatdata.Catalog{Enemies: map[string]combatdata.Enemy{
		"ENM-01-301": {Abilities: map[string]combatdata.Ability{
			"C": {ID: "AB-ENM-01-301-C", HasCoef: true, Coef: 1.8, Scale: "SP", CooldownS: 25},
		}},
		"ENM-01-001": {Name: "Bandit", Level: 1, Category: "обычный"},
	}}
	def := WitnessDef{BossSpawn: "boss", AddEnemy: "ENM-01-001", Crystals: []WitnessCrystal{
		{ID: "first", Position: Vec{280, 75}}, {ID: "second", Position: Vec{475, 75}},
	}}
	if err := s.setupWitness(def, catalog); err != nil {
		t.Fatal(err)
	}
	return s, boss, p
}

func TestWitnessSafeAreasAndServerDamage(t *testing.T) {
	s, boss, safePlayer := witnessFixture(t)
	s.Step()
	if s.witness.deadline != 0 {
		t.Fatal("phase 2 started above 70%")
	}
	boss.HP = 700
	s.Step()
	w := s.witness
	if w.deadline != s.tick+22 || w.ready != s.tick+250 {
		t.Fatalf("cast or cooldown ticks are wrong: %+v", w)
	}
	events := s.DrainEvents()
	if len(events) != 1 || events[0].Type != "witness_telegraph" || len(events[0].SafeAreas) != 2 || events[0].Amount != 2200 {
		t.Fatalf("phase telegraph = %+v", events)
	}
	unsafe := Vec{760, 0}
	if inSafeArea(unsafe, w.safe) {
		unsafe = Vec{760, 560}
	}
	safePlayer.Pos = unsafe
	exposed := s.entities[s.AddPlayer("8", "Other", 10, 1000)]
	exposed.Pos = unsafe
	coverY := 285.0
	if unsafe.Y > 0 {
		coverY = 275
	}
	if !s.Move(safePlayer.ID, 1, Vec{760, coverY}) {
		t.Fatal("could not move to cover")
	}
	for range 21 {
		s.Step()
	}
	if exposed.HP != 1000 {
		t.Fatal("damage landed before the warning elapsed")
	}
	s.Step()
	if safePlayer.HP != 1000 || exposed.HP != 820 {
		t.Fatalf("safe player HP=%d; exposed player HP=%d", safePlayer.HP, exposed.HP)
	}
	adds := 0
	for _, entity := range s.entities {
		if entity.temporary {
			adds++
			s.killEnemy(entity)
		}
	}
	if adds != 2 || len(s.DrainKills()) != 0 {
		t.Fatalf("reinforcement count or room credit is wrong: %d", adds)
	}
	nextCast := w.ready
	for s.tick < nextCast {
		s.Step()
	}
	if w.deadline == 0 || len(s.entities) != 5 {
		t.Fatal("repeat cast did not start or summoned a second wave")
	}
}

func TestWitnessCrystalInterrupt(t *testing.T) {
	s, boss, player := witnessFixture(t)
	w := s.witness
	if err := s.breakWitnessCrystal(player.ID, "first"); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("early crystal: %v", err)
	}
	boss.HP = 500
	s.Step()
	if err := s.breakWitnessCrystal(player.ID, "first"); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("distant crystal: %v", err)
	}
	player.Pos = w.def.Crystals[0].Position
	for range 2 {
		if err := s.breakWitnessCrystal(player.ID, "first"); err != nil {
			t.Fatal(err)
		}
	}
	if w.deadline == 0 {
		t.Fatal("one crystal interrupted the cast")
	}
	if !s.Move(player.ID, 1, w.def.Crystals[1].Position) {
		t.Fatal("could not move toward the second crystal")
	}
	for range 11 {
		s.Step()
	}
	if err := s.breakWitnessCrystal(player.ID, "second"); err != nil {
		t.Fatal(err)
	}
	for range 22 {
		s.Step()
	}
	if w.deadline != 0 || player.HP != 1000 {
		t.Fatal("both crystals must cancel the hit")
	}
	events := s.DrainEvents()
	if len(events) != 4 || events[3].Type != "witness_end" {
		t.Fatalf("unexpected interruption events: %+v", events)
	}
}
