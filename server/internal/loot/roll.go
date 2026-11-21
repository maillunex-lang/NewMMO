package loot

import (
	"crypto/rand"
	"encoding/binary"
	"math"

	"github.com/maillunex-lang/NewMMO/server/internal/equipment"
)

// Roller supplies uniform integers; production uses crypto/rand, tests inject a fixed sequence.
type Roller interface {
	// Intn returns a uniform value in [0, n).
	Intn(n int64) int64
}

type CryptoRoller struct{}

func (CryptoRoller) Intn(n int64) int64 {
	if n <= 1 {
		return 0
	}
	// Rejection sampling to avoid modulo bias.
	limit := math.MaxUint64 - (math.MaxUint64 % uint64(n))
	for {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return 0
		}
		v := binary.BigEndian.Uint64(b[:])
		if v < limit {
			return int64(v % uint64(n))
		}
	}
}

// Drop is one rolled stack. Equipment drops carry the unique instance (quantity 1).
type Drop struct {
	ItemID     string `json:"item_id"`
	Quantity   int    `json:"quantity"`
	InstanceID string `json:"instance_id,omitempty"`
	Rarity     string `json:"rarity,omitempty"`
	ItemLevel  int    `json:"item_level,omitempty"`
}

type Rolled struct {
	Gold  int64
	Items []Drop
}

// bp converts canon percent (e.g. 3.5) to basis points (350); canon unit is 10 000 = 100%.
func bp(pct float64) int64 { return int64(math.Round(pct * 100)) }

func hit(r Roller, pct float64) bool {
	p := bp(pct)
	if p <= 0 {
		return false
	}
	if p >= 10000 {
		return true
	}
	return r.Intn(10000) < p
}

// Roll applies canon 11 §9: guaranteed gold in [min,max]×regionCoef, then independent MAT-001, MAT-003
// and special-item rolls. The equipment roll is separate (RollEquipment) because it needs the character class.
func Roll(t Table, regionCoef float64, r Roller) Rolled {
	if regionCoef <= 0 {
		regionCoef = 1
	}
	span := t.GoldMax - t.GoldMin + 1
	base := t.GoldMin + r.Intn(span)
	out := Rolled{Gold: int64(math.Floor(float64(base) * regionCoef)), Items: []Drop{}}
	region := t.RegionID
	if len(region) == 6 { // REG-0N → N
		n := region[5:6]
		if hit(r, t.Mat1Pct) {
			out.Items = append(out.Items, Drop{ItemID: "MAT-0" + n + "-001", Quantity: 1})
		}
		if hit(r, t.Mat3Pct) {
			out.Items = append(out.Items, Drop{ItemID: "MAT-0" + n + "-003", Quantity: 1})
		}
	}
	if t.SpecialItem != "" && hit(r, t.SpecialPct) {
		out.Items = append(out.Items, Drop{ItemID: t.SpecialItem, Quantity: 1})
	}
	return out
}

// RollEquipment performs canon 11 §9's single personal equipment roll: with probability EquipPct a slot is
// chosen uniformly among the 9 slots and a template of the character's OWN class from the table's region
// (never another class' gear). ok=false ⇒ no drop (no hit, no catalog, or no template for that slot).
func RollEquipment(t Table, classID string, cat *equipment.Catalog, r Roller) (equipment.Template, string, bool) {
	if cat == nil || t.EquipRarity == "" || !equipment.ValidRarity(t.EquipRarity) || !hit(r, t.EquipPct) {
		return equipment.Template{}, "", false
	}
	slot := equipment.Slots[r.Intn(int64(len(equipment.Slots)))]
	cands := cat.Candidates(classID, t.RegionID, slot)
	if len(cands) == 0 {
		return equipment.Template{}, "", false
	}
	tpl, ok := cat.Get(cands[r.Intn(int64(len(cands)))])
	return tpl, t.EquipRarity, ok
}
