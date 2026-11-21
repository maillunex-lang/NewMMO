package loot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/NewMMO/server/internal/inventory"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
)

// Config holds balance-neutral interim parameters (contract loot-engine.md §4, conflict C-LOOT-001).
type Config struct {
	RegionCoef   map[string]float64 // REG-0N → multiplier for canon «регион.коэф»; missing = 1.0
	MaxPerMinute int                // accepted loot claims per character per rolling minute
	Regions      quest.RegionConfig
}

func LoadConfig(getenv func(string) string) Config {
	cfg := Config{RegionCoef: map[string]float64{}, MaxPerMinute: 30, Regions: quest.LoadRegionConfig(getenv)}
	if n, err := strconv.Atoi(strings.TrimSpace(getenv("NEWMMO_LOOT_MAX_PER_MINUTE"))); err == nil && n > 0 {
		cfg.MaxPerMinute = n
	}
	if raw := strings.TrimSpace(getenv("NEWMMO_LOOT_REGION_COEF_JSON")); raw != "" {
		var m map[string]float64
		if json.Unmarshal([]byte(raw), &m) == nil {
			for k, v := range m {
				if v > 0 && v <= 10 {
					cfg.RegionCoef[k] = v
				}
			}
		}
	}
	return cfg
}

type Engine struct {
	pool   *pgxpool.Pool
	cat    *Catalog
	cfg    Config
	roller Roller
	now    func() time.Time
}

func NewEngine(pool *pgxpool.Pool, cat *Catalog, cfg Config) *Engine {
	return &Engine{pool: pool, cat: cat, cfg: cfg, roller: CryptoRoller{}, now: time.Now}
}

// WithRoller / WithClock are for tests.
func (e *Engine) WithRoller(r Roller) *Engine          { e.roller = r; return e }
func (e *Engine) WithClock(f func() time.Time) *Engine { e.now = f; return e }

type Result struct {
	EnemyID           string            `json:"enemy_id"`
	LootTableID       string            `json:"loot_table_id"`
	Gold              int64             `json:"gold"`
	Items             []Drop            `json:"items"`
	GoldTotal         int64             `json:"gold_total"`
	InventoryRevision int64             `json:"inventory_revision"`
	Revision          int64             `json:"revision"`
	Changes           []inventory.Stack `json:"changes"`
}

// Claim rolls the drop for one reported kill, server-side, exactly once per request_id.
// INTERIM (R-KILL): the server cannot yet verify the kill (no server combat/position until EX-P09);
// it validates enemy id/category/region and rate-limits claims. See loot-engine.md §1.
func (e *Engine) Claim(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (Result, error) {
	enemyID, _ := payload["enemy_id"].(string)
	if requestID == "" || enemyID == "" || len(payload) != 1 {
		return Result{}, ErrBadRequest
	}
	reqUUID, err := parseUUID(requestID)
	if err != nil {
		return Result{}, ErrBadRequest
	}
	table, ok := e.cat.ForEnemy(enemyID)
	if !ok {
		return Result{}, ErrNotFound
	}
	if !table.Claimable() {
		return Result{}, ErrUnsupported
	}
	if !e.cfg.Regions.RegionOpen(table.RegionID, accountID, e.now()) {
		return Result{}, ErrRegionClosed
	}
	hash := hashPayload(payload)

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return Result{}, ErrInternal
	}
	defer tx.Rollback(ctx)

	var gold, stateRev, invRev int64
	err = tx.QueryRow(ctx, `
		SELECT gold, state_revision, inventory_revision FROM public.characters
		WHERE account_id=$1 AND character_id=$2 FOR UPDATE`, accountID, characterID).Scan(&gold, &stateRev, &invRev)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, ErrInternal
	}

	var storedHash string
	var stored []byte
	err = tx.QueryRow(ctx, `
		SELECT payload_sha256, result FROM public.idempotency_keys
		WHERE character_id=$1 AND request_id=$2`, characterID, reqUUID).Scan(&storedHash, &stored)
	switch {
	case err == nil:
		if storedHash != hash {
			return Result{}, ErrIdempotencyConflict
		}
		var replay Result
		if json.Unmarshal(stored, &replay) != nil {
			return Result{}, ErrInternal
		}
		return replay, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Result{}, ErrInternal
	}

	var recent int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM public.character_loot_ledger
		WHERE character_id=$1 AND created_at > now() - interval '1 minute'`, characterID).Scan(&recent); err != nil {
		return Result{}, ErrInternal
	}
	if recent >= e.cfg.MaxPerMinute {
		return Result{}, ErrRateLimited
	}

	coef := e.cfg.RegionCoef[table.RegionID]
	rolled := Roll(table, coef, e.roller)

	res := Result{EnemyID: enemyID, LootTableID: table.ID, Gold: rolled.Gold, Items: rolled.Items,
		Changes: []inventory.Stack{}}
	for _, d := range rolled.Items {
		changes, newRev, err := inventory.GrantMaterialTx(ctx, tx, characterID, invRev, d.ItemID, d.Quantity)
		if err != nil {
			return Result{}, ErrInternal
		}
		invRev = newRev
		res.Changes = mergeChanges(res.Changes, changes)
	}
	gold += rolled.Gold
	stateRev++
	if _, err := tx.Exec(ctx, `UPDATE public.characters SET gold=$1, state_revision=$2 WHERE character_id=$3`,
		gold, stateRev, characterID); err != nil {
		return Result{}, ErrInternal
	}
	res.GoldTotal, res.InventoryRevision, res.Revision = gold, invRev, stateRev

	itemsJSON, _ := json.Marshal(rolled.Items)
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.character_loot_ledger
			(character_id, request_id, enemy_id, loot_table_id, gold, items, gold_after)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)`,
		characterID, reqUUID, enemyID, table.ID, rolled.Gold, string(itemsJSON), gold); err != nil {
		return Result{}, ErrInternal
	}
	raw, _ := json.Marshal(res)
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.idempotency_keys (character_id, request_id, message_type, payload_sha256, result)
		VALUES ($1,$2,$3,$4,$5::jsonb)`, characterID, reqUUID, typ, hash, string(raw)); err != nil {
		return Result{}, ErrInternal
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, ErrInternal
	}
	return res, nil
}

// mergeChanges keeps the last state per (container,slot).
func mergeChanges(have, add []inventory.Stack) []inventory.Stack {
	for _, a := range add {
		replaced := false
		for i := range have {
			if have[i].Container == a.Container && have[i].Slot == a.Slot {
				have[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			have = append(have, a)
		}
	}
	return have
}

func hashPayload(payload map[string]any) string {
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
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
