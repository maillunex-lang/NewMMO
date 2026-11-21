package world

import (
	"math"
	"strings"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/damage"
)

// EnemyDef is what the AI needs from content (06 §3, §5): profile, element and abilities A (attack) / B (signature).
type EnemyDef struct {
	AI      string
	Element string
	A, B    combatdata.Ability
}

const (
	stIdle  = ""
	stAlert = "ALERT"
	stChase = "CHASE"
	stCast  = "CAST"
	stLeash = "LEASH_RETURN"
)

type aiState struct {
	state    string
	until    uint64 // ALERT end / CAST end
	target   EntityID
	threat   map[EntityID]float64
	nextA    uint64
	nextB    uint64
	noTarget uint64 // tick since which there is no valid target (LoseAggro timer)
	aim      Vec
	hasAim   bool
}

func (s *Sim) SetEnemyDef(id EntityID, d EnemyDef) {
	if e, ok := s.entities[id]; ok && e.Kind == KindEnemy {
		e.def = d
	}
}

func (s *Sim) addThreat(enemy *Entity, player EntityID, amount float64) {
	if enemy.ai.threat == nil {
		enemy.ai.threat = map[EntityID]float64{}
	}
	enemy.ai.threat[player] += amount
}

func (s *Sim) metres(m float64) float64 { return m * s.combat.UnitsPerMeter }

func (e *Entity) ranged() bool {
	return strings.Contains(e.def.AI, "ranged") || strings.Contains(e.def.AI, "caster")
}

func (e *Entity) defender() bool { return strings.Contains(e.def.AI, "defender") }

func (s *Sim) aggroRadius(e *Entity) float64 {
	if e.ranged() {
		return s.metres(9)
	}
	return s.metres(7)
}

func (s *Sim) attackRange(e *Entity) float64 {
	if e.ranged() {
		return s.metres(s.combat.RangedAttackM)
	}
	return s.metres(s.combat.MeleeAttackM)
}

func (s *Sim) validTarget(e *Entity, id EntityID) *Entity {
	t, ok := s.entities[id]
	if !ok || t.Kind != KindPlayer || t.Dead || t.HP <= 0 {
		return nil
	}
	if dist(t.Pos, e.home) > s.metres(s.combat.LeashM) {
		return nil
	}
	return t
}

// pickTarget: highest threat among valid targets, else nearest player inside the aggro radius.
func (s *Sim) pickTarget(e *Entity) *Entity {
	var best *Entity
	bestThreat := -1.0
	for id, th := range e.ai.threat {
		if t := s.validTarget(e, id); t != nil && (th > bestThreat || (th == bestThreat && t.ID < best.ID)) {
			best, bestThreat = t, th
		}
	}
	if best != nil {
		return best
	}
	var near *Entity
	nd := s.aggroRadius(e)
	for _, p := range s.entities {
		if p.Kind != KindPlayer || p.Dead || p.HP <= 0 {
			continue
		}
		if d := dist(p.Pos, e.Pos); d <= nd && (near == nil || d < dist(near.Pos, e.Pos) || (d == dist(near.Pos, e.Pos) && p.ID < near.ID)) {
			near = p
		}
	}
	return near
}

func (s *Sim) enemyPower(e *Entity, a combatdata.Ability) (power float64, physical bool) {
	physical = e.def.Element == "" || strings.Contains(e.def.Element, "физич")
	switch a.Scale {
	case "SP":
		return float64(e.estats.SP), false
	case "AP":
		return float64(e.estats.AP), true
	}
	if physical {
		return float64(e.estats.AP), true
	}
	return float64(e.estats.SP), false
}

func (s *Sim) hitPlayer(e *Entity, p *Entity, a combatdata.Ability) {
	power, physical := s.enemyPower(e, a)
	def := float64(p.pstats.Resist)
	if physical {
		def = float64(p.pstats.Armor)
	}
	d := damage.Damage(a.Coef, power, 0, def, 1, 1)
	absorbed := min(p.shield, d)
	p.shield -= absorbed
	p.HP = max(0, p.HP-(d-absorbed))
	p.changed, p.selfDirty = s.tick, true
	p.combatUntil = s.tick + s.ticks(s.combat.CombatLeaveS)
	s.emit(Event{Type: "damage", Caster: e.ID, Target: p.ID, Skill: a.ID, Amount: d, HP: p.HP})
	if p.HP == 0 {
		s.killPlayer(p)
	}
}

func (s *Sim) killPlayer(p *Entity) {
	p.Dead, p.Moving, p.target = true, false, nil
	p.respawnAt = s.tick + s.ticks(s.combat.PlayerRespawnS)
	p.changed, p.selfDirty = s.tick, true
	s.emit(Event{Type: "death", Target: p.ID, HP: 0})
}

func (s *Sim) respawnPlayer(p *Entity) {
	p.Dead, p.HP, p.Pos = false, p.MaxHP, s.cfg.Bounds.Clamp(s.cfg.PlayerSpawn)
	p.resource = resourceRules[p.classID].start
	p.cooldowns, p.gcdUntil, p.shield = map[string]uint64{}, 0, 0
	p.changed, p.selfDirty = s.tick, true
	s.emit(Event{Type: "respawn", Target: p.ID, HP: p.HP})
}

func (s *Sim) moveEnemy(e *Entity, to Vec, speed float64) (arrived bool) {
	dx, dy := to.X-e.Pos.X, to.Y-e.Pos.Y
	d := math.Hypot(dx, dy)
	step := speed * s.cfg.Dt()
	if d <= 1e-9 {
		return true
	}
	e.Facing = facing(dx, dy, e.Facing)
	if d <= step {
		e.Pos, e.Moving = to, false
		e.changed = s.tick
		return true
	}
	e.Pos = Vec{e.Pos.X + dx/d*step, e.Pos.Y + dy/d*step}
	e.Moving = true
	e.changed = s.tick
	return false
}

// stepEnemy is the FSM of 06 §3 (IDLE → ALERT → CHASE → ATTACK/CAST → … → LEASH_RETURN). Patrol is not
// modelled (enemies stand at their spawn until aggro). Dead enemies are handled by the respawn logic.
func (s *Sim) stepEnemy(e *Entity) {
	if !e.def.A.HasCoef {
		return
	}
	a := &e.ai
	speed := s.cfg.PlayerSpeed * s.combat.EnemySpeedFrac
	switch a.state {
	case stIdle:
		if t := s.pickTarget(e); t != nil {
			a.state, a.target, a.until = stAlert, t.ID, s.tick+s.ticks(s.combat.AlertS)
			s.addThreat(e, t.ID, 1)                                                 // a seen player stays a target until leash/death
			a.nextB = s.tick + s.ticks(max(e.def.B.CooldownS*0.5, s.combat.AlertS)) // open with the basic attack, signature later
		}
	case stAlert:
		if s.tick >= a.until {
			a.state, a.noTarget = stChase, 0
		}
	case stChase, stCast:
		t := s.pickTarget(e)
		if t == nil || dist(e.Pos, e.home) > s.metres(s.combat.LeashM) {
			if a.noTarget == 0 {
				a.noTarget = s.tick
			}
			if t == nil && s.tick-a.noTarget < s.ticks(s.combat.LoseAggroS) && dist(e.Pos, e.home) <= s.metres(s.combat.LeashM) {
				return // keep waiting at the spot for 8 s (canon)
			}
			a.state, a.target, a.threat, a.hasAim = stLeash, 0, nil, false
			return
		}
		a.noTarget, a.target = 0, t.ID
		if a.state == stCast {
			if s.tick >= a.until {
				s.resolveBlast(e)
				a.state = stChase
				a.nextB = s.tick + s.ticks(e.def.B.CooldownS)
			}
			return
		}
		rng := s.attackRange(e)
		d := dist(e.Pos, t.Pos)
		if d > rng*0.95 {
			s.moveEnemy(e, t.Pos, speed)
			return
		}
		e.Moving = false
		e.Facing = facing(t.Pos.X-e.Pos.X, t.Pos.Y-e.Pos.Y, e.Facing)
		if e.def.B.HasCoef && s.tick >= a.nextB {
			s.startSignature(e, t)
			return
		}
		if s.tick >= a.nextA {
			s.hitPlayer(e, t, e.def.A)
			a.nextA = s.tick + s.ticks(max(e.def.A.CooldownS, 0.5))
		}
	case stLeash:
		if s.moveEnemy(e, e.home, speed*2) {
			e.HP, e.ai = e.MaxHP, aiState{}
			e.contrib = map[string]int{}
			e.shield = 0
			e.changed = s.tick
		}
	}
}

// startSignature begins ability B: a telegraph (canon ≥0.8 s) and, for ranged/caster profiles, a ground circle
// fixed at the target's position at that moment; melee-like profiles slam around themselves. defender B = self shield.
func (s *Sim) startSignature(e *Entity, t *Entity) {
	a := &e.ai
	if e.defender() {
		amt := int(math.Round(0.15 * float64(e.MaxHP)))
		e.shield = amt
		a.nextB = s.tick + s.ticks(e.def.B.CooldownS)
		s.emit(Event{Type: "shield", Caster: e.ID, Target: e.ID, Skill: e.def.B.ID, Amount: amt, HP: e.HP})
		return
	}
	dur := math.Max(e.def.B.TelegraphS, s.combat.TelegraphMinS)
	a.aim, a.hasAim = e.Pos, true
	if e.ranged() {
		a.aim = t.Pos
	}
	a.state, a.until = stCast, s.tick+s.ticks(dur)
	s.emit(Event{Type: "telegraph", Caster: e.ID, Skill: e.def.B.ID, Amount: int(dur * 1000), HP: e.HP,
		AimX: round2(a.aim.X), AimY: round2(a.aim.Y), Radius: s.blastRadius(e)})
}

func (s *Sim) blastRadius(e *Entity) float64 {
	r := s.metres(s.combat.BlastRadiusM)
	if !e.ranged() {
		r += s.metres(s.combat.MeleeAttackM) // slam around the enemy: reach = attack range + blast radius
	}
	return r
}

// resolveBlast applies B to every living player still inside the telegraphed circle (moving out dodges it).
func (s *Sim) resolveBlast(e *Entity) {
	a := &e.ai
	if !a.hasAim {
		return
	}
	r := s.blastRadius(e)
	for _, p := range s.entities {
		if p.Kind == KindPlayer && !p.Dead && p.HP > 0 && dist(p.Pos, a.aim) <= r {
			s.hitPlayer(e, p, e.def.B)
		}
	}
	a.hasAim = false
}
