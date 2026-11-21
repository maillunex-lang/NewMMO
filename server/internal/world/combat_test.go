package world

import (
	"errors"
	"testing"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/damage"
)

func skillSet() map[string]combatdata.Skill {
	list := []combatdata.Skill{
		{ID: "CLS-01-SK01", ClassID: "CLS-01", Level: 1, Target: "enemy", Cost: 0, CooldownS: 5, EffectKind: "damage", Coef: 1.10, Scale: "AP", Gain: 10},
		{ID: "CLS-01-SK02", ClassID: "CLS-01", Level: 3, Target: "self", Cost: 20, CooldownS: 18, EffectKind: "shield", Coef: 0.18, Scale: "MaxHP", DurationS: 6},
		{ID: "CLS-01-SK10", ClassID: "CLS-01", Level: 1, Target: "self", Cost: 0, CooldownS: 1, EffectKind: "cleanse", Coef: 1},
		{ID: "CLS-01-SK04", ClassID: "CLS-01", Level: 1, Target: "enemy", Cost: 20, CooldownS: 0, EffectKind: "damage", Coef: 0.85, Scale: "AP"},
		{ID: "CLS-03-SK01", ClassID: "CLS-03", Level: 1, Target: "enemy", Cost: 5, CooldownS: 1, EffectKind: "damage", Coef: 1.0, Scale: "SP"},
		{ID: "CLS-05-SK02", ClassID: "CLS-05", Level: 1, Target: "ally", Cost: 0, CooldownS: 4, EffectKind: "heal", Coef: 1.10, Scale: "SP"},
		{ID: "CLS-02-SK02", ClassID: "CLS-02", Level: 1, Target: "enemy", Cost: 10, CooldownS: 0, EffectKind: "damage", Coef: 1.0, Scale: "AP"},
		{ID: "CLS-02-SK01", ClassID: "CLS-02", Level: 1, Target: "enemy", Cost: 0, CooldownS: 0, EffectKind: "damage", Coef: 1.0, Scale: "AP", Gain: 12},
	}
	m := map[string]combatdata.Skill{}
	for _, s := range list {
		m[s.ID] = s
	}
	return m
}

func combatSim(class string, level int, enemyAt Vec) (*Sim, EntityID, EntityID) {
	s := New(testCfg())
	c := DefaultCombat()
	c.Skills, c.CritChance = skillSet(), 0
	s.SetCombat(c)
	p := s.AddPlayer("1", "Hero", level, 1)
	s.SetPlayerCombat(p, class, damage.PlayerStats(class, level, damage.StartAttributes, damage.Gear{}))
	st, _ := damage.EnemyStats(1, "обычный", damage.NoScale)
	e := s.AddEnemy("ZS-1", "ENM-01-001", "Грабитель", 1, st.MaxHP, enemyAt)
	s.SetEnemyStats(e, st)
	return s, p, e
}

func step(s *Sim, n int) {
	for i := 0; i < n; i++ {
		s.Step()
	}
}

func TestCastDamageGoldenAndEvents(t *testing.T) {
	s, p, e := combatSim("CLS-01", 1, Vec{160, 100}) // 60 units = 1.9 m from spawn (100,100)
	out, err := s.Cast(p, "CLS-01-SK01", e)
	if err != nil {
		t.Fatal(err)
	}
	// 1.10*AP34 vs Armor 10 → 34 (see damage golden); enemy HP 150 → 116.
	if out.Amount != 34 || out.TargetHP != 116 || out.Crit {
		t.Fatalf("outcome: %+v", out)
	}
	evs := s.DrainEvents()
	if len(evs) != 2 || evs[0].Type != "cast" || evs[1].Type != "damage" || evs[1].Amount != 34 {
		t.Fatalf("events: %+v", evs)
	}
	if got := s.EventsFor(p, evs); len(got) != 2 {
		t.Fatalf("caster must see its events: %+v", got)
	}
}

func TestCooldownGCDAndGain(t *testing.T) {
	s, p, e := combatSim("CLS-01", 1, Vec{160, 100})
	if _, err := s.Cast(p, "CLS-01-SK01", e); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cast(p, "CLS-01-SK04", e); !errors.Is(err, ErrGCD) {
		t.Fatalf("second cast in the same tick: %v", err)
	}
	step(s, 8) // GCD 0.75 s = 8 ticks
	if _, err := s.Cast(p, "CLS-01-SK01", e); !errors.Is(err, ErrOnCooldown) {
		t.Fatalf("SK01 cooldown 5 s: %v", err)
	}
	step(s, 42) // total 50 ticks = 5 s
	if _, err := s.Cast(p, "CLS-01-SK01", e); err != nil {
		t.Fatalf("after cooldown: %v", err)
	}
	// Guard starts at 100 (full); the +10 gain is clamped, cost-free skill keeps it at 100.
	if r := s.entities[p].resource; r != 100 {
		t.Fatalf("guard resource %v", r)
	}
}

func TestResourceCostAndRageGain(t *testing.T) {
	// Berserker starts at 0 Rage: SK02 (cost 10) is refused until SK01 (+12) is cast.
	s, p, e := combatSim("CLS-02", 1, Vec{160, 100})
	if _, err := s.Cast(p, "CLS-02-SK02", e); !errors.Is(err, ErrNotEnoughRes) {
		t.Fatalf("no rage: %v", err)
	}
	if _, err := s.Cast(p, "CLS-02-SK01", e); err != nil {
		t.Fatal(err)
	}
	if r := s.entities[p].resource; r != 12 {
		t.Fatalf("rage after SK01: %v", r)
	}
	step(s, 8)
	if _, err := s.Cast(p, "CLS-02-SK02", e); err != nil {
		t.Fatalf("with rage: %v", err)
	}
	if r := s.entities[p].resource; r != 2 {
		t.Fatalf("rage after SK02: %v", r)
	}
	// Out of combat (5 s without actions) rage decays 5/s.
	step(s, 60)
	if r := s.entities[p].resource; r != 0 {
		t.Fatalf("rage should have decayed to 0, got %v", r)
	}
}

func TestRangeAndTargetRules(t *testing.T) {
	s, p, e := combatSim("CLS-01", 1, Vec{400, 100}) // 300 units ≈ 9.4 m: out of melee (3.5 m with slack)
	if _, err := s.Cast(p, "CLS-01-SK01", e); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("melee too far: %v", err)
	}
	if _, err := s.Cast(p, "CLS-01-SK01", 999); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("unknown target: %v", err)
	}
	if _, err := s.Cast(p, "CLS-01-SK01", p); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("player as damage target: %v", err)
	}
	// A mage reaches the same enemy at 8 m (256 units) — set up a mage at the same distance.
	s2, mage, e2 := combatSim("CLS-03", 1, Vec{300, 100}) // 200 units = 6.25 m
	if out, err := s2.Cast(mage, "CLS-03-SK01", e2); err != nil || out.Amount == 0 {
		t.Fatalf("mage ranged cast: %+v %v", out, err)
	}
}

func TestLockedUnsupportedAndWrongClass(t *testing.T) {
	s, p, e := combatSim("CLS-01", 1, Vec{160, 100})
	if _, err := s.Cast(p, "CLS-01-SK02", 0); !errors.Is(err, ErrSkillLocked) {
		t.Fatalf("level 3 skill at level 1: %v", err)
	}
	if _, err := s.Cast(p, "CLS-03-SK01", e); !errors.Is(err, ErrSkillLocked) {
		t.Fatalf("other class skill: %v", err)
	}
	if _, err := s.Cast(p, "CLS-01-SK10", 0); !errors.Is(err, ErrUnsupportedSkill) {
		t.Fatalf("cleanse is not implemented yet: %v", err)
	}
	if _, err := s.Cast(p, "CLS-99-SK01", e); !errors.Is(err, ErrSkillLocked) {
		t.Fatalf("unknown skill: %v", err)
	}
}

func TestShieldAndHeal(t *testing.T) {
	s, p, _ := combatSim("CLS-01", 3, Vec{160, 100})
	out, err := s.Cast(p, "CLS-01-SK02", 0) // 0.18*400... HP at L3 = (200+36+120)*1.25=445 → 80, cap 35% = 155
	if err != nil || out.Amount != 80 || out.Self.Shield != 80 {
		t.Fatalf("shield: %+v %v", out, err)
	}
	step(s, 70) // 6 s duration expires
	if s.entities[p].shield != 0 {
		t.Fatal("shield must expire")
	}
	s2, o, _ := combatSim("CLS-05", 1, Vec{160, 100})
	s2.entities[o].HP = s2.entities[o].MaxHP - 50
	h, err := s2.Cast(o, "CLS-05-SK02", 0)
	if err != nil || h.Amount == 0 || h.Amount > 50 || s2.entities[o].HP != s2.entities[o].MaxHP-50+h.Amount {
		t.Fatalf("heal: %+v %v", h, err)
	}
}

func TestDeathContributionLootEligibilityAndRespawn(t *testing.T) {
	s := New(testCfg())
	c := DefaultCombat()
	c.Skills, c.CritChance = skillSet(), 0
	s.SetCombat(c)
	mk := func(ref string) EntityID {
		id := s.AddPlayer(ref, ref, 1, 1)
		s.SetPlayerCombat(id, "CLS-02", damage.PlayerStats("CLS-02", 1, damage.StartAttributes, damage.Gear{}))
		return id
	}
	a, b := mk("1"), mk("2")
	st, _ := damage.EnemyStats(1, "обычный", damage.NoScale) // HP 150
	e := s.AddEnemy("ZS-1", "ENM-01-001", "X", 1, st.MaxHP, Vec{160, 100})
	s.SetEnemyStats(e, st)
	// B deals one hit (below the 10% threshold of 15 HP? one hit is ≥15) → make B too weak by lowering its AP.
	s.entities[b].pstats.AP = 2
	if _, err := s.Cast(b, "CLS-02-SK01", e); err != nil { // 1.0*2 → 1 damage < 15
		t.Fatal(err)
	}
	var killed bool
	for i := 0; i < 60 && !killed; i++ {
		if _, err := s.Cast(a, "CLS-02-SK01", e); err == nil {
			if s.entities[e].Dead {
				killed = true
			}
		}
		s.Step()
	}
	if !killed {
		t.Fatal("enemy never died")
	}
	kills := s.DrainKills()
	if len(kills) != 1 || kills[0].EnemyID != "ENM-01-001" || len(kills[0].Contributors) != 1 || kills[0].Contributors[0].Ref != "1" {
		t.Fatalf("kills: %+v", kills)
	}
	step(s, 10) // clear the global cooldown first
	if _, err := s.Cast(a, "CLS-02-SK01", e); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("dead target: %v", err)
	}
	// Respawn after RespawnS (30 s = 300 ticks) at full HP and home position.
	step(s, 300)
	en := s.entities[e]
	if en.Dead || en.HP != en.MaxHP || en.Pos != (Vec{160, 100}) {
		t.Fatalf("respawn: %+v", en)
	}
}

func TestDeadPlayerCannotCastAndCritIsReproducible(t *testing.T) {
	s, p, e := combatSim("CLS-01", 1, Vec{160, 100})
	s.entities[p].Dead, s.entities[p].HP = true, 0
	if _, err := s.Cast(p, "CLS-01-SK01", e); !errors.Is(err, ErrDead) {
		t.Fatalf("dead caster: %v", err)
	}
	s2, p2, e2 := combatSim("CLS-01", 1, Vec{160, 100})
	s2.combat.CritChance = 1 // 100%, clamped to the PvE cap of 35%
	crits := 0
	for i := 0; i < 400; i++ {
		s2.entities[p2].cooldowns = map[string]uint64{}
		s2.entities[p2].gcdUntil = 0
		s2.entities[e2].Dead, s2.entities[e2].HP = false, 150
		o, err := s2.Cast(p2, "CLS-01-SK01", e2)
		if err != nil {
			t.Fatal(err)
		}
		if o.Crit {
			crits++
			if o.Amount != 51 { // round(34*1.5)
				t.Fatalf("crit damage %d", o.Amount)
			}
		}
	}
	if crits < 100 || crits > 180 { // ~35% of 400 = 140
		t.Fatalf("crit rate off: %d/400 (cap 35%%)", crits)
	}
}

func TestCastPublishesStationaryEnemyDeathOnNextTick(t *testing.T) {
	s, p, e := combatSim("CLS-01", 1, Vec{160, 100})
	s.entities[e].HP = 20
	s.SnapshotFor(p)
	if _, err := s.Cast(p, "CLS-01-SK01", e); err != nil {
		t.Fatal(err)
	}
	s.Step()
	delta, _ := s.DeltaFor(p)
	for _, view := range delta.Upsert {
		if view.ID == e && view.HP == 0 && view.Dead {
			return
		}
	}
	t.Fatalf("delta missed the death of a stationary enemy: %+v", delta)
}

func TestPlayerRegeneratesHPOnlyOutOfCombat(t *testing.T) {
	s, id, _ := combatSim("CLS-01", 1, Vec{1500, 100}) // enemy far away: no aggro
	p := s.entities[id]
	p.HP = p.MaxHP / 2
	p.combatUntil = s.tick + 1000 // in combat: no regeneration
	before := p.HP
	step(s, 50)
	if p.HP != before {
		t.Fatalf("HP changed in combat: %d -> %d", before, p.HP)
	}
	p.combatUntil = 0
	step(s, 100) // 10 s at 10 Hz, 2%/s of MaxHP
	gain := p.HP - before
	want := int(float64(p.MaxHP) * 0.02 * 10)
	if gain < want-2 || gain > want+2 {
		t.Fatalf("regen gained %d HP in 10 s, want about %d", gain, want)
	}
}
