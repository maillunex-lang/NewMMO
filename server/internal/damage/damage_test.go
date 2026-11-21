package damage

import "testing"

func TestPlayerStatsGolden(t *testing.T) {
	// Hand-computed from 05 §3 (see comments).
	cases := []struct {
		name  string
		class string
		level int
		a     Attributes
		want  Stats
	}{
		// Страж L1, all 10: HP (200+0+120)*1.25=400; AP 15+14+5=34; SP 34; Armor (10+7)*1.15=19.55→20; Resist 17.
		{"guardian L1", "CLS-01", 1, StartAttributes, Stats{400, 34, 34, 20, 17}},
		// Маг L10, INT 30: HP (200+162+120)*0.90=433.8→434; SP (15+13.5+42+5)*1.15=86.825→87; AP 15+13.5+14+5=47.5→48.
		{"mage L10", "CLS-03", 10, Attributes{10, 10, 30, 10, 10}, Stats{434, 48, 87, 17 + 11, 17 + 11}},
		// Следопыт L20, DEX 40: AP (15+28.5+14+0.5*40*1.10)*1.10=(57.5+22)*1.10=87.45→87.
		{"ranger L20", "CLS-04", 20, Attributes{10, 40, 10, 10, 10}, Stats{0, 87, 0, 0, 0}},
		// Оракул L30, WIS 50: Resist 10+1.2*29+0.7*50*1.15=85.05→85; SP (15+43.5+14+25)*1.08=105.3→105.
		{"oracle L30", "CLS-05", 30, Attributes{10, 10, 10, 10, 50}, Stats{0, 0, 105, 0, 85}},
		// Берсерк L1: AP (15+14+5)*1.12=38.08→38; HP 320*1.05=336.
		{"berserker L1", "CLS-02", 1, StartAttributes, Stats{336, 38, 34, 17, 17}},
	}
	for _, c := range cases {
		got := PlayerStats(c.class, c.level, c.a, Gear{})
		// A zero in "want" means "not asserted for this case".
		check := func(name string, g, w int) {
			if w != 0 && g != w {
				t.Errorf("%s %s = %d, want %d", c.name, name, g, w)
			}
		}
		check("MaxHP", got.MaxHP, c.want.MaxHP)
		check("AP", got.AP, c.want.AP)
		check("SP", got.SP, c.want.SP)
		check("Armor", got.Armor, c.want.Armor)
		check("Resist", got.Resist, c.want.Resist)
	}
}

func TestGearIsAddedBeforeClassMultiplier(t *testing.T) {
	// Страж L1 with +100 flat HP: (320+100)*1.25 = 525.
	if got := PlayerStats("CLS-01", 1, StartAttributes, Gear{HP: 100}).MaxHP; got != 525 {
		t.Fatalf("MaxHP = %d, want 525", got)
	}
}

func TestEnemyStatsGolden(t *testing.T) {
	cases := []struct {
		level    int
		category string
		scale    Scale
		want     EnemyStatsOut
	}{
		// L1 normal: HP 120+28+1.6=149.6→150; AP 14.1→14; Armor 9.5→10.
		{1, "обычный", NoScale, EnemyStatsOut{150, 14, 14, 10, 10}},
		// L10 elite: HP (120+280+160)*3=1680; AP 33*1.35=44.55→45; Armor 23*1.3=29.9→30.
		{10, "элита", NoScale, EnemyStatsOut{1680, 45, 45, 30, 30}},
		// L10 mini-boss: HP 560*5=2800; AP 33*1.5=49.5→50; Armor 23*1.35=31.05→31.
		{10, "мини-босс", NoScale, EnemyStatsOut{2800, 50, 50, 31, 31}},
		// L10 dungeon boss, solo: HP 560*12=6720; AP 33*1.65=54.45→54; Armor 23*1.5=34.5→35.
		{10, "босс DNG", NoScale, EnemyStatsOut{6720, 54, 54, 35, 35}},
		// L10 boss in a type-A instance for 5: HP 6720*(1+0.45*4)=6720*2.8=18816; AP 33*1.65*1.24=67.518→68.
		{10, "босс DNG", InstanceScale(true, 5), EnemyStatsOut{18816, 68, 68, 35, 35}},
	}
	for _, c := range cases {
		got, ok := EnemyStats(c.level, c.category, c.scale)
		if !ok || got != c.want {
			t.Errorf("L%d %s = %+v (ok=%v), want %+v", c.level, c.category, got, ok, c.want)
		}
	}
	if _, ok := EnemyStats(1, "несуществующий", NoScale); ok {
		t.Error("unknown category must not be accepted")
	}
}

func TestScales(t *testing.T) {
	if s := InstanceScale(false, 3); s.HP != 1.7 || s.Atk != 1.08 {
		t.Fatalf("type B n=3: %+v", s)
	}
	if s := InstanceScale(true, 99); s.HP != 2.8 { // clamped to n=5
		t.Fatalf("n clamp: %+v", s)
	}
	if s := WorldBossScale(1); s.HP != 1 {
		t.Fatalf("world boss solo: %+v", s)
	}
	if s := WorldBossScale(100); s.HP != 4 {
		t.Fatalf("world boss clamp: %+v", s)
	}
}

func TestDamageGolden(t *testing.T) {
	// 05 SK01 Удар бастиона 1.10*AP, AP 34 vs enemy armor 10: raw 37.4 × 100/110 = 34.0 → 34.
	if got := Damage(1.10, 34, 0, 10, 1, 1); got != 34 {
		t.Fatalf("got %d, want 34", got)
	}
	// Armor 14 would give 37.4*100/114 = 32.8 → 33.
	if got := Damage(1.10, 34, 0, 14, 1, 1); got != 33 {
		t.Fatalf("got %d, want 33", got)
	}
	// Bonus applies to raw once: 1.0*100 ×1.5 vs armor 100 → 150*0.5 = 75.
	if got := Damage(1.0, 100, 0.5, 100, 1, 1); got != 75 {
		t.Fatalf("got %d, want 75", got)
	}
	// Floor of 1 (huge armor, tiny coefficient).
	if got := Damage(0.01, 1, 0, 10000, 1, 1); got != 1 {
		t.Fatalf("damage floor: %d", got)
	}
	// Negative defense counts as 0 (no amplification beyond raw).
	if got := Damage(1.0, 50, 0, -40, 1, 1); got != 50 {
		t.Fatalf("negative defense: %d", got)
	}
}

func TestEnemyBasicAttackAgainstGuardian(t *testing.T) {
	// L1 normal enemy AP 14, ability A 0.60*AP vs Страж L1 armor 20: 8.4 × 100/120 = 7.0 → 7.
	enemy, _ := EnemyStats(1, "обычный", NoScale)
	player := PlayerStats("CLS-01", 1, StartAttributes, Gear{})
	if got := Damage(0.60, float64(enemy.AP), 0, float64(player.Armor), 1, 1); got != 7 {
		t.Fatalf("got %d, want 7", got)
	}
}

func TestHealAndShield(t *testing.T) {
	if got := Heal(1.10, 100, 0, 1, 500); got != 110 {
		t.Fatalf("heal %d", got)
	}
	if got := Heal(1.10, 100, 0, 1, 40); got != 40 {
		t.Fatalf("heal must not exceed missing HP: %d", got)
	}
	if got := ShieldCap(400); got != 140 {
		t.Fatalf("shield cap %d", got)
	}
	if ClampCrit(0.9) != CritMaxPvE || ClampCrit(-1) != 0 {
		t.Fatal("crit clamp")
	}
}
