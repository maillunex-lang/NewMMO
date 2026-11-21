// Package equipment implements canon 11_ITEMS_LOOT.md §4/§6: equipment templates, the stat budget model and
// server-owned equipment instances (drop, equip, unequip). Contract: docs/contracts/equipment-engine.md.
package equipment

import (
	"math"
	"sort"
)

// Slots in canon order (11 §6). Codes are the template-id suffix.
var Slots = []string{"MH", "OH", "HD", "CH", "HA", "LG", "FT", "RG", "AM"}

// rarityMult: canon 11 §4 table "Бонус к базовым статам".
var rarityMult = map[string]float64{
	"COMMON": 1.00, "UNCOMMON": 1.08, "RARE": 1.18, "EPIC": 1.30, "LEGENDARY": 1.43, "MYTHIC": 1.55,
}

// slotMult: canon 11 §4 (mainhand 1.4 … amulet 0.65).
var slotMult = map[string]float64{
	"MH": 1.4, "OH": 0.85, "CH": 1.25, "HD": 0.9, "HA": 0.7, "LG": 1.0, "FT": 0.7, "RG": 0.55, "AM": 0.65,
}

type share struct {
	Stat string
	Pct  int
}

// classShares: canon 11 §4 "Распределение" (order = remainder priority). Conversion of these stats into
// AP/SP/HP/armor belongs to the single class model (05_CLASSES.md) and is NOT done here.
var classShares = map[string][]share{
	"CLS-01": {{"VIT", 55}, {"DEF", 30}, {"STR", 15}},
	"CLS-02": {{"STR", 50}, {"CRIT", 30}, {"VIT", 20}},
	"CLS-03": {{"INT", 55}, {"HASTE", 25}, {"VIT", 20}},
	"CLS-04": {{"DEX", 50}, {"CRIT", 30}, {"VIT", 20}},
	"CLS-05": {{"WIS", 55}, {"HASTE", 25}, {"VIT", 20}},
}

// ValidRarity reports whether r is a canon rarity code.
func ValidRarity(r string) bool { _, ok := rarityMult[r]; return ok }

// ValidSlot reports whether s is a canon slot code.
func ValidSlot(s string) bool { _, ok := slotMult[s]; return ok }

// BasePower is canon P = 8 + 2.4*L + 0.045*L².
func BasePower(itemLevel int) float64 {
	l := float64(itemLevel)
	return 8 + 2.4*l + 0.045*l*l
}

// StatBudget is canon round(P * rarity_mult * slot_mult). Unknown rarity/slot ⇒ 0.
func StatBudget(itemLevel int, rarity, slot string) int {
	r, ok1 := rarityMult[rarity]
	s, ok2 := slotMult[slot]
	if !ok1 || !ok2 || itemLevel < 1 {
		return 0
	}
	return int(math.Round(BasePower(itemLevel) * r * s))
}

// Distribute splits a budget between the class' stats; the rounding remainder goes to the primary stat so
// the parts always sum to the budget. Unknown class ⇒ nil.
func Distribute(classID string, budget int) map[string]int {
	sh, ok := classShares[classID]
	if !ok || budget <= 0 {
		return nil
	}
	out := map[string]int{}
	used := 0
	for _, x := range sh {
		v := budget * x.Pct / 100
		out[x.Stat] = v
		used += v
	}
	out[sh[0].Stat] += budget - used
	return out
}

// RequiredLevel is the interim wear requirement (conflict C-EQUIP-001): item_level-3, minimum 1, which keeps
// canon's "gap of at most 3" between requirement and item level.
func RequiredLevel(itemLevel int) int {
	if itemLevel-3 < 1 {
		return 1
	}
	return itemLevel - 3
}

// StatKeys returns the stat names of a map in stable order (for deterministic output/tests).
func StatKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
