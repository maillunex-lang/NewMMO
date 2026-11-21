package world

import (
	"math"
	"testing"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
)

func TestBrokenCircleLocksTwoSectorsAndUsesPublishedDamage(t *testing.T) {
	s, boss, player := witnessFixture(t)
	boss.def.A = combatdata.Ability{HasCoef: true, Coef: 0.01, Scale: "AP", CooldownS: 2.5}
	boss.def.B = combatdata.Ability{ID: "AB-ENM-01-301-B", HasCoef: true, Coef: 1.6, Scale: "SP", CooldownS: 18}
	player.Pos = Vec{boss.Pos.X + 50, boss.Pos.Y}
	exposed := s.entities[s.AddPlayer("8", "Exposed", 10, 1000)]
	exposed.Pos = Vec{boss.Pos.X - 50, boss.Pos.Y}
	s.Step()
	events := s.DrainEvents()
	if len(events) != 1 || events[0].Type != "sector_telegraph" || events[0].Amount != 1500 || len(events[0].Sectors) != 2 {
		t.Fatalf("first phase warning = %+v", events)
	}
	areas := events[0].Sectors
	if !insideSector(player.Pos, areas[0]) || !insideSector(exposed.Pos, areas[1]) ||
		insideSector(Vec{boss.Pos.X, boss.Pos.Y + 50}, areas[0]) {
		t.Fatal("attack sectors are not opposed with open sides")
	}
	if boss.ai.nextB != s.tick+180 {
		t.Fatal("ability B did not start its canon cooldown")
	}
	s.Move(player.ID, 1, Vec{boss.Pos.X, boss.Pos.Y + 105})
	for range 14 {
		s.Step()
	}
	if exposed.HP != 1000 {
		t.Fatal("damage landed early")
	}
	s.Step()
	if exposed.HP != 840 || player.HP != 1000 {
		t.Fatalf("exposed HP=%d, dodge HP=%d", exposed.HP, player.HP)
	}
	if boss.Pos != areas[0][0] || s.witness.circle.resolve != 0 {
		t.Fatal("the attack origin moved or the cast did not close")
	}
}

// In the current unobstructed arena a point 129 px from the boss towards the
// far corner is in bounds and outside both sectors. Any position within the
// sectors is at most 128 px from the boss, giving a path of at most 257 px.
func TestBrokenCircleHasAnEscapeAtArenaEdges(t *testing.T) {
	s, boss, _ := witnessFixture(t)
	radius := s.blastRadius(boss)
	reach := s.cfg.PlayerSpeed * float64(brokenCircleMS) / 1000
	if reach <= 2*radius+1 {
		t.Fatal("windup is too short for guaranteed radial escape")
	}
	b := s.cfg.Bounds
	corners := []Vec{{b.MinX, b.MinY}, {b.MaxX, b.MinY}, {b.MaxX, b.MaxY}, {b.MinX, b.MaxY}}
	minFarthestDistance := math.Hypot(b.MaxX-b.MinX, b.MaxY-b.MinY) / 2
	if minFarthestDistance <= radius+1 {
		t.Fatal("arena cannot guarantee a safe radial endpoint")
	}
	for i, origin := range corners {
		far := corners[(i+2)%len(corners)]
		angle := math.Atan2(far.Y-origin.Y, far.X-origin.X)
		escape := Vec{origin.X + (radius+1)*math.Cos(angle), origin.Y + (radius+1)*math.Sin(angle)}
		if b.Clamp(escape) != escape || insideSector(escape, quarterCircle(origin, angle, radius)) {
			t.Fatalf("unsafe escape from arena corner %v via %v", origin, escape)
		}
	}
}
