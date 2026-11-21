package dungeon

import (
	"testing"
	"time"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

func TestRoomSimulationKeepsDefeatsInsideTheInstance(t *testing.T) {
	registry, err := LoadDir(ResolveDir(""))
	if err != nil {
		t.Fatal(err)
	}
	def, _ := registry.Get("DNG-01-001")
	room := def.Rooms[1]
	spawn := room.Spawns[0]
	catalog := &combatdata.Catalog{
		Enemies: map[string]combatdata.Enemy{
			"ENM-01-001": {Name: "Bandit", Level: 1, Category: "обычный"},
			"ENM-01-002": {Name: "Goblin", Level: 1, Category: "обычный"},
		},
		Skills: map[string][]combatdata.Skill{
			"CLS-03": {{ID: "CLS-03-SK01", ClassID: "CLS-03", Level: 1, Target: "enemy", EffectKind: "damage", Coef: 100, Scale: "AP"}},
		},
	}
	join := func(z *world.Zone, character string) world.Snapshot {
		t.Helper()
		snapshot, err := z.Join(character, character, world.PlayerInfo{ClassID: "CLS-03", Level: 7},
			func(_ string, _ any) bool { return true }, nil)
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	newZone := func(defeated map[string]bool) *world.Zone {
		t.Helper()
		z, err := room.NewRoomZone(RoomView{Spawns: defeated}, catalog)
		if err != nil {
			t.Fatal(err)
		}
		return z
	}

	run, other := newZone(nil), newZone(nil)
	snapshot := join(run, "first")
	if len(snapshot.Entities) != len(room.Spawns)+1 {
		t.Fatalf("snapshot has %d entities", len(snapshot.Entities))
	}
	enemy := snapshot.Entities[0]
	if enemy.Ref != spawn.EnemyID || enemy.X != spawn.Position.X || enemy.Y != spawn.Position.Y {
		t.Fatalf("first spawn = %+v, want %+v", enemy, spawn)
	}
	if snapshot.Bounds.MaxX != room.Layout.Width || snapshot.Bounds.MaxY != room.Layout.Height {
		t.Fatalf("bounds = %+v, want %+v", snapshot.Bounds, room.Layout)
	}
	kills := make(chan world.KillEvent, 1)
	run.SetKillHook(func(k world.KillEvent) { kills <- k })
	if err := run.Move("first", "first", 1, *spawn.Position); err != nil {
		t.Fatal(err)
	}
	for range 15 {
		run.Tick()
	}
	if _, err := run.Cast("first", "first", "cast", "CLS-03-SK01", enemy.ID); err != nil {
		t.Fatal(err)
	}
	for range 350 {
		run.Tick()
	}
	select {
	case k := <-kills:
		if k.Spawn != spawn.ID || k.EnemyID != spawn.EnemyID {
			t.Fatalf("kill = %+v", k)
		}
	case <-time.After(time.Second):
		t.Fatal("missing server-observed kill")
	}
	if got := join(run, "first").Entities[0]; got.HP != 0 || !got.Dead {
		t.Fatalf("enemy respawned in dungeon: %+v", got)
	}
	if got := join(other, "second").Entities[0]; got.HP != got.MaxHP || got.Dead {
		t.Fatalf("other instance changed: %+v", got)
	}
	resumed := join(newZone(map[string]bool{spawn.ID: true}), "first")
	if len(resumed.Entities) != len(room.Spawns) {
		t.Fatalf("restored snapshot has %d entities", len(resumed.Entities))
	}
	if got := resumed.Entities[0]; got.X != room.Spawns[1].Position.X {
		t.Fatalf("restored wrong spawn: %+v", got)
	}
}
