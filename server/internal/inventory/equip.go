package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

// ItemDetail is the server-rolled data of one equipment instance (never client-authored).
type ItemDetail struct {
	TemplateID    string         `json:"template_id"`
	Rarity        string         `json:"rarity"`
	ItemLevel     int            `json:"item_level"`
	RequiredLevel int            `json:"required_level"`
	Stats         map[string]int `json:"stats"`
}

// EquippedItem is one worn slot; InstanceID "" in a delta means the slot is now empty.
type EquippedItem struct {
	Slot       string `json:"slot"`
	InstanceID string `json:"instance_id"`
}

// slotOf / classOf read the canon template id EQ-CC-RR-SS-NN.
func slotOf(templateID string) string {
	parts := strings.Split(templateID, "-")
	if len(parts) != 5 || parts[0] != "EQ" {
		return ""
	}
	return parts[3]
}

func classOf(templateID string) string {
	parts := strings.Split(templateID, "-")
	if len(parts) != 5 || parts[0] != "EQ" {
		return ""
	}
	return "CLS-" + parts[1]
}

func validGearSlot(s string) bool {
	switch s {
	case "MH", "OH", "HD", "CH", "HA", "LG", "FT", "RG", "AM":
		return true
	}
	return false
}

func gearScore(items map[string]ItemDetail, equipped []EquippedItem) int {
	total := 0
	for _, w := range equipped {
		for _, v := range items[w.InstanceID].Stats {
			total += v
		}
	}
	return total
}

// loadGear returns worn slots, details for every gear instance the character owns (bag + worn) and the gear
// score (canon 11 §4: informational sum of the equipped stat budgets).
func loadGear(ctx context.Context, tx pgx.Tx, characterID int64) ([]EquippedItem, map[string]ItemDetail, int, error) {
	items := map[string]ItemDetail{}
	rows, err := tx.Query(ctx, `
		SELECT i.instance_id::text, i.template_id, i.rarity, i.item_level, i.required_level, i.stats
		FROM public.character_equipment_items i
		WHERE i.character_id=$1
		  AND (EXISTS (SELECT 1 FROM public.character_inventory v WHERE v.instance_id=i.instance_id)
		    OR EXISTS (SELECT 1 FROM public.character_equipped w WHERE w.instance_id=i.instance_id))`, characterID)
	if err != nil {
		return nil, nil, 0, ErrInternal
	}
	for rows.Next() {
		var id string
		var d ItemDetail
		var raw []byte
		if err := rows.Scan(&id, &d.TemplateID, &d.Rarity, &d.ItemLevel, &d.RequiredLevel, &raw); err != nil {
			rows.Close()
			return nil, nil, 0, ErrInternal
		}
		if json.Unmarshal(raw, &d.Stats) != nil {
			rows.Close()
			return nil, nil, 0, ErrInternal
		}
		items[id] = d
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, nil, 0, ErrInternal
	}
	eq := []EquippedItem{}
	wr, err := tx.Query(ctx, `SELECT slot, instance_id::text FROM public.character_equipped WHERE character_id=$1 ORDER BY slot`, characterID)
	if err != nil {
		return nil, nil, 0, ErrInternal
	}
	for wr.Next() {
		var w EquippedItem
		if err := wr.Scan(&w.Slot, &w.InstanceID); err != nil {
			wr.Close()
			return nil, nil, 0, ErrInternal
		}
		eq = append(eq, w)
	}
	wr.Close()
	if wr.Err() != nil {
		return nil, nil, 0, ErrInternal
	}
	return eq, items, gearScore(items, eq), nil
}

// Equip wears a bag instance. Order (equipment-engine.md §4): ownership (row lives in this character's bag)
// → class → required level → slot swap (the previous item takes the freed bag slot) → revision++ → idempotent result.
func (e *Engine) Equip(ctx context.Context, accountID string, characterID int64, env Envelope) (Delta, error) {
	instID, _ := env.Payload["instance_id"].(string)
	if env.RequestID == "" || len(env.Payload) != 1 {
		return Delta{}, ErrBadRequest
	}
	instID, err := parseUUID(instID)
	if err != nil {
		return Delta{}, ErrBadRequest
	}
	return e.gearTx(ctx, accountID, characterID, env, func(tx pgx.Tx, c charInfo) (Delta, error) {
		var templateID string
		var slotInBag int
		err := tx.QueryRow(ctx, `
			SELECT v.item_id, v.slot FROM public.character_inventory v
			JOIN public.character_equipment_items i ON i.instance_id = v.instance_id
			WHERE v.character_id=$1 AND v.container='bag' AND v.instance_id=$2 AND i.character_id=$1`,
			characterID, instID).Scan(&templateID, &slotInBag)
		if errors.Is(err, pgx.ErrNoRows) {
			return Delta{}, ErrNotFound
		}
		if err != nil {
			return Delta{}, ErrInternal
		}
		var required int
		if err := tx.QueryRow(ctx, `SELECT required_level FROM public.character_equipment_items WHERE instance_id=$1`, instID).Scan(&required); err != nil {
			return Delta{}, ErrInternal
		}
		slot := slotOf(templateID)
		if slot == "" {
			return Delta{}, ErrInternal
		}
		if classOf(templateID) != c.classID {
			return Delta{}, ErrWrongClass
		}
		if c.level < required {
			return Delta{}, ErrLevelTooLow
		}
		var prev string
		err = tx.QueryRow(ctx, `SELECT instance_id::text FROM public.character_equipped WHERE character_id=$1 AND slot=$2`, characterID, slot).Scan(&prev)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Delta{}, ErrInternal
		}
		newRev := c.invRev + 1
		if _, err := tx.Exec(ctx, `DELETE FROM public.character_inventory WHERE character_id=$1 AND container='bag' AND slot=$2`, characterID, slotInBag); err != nil {
			return Delta{}, ErrInternal
		}
		changes := []Stack{{Container: "bag", Slot: slotInBag, ItemID: templateID, Quantity: 0}}
		if prev != "" {
			var prevTpl string
			if err := tx.QueryRow(ctx, `SELECT template_id FROM public.character_equipment_items WHERE instance_id=$1`, prev).Scan(&prevTpl); err != nil {
				return Delta{}, ErrInternal
			}
			if _, err := tx.Exec(ctx, `DELETE FROM public.character_equipped WHERE character_id=$1 AND slot=$2`, characterID, slot); err != nil {
				return Delta{}, ErrInternal
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.character_inventory
				(character_id, container, slot, item_id, quantity, inventory_revision, instance_id)
				VALUES ($1,'bag',$2,$3,1,$4,$5)`, characterID, slotInBag, prevTpl, newRev, prev); err != nil {
				return Delta{}, ErrInternal
			}
			changes = []Stack{{Container: "bag", Slot: slotInBag, ItemID: prevTpl, Quantity: 1, InstanceID: prev}}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.character_equipped (character_id, slot, instance_id) VALUES ($1,$2,$3)`, characterID, slot, instID); err != nil {
			return Delta{}, ErrInternal
		}
		return e.finishGear(ctx, tx, characterID, newRev, changes, []EquippedItem{{Slot: slot, InstanceID: instID}})
	})
}

// Unequip takes a worn slot back into the first free bag slot (STATE_CONFLICT when the bag is full).
func (e *Engine) Unequip(ctx context.Context, accountID string, characterID int64, env Envelope) (Delta, error) {
	slot, _ := env.Payload["slot"].(string)
	if env.RequestID == "" || len(env.Payload) != 1 || !validGearSlot(slot) {
		return Delta{}, ErrBadRequest
	}
	return e.gearTx(ctx, accountID, characterID, env, func(tx pgx.Tx, c charInfo) (Delta, error) {
		var inst, tpl string
		err := tx.QueryRow(ctx, `
			SELECT w.instance_id::text, i.template_id FROM public.character_equipped w
			JOIN public.character_equipment_items i ON i.instance_id = w.instance_id
			WHERE w.character_id=$1 AND w.slot=$2`, characterID, slot).Scan(&inst, &tpl)
		if errors.Is(err, pgx.ErrNoRows) {
			return Delta{}, ErrNotFound
		}
		if err != nil {
			return Delta{}, ErrInternal
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.character_equipped WHERE character_id=$1 AND slot=$2`, characterID, slot); err != nil {
			return Delta{}, ErrInternal
		}
		stack, newRev, err := GrantInstanceTx(ctx, tx, characterID, c.invRev, tpl, inst)
		if err != nil {
			return Delta{}, err // ErrStateConflict ⇒ bag full; the whole transaction rolls back
		}
		return e.finishGear(ctx, tx, characterID, newRev, []Stack{stack}, []EquippedItem{{Slot: slot, InstanceID: ""}})
	})
}

type charInfo struct {
	classID string
	level   int
	invRev  int64
}

func (e *Engine) finishGear(ctx context.Context, tx pgx.Tx, characterID, newRev int64, changes []Stack, worn []EquippedItem) (Delta, error) {
	if _, err := tx.Exec(ctx, `UPDATE public.characters SET inventory_revision=$1 WHERE character_id=$2 AND inventory_revision<$1`, newRev, characterID); err != nil {
		return Delta{}, ErrInternal
	}
	_, items, score, err := loadGear(ctx, tx, characterID)
	if err != nil {
		return Delta{}, err
	}
	out := Delta{InventoryRevision: newRev, Changes: changes, Equipped: worn, Items: items, GearScore: &score}
	return out, nil
}

// gearTx runs fn under the character row lock with idempotency, like Move.
func (e *Engine) gearTx(ctx context.Context, accountID string, characterID int64, env Envelope, fn func(pgx.Tx, charInfo) (Delta, error)) (Delta, error) {
	hash := hashPayload(env.Payload)
	var out Delta
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return out, ErrInternal
	}
	defer tx.Rollback(ctx)
	var c charInfo
	var xp int64
	err = tx.QueryRow(ctx, `SELECT inventory_revision, class_id, xp FROM public.characters
		WHERE account_id=$1 AND character_id=$2 FOR UPDATE`, accountID, characterID).Scan(&c.invRev, &c.classID, &xp)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, ErrInternal
	}
	c.level = auth.Level(xp)
	if replay, err := e.checkIdempotency(ctx, tx, characterID, env, hash, &out); err != nil {
		return Delta{}, err
	} else if replay {
		return out, nil
	}
	if out, err = fn(tx, c); err != nil {
		return Delta{}, err
	}
	if err := e.storeIdempotency(ctx, tx, characterID, env, hash, out); err != nil {
		return Delta{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Delta{}, ErrInternal
	}
	return out, nil
}
