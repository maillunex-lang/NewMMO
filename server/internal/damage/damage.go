// Package damage holds the canon combat formulas (docs/content/05_CLASSES.md §3, 06_ENEMIES.md §2) as
// pure functions, so the simulation (T-038b/c) and every test share one implementation.
// Rounding: float math, rounded once at the end with math.Round (half away from zero).
package damage

import "math"

// Canon constants (05 §3).
const (
	GCD               = 0.75 // seconds
	CritBase          = 0.05
	CritMaxPvE        = 0.35
	CritMultPvE       = 1.5
	DodgeMaxPvE       = 0.20
	MoveSpeedBonusCap = 0.35
	AttackSpeedCap    = 0.40
	ShieldCapFracPvE  = 0.35 // of MaxHP
)

type Attributes struct{ STR, DEX, INT, VIT, WIS int }

// StartAttributes: every primary attribute starts at 10 (05 §3).
var StartAttributes = Attributes{10, 10, 10, 10, 10}

type Gear struct{ HP, AP, SP, Armor, Resist float64 }

type Stats struct {
	MaxHP, AP, SP, Armor, Resist int
}

func round(x float64) int { return int(math.Round(x)) }

// PlayerStats applies 05 §3: base formulas, then class multipliers (before temporary effects).
// Class ids: CLS-01 Страж, CLS-02 Берсерк, CLS-03 Маг, CLS-04 Следопыт, CLS-05 Оракул.
func PlayerStats(classID string, level int, a Attributes, g Gear) Stats {
	l := float64(level - 1)
	dexInfluence, wisInfluence := 1.0, 1.0
	hpMul, apMul, spMul, armorMul := 1.0, 1.0, 1.0, 1.0
	switch classID {
	case "CLS-01":
		hpMul, armorMul = 1.25, 1.15
	case "CLS-02":
		apMul, hpMul = 1.12, 1.05
	case "CLS-03":
		spMul, hpMul = 1.15, 0.90
	case "CLS-04":
		apMul, dexInfluence = 1.10, 1.10
	case "CLS-05":
		spMul, wisInfluence = 1.08, 1.15
	}
	maxHP := (200 + 18*l + 12*float64(a.VIT) + g.HP) * hpMul
	ap := (15 + 1.5*l + 1.4*float64(a.STR) + 0.5*float64(a.DEX)*dexInfluence + g.AP) * apMul
	sp := (15 + 1.5*l + 1.4*float64(a.INT) + 0.5*float64(a.WIS) + g.SP) * spMul
	armor := (10 + 1.2*l + 0.7*float64(a.VIT) + g.Armor) * armorMul
	resist := 10 + 1.2*l + 0.7*float64(a.WIS)*wisInfluence + g.Resist
	return Stats{MaxHP: round(maxHP), AP: round(ap), SP: round(sp), Armor: round(armor), Resist: round(resist)}
}

// Category coefficients K (06 §2). Keys match the exported `enemy.threat` values.
type Category struct{ HP, Atk, Def float64 }

var Categories = map[string]Category{
	"обычный":      {1.0, 1.0, 1.0},
	"элита":        {3.0, 1.35, 1.30},
	"мини-босс":    {5.0, 1.50, 1.35},
	"босс DNG":     {12.0, 1.65, 1.50},
	"мировой босс": {45.0, 1.90, 1.70},
}

// Scale is a group scaling pair (HP and attack multipliers).
type Scale struct{ HP, Atk float64 }

// NoScale is the open-world scale (group_scale = 1).
var NoScale = Scale{1, 1}

// InstanceScale: instance type A: HP 1+0.45(n-1), attack 1+0.06(n-1); type B: 0.35 / 0.04 (06 §2), n = 1…5.
func InstanceScale(typeA bool, n int) Scale {
	if n < 1 {
		n = 1
	}
	if n > 5 {
		n = 5
	}
	d := float64(n - 1)
	if typeA {
		return Scale{1 + 0.45*d, 1 + 0.06*d}
	}
	return Scale{1 + 0.35*d, 1 + 0.04*d}
}

// WorldBossScale: HP clamp(1+0.22(contributors-1), 1, 4), fixed at phase start (06 §2).
func WorldBossScale(contributors int) Scale {
	hp := 1 + 0.22*float64(contributors-1)
	return Scale{HP: math.Min(4, math.Max(1, hp)), Atk: 1}
}

type EnemyStatsOut struct{ MaxHP, AP, SP, Armor, Resist int }

// EnemyStats: 06 §2 base formulas × category K × group scale. ok=false for an unknown category.
func EnemyStats(level int, category string, s Scale) (EnemyStatsOut, bool) {
	k, ok := Categories[category]
	if !ok {
		return EnemyStatsOut{}, false
	}
	l := float64(level)
	hp := (120 + 28*l + 1.6*l*l) * k.HP * s.HP
	ap := (12 + 2.1*l) * k.Atk * s.Atk
	armor := (8 + 1.5*l) * k.Def
	return EnemyStatsOut{MaxHP: round(hp), AP: round(ap), SP: round(ap), Armor: round(armor), Resist: round(armor)}, true
}

// Mitigation is 100/(100+defense); defense below 0 counts as 0 (05 §3). Apply once only.
func Mitigation(defense float64) float64 {
	if defense < 0 {
		defense = 0
	}
	return 100 / (100 + defense)
}

// Damage = max(1, round(coef × power × (1+bonus) × mitigation × pvpScalar × targetMod)).
// power is AP or SP of the attacker, defense the matching Armor (physical) or Resist (magical).
func Damage(coef, power, bonus, defense, pvpScalar, targetMod float64) int {
	raw := coef * power * (1 + bonus)
	v := round(raw * Mitigation(defense) * pvpScalar * targetMod)
	if v < 1 {
		return 1
	}
	return v
}

// Heal = round(coef × SP × (1+bonus) × pvpHealScalar), never above the missing HP (05 §3).
func Heal(coef, sp, bonus, pvpScalar float64, missingHP int) int {
	h := round(coef * sp * (1 + bonus) * pvpScalar)
	if h > missingHP {
		h = missingHP
	}
	if h < 0 {
		h = 0
	}
	return h
}

// ShieldCap is the maximum summed active shield on a target (35% of MaxHP in PvE).
func ShieldCap(maxHP int) int { return int(math.Floor(float64(maxHP) * ShieldCapFracPvE)) }

// ClampCrit limits a crit chance to the PvE cap.
func ClampCrit(chance float64) float64 { return math.Min(CritMaxPvE, math.Max(0, chance)) }
