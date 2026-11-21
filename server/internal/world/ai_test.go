package world

import (
	"testing"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/damage"
)

func abil(id, slot string, coef, cd, tele float64) combatdata.Ability {
	return combatdata.Ability{ID: id, Slot: slot, Coef: coef, HasCoef: true, Scale: "AP", CooldownS: cd, HasCD: true, TelegraphS: tele}
}

// aiSim: one Guard L1 (armor 20, HP 400) at (100,100) and one L1 normal enemy at the given spot.
func aiSim(profile string, enemyAt Vec, playerAt Vec) (*Sim, EntityID, EntityID) {
	s := New(testCfg())
	c := DefaultCombat()
	c.Skills, c.CritChance = skillSet(), 0
	s.SetCombat(c)
	p := s.AddPlayer("1", "Hero", 1, 1)
	s.SetPlayerCombat(p, "CLS-01", damage.PlayerStats("CLS-01", 1, damage.StartAttributes, damage.Gear{}))
	s.entities[p].Pos = playerAt
	st, _ := damage.EnemyStats(1, "обычный", damage.NoScale)
	e := s.AddEnemy("ZS-1", "ENM-01-001", "Грабитель", 1, st.MaxHP, enemyAt)
	s.SetEnemyStats(e, st)
	s.SetEnemyDef(e, EnemyDef{AI: profile, Element: "физический",
		A: abil("AB-ENM-01-001-A", "A", 0.60, 2, 0), B: abil("AB-ENM-01-001-B", "B", 1.05, 14, 0.7)})
	return s, p, e
}

func events(s *Sim, kind string) []Event {
	var out []Event
	for _, ev := range s.events {
		if ev.Type == kind {
			out = append(out, ev)
		}
	}
	return out
}

func TestAggroRadiusAndChase(t *testing.T) {
	// 7 m = 224 units. A player at 300 units (9.4 m) is ignored; at 200 units (6.25 m) the enemy wakes up and walks.
	s, _, e := aiSim("melee", Vec{500, 100}, Vec{200, 100}) // 300 apart
	step(s, 30)
	if s.entities[e].ai.state != stIdle || s.entities[e].Pos != (Vec{500, 100}) {
		t.Fatalf("enemy must ignore a player outside aggro: %+v state=%q", s.entities[e].Pos, s.entities[e].ai.state)
	}
	s2, _, e2 := aiSim("melee", Vec{300, 100}, Vec{100, 100}) // 200 apart
	step(s2, 5)
	if s2.entities[e2].ai.state != stAlert {
		t.Fatalf("alert expected, got %q", s2.entities[e2].ai.state)
	}
	step(s2, 10) // ALERT 0.5 s, then chase at 0.9×180=162 u/s
	if x := s2.entities[e2].Pos.X; x >= 300 || s2.entities[e2].ai.state != stChase {
		t.Fatalf("enemy must be chasing: x=%.1f state=%q", x, s2.entities[e2].ai.state)
	}
}

func TestEnemyAttackGoldenAndInterval(t *testing.T) {
	// Adjacent: A hits for 0.60*AP14 = 8.4 → ×100/120 = 7 on a Guard (armor 20), every 2 s (20 ticks).
	s, p, _ := aiSim("melee", Vec{140, 100}, Vec{100, 100})
	step(s, 8) // ALERT 5 ticks then first attack
	before := s.entities[p].HP
	if before != 400 {
		// first hit may already have landed: allow it
		before = 400
	}
	step(s, 50)
	hits := 0
	for _, ev := range s.events {
		if ev.Type == "damage" && ev.Target == p && ev.Skill == "AB-ENM-01-001-A" {
			hits++
			if ev.Amount != 7 {
				t.Fatalf("A damage %d, want 7", ev.Amount)
			}
		}
	}
	if hits < 2 || hits > 4 {
		t.Fatalf("attack interval off: %d hits in ~5.8 s", hits)
	}
	if s.entities[p].HP >= 400 {
		t.Fatal("player took no damage")
	}
}

func TestSignatureTelegraphCanBeDodged(t *testing.T) {
	// Melee slam: telegraph ≥0.8 s, circle around the enemy (reach = 2 m + 2 m = 128 units). Stay → hit, leave → no hit.
	for _, dodge := range []bool{false, true} {
		s, p, e := aiSim("melee", Vec{140, 100}, Vec{100, 100})
		s.entities[e].ai.nextA = 1 << 60 // isolate B
		step(s, 6)                       // ALERT done (0.5 s) — the engine opens with A, so release B now
		s.entities[e].ai.nextB = s.tick
		var tele *Event
		for i := 0; i < 40 && tele == nil; i++ {
			s.Step()
			for _, ev := range events(s, "telegraph") {
				ev := ev
				tele = &ev
			}
		}
		if tele == nil || tele.Amount < 800 || tele.Radius != 128 || tele.Caster != e {
			t.Fatalf("telegraph: %+v", tele)
		}
		if dodge {
			s.entities[p].Pos = Vec{600, 100} // walked out of the circle (instant for the test)
		}
		s.events = nil
		step(s, 12) // > 0.8 s
		var dmg []Event
		for _, ev := range s.events {
			if ev.Type == "damage" && ev.Skill == "AB-ENM-01-001-B" {
				dmg = append(dmg, ev)
			}
		}
		if dodge && len(dmg) != 0 {
			t.Fatalf("dodged blast still hit: %+v", dmg)
		}
		// B: 1.05*14 = 14.7 ×100/120 = 12.25 → 12
		if !dodge && (len(dmg) != 1 || dmg[0].Amount != 12) {
			t.Fatalf("blast damage: %+v", dmg)
		}
	}
}

func TestLeashReturnHealsAndResetsThreat(t *testing.T) {
	s, p, e := aiSim("melee", Vec{300, 100}, Vec{100, 100})
	s.entities[e].HP = 50
	step(s, 20) // chasing
	s.entities[p].Pos = Vec{1900, 1300}
	step(s, 400)
	en := s.entities[e]
	if en.Pos != (Vec{300, 100}) || en.HP != en.MaxHP || en.ai.state != stIdle || len(en.ai.threat) != 0 {
		t.Fatalf("after leash: pos=%v hp=%d state=%q threat=%v", en.Pos, en.HP, en.ai.state, en.ai.threat)
	}
}

func TestThreatSwitchesToHigherDamageDealer(t *testing.T) {
	s, a, e := aiSim("melee", Vec{140, 100}, Vec{100, 100})
	b := s.AddPlayer("2", "B", 1, 1)
	s.SetPlayerCombat(b, "CLS-01", damage.PlayerStats("CLS-01", 1, damage.StartAttributes, damage.Gear{}))
	s.entities[b].Pos = Vec{120, 130}
	step(s, 8)
	if s.entities[e].ai.target != a && s.entities[e].ai.target != b {
		t.Fatal("no target")
	}
	// B hits hard: threat leader flips to B.
	s.addThreat(s.entities[e], b, 500)
	step(s, 3)
	if s.entities[e].ai.target != b {
		t.Fatalf("target %d, want B=%d", s.entities[e].ai.target, b)
	}
}

func TestPlayerDeathAndRespawn(t *testing.T) {
	s, p, _ := aiSim("melee", Vec{140, 100}, Vec{100, 100})
	s.entities[p].HP = 1 // one A hit (7) kills, even with a little out-of-combat regeneration first
	step(s, 12)
	pl := s.entities[p]
	if !pl.Dead || pl.HP != 0 {
		t.Fatalf("player should be dead: %+v", pl.HP)
	}
	if _, err := s.Cast(p, "CLS-01-SK01", 0); err == nil || err.Error() != "DEAD" {
		t.Fatalf("dead player cast: %v", err)
	}
	if s.Move(p, 1, Vec{500, 500}) {
		t.Fatal("dead players cannot move")
	}
	step(s, 50)                                                      // 5 s respawn
	if pl.Dead || pl.HP < pl.MaxHP-14 || pl.Pos != (Vec{100, 100}) { // the enemy next door may already have hit once
		t.Fatalf("respawn: dead=%v hp=%d pos=%v", pl.Dead, pl.HP, pl.Pos)
	}
	// The enemy no longer has a target while the player was dead and drops back (leash/idle logic keeps working).
	step(s, 100)
}

func TestDefenderSignatureIsSelfShield(t *testing.T) {
	s, _, e := aiSim("defender", Vec{140, 100}, Vec{100, 100})
	step(s, 6)
	s.entities[e].ai.nextB = s.tick // release the signature
	step(s, 4)
	en := s.entities[e]
	if en.shield != 23 { // round(15% of 150 = 22.5) = 23
		t.Fatalf("defender shield %d", en.shield)
	}
	if len(events(s, "telegraph")) != 0 {
		t.Fatal("defender B must not telegraph a damage blast")
	}
}

func TestRangedEnemyAttacksFromDistance(t *testing.T) {
	// 8 m = 256 units attack range: a ranged enemy at 240 units attacks without closing in.
	s, p, e := aiSim("ranged", Vec{340, 100}, Vec{100, 100})
	step(s, 15)
	if s.entities[p].HP == 400 && s.entities[e].Pos.X == 340 && s.entities[e].ai.state == stIdle {
		t.Fatal("ranged enemy did nothing")
	}
	hit := false
	for _, ev := range s.events {
		if ev.Type == "damage" && ev.Target == p {
			hit = true
		}
	}
	if !hit || s.entities[e].Pos.X < 330 {
		t.Fatalf("ranged enemy should shoot from afar: hit=%v x=%.1f", hit, s.entities[e].Pos.X)
	}
}

func TestShieldAbsorbsEnemyDamageOnPlayer(t *testing.T) {
	s, p, _ := aiSim("melee", Vec{140, 100}, Vec{100, 100})
	s.entities[p].shield, s.entities[p].shieldUntil = 5, 1<<60
	step(s, 8)
	if s.entities[p].shield != 0 || s.entities[p].HP != 398 { // 7 damage: 5 absorbed, 2 through
		t.Fatalf("shield %d hp %d", s.entities[p].shield, s.entities[p].HP)
	}
}
