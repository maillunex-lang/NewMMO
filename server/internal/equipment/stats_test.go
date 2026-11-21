package equipment

import "testing"

func TestStatBudgetCanon(t *testing.T) {
	// canon 11 §4 example check: level 8, UNCOMMON, mainhand: P = 8+19.2+2.88 = 30.08; ×1.08×1.4 = 45.48 → 45.
	if got := StatBudget(8, "UNCOMMON", "MH"); got != 45 {
		t.Fatalf("budget = %d, want 45", got)
	}
	if got := StatBudget(8, "COMMON", "RG"); got != 17 { // 30.08*0.55 = 16.54
		t.Fatalf("ring budget = %d, want 17", got)
	}
	if StatBudget(8, "BOGUS", "MH") != 0 || StatBudget(8, "RARE", "XX") != 0 || StatBudget(0, "RARE", "MH") != 0 {
		t.Fatal("unknown inputs must yield 0")
	}
}

func TestDistributeSumsToBudget(t *testing.T) {
	for cls := range classShares {
		for b := 1; b < 400; b++ {
			sum := 0
			for _, v := range Distribute(cls, b) {
				sum += v
			}
			if sum != b {
				t.Fatalf("%s budget %d distributed to %d", cls, b, sum)
			}
		}
	}
	if Distribute("CLS-09", 10) != nil || Distribute("CLS-01", 0) != nil {
		t.Fatal("unknown class / zero budget must give nil")
	}
	m := Distribute("CLS-01", 100)
	if m["VIT"] != 55 || m["DEF"] != 30 || m["STR"] != 15 {
		t.Fatalf("warrior split = %v", m)
	}
}

func TestRequiredLevel(t *testing.T) {
	if RequiredLevel(8) != 5 || RequiredLevel(2) != 1 || RequiredLevel(1) != 1 {
		t.Fatal("required level")
	}
}
