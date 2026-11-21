package loot

import (
	"testing"

	"github.com/maillunex-lang/NewMMO/server/internal/equipment"
)

type seq struct {
	vals []int64
	i    int
}

func (s *seq) Intn(n int64) int64 {
	v := s.vals[s.i%len(s.vals)]
	s.i++
	return v % n
}

var common = Table{ID: "LOOT-ENM-01-001", EnemyID: "ENM-01-001", RegionID: "REG-01", Category: "обычный",
	GoldMin: 2, GoldMax: 5, Mat1Pct: 65, Mat3Pct: 8}

var elite = Table{ID: "LOOT-ENM-01-101", EnemyID: "ENM-01-101", RegionID: "REG-01", Category: "элита",
	GoldMin: 8, GoldMax: 15, Mat1Pct: 90, Mat3Pct: 25, SpecialItem: "MAT-01-004", SpecialPct: 8}

func TestRollBounds(t *testing.T) {
	// gold roll 0 → min; item rolls 0 → every chance hits.
	r := Roll(common, 1, &seq{vals: []int64{0}})
	if r.Gold != 2 || len(r.Items) != 2 || r.Items[0].ItemID != "MAT-01-001" || r.Items[1].ItemID != "MAT-01-003" {
		t.Fatalf("all-hit roll: %#v", r)
	}
	// gold roll max → max; item rolls 9999 → nothing hits (<6500 needed).
	r = Roll(common, 1, &seq{vals: []int64{3, 9999, 9999}})
	if r.Gold != 5 || len(r.Items) != 0 {
		t.Fatalf("all-miss roll: %#v", r)
	}
}

func TestRollThresholdsAreBasisPoints(t *testing.T) {
	// 65% = 6500 bp: roll 6499 hits, 6500 misses; 8% = 800 bp; 3.5% would be 350 bp.
	r := Roll(common, 1, &seq{vals: []int64{0, 6499, 9999}})
	if len(r.Items) != 1 || r.Items[0].ItemID != "MAT-01-001" {
		t.Fatalf("6499 must hit MAT-001: %#v", r)
	}
	r = Roll(common, 1, &seq{vals: []int64{0, 6500, 799}})
	if len(r.Items) != 1 || r.Items[0].ItemID != "MAT-01-003" {
		t.Fatalf("6500 must miss MAT-001, 799 hit MAT-003: %#v", r)
	}
}

func TestRollSpecialAndCoef(t *testing.T) {
	r := Roll(elite, 1.5, &seq{vals: []int64{7, 9999, 9999, 799}})
	// base gold 8+7=15 → ×1.5 floor = 22; MAT-001 90% = 9000 (9999 miss), MAT-003 25% = 2500 (miss), special 8% = 800 (799 hit)
	if r.Gold != 22 || len(r.Items) != 1 || r.Items[0].ItemID != "MAT-01-004" {
		t.Fatalf("elite roll: %#v", r)
	}
}

func TestClaimable(t *testing.T) {
	for cat, want := range map[string]bool{"обычный": true, "элита": true, "мини-босс": true, "босс DNG": false, "мировой босс": false, "": false} {
		if (Table{Category: cat}).Claimable() != want {
			t.Fatalf("category %q claimable=%v", cat, !want)
		}
	}
}

func TestCryptoRollerRange(t *testing.T) {
	var r CryptoRoller
	for i := 0; i < 2000; i++ {
		if v := r.Intn(10000); v < 0 || v >= 10000 {
			t.Fatalf("out of range: %d", v)
		}
	}
	if r.Intn(1) != 0 {
		t.Fatal("Intn(1) must be 0")
	}
}

func eqCatalog() *equipment.Catalog {
	c := equipment.NewCatalog()
	for _, s := range equipment.Slots {
		c.Put(equipment.Template{ID: "EQ-01-01-" + s + "-01", ClassID: "CLS-01", Region: "REG-01", Slot: s, Level: 8})
		c.Put(equipment.Template{ID: "EQ-02-01-" + s + "-01", ClassID: "CLS-02", Region: "REG-01", Slot: s, Level: 8})
	}
	return c
}

func TestRollEquipment(t *testing.T) {
	tb := Table{RegionID: "REG-01", EquipPct: 1, EquipRarity: "UNCOMMON"}
	cat := eqCatalog()
	// 1% = 100 bp: roll 99 hits, 100 misses.
	if _, _, ok := RollEquipment(tb, "CLS-01", cat, &seq{vals: []int64{100}}); ok {
		t.Fatal("100 bp must miss a 1% roll")
	}
	// hit, slot index 3 (CH), template pick 0.
	tpl, rarity, ok := RollEquipment(tb, "CLS-01", cat, &seq{vals: []int64{99, 3, 0}})
	if !ok || tpl.ID != "EQ-01-01-CH-01" || rarity != "UNCOMMON" {
		t.Fatalf("got %v %s %v", tpl, rarity, ok)
	}
	// Only the character's own class ever drops.
	tpl, _, ok = RollEquipment(tb, "CLS-02", cat, &seq{vals: []int64{0, 0, 0}})
	if !ok || tpl.ClassID != "CLS-02" {
		t.Fatalf("class filter: %v", tpl)
	}
	// No catalog / unknown rarity / 0% never drop; a class with no templates drops nothing.
	if _, _, ok := RollEquipment(tb, "CLS-01", nil, &seq{vals: []int64{0}}); ok {
		t.Fatal("nil catalog")
	}
	if _, _, ok := RollEquipment(Table{RegionID: "REG-01", EquipPct: 100, EquipRarity: "BOGUS"}, "CLS-01", cat, &seq{vals: []int64{0}}); ok {
		t.Fatal("bad rarity")
	}
	if _, _, ok := RollEquipment(Table{RegionID: "REG-01", EquipRarity: "RARE"}, "CLS-01", cat, &seq{vals: []int64{0}}); ok {
		t.Fatal("0%")
	}
	if _, _, ok := RollEquipment(tb, "CLS-05", cat, &seq{vals: []int64{0}}); ok {
		t.Fatal("class without templates")
	}
}
