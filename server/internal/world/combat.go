package world

import (
	"errors"
	"math"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/damage"
)

// Combat holds canon data and interim tunables for the combat rules.
type Combat struct {
	Skills        map[string]combatdata.Skill // by skill id
	UnitsPerMeter float64                     // world units per metre (interim: 32, see C-RANGE-001)
	MeleeRangeM   float64                     // CLS-01/02 single-target skills
	RangedRangeM  float64                     // CLS-03/04/05
	RangeSlackM   float64                     // latency tolerance on range checks
	RespawnS      float64                     // enemy respawn delay (interim, canon says 45–90 s per spawn)
	CombatLeaveS  float64                     // seconds without combat actions before regen switches to out-of-combat
	MinContribHP  float64                     // fraction of enemy MaxHP a player must deal to earn loot
	CritChance    float64                     // base crit chance (canon 5%)
}

func NewCombat(cat *combatdata.Catalog) *Combat {
	c := &Combat{Skills: map[string]combatdata.Skill{}, UnitsPerMeter: 32, MeleeRangeM: 2.5, RangedRangeM: 9,
		RangeSlackM: 1.0, RespawnS: 30, CombatLeaveS: 5, MinContribHP: 0.10, CritChance: damage.CritBase}
	for _, list := range cat.Skills {
		for _, s := range list {
			c.Skills[s.ID] = s
		}
	}
	return c
}

// Errors double as protocol error codes (ws.questErrorCode passes them through).
var (
	ErrOnCooldown       = errors.New("ON_COOLDOWN")
	ErrGCD              = errors.New("GCD")
	ErrNotEnoughRes     = errors.New("NOT_ENOUGH_RESOURCE")
	ErrOutOfRange       = errors.New("OUT_OF_RANGE")
	ErrInvalidTarget    = errors.New("INVALID_TARGET")
	ErrSkillLocked      = errors.New("SKILL_LOCKED")
	ErrUnsupportedSkill = errors.New("UNSUPPORTED_SKILL")
	ErrDead             = errors.New("DEAD")
)

type Event struct {
	Type   string   `json:"type"` // cast | damage | heal | shield | death | respawn
	Caster EntityID `json:"caster,omitempty"`
	Target EntityID `json:"target,omitempty"`
	Skill  string   `json:"skill,omitempty"`
	Amount int      `json:"amount,omitempty"`
	Crit   bool     `json:"crit,omitempty"`
	HP     int      `json:"hp"`
}

type Contributor struct {
	Ref    string
	Damage int
}

// Kill is emitted when an enemy dies; Contributors lists the players that dealt enough damage for loot.
type Kill struct {
	EnemyID      string
	Spawn        string
	Contributors []Contributor
}

type SelfState struct {
	Resource    int            `json:"resource"`
	ResourceMax int            `json:"resource_max"`
	GCDMS       int            `json:"gcd_ms"`
	Cooldowns   map[string]int `json:"cooldowns,omitempty"` // skill id -> ms remaining
	Shield      int            `json:"shield,omitempty"`
}

// CastOutcome is the reply to a successful cast.
type CastOutcome struct {
	Skill    string    `json:"skill_id"`
	Target   EntityID  `json:"target_id,omitempty"`
	Amount   int       `json:"amount,omitempty"`
	Crit     bool      `json:"crit,omitempty"`
	TargetHP int       `json:"target_hp,omitempty"`
	Self     SelfState `json:"self"`
}

// SetCombat enables combat; seed makes crits reproducible in tests.
func (s *Sim) SetCombat(c *Combat) { s.combat = c }

func (s *Sim) ticks(seconds float64) uint64 {
	return uint64(math.Ceil(seconds * float64(s.cfg.TickRate)))
}

func (s *Sim) emit(e Event) { s.events = append(s.events, e) }

// DrainEvents returns and clears events produced since the last call.
func (s *Sim) DrainEvents() []Event { out := s.events; s.events = nil; return out }

// DrainKills returns and clears kills since the last call.
func (s *Sim) DrainKills() []Kill { out := s.kills; s.kills = nil; return out }

// EventsFor filters events to those a viewer can see (it knows the caster or the target).
func (s *Sim) EventsFor(id EntityID, evs []Event) []Event {
	known := s.views[id]
	var out []Event
	for _, e := range evs {
		_, c := known[e.Caster]
		_, t := known[e.Target]
		if id == e.Caster || id == e.Target || c || t {
			out = append(out, e)
		}
	}
	return out
}

type resourceRule struct {
	start, regenOut, regenIn float64
}

// Canon 05 §3 resource model (Guard/Focus 6/3, Mana 5/2, Rage -5 out/0 in from 0, Light starts at 30 and drifts).
var resourceRules = map[string]resourceRule{
	"CLS-01": {100, 6, 3},
	"CLS-02": {0, -5, 0},
	"CLS-03": {100, 5, 2},
	"CLS-04": {100, 6, 3},
	"CLS-05": {30, 0, 0}, // drifts toward 30 (handled in stepCombat)
}

// SetPlayerCombat binds a joined player to a class and derived stats.
func (s *Sim) SetPlayerCombat(id EntityID, classID string, st damage.Stats) {
	e, ok := s.entities[id]
	if !ok || e.Kind != KindPlayer {
		return
	}
	e.classID, e.pstats = classID, st
	e.MaxHP, e.HP = st.MaxHP, st.MaxHP
	if e.cooldowns == nil {
		e.cooldowns = map[string]uint64{}
	}
	e.resource = resourceRules[classID].start
	e.selfDirty = true
}

// SetEnemyStats stores canon stats (HP already set by AddEnemy).
func (s *Sim) SetEnemyStats(id EntityID, st damage.EnemyStatsOut) {
	if e, ok := s.entities[id]; ok && e.Kind == KindEnemy {
		e.estats = st
	}
}

func (s *Sim) stepCombat() {
	if s.combat == nil {
		return
	}
	dt := s.cfg.Dt()
	for _, e := range s.entities {
		switch e.Kind {
		case KindPlayer:
			if e.classID == "" {
				continue
			}
			rule := resourceRules[e.classID]
			in := s.tick < e.combatUntil
			var rate float64
			switch {
			case e.classID == "CLS-05": // Light drifts to 30 out of combat
				if !in {
					diff := 30 - e.resource
					rate = math.Copysign(math.Min(math.Abs(diff)/dt, 5), diff)
				}
			case in:
				rate = rule.regenIn
			default:
				rate = rule.regenOut
			}
			if rate != 0 {
				next := math.Min(100, math.Max(0, e.resource+rate*dt))
				if next != e.resource {
					e.resource, e.selfDirty = next, true
				}
			}
			if e.shield > 0 && s.tick >= e.shieldUntil {
				e.shield, e.selfDirty = 0, true
			}
			if len(e.cooldowns) > 0 { // keep the HUD cooldown timers moving
				e.selfDirty = true
			}
		case KindEnemy:
			if e.Dead && s.tick >= e.respawnAt {
				e.Dead, e.HP, e.Pos = false, e.MaxHP, e.home
				e.contrib = map[string]int{}
				e.changed = s.tick
				s.emit(Event{Type: "respawn", Target: e.ID, HP: e.HP})
			}
		}
	}
}

func (s *Sim) selfState(e *Entity) SelfState {
	st := SelfState{Resource: int(math.Round(e.resource)), ResourceMax: 100, Shield: e.shield}
	rate := float64(s.cfg.TickRate)
	if e.gcdUntil > s.tick {
		st.GCDMS = int(float64(e.gcdUntil-s.tick) / rate * 1000)
	}
	for id, ready := range e.cooldowns {
		if ready > s.tick {
			if st.Cooldowns == nil {
				st.Cooldowns = map[string]int{}
			}
			st.Cooldowns[id] = int(float64(ready-s.tick) / rate * 1000)
		} else {
			delete(e.cooldowns, id)
		}
	}
	return st
}

func (s *Sim) skillRange(classID string) float64 {
	m := s.combat.RangedRangeM
	if classID == "CLS-01" || classID == "CLS-02" {
		m = s.combat.MeleeRangeM
	}
	return (m + s.combat.RangeSlackM) * s.combat.UnitsPerMeter
}

// Cast validates and applies a player skill. Supported effects (T-038c1): single-target damage on an
// enemy, shield on self, heal on self/ally (solo = self). Everything else returns ErrUnsupportedSkill.
func (s *Sim) Cast(id EntityID, skillID string, target EntityID) (CastOutcome, error) {
	if s.combat == nil {
		return CastOutcome{}, ErrUnsupportedSkill
	}
	p, ok := s.entities[id]
	if !ok || p.Kind != KindPlayer || p.classID == "" {
		return CastOutcome{}, ErrInvalidTarget
	}
	if p.Dead || p.HP <= 0 {
		return CastOutcome{}, ErrDead
	}
	sk, ok := s.combat.Skills[skillID]
	if !ok || sk.ClassID != p.classID {
		return CastOutcome{}, ErrSkillLocked
	}
	if sk.Level > p.Level {
		return CastOutcome{}, ErrSkillLocked
	}
	kind := sk.EffectKind
	supported := (kind == "damage" && sk.Target == "enemy") || (kind == "shield" && sk.Target == "self") ||
		(kind == "heal" && (sk.Target == "ally" || sk.Target == "self"))
	if !supported {
		return CastOutcome{}, ErrUnsupportedSkill
	}
	if s.tick < p.gcdUntil {
		return CastOutcome{}, ErrGCD
	}
	if ready, cd := p.cooldowns[skillID]; cd && s.tick < ready {
		return CastOutcome{}, ErrOnCooldown
	}
	if p.resource+1e-9 < float64(sk.Cost) {
		return CastOutcome{}, ErrNotEnoughRes
	}
	var victim *Entity
	if kind == "damage" {
		v, ok := s.entities[target]
		if !ok || v.Kind != KindEnemy || v.Dead || v.HP <= 0 {
			return CastOutcome{}, ErrInvalidTarget
		}
		if dist(p.Pos, v.Pos) > s.skillRange(p.classID) {
			return CastOutcome{}, ErrOutOfRange
		}
		victim = v
	} else if target != 0 && target != id {
		return CastOutcome{}, ErrInvalidTarget // allies other than self need parties (later)
	}

	// All checks passed: pay, start cooldowns, apply.
	p.resource = math.Min(100, math.Max(0, p.resource-float64(sk.Cost)+float64(sk.Gain)))
	p.gcdUntil = s.tick + s.ticks(damage.GCD)
	if sk.CooldownS > 0 {
		p.cooldowns[skillID] = s.tick + s.ticks(sk.CooldownS)
	}
	p.combatUntil = s.tick + s.ticks(s.combat.CombatLeaveS)
	p.selfDirty = true
	out := CastOutcome{Skill: skillID}
	s.emit(Event{Type: "cast", Caster: id, Target: target, Skill: skillID, HP: p.HP})
	switch kind {
	case "damage":
		power, def := float64(p.pstats.AP), float64(victim.estats.Armor)
		if sk.Scale == "SP" {
			power, def = float64(p.pstats.SP), float64(victim.estats.Resist)
		}
		d := damage.Damage(sk.Coef, power, 0, def, 1, 1)
		crit := s.rng.Float64() < damage.ClampCrit(s.combat.CritChance)
		if crit {
			d = int(math.Round(float64(d) * damage.CritMultPvE))
		}
		victim.HP = max(0, victim.HP-d)
		victim.changed = s.tick
		victim.contrib[p.Ref] += d
		out.Target, out.Amount, out.Crit, out.TargetHP = victim.ID, d, crit, victim.HP
		s.emit(Event{Type: "damage", Caster: id, Target: victim.ID, Skill: skillID, Amount: d, Crit: crit, HP: victim.HP})
		if victim.HP == 0 {
			s.killEnemy(victim)
		}
	case "shield":
		amt := min(int(math.Round(sk.Coef*float64(p.MaxHP))), damage.ShieldCap(p.MaxHP))
		p.shield = amt
		p.shieldUntil = s.tick + s.ticks(float64(max(sk.DurationS, 1)))
		out.Amount = amt
		s.emit(Event{Type: "shield", Caster: id, Target: id, Skill: skillID, Amount: amt, HP: p.HP})
	case "heal":
		h := damage.Heal(sk.Coef, float64(p.pstats.SP), 0, 1, p.MaxHP-p.HP)
		p.HP += h
		p.changed = s.tick
		out.Amount = h
		s.emit(Event{Type: "heal", Caster: id, Target: id, Skill: skillID, Amount: h, HP: p.HP})
	}
	out.Self = s.selfState(p)
	return out, nil
}

func (s *Sim) killEnemy(v *Entity) {
	v.Dead, v.HP, v.Moving = true, 0, false
	v.respawnAt = s.tick + s.ticks(s.combat.RespawnS)
	v.changed = s.tick
	s.emit(Event{Type: "death", Target: v.ID, HP: 0})
	k := Kill{EnemyID: v.Ref, Spawn: v.Spawn}
	need := int(math.Ceil(s.combat.MinContribHP * float64(v.MaxHP)))
	for ref, dmg := range v.contrib {
		if dmg >= need {
			k.Contributors = append(k.Contributors, Contributor{Ref: ref, Damage: dmg})
		}
	}
	sortContributors(k.Contributors)
	v.contrib = map[string]int{}
	s.kills = append(s.kills, k)
}

func sortContributors(c []Contributor) {
	for i := 1; i < len(c); i++ {
		for j := i; j > 0 && c[j].Ref < c[j-1].Ref; j-- {
			c[j], c[j-1] = c[j-1], c[j]
		}
	}
}
