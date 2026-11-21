package world

import (
	"math"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
)

const brokenCircleMS = 1500

type sectorCast struct {
	resolve uint64
	areas   [][]Vec
}

// A quarter circle leaves alternating open sectors. Its vertices are shared
// verbatim with the client so the painted outline is the damage boundary.
func quarterCircle(center Vec, direction, radius float64) []Vec {
	const segments = 16
	points := make([]Vec, 1, segments+2)
	points[0] = center
	for i := 0; i <= segments; i++ {
		angle := direction - math.Pi/4 + float64(i)*math.Pi/(2*segments)
		points = append(points, Vec{center.X + radius*math.Cos(angle), center.Y + radius*math.Sin(angle)})
	}
	return points
}

func insideSector(pos Vec, polygon []Vec) bool {
	for i, a := range polygon {
		b := polygon[(i+1)%len(polygon)]
		if (b.X-a.X)*(pos.Y-a.Y)-(b.Y-a.Y)*(pos.X-a.X) < -1e-7 {
			return false
		}
	}
	return true
}

func (s *Sim) startBrokenCircle(boss, target *Entity) {
	angle := math.Atan2(target.Pos.Y-boss.Pos.Y, target.Pos.X-boss.Pos.X)
	radius := s.blastRadius(boss)
	cast := &s.witness.circle
	cast.resolve = s.tick + s.ticks(float64(brokenCircleMS)/1000)
	cast.areas = [][]Vec{quarterCircle(boss.Pos, angle, radius), quarterCircle(boss.Pos, angle+math.Pi, radius)}
	boss.ai.nextB = s.tick + s.ticks(boss.def.B.CooldownS)
	s.emit(Event{Type: "sector_telegraph", Caster: boss.ID, Skill: boss.def.B.ID, Amount: brokenCircleMS, Sectors: cast.areas})
}

func (s *Sim) stepBrokenCircle(boss *Entity) bool {
	cast := &s.witness.circle
	if cast.resolve == 0 {
		return false
	}
	if s.tick >= cast.resolve {
		s.hitSectors(boss, boss.def.B, cast.areas)
		s.endBrokenCircle(boss)
	}
	return true
}

func (s *Sim) hitSectors(boss *Entity, ability combatdata.Ability, areas [][]Vec) {
	for _, player := range s.entities {
		if player.Kind != KindPlayer || player.Dead {
			continue
		}
		for _, area := range areas {
			if insideSector(player.Pos, area) {
				s.hitPlayer(boss, player, ability)
				break
			}
		}
	}
}

func (s *Sim) endBrokenCircle(boss *Entity) {
	if s.witness.circle.resolve != 0 {
		s.witness.circle = sectorCast{}
		s.emit(Event{Type: "sector_end", Caster: boss.ID, Skill: boss.def.B.ID})
	}
}

const (
	countdownStrikeMS = 1100
	countdownExposeMS = 3000
)

type countdownCast struct {
	deadline      uint64
	vulnerableEnd uint64
	strike        int // -1 during preparation
	areas         [][]Vec
}

func (s *Sim) inCountdownPhase(boss *Entity) bool {
	return s.witness != nil && s.witness.boss == boss.ID && boss.HP*100 < boss.MaxHP*35
}

func (s *Sim) stepCountdown(boss *Entity) bool {
	w := s.witness
	cast := &w.countdown
	if cast.deadline == 0 {
		if !s.inCountdownPhase(boss) || s.tick < cast.vulnerableEnd || boss.ai.state != stChase {
			return false
		}
		target := s.pickTarget(boss)
		if target == nil {
			return false
		}
		angle := math.Atan2(target.Pos.Y-boss.Pos.Y, target.Pos.X-boss.Pos.X)
		radius := s.blastRadius(boss)
		cast.areas = make([][]Vec, 3)
		for i := range cast.areas {
			cast.areas[i] = quarterCircle(boss.Pos, angle+float64(i)*2*math.Pi/3, radius)
		}
		boss.Moving, boss.changed = false, s.tick
		cast.strike = -1
		cast.deadline = s.tick + s.ticks(w.finisher.TelegraphS)
		s.emit(Event{Type: "countdown_prepare", Caster: boss.ID, Skill: w.finisher.ID,
			Amount: int(math.Round(w.finisher.TelegraphS * 1000)), Sectors: cast.areas})
		return true
	}
	if s.tick < cast.deadline {
		return true
	}
	if cast.strike >= 0 {
		s.hitSectors(boss, w.finisher, cast.areas[cast.strike:cast.strike+1])
	}
	cast.strike++
	if cast.strike >= len(cast.areas) {
		cast.deadline, cast.areas = 0, nil
		cast.vulnerableEnd = s.tick + s.ticks(float64(countdownExposeMS)/1000)
		s.emit(Event{Type: "sector_end", Caster: boss.ID, Skill: w.finisher.ID})
		s.emit(Event{Type: "boss_vulnerable", Caster: boss.ID, Skill: w.finisher.ID, Amount: countdownExposeMS})
		return true
	}
	cast.deadline = s.tick + s.ticks(float64(countdownStrikeMS)/1000)
	s.emit(Event{Type: "sector_telegraph", Caster: boss.ID, Skill: w.finisher.ID, Amount: countdownStrikeMS,
		Sectors: cast.areas[cast.strike : cast.strike+1]})
	return true
}

func (s *Sim) endCountdown(boss *Entity) {
	w := s.witness
	if w.countdown.deadline != 0 {
		s.emit(Event{Type: "sector_end", Caster: boss.ID, Skill: w.finisher.ID})
	}
	w.countdown = countdownCast{}
}
