package world

import "math"

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
		for _, player := range s.entities {
			if player.Kind != KindPlayer || player.Dead {
				continue
			}
			for _, area := range cast.areas {
				if insideSector(player.Pos, area) {
					s.hitPlayer(boss, player, boss.def.B)
					break
				}
			}
		}
		s.endBrokenCircle(boss)
	}
	return true
}

func (s *Sim) endBrokenCircle(boss *Entity) {
	if s.witness.circle.resolve != 0 {
		s.witness.circle = sectorCast{}
		s.emit(Event{Type: "sector_end", Caster: boss.ID, Skill: boss.def.B.ID})
	}
}
