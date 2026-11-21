package world

import (
	"fmt"
	"math"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
)

const (
	witnessWindupMS = 2200
	crystalReach    = 100 // authoritative interaction radius in world units
)

type WitnessCrystal struct {
	ID       string `json:"id"`
	Position Vec    `json:"position"`
}

type WitnessDef struct {
	BossSpawn string           `json:"boss_spawn"`
	Crystals  []WitnessCrystal `json:"crystals"`
	AddEnemy  string           `json:"add_enemy"`
}

type witnessFight struct {
	boss       EntityID
	def        WitnessDef
	ability    combatdata.Ability
	add        zoneEnemy
	deadline   uint64
	ready      uint64
	safe       []Rect2
	broken     map[string]bool
	reinforced bool
}

func (s *Sim) setupWitness(def WitnessDef, cat *combatdata.Catalog) error {
	bounds := s.cfg.Bounds
	maxWalk := math.Min(bounds.MaxX-bounds.MinX, bounds.MaxY-bounds.MinY) / 2
	if maxWalk >= s.cfg.PlayerSpeed*(float64(witnessWindupMS)/1000-s.cfg.Dt()) {
		return fmt.Errorf("phase 2 safe areas are too far apart")
	}
	for id, boss := range s.entities {
		if boss.Spawn != def.BossSpawn {
			continue
		}
		ability := cat.Enemies[boss.Ref].Abilities["C"]
		add, err := loadZoneEnemy(cat, def.AddEnemy)
		if err != nil {
			return err
		}
		if ability.ID != "AB-ENM-01-301-C" || !ability.HasCoef || ability.CooldownS <= 0 {
			return fmt.Errorf("missing phase 2 ability for %s", boss.Ref)
		}
		s.witness = &witnessFight{boss: id, def: def, ability: ability, add: add, broken: map[string]bool{}}
		break
	}
	// A persisted boss defeat restores the room without a live encounter.
	return nil
}

func witnessAreas(bounds Rect, diagonal int) []Rect2 {
	x, y := (bounds.MinX+bounds.MaxX)/2, (bounds.MinY+bounds.MaxY)/2
	if diagonal == 0 {
		return []Rect2{{bounds.MinX, bounds.MinY, x, y}, {x, y, bounds.MaxX, bounds.MaxY}}
	}
	return []Rect2{{x, bounds.MinY, bounds.MaxX, y}, {bounds.MinX, y, x, bounds.MaxY}}
}

func (s *Sim) inWitnessPhase(boss *Entity) bool {
	return s.witness != nil && s.witness.boss == boss.ID &&
		boss.HP*100 <= boss.MaxHP*70 && boss.HP*100 >= boss.MaxHP*35
}

func (s *Sim) stepWitness(boss *Entity) bool {
	w := s.witness
	if w == nil || w.boss != boss.ID {
		return false
	}
	if w.deadline != 0 {
		if s.tick >= w.deadline {
			for _, player := range s.entities {
				if player.Kind == KindPlayer && !player.Dead && !inSafeArea(player.Pos, w.safe) {
					s.hitPlayer(boss, player, w.ability)
				}
			}
			s.endWitness()
		}
		return true
	}
	if boss.ai.state != stChase || !s.inWitnessPhase(boss) ||
		s.tick < w.ready || s.pickTarget(boss) == nil {
		return false
	}
	boss.Moving, boss.changed = false, s.tick
	w.deadline = s.tick + s.ticks(float64(witnessWindupMS)/1000)
	w.ready = s.tick + s.ticks(w.ability.CooldownS)
	w.safe = witnessAreas(s.cfg.Bounds, s.rng.IntN(2))
	if !w.reinforced {
		for _, crystal := range w.def.Crystals {
			s.entities[w.add.spawn(s, "", crystal.Position)].temporary = true
		}
		w.reinforced = true
	}
	s.emit(Event{Type: "witness_telegraph", Caster: boss.ID, Skill: w.ability.ID, Amount: witnessWindupMS, SafeAreas: w.safe})
	return true
}

func inSafeArea(pos Vec, areas []Rect2) bool {
	for _, area := range areas {
		if pos.X >= area.MinX && pos.X <= area.MaxX && pos.Y >= area.MinY && pos.Y <= area.MaxY {
			return true
		}
	}
	return false
}

func (s *Sim) endWitness() {
	w := s.witness
	if w != nil && w.deadline != 0 {
		w.deadline, w.safe = 0, nil
		s.emit(Event{Type: "witness_end", Caster: w.boss, Skill: w.ability.ID})
	}
}

// BreakWitnessCrystal validates the player's authoritative room position.
func (z *Zone) BreakWitnessCrystal(ref string, owner any, id string) error {
	z.mu.Lock()
	defer z.mu.Unlock()
	session, err := z.lookup(ref, owner)
	if err != nil {
		return err
	}
	return z.sim.breakWitnessCrystal(session.entity, id)
}

func (s *Sim) breakWitnessCrystal(playerID EntityID, id string) error {
	w := s.witness
	if w == nil {
		return ErrInvalidTarget
	}
	for _, crystal := range w.def.Crystals {
		if crystal.ID != id {
			continue
		}
		if w.broken[id] {
			return nil
		}
		if w.deadline == 0 || s.tick >= w.deadline {
			return ErrStateConflict
		}
		player := s.entities[playerID]
		if player == nil || player.Dead {
			return ErrDead
		}
		if dist(player.Pos, crystal.Position) > crystalReach {
			return ErrOutOfRange
		}
		w.broken[id] = true
		s.emit(Event{Type: "witness_crystal", Caster: w.boss, ObjectID: id})
		if len(w.broken) == len(w.def.Crystals) {
			s.endWitness()
		}
		return nil
	}
	return ErrInvalidTarget
}
