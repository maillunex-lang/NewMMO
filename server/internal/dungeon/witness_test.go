package dungeon

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

func TestWitnessEventsAndCrystalCommandsUseRoomScope(t *testing.T) {
	registry, err := LoadDir(ResolveDir(""))
	if err != nil {
		t.Fatal(err)
	}
	catalog := roomTestCatalog()
	catalog.Enemies["ENM-01-301"] = combatdata.Enemy{
		Name: "Boss", Level: 1, Category: "обычный",
		Abilities: map[string]combatdata.Ability{
			"A": {HasCoef: true, Coef: 0.1, Scale: "AP", CooldownS: 2.5},
			"C": {ID: "AB-ENM-01-301-C", HasCoef: true, Coef: 1.8, Scale: "SP", CooldownS: 25},
		},
	}
	catalog.Skills["CLS-03"][0].Coef = 1.8
	engine := NewEngine(nil, registry, nil, Config{})
	var witnessEvents []world.Event
	var encounter string
	ctx := t.Context()
	s := engine.NewSession(ctx, catalog, func(_ context.Context, _ string, _ int64) (world.PlayerInfo, error) {
		return world.PlayerInfo{ClassID: "CLS-03", Level: 7}, nil
	}, "account", 7, func(_ string, _ any) bool { return true }, func(typ string, _ int64, payload any) bool {
		events := payload.(map[string]any)
		if typ != "world.event" || events["encounter_id"] != encounter {
			t.Errorf("critical witness event has wrong scope: %v", payload)
		}
		witnessEvents = append(witnessEvents, events["events"].([]world.Event)...)
		return true
	}, slog.Default())
	s.stop = func() {} // Tick the actual room directly to control the test clock.
	initial, err := s.applySnapshot(ctx, Snapshot{InstanceID: "run", DungeonID: "DNG-01-001",
		RoomID: "DNG-01-001-R06", Status: StatusBoss}, false)
	if err != nil {
		t.Fatal(err)
	}
	encounter = initial.Combat.Encounter
	advance := func(ticks int) {
		for range ticks {
			s.zone.TickEncounter(s.ref)
		}
	}
	move := func(seq, x float64) {
		t.Helper()
		_, _, handled, err := s.HandleWorld(ctx, "world.move.request", "", map[string]any{
			"seq": seq, "x": x, "y": float64(75), "encounter_id": encounter,
		})
		if !handled || err != nil {
			t.Fatalf("room movement: handled=%t, error=%v", handled, err)
		}
	}
	move(1, 280)
	advance(25)
	_, _, handled, err := s.HandleWorld(ctx, "world.cast.request", "cast", map[string]any{
		"encounter_id": encounter, "skill_id": "CLS-03-SK01", "target_id": float64(initial.Combat.Entities[0].ID),
	})
	if !handled || err != nil {
		t.Fatalf("trigger phase 2: handled=%t, error=%v", handled, err)
	}
	advance(7)
	if !slices.ContainsFunc(witnessEvents, func(event world.Event) bool { return event.Type == "witness_telegraph" }) {
		t.Fatalf("missing critical warning: %v", witnessEvents)
	}
	breakCrystal := func(stream, id string) error {
		_, _, err := s.Handle(ctx, "dungeon.interact.request", "break-"+id, map[string]any{
			"instance_id": "run", "object_id": id, "encounter_id": stream,
		})
		return err
	}
	if err := breakCrystal("stale", "OBJ-DNG-01-001-CRYSTAL-1"); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("stale crystal request: %v", err)
	}
	if err := breakCrystal(encounter, "OBJ-DNG-01-001-CRYSTAL-1"); err != nil {
		t.Fatal(err)
	}
	move(2, 475)
	advance(11)
	if err := breakCrystal(encounter, "OBJ-DNG-01-001-CRYSTAL-2"); err != nil {
		t.Fatal(err)
	}
	advance(1)
	if witnessEvents[len(witnessEvents)-1].Type != "witness_end" {
		t.Fatalf("missing interruption update: %v", witnessEvents)
	}
}
