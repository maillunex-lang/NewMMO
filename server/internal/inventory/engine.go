package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	BagSlots       = 80
	MaterialsSlots = 200
	MATMaxStack    = 999
)

type Engine struct {
	pool *pgxpool.Pool
}

func NewEngine(pool *pgxpool.Pool) *Engine { return &Engine{pool: pool} }

type Stack struct {
	Container string `json:"container"`
	Slot      int    `json:"slot"`
	ItemID    string `json:"item_id"`
	Quantity  int    `json:"quantity"`
	// InstanceID is set for unique equipment rows (quantity is always 1); details live in the
	// equipment package (T-039a). Plain stacks omit it.
	InstanceID string `json:"instance_id,omitempty"`
}

type Scope struct {
	Revision  int64    `json:"inventory_revision"`
	Bag       []Stack  `json:"bag"`
	Materials []Stack  `json:"materials"`
	QuestKeys []string `json:"quest_keys"`
	// T-039b: worn gear, details of every gear instance (bag + worn) and the informational gear score.
	Equipped  []EquippedItem        `json:"equipped"`
	Items     map[string]ItemDetail `json:"items"`
	GearScore int                   `json:"gear_score"`
}

type Delta struct {
	InventoryRevision int64   `json:"inventory_revision"`
	Changes           []Stack `json:"changes"`
	// Equip/unequip only: changed worn slots (instance_id "" = now empty), details of instances that
	// appear, and the new gear score.
	Equipped  []EquippedItem        `json:"equipped,omitempty"`
	Items     map[string]ItemDetail `json:"items,omitempty"`
	GearScore *int                  `json:"gear_score,omitempty"`
}

type Envelope struct {
	Type, RequestID string
	Payload         map[string]any
}

func (e *Engine) Snapshot(ctx context.Context, accountID string, characterID int64) (Scope, error) {
	var out Scope
	err := e.withCharacter(ctx, accountID, characterID, func(tx pgx.Tx, invRev int64) error {
		out.Revision = invRev
		bag, mats, err := e.loadStacks(ctx, tx, characterID)
		if err != nil {
			return err
		}
		out.Bag, out.Materials = bag, mats
		if out.Equipped, out.Items, out.GearScore, err = loadGear(ctx, tx, characterID); err != nil {
			return err
		}
		keys, err := e.loadQuestKeys(ctx, tx, characterID)
		if err != nil {
			return err
		}
		out.QuestKeys = keys
		return nil
	})
	return out, err
}

// GrantMaterial stacks MAT into materials (overflow bag). Used by tests and future loot.
func (e *Engine) GrantMaterial(ctx context.Context, accountID string, characterID int64, itemID string, qty int) (Delta, error) {
	if !strings.HasPrefix(itemID, "MAT-") || qty < 1 || qty > MATMaxStack*MaterialsSlots {
		return Delta{}, ErrBadRequest
	}
	var out Delta
	err := e.withCharacter(ctx, accountID, characterID, func(tx pgx.Tx, invRev int64) error {
		changes, newRev, err := GrantMaterialTx(ctx, tx, characterID, invRev, itemID, qty)
		if err != nil {
			return err
		}
		out = Delta{InventoryRevision: newRev, Changes: changes}
		return nil
	})
	return out, err
}

// GrantMaterialTx stacks MAT into an already-open transaction that holds the character row lock
// (SELECT … FOR UPDATE) and knows the current inventory_revision. It returns the changed stacks and
// the new revision. Callers (loot, future crafting) own commit/rollback.
func GrantMaterialTx(ctx context.Context, tx pgx.Tx, characterID int64, invRev int64, itemID string, qty int) ([]Stack, int64, error) {
	if !strings.HasPrefix(itemID, "MAT-") || qty < 1 || qty > MATMaxStack*MaterialsSlots {
		return nil, invRev, ErrBadRequest
	}
	remaining := qty
	changes := []Stack{}
	for remaining > 0 {
		container := "materials"
		maxSlots := MaterialsSlots
		stack, slot, err := findStackOrSlot(ctx, tx, characterID, container, itemID, maxSlots)
		if err == ErrStateConflict {
			container = "bag"
			maxSlots = BagSlots
			stack, slot, err = findStackOrSlot(ctx, tx, characterID, container, itemID, maxSlots)
		}
		if err != nil {
			return nil, invRev, err
		}
		add := remaining
		if stack.Quantity+add > MATMaxStack {
			add = MATMaxStack - stack.Quantity
		}
		if add <= 0 {
			return nil, invRev, ErrStateConflict
		}
		newQty := stack.Quantity + add
		newRev := invRev + 1
		if stack.Quantity == 0 {
			_, err = tx.Exec(ctx, `
				INSERT INTO public.character_inventory
					(character_id, container, slot, item_id, quantity, inventory_revision)
				VALUES ($1,$2,$3,$4,$5,$6)`, characterID, container, slot, itemID, newQty, newRev)
		} else {
			_, err = tx.Exec(ctx, `
				UPDATE public.character_inventory
				SET quantity=$1, inventory_revision=$2, updated_at=now()
				WHERE character_id=$3 AND container=$4 AND slot=$5`,
				newQty, newRev, characterID, container, slot)
		}
		if err != nil {
			return nil, invRev, ErrInternal
		}
		if _, err := tx.Exec(ctx, `UPDATE public.characters SET inventory_revision=$1 WHERE character_id=$2`, newRev, characterID); err != nil {
			return nil, invRev, ErrInternal
		}
		invRev = newRev
		changes = append(changes, Stack{Container: container, Slot: slot, ItemID: itemID, Quantity: newQty})
		remaining -= add
	}
	return changes, invRev, nil
}

func (e *Engine) Move(ctx context.Context, accountID string, characterID int64, env Envelope) (Delta, error) {
	fromC, _ := env.Payload["from_container"].(string)
	toC, _ := env.Payload["to_container"].(string)
	fromSlot := intPayload(env.Payload["from_slot"])
	toSlot := intPayload(env.Payload["to_slot"])
	qty := intPayload(env.Payload["quantity"])
	if env.RequestID == "" || (fromC != "bag" && fromC != "materials") || (toC != "bag" && toC != "materials") {
		return Delta{}, ErrBadRequest
	}
	if fromSlot < 0 || toSlot < 0 {
		return Delta{}, ErrBadRequest
	}
	payloadHash := hashPayload(env.Payload)
	var out Delta
	err := e.withCharacter(ctx, accountID, characterID, func(tx pgx.Tx, invRev int64) error {
		if replay, err := e.checkIdempotency(ctx, tx, characterID, env, payloadHash, &out); err != nil {
			return err
		} else if replay {
			return nil
		}
		var itemID, instanceID string
		var have int
		err := tx.QueryRow(ctx, `
			SELECT item_id, quantity, coalesce(instance_id::text, '') FROM public.character_inventory
			WHERE character_id=$1 AND container=$2 AND slot=$3`, characterID, fromC, fromSlot).Scan(&itemID, &have, &instanceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return ErrInternal
		}
		if instanceID != "" {
			return moveInstance(ctx, tx, e, characterID, invRev, env, payloadHash, fromC, fromSlot, toC, toSlot, itemID, instanceID, &out)
		}
		moveQty := have
		if qty > 0 && qty < have {
			moveQty = qty
		}
		if strings.HasPrefix(itemID, "MAT-") && toC == "materials" {
			// ok
		} else if toC == "materials" && !strings.HasPrefix(itemID, "MAT-") {
			return ErrStateConflict
		}
		newRev := invRev + 1
		// Clear/reduce source.
		left := have - moveQty
		if left == 0 {
			if _, err := tx.Exec(ctx, `
				DELETE FROM public.character_inventory
				WHERE character_id=$1 AND container=$2 AND slot=$3`, characterID, fromC, fromSlot); err != nil {
				return ErrInternal
			}
		} else {
			if _, err := tx.Exec(ctx, `
				UPDATE public.character_inventory SET quantity=$1, inventory_revision=$2, updated_at=now()
				WHERE character_id=$3 AND container=$4 AND slot=$5`, left, newRev, characterID, fromC, fromSlot); err != nil {
				return ErrInternal
			}
		}
		// Destination: stack or place.
		var destItem string
		var destQty int
		err = tx.QueryRow(ctx, `
			SELECT item_id, quantity FROM public.character_inventory
			WHERE character_id=$1 AND container=$2 AND slot=$3`, characterID, toC, toSlot).Scan(&destItem, &destQty)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if _, err := tx.Exec(ctx, `
				INSERT INTO public.character_inventory
					(character_id, container, slot, item_id, quantity, inventory_revision)
				VALUES ($1,$2,$3,$4,$5,$6)`, characterID, toC, toSlot, itemID, moveQty, newRev); err != nil {
				return ErrInternal
			}
			destQty = moveQty
			destItem = itemID
		case err != nil:
			return ErrInternal
		default:
			if destItem != itemID {
				return ErrStateConflict
			}
			if destQty+moveQty > MATMaxStack {
				return ErrStateConflict
			}
			destQty += moveQty
			if _, err := tx.Exec(ctx, `
				UPDATE public.character_inventory SET quantity=$1, inventory_revision=$2, updated_at=now()
				WHERE character_id=$3 AND container=$4 AND slot=$5`, destQty, newRev, characterID, toC, toSlot); err != nil {
				return ErrInternal
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.characters SET inventory_revision=$1 WHERE character_id=$2`, newRev, characterID); err != nil {
			return ErrInternal
		}
		out = Delta{
			InventoryRevision: newRev,
			Changes: []Stack{
				{Container: fromC, Slot: fromSlot, ItemID: itemID, Quantity: left},
				{Container: toC, Slot: toSlot, ItemID: destItem, Quantity: destQty},
			},
		}
		return e.storeIdempotency(ctx, tx, characterID, env, payloadHash, out)
	})
	return out, err
}

func findStackOrSlot(ctx context.Context, tx pgx.Tx, characterID int64, container, itemID string, maxSlots int) (Stack, int, error) {
	var slot, qty int
	err := tx.QueryRow(ctx, `
		SELECT slot, quantity FROM public.character_inventory
		WHERE character_id=$1 AND container=$2 AND item_id=$3 AND instance_id IS NULL
		ORDER BY slot LIMIT 1`, characterID, container, itemID).Scan(&slot, &qty)
	if err == nil {
		if qty >= MATMaxStack {
			return Stack{}, 0, ErrStateConflict
		}
		return Stack{Container: container, Slot: slot, ItemID: itemID, Quantity: qty}, slot, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Stack{}, 0, ErrInternal
	}
	occupied := map[int]bool{}
	rows, err := tx.Query(ctx, `
		SELECT slot FROM public.character_inventory WHERE character_id=$1 AND container=$2`, characterID, container)
	if err != nil {
		return Stack{}, 0, ErrInternal
	}
	defer rows.Close()
	for rows.Next() {
		var s int
		if err := rows.Scan(&s); err != nil {
			return Stack{}, 0, ErrInternal
		}
		occupied[s] = true
	}
	for s := 0; s < maxSlots; s++ {
		if !occupied[s] {
			return Stack{Container: container, Slot: s, ItemID: itemID, Quantity: 0}, s, nil
		}
	}
	return Stack{}, 0, ErrStateConflict
}

func (e *Engine) loadStacks(ctx context.Context, tx pgx.Tx, characterID int64) ([]Stack, []Stack, error) {
	rows, err := tx.Query(ctx, `
		SELECT container, slot, item_id, quantity, coalesce(instance_id::text, '') FROM public.character_inventory
		WHERE character_id=$1 ORDER BY container, slot`, characterID)
	if err != nil {
		return nil, nil, ErrInternal
	}
	defer rows.Close()
	var bag, mats []Stack
	for rows.Next() {
		var s Stack
		if err := rows.Scan(&s.Container, &s.Slot, &s.ItemID, &s.Quantity, &s.InstanceID); err != nil {
			return nil, nil, ErrInternal
		}
		if s.Container == "materials" {
			mats = append(mats, s)
		} else {
			bag = append(bag, s)
		}
	}
	if bag == nil {
		bag = []Stack{}
	}
	if mats == nil {
		mats = []Stack{}
	}
	return bag, mats, rows.Err()
}

func (e *Engine) loadQuestKeys(ctx context.Context, tx pgx.Tx, characterID int64) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT item_id FROM public.character_quest_items WHERE character_id=$1 ORDER BY item_id`, characterID)
	if err != nil {
		return nil, ErrInternal
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, ErrInternal
		}
		keys = append(keys, id)
	}
	if keys == nil {
		keys = []string{}
	}
	return keys, rows.Err()
}

func (e *Engine) withCharacter(ctx context.Context, accountID string, characterID int64, fn func(pgx.Tx, int64) error) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return ErrInternal
	}
	defer tx.Rollback(ctx)
	var invRev int64
	err = tx.QueryRow(ctx, `
		SELECT inventory_revision FROM public.characters
		WHERE account_id=$1 AND character_id=$2 FOR UPDATE`, accountID, characterID).Scan(&invRev)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return ErrInternal
	}
	if err := fn(tx, invRev); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (e *Engine) checkIdempotency(ctx context.Context, tx pgx.Tx, characterID int64, env Envelope, payloadHash string, dest any) (bool, error) {
	requestUUID, err := parseUUID(env.RequestID)
	if err != nil {
		return false, ErrBadRequest
	}
	var storedHash string
	var result []byte
	err = tx.QueryRow(ctx, `
		SELECT payload_sha256, result FROM public.idempotency_keys
		WHERE character_id=$1 AND request_id=$2`, characterID, requestUUID).Scan(&storedHash, &result)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, ErrInternal
	}
	if storedHash != payloadHash {
		return false, ErrIdempotencyConflict
	}
	if err := json.Unmarshal(result, dest); err != nil {
		return false, ErrInternal
	}
	return true, nil
}

func (e *Engine) storeIdempotency(ctx context.Context, tx pgx.Tx, characterID int64, env Envelope, payloadHash string, result any) error {
	requestUUID, err := parseUUID(env.RequestID)
	if err != nil {
		return ErrBadRequest
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return ErrInternal
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO public.idempotency_keys (character_id, request_id, message_type, payload_sha256, result)
		VALUES ($1,$2,$3,$4,$5::jsonb)`, characterID, requestUUID, env.Type, payloadHash, string(raw))
	if err != nil {
		return ErrInternal
	}
	return nil
}

func hashPayload(payload map[string]any) string {
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func intPayload(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func parseUUID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return "", ErrBadRequest
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", ErrBadRequest
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return "", ErrBadRequest
			}
		}
	}
	return strings.ToLower(s), nil
}

// moveInstance repositions a unique equipment row inside the bag. Gear never stacks, never swaps and never
// enters the materials pocket: the destination must be an empty bag slot.
func moveInstance(ctx context.Context, tx pgx.Tx, e *Engine, characterID, invRev int64, env Envelope, payloadHash,
	fromC string, fromSlot int, toC string, toSlot int, itemID, instanceID string, out *Delta) error {
	if fromC != "bag" || toC != "bag" || toSlot >= BagSlots {
		return ErrStateConflict
	}
	var taken int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.character_inventory
		WHERE character_id=$1 AND container='bag' AND slot=$2`, characterID, toSlot).Scan(&taken); err != nil {
		return ErrInternal
	}
	if taken > 0 && toSlot != fromSlot {
		return ErrStateConflict
	}
	newRev := invRev + 1
	if toSlot != fromSlot {
		if _, err := tx.Exec(ctx, `UPDATE public.character_inventory SET slot=$1, inventory_revision=$2, updated_at=now()
			WHERE character_id=$3 AND container='bag' AND slot=$4`, toSlot, newRev, characterID, fromSlot); err != nil {
			return ErrInternal
		}
		if _, err := tx.Exec(ctx, `UPDATE public.characters SET inventory_revision=$1 WHERE character_id=$2`, newRev, characterID); err != nil {
			return ErrInternal
		}
	} else {
		newRev = invRev
	}
	*out = Delta{InventoryRevision: newRev, Changes: []Stack{
		{Container: "bag", Slot: fromSlot, ItemID: itemID, Quantity: 0},
		{Container: "bag", Slot: toSlot, ItemID: itemID, Quantity: 1, InstanceID: instanceID},
	}}
	return e.storeIdempotency(ctx, tx, characterID, env, payloadHash, *out)
}

// GrantInstanceTx puts one equipment instance into the first free bag slot (caller holds the character lock
// and owns the transaction). The instance row must already exist. ErrStateConflict ⇒ bag full.
func GrantInstanceTx(ctx context.Context, tx pgx.Tx, characterID, invRev int64, templateID, instanceID string) (Stack, int64, error) {
	rows, err := tx.Query(ctx, `SELECT slot FROM public.character_inventory WHERE character_id=$1 AND container='bag'`, characterID)
	if err != nil {
		return Stack{}, invRev, ErrInternal
	}
	occupied := map[int]bool{}
	for rows.Next() {
		var sl int
		if err := rows.Scan(&sl); err != nil {
			rows.Close()
			return Stack{}, invRev, ErrInternal
		}
		occupied[sl] = true
	}
	rows.Close()
	if rows.Err() != nil {
		return Stack{}, invRev, ErrInternal
	}
	slot := -1
	for s := 0; s < BagSlots; s++ {
		if !occupied[s] {
			slot = s
			break
		}
	}
	if slot < 0 {
		return Stack{}, invRev, ErrStateConflict
	}
	newRev := invRev + 1
	if _, err := tx.Exec(ctx, `INSERT INTO public.character_inventory
		(character_id, container, slot, item_id, quantity, inventory_revision, instance_id)
		VALUES ($1,'bag',$2,$3,1,$4,$5)`, characterID, slot, templateID, newRev, instanceID); err != nil {
		return Stack{}, invRev, ErrInternal
	}
	if _, err := tx.Exec(ctx, `UPDATE public.characters SET inventory_revision=$1 WHERE character_id=$2`, newRev, characterID); err != nil {
		return Stack{}, invRev, ErrInternal
	}
	return Stack{Container: "bag", Slot: slot, ItemID: templateID, Quantity: 1, InstanceID: instanceID}, newRev, nil
}
