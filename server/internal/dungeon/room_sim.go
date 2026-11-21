package dungeon

import (
	"fmt"
	"math"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

// ValidateCombat checks that every room can start with the published combat data.
func (r *Registry) ValidateCombat(catalog *combatdata.Catalog) error {
	for _, id := range r.IDs() {
		def := r.byID[id]
		for _, room := range def.Rooms {
			if _, err := room.NewRoomZone(RoomView{}, catalog); err != nil {
				return err
			}
		}
	}
	return nil
}

// NewRoomZone builds one private combat simulation for the active room. Defeated
// spawns in the persisted room state stay absent when the simulation is rebuilt.
func (r Room) NewRoomZone(state RoomView, catalog *combatdata.Catalog) (*world.Zone, error) {
	def, err := r.zoneDef(state.Spawns)
	if err != nil {
		return nil, err
	}
	return world.NewZone(def, catalog)
}

func (r Room) zoneDef(defeated map[string]bool) (world.ZoneDef, error) {
	if r.Layout == nil || r.Layout.Entry == nil || !validDimension(r.Layout.Width) || !validDimension(r.Layout.Height) {
		return world.ZoneDef{}, fmt.Errorf("room %s: missing entry or invalid floor bounds", r.ID)
	}
	d := world.ZoneDef{
		ZoneID: r.ID, TickRate: 10, PlayerSpeed: 180,
		InterestRadius: math.Hypot(r.Layout.Width, r.Layout.Height),
		PlayerSpawn:    *r.Layout.Entry, NoRespawn: true,
	}
	d.Bounds.MaxX, d.Bounds.MaxY = r.Layout.Width, r.Layout.Height
	bounds := d.Config().Bounds
	if bounds.Clamp(d.PlayerSpawn) != d.PlayerSpawn {
		return world.ZoneDef{}, fmt.Errorf("room %s: entry outside floor bounds", r.ID)
	}
	for _, spawn := range r.Spawns {
		if spawn.Position == nil || bounds.Clamp(*spawn.Position) != *spawn.Position {
			return world.ZoneDef{}, fmt.Errorf("room %s: missing or out-of-bounds position for spawn %s", r.ID, spawn.ID)
		}
		if !defeated[spawn.ID] {
			d.Enemies = append(d.Enemies, world.SpawnDef{
				SpawnID: spawn.ID, EnemyID: spawn.EnemyID, X: spawn.Position.X, Y: spawn.Position.Y,
			})
		}
	}
	if w := r.Witness; w != nil {
		hasBoss := false
		for _, spawn := range r.Spawns {
			hasBoss = hasBoss || (spawn.ID == w.BossSpawn && spawn.EnemyID == "ENM-01-301")
		}
		if !hasBoss || len(w.Crystals) != 2 || w.AddEnemy == "" {
			return world.ZoneDef{}, fmt.Errorf("room %s: phase 2 requires a boss, two crystals and a reinforcement", r.ID)
		}
		ids := make(map[string]bool, len(w.Crystals))
		for _, crystal := range w.Crystals {
			if crystal.ID == "" || ids[crystal.ID] || contains(r.Objects, crystal.ID) ||
				contains(r.OptionalObjects, crystal.ID) || bounds.Clamp(crystal.Position) != crystal.Position {
				return world.ZoneDef{}, fmt.Errorf("room %s: invalid witness crystal %s", r.ID, crystal.ID)
			}
			ids[crystal.ID] = true
		}
		d.Witness = w
	}
	return d, d.Validate()
}

func validDimension(value float64) bool {
	return value > 0 && !math.IsInf(value, 0)
}
