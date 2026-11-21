package loot

import "testing"

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
