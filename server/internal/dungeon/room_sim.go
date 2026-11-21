package dungeon

import (
	"fmt"
	"math"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

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
	return d, d.Validate()
}

func validDimension(value float64) bool {
	return value > 0 && !math.IsInf(value, 0)
}
