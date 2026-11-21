package dungeon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/inventory"
	"github.com/maillunex-lang/NewMMO/server/internal/loot"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
)

type Config struct {
	Regions    quest.RegionConfig
	RegionCoef map[string]float64
}

type Engine struct {
	pool   *pgxpool.Pool
	defs   *Registry
	loot   *loot.Catalog
	cfg    Config
	roller loot.Roller
	now    func() time.Time
}

func NewEngine(pool *pgxpool.Pool, defs *Registry, lootCat *loot.Catalog, cfg Config) *Engine {
	return &Engine{pool: pool, defs: defs, loot: lootCat, cfg: cfg, roller: loot.CryptoRoller{}, now: time.Now}
}

func (e *Engine) WithRoller(r loot.Roller) *Engine     { e.roller = r; return e }
func (e *Engine) WithClock(f func() time.Time) *Engine { e.now = f; return e }

// ValidateAgainstContent checks that every enemy and reward table a definition names exists in the
// loaded loot catalog (call at startup after the catalog is loaded from the published build).
func (e *Engine) ValidateAgainstContent() error {
	for _, id := range e.defs.IDs() {
		d, _ := e.defs.Get(id)
		for _, r := range d.Rooms {
			for _, s := range r.Spawns {
				if _, ok := e.loot.ForEnemy(s.EnemyID); !ok {
					return errors.New(id + ": enemy without loot table: " + s.EnemyID)
				}
			}
		}
		for _, t := range d.Rewards.Tables {
			found := false
			for _, enemy := range []string{d.BossID, d.MiniBossID} {
				if tb, ok := e.loot.ForEnemy(enemy); ok && tb.ID == t {
					found = true
				}
			}
			if !found {
				return errors.New(id + ": reward table not mapped to boss/miniboss: " + t)
			}
		}
	}
	return nil
}

type RoomView struct {
	State          string          `json:"state"`
	Objects        map[string]bool `json:"objects"`
	Spawns         map[string]bool `json:"spawns"`
	PuzzleProgress int             `json:"puzzle_progress,omitempty"`
}

type Snapshot struct {
	InstanceID string              `json:"instance_id"`
	DungeonID  string              `json:"dungeon_id"`
	Mode       string              `json:"mode"`
	Status     string              `json:"status"`
	RoomID     string              `json:"room_id"`
	Checkpoint string              `json:"checkpoint_room_id"`
	EventSeq   int64               `json:"event_seq"`
	Rooms      map[string]RoomView `json:"rooms"`
	Attempts   int                 `json:"attempts"`
	Revision   int64               `json:"revision"`
}

type Reward struct {
	InstanceID        string            `json:"instance_id"`
	Gold              int64             `json:"gold"`
	Items             []loot.Drop       `json:"items"`
	FirstClear        bool              `json:"first_clear"`
	GoldTotal         int64             `json:"gold_total"`
	InventoryRevision int64             `json:"inventory_revision"`
	Revision          int64             `json:"revision"`
	Changes           []inventory.Stack `json:"changes"`
}

type instance struct {
	id          string
	characterID int64
	dungeonID   string
	mode        string
	eventSeq    int64
	seed        int64
	lastSeen    time.Time
	m           *Machine
}

// Handle dispatches one dungeon.* client message; it returns the S→C message type and payload.
func (e *Engine) Handle(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (string, any, error) {
	reqUUID, err := parseUUID(requestID)
	if err != nil {
		return "", nil, ErrBadRequest
	}
	hash := hashPayload(payload)
	reply := "dungeon.snapshot"
	if typ == "dungeon.claim.request" {
		reply = "dungeon.reward"
	}
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return "", nil, ErrInternal
	}
	defer tx.Rollback(ctx)

	var rev int64
	var xp, gold int64
	err = tx.QueryRow(ctx, `SELECT xp, gold, state_revision FROM public.characters
		WHERE account_id=$1 AND character_id=$2 FOR UPDATE`, accountID, characterID).Scan(&xp, &gold, &rev)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, ErrInternal
	}
	var storedHash string
	var stored []byte
	switch err := tx.QueryRow(ctx, `SELECT payload_sha256, result FROM public.idempotency_keys
		WHERE character_id=$1 AND request_id=$2`, characterID, reqUUID).Scan(&storedHash, &stored); {
	case err == nil:
		if storedHash != hash {
			return "", nil, ErrIdempotencyConflict
		}
		var replay any
		if json.Unmarshal(stored, &replay) != nil {
			return "", nil, ErrInternal
		}
		return reply, replay, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return "", nil, ErrInternal
	}

	var result any
	switch typ {
	case "dungeon.enter.request":
		result, err = e.enter(ctx, tx, accountID, characterID, xp, rev, payload)
	case "dungeon.interact.request":
		result, err = e.mutate(ctx, tx, characterID, rev, payload, func(def Definition, in *instance) ([]Event, error) {
			obj, _ := payload["object_id"].(string)
			if obj == "" || len(payload) != 2 {
				return nil, ErrBadRequest
			}
			return in.m.Interact(def, obj)
		})
	case "dungeon.encounter.report":
		result, err = e.mutate(ctx, tx, characterID, rev, payload, func(def Definition, in *instance) ([]Event, error) {
			outcome, _ := payload["outcome"].(string)
			spawn, _ := payload["spawn_id"].(string)
			switch outcome {
			case "enemy_defeated":
				if spawn == "" {
					return nil, ErrBadRequest
				}
				return in.m.EnemyDefeated(def, spawn)
			case "player_defeated":
				return in.m.PlayerDefeated(def)
			}
			return nil, ErrBadRequest
		})
	case "dungeon.leave.request":
		result, err = e.leave(ctx, tx, characterID, rev, payload)
	case "dungeon.claim.request":
		result, err = e.claim(ctx, tx, characterID, rev, payload)
	default:
		return "", nil, ErrBadRequest
	}
	if err != nil {
		return "", nil, err
	}
	raw, _ := json.Marshal(result)
	if _, err := tx.Exec(ctx, `INSERT INTO public.idempotency_keys (character_id, request_id, message_type, payload_sha256, result)
		VALUES ($1,$2,$3,$4,$5::jsonb)`, characterID, reqUUID, typ, hash, string(raw)); err != nil {
		return "", nil, ErrInternal
	}
	if err := tx.Commit(ctx); err != nil {
		return "", nil, ErrInternal
	}
	return reply, result, nil
}

func (e *Engine) idle(def Definition) time.Duration {
	return time.Duration(def.IdleExpiryMinutes) * time.Minute
}

func (e *Engine) enter(ctx context.Context, tx pgx.Tx, accountID string, characterID, xp, rev int64, payload map[string]any) (any, error) {
	dungeonID, _ := payload["dungeon_id"].(string)
	mode, _ := payload["mode"].(string)
	def, ok := e.defs.Get(dungeonID)
	if !ok {
		return nil, ErrNotFound
	}
	if mode == "" {
		mode = "NORMAL"
	}
	for k := range payload {
		if k != "dungeon_id" && k != "mode" && k != "party_id" {
			return nil, ErrBadRequest
		}
	}
	if !def.HasMode(mode) {
		return nil, ErrBadRequest
	}
	// Reconnect: return the existing live instance (expire it first when idle too long).
	if in, err := e.loadLive(ctx, tx, characterID); err == nil {
		if e.now().Sub(in.lastSeen) > e.idle(def) && in.m.Status != StatusCompleted {
			if err := e.finish(ctx, tx, in, StatusExpired, "INSTANCE_EXPIRED"); err != nil {
				return nil, err
			}
		} else {
			if in.dungeonID != dungeonID {
				return nil, ErrStateConflict // already inside another dungeon
			}
			if err := e.touch(ctx, tx, in); err != nil {
				return nil, err
			}
			return e.snapshot(def, in, rev), nil
		}
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if auth.Level(xp) < def.EntryLevel {
		return nil, ErrLevelTooLow
	}
	if !e.cfg.Regions.RegionOpen(def.RegionID, accountID, e.now()) {
		return nil, ErrRegionClosed
	}
	m := NewMachine(def)
	state, _ := json.Marshal(m.Rooms)
	in := &instance{characterID: characterID, dungeonID: dungeonID, mode: mode, m: m, seed: e.now().UnixNano()}
	err := tx.QueryRow(ctx, `
		INSERT INTO public.dungeon_instances (character_id, dungeon_id, mode, status, room_id, checkpoint_room_id, state, seed, last_seen_at)
		VALUES ($1,$2,$3,'ACTIVE',$4,$5,$6::jsonb,$7,$8) RETURNING instance_id::text`,
		characterID, dungeonID, mode, m.RoomID, m.Checkpoint, string(state), in.seed, e.now()).Scan(&in.id)
	if err != nil {
		return nil, ErrStateConflict // unique live-instance index lost a race
	}
	in.lastSeen = e.now()
	if err := e.logEvents(ctx, tx, in, []Event{{"INSTANCE_CREATED", map[string]any{"mode": mode}}}); err != nil {
		return nil, err
	}
	if err := e.save(ctx, tx, in); err != nil {
		return nil, err
	}
	return e.snapshot(def, in, rev), nil
}

func (e *Engine) mutate(ctx context.Context, tx pgx.Tx, characterID, rev int64, payload map[string]any,
	apply func(Definition, *instance) ([]Event, error)) (any, error) {
	in, def, err := e.loadByID(ctx, tx, characterID, payload)
	if err != nil {
		return nil, err
	}
	evs, err := apply(def, in)
	if err != nil {
		return nil, err
	}
	if err := e.logEvents(ctx, tx, in, evs); err != nil {
		return nil, err
	}
	if err := e.save(ctx, tx, in); err != nil {
		return nil, err
	}
	return e.snapshot(def, in, rev), nil
}

func (e *Engine) leave(ctx context.Context, tx pgx.Tx, characterID, rev int64, payload map[string]any) (any, error) {
	in, def, err := e.loadByID(ctx, tx, characterID, payload)
	if err != nil {
		return nil, err
	}
	if in.m.Status == StatusCompleted {
		return nil, ErrStateConflict // claim the reward first
	}
	if err := e.finish(ctx, tx, in, StatusAbandoned, "INSTANCE_ABANDONED"); err != nil {
		return nil, err
	}
	return e.snapshot(def, in, rev), nil
}

func (e *Engine) claim(ctx context.Context, tx pgx.Tx, characterID, rev int64, payload map[string]any) (any, error) {
	in, def, err := e.loadByID(ctx, tx, characterID, payload)
	if err != nil {
		return nil, err
	}
	if in.m.Status != StatusCompleted {
		return nil, ErrStateConflict
	}
	var already int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.dungeon_reward_claims
		WHERE character_id=$1 AND instance_id=$2::uuid AND reward_kind=$3`, characterID, in.id, def.Rewards.Kind).Scan(&already); err != nil {
		return nil, ErrInternal
	}
	if already > 0 {
		return nil, ErrStateConflict
	}
	var gold, invRev int64
	if err := tx.QueryRow(ctx, `SELECT gold, inventory_revision FROM public.characters WHERE character_id=$1`, characterID).Scan(&gold, &invRev); err != nil {
		return nil, ErrInternal
	}

	coef := e.cfg.RegionCoef[def.RegionID]
	var totalGold int64
	merged := map[string]int{}
	for _, enemy := range []string{def.MiniBossID, def.BossID} {
		tb, ok := e.loot.ForEnemy(enemy)
		if !ok {
			return nil, ErrInternal
		}
		r := loot.Roll(tb, coef, e.roller)
		totalGold += r.Gold
		for _, d := range r.Items {
			merged[d.ItemID] += d.Quantity
		}
	}
	var firstAt *time.Time
	progErr := tx.QueryRow(ctx, `SELECT first_clear_at FROM public.dungeon_progress
		WHERE character_id=$1 AND dungeon_id=$2 FOR UPDATE`, characterID, def.DungeonID).Scan(&firstAt)
	hasProgress := progErr == nil
	if progErr != nil && !errors.Is(progErr, pgx.ErrNoRows) {
		return nil, ErrInternal
	}
	first := firstAt == nil
	if first && def.FirstClearMaterial != "" {
		merged[def.FirstClearMaterial]++
	}
	ids := make([]string, 0, len(merged))
	for id := range merged {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	items := []loot.Drop{}
	changes := []inventory.Stack{}
	for _, id := range ids {
		items = append(items, loot.Drop{ItemID: id, Quantity: merged[id]})
		ch, newRev, err := inventory.GrantMaterialTx(ctx, tx, characterID, invRev, id, merged[id])
		if err != nil {
			// Bag full etc.: nothing is changed (transaction rolls back) and the claim stays retryable.
			return nil, ErrStateConflict
		}
		invRev = newRev
		changes = mergeStacks(changes, ch)
	}
	gold += totalGold
	rev++
	if _, err := tx.Exec(ctx, `UPDATE public.characters SET gold=$1, state_revision=$2,
		flags = jsonb_set(jsonb_set(flags, ARRAY[$4]::text[], 'true'::jsonb, true), ARRAY[$5]::text[], 'true'::jsonb, true)
		WHERE character_id=$3`, gold, rev, characterID, def.Flags.NormalClear, firstFlag(first, def)); err != nil {
		return nil, ErrInternal
	}
	if hasProgress {
		_, err = tx.Exec(ctx, `UPDATE public.dungeon_progress SET normal_clear_count = normal_clear_count + 1,
			first_clear_at = coalesce(first_clear_at, now()) WHERE character_id=$1 AND dungeon_id=$2`, characterID, def.DungeonID)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO public.dungeon_progress (character_id, dungeon_id, first_clear_at, normal_clear_count)
			VALUES ($1,$2,now(),1)`, characterID, def.DungeonID)
	}
	if err != nil {
		return nil, ErrInternal
	}
	res := Reward{InstanceID: in.id, Gold: totalGold, Items: items, FirstClear: first, GoldTotal: gold,
		InventoryRevision: invRev, Revision: rev, Changes: changes}
	raw, _ := json.Marshal(res)
	if _, err := tx.Exec(ctx, `INSERT INTO public.dungeon_reward_claims (character_id, instance_id, reward_kind, payload)
		VALUES ($1,$2::uuid,$3,$4::jsonb)`, characterID, in.id, def.Rewards.Kind, string(raw)); err != nil {
		return nil, ErrStateConflict
	}
	if err := e.finish(ctx, tx, in, StatusClosed, "REWARD_CLAIMED"); err != nil {
		return nil, err
	}
	return res, nil
}

// firstFlag returns the flag key to set on a first clear; on later clears it re-sets NORMAL_CLEAR (no-op).
func firstFlag(first bool, def Definition) string {
	if first {
		return def.Flags.FirstClear
	}
	return def.Flags.NormalClear
}

func mergeStacks(have, add []inventory.Stack) []inventory.Stack {
	for _, a := range add {
		replaced := false
		for i := range have {
			if have[i].Container == a.Container && have[i].Slot == a.Slot {
				have[i], replaced = a, true
				break
			}
		}
		if !replaced {
			have = append(have, a)
		}
	}
	return have
}

func (e *Engine) loadByID(ctx context.Context, tx pgx.Tx, characterID int64, payload map[string]any) (*instance, Definition, error) {
	id, _ := payload["instance_id"].(string)
	if _, err := parseUUID(id); err != nil {
		return nil, Definition{}, ErrBadRequest
	}
	in, err := e.loadLive(ctx, tx, characterID)
	if err != nil {
		return nil, Definition{}, err
	}
	if in.id != strings.ToLower(id) {
		return nil, Definition{}, ErrNotFound
	}
	def, ok := e.defs.Get(in.dungeonID)
	if !ok {
		return nil, Definition{}, ErrInternal
	}
	if e.now().Sub(in.lastSeen) > e.idle(def) && in.m.Status != StatusCompleted {
		if err := e.finish(ctx, tx, in, StatusExpired, "INSTANCE_EXPIRED"); err != nil {
			return nil, Definition{}, err
		}
		return nil, Definition{}, ErrStateConflict
	}
	return in, def, nil
}

func (e *Engine) loadLive(ctx context.Context, tx pgx.Tx, characterID int64) (*instance, error) {
	in := &instance{characterID: characterID, m: &Machine{}}
	var state []byte
	err := tx.QueryRow(ctx, `
		SELECT instance_id::text, dungeon_id, mode, status, room_id, checkpoint_room_id, event_seq, state,
		       boss_attempts, seed, last_seen_at
		FROM public.dungeon_instances
		WHERE character_id=$1 AND status IN ('ACTIVE','BOSS_ACTIVE','COMPLETED') FOR UPDATE`, characterID).
		Scan(&in.id, &in.dungeonID, &in.mode, &in.m.Status, &in.m.RoomID, &in.m.Checkpoint, &in.eventSeq, &state,
			&in.m.BossAttempts, &in.seed, &in.lastSeen)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, ErrInternal
	}
	if json.Unmarshal(state, &in.m.Rooms) != nil {
		return nil, ErrInternal
	}
	return in, nil
}

func (e *Engine) save(ctx context.Context, tx pgx.Tx, in *instance) error {
	state, _ := json.Marshal(in.m.Rooms)
	var completed any
	if in.m.Status == StatusCompleted {
		completed = e.now()
	}
	_, err := tx.Exec(ctx, `
		UPDATE public.dungeon_instances
		SET status=$2, room_id=$3, checkpoint_room_id=$4, event_seq=$5, state=$6::jsonb, boss_attempts=$7,
		    last_seen_at=$8, completed_at=coalesce(completed_at, $9::timestamptz)
		WHERE instance_id=$1::uuid`, in.id, in.m.Status, in.m.RoomID, in.m.Checkpoint, in.eventSeq, string(state),
		in.m.BossAttempts, e.now(), completed)
	if err != nil {
		return ErrInternal
	}
	in.lastSeen = e.now()
	return nil
}

func (e *Engine) touch(ctx context.Context, tx pgx.Tx, in *instance) error {
	return e.save(ctx, tx, in)
}

func (e *Engine) finish(ctx context.Context, tx pgx.Tx, in *instance, status, event string) error {
	in.m.Status = status
	if err := e.logEvents(ctx, tx, in, []Event{{event, map[string]any{}}}); err != nil {
		return err
	}
	return e.save(ctx, tx, in)
}

func (e *Engine) logEvents(ctx context.Context, tx pgx.Tx, in *instance, evs []Event) error {
	for _, ev := range evs {
		in.eventSeq++
		raw, _ := json.Marshal(ev.Payload)
		if _, err := tx.Exec(ctx, `INSERT INTO public.dungeon_event_log (instance_id, seq, event_type, payload)
			VALUES ($1::uuid,$2,$3,$4::jsonb)`, in.id, in.eventSeq, ev.Type, string(raw)); err != nil {
			return ErrInternal
		}
	}
	return nil
}

func (e *Engine) snapshot(def Definition, in *instance, rev int64) Snapshot {
	rooms := make(map[string]RoomView, len(in.m.Rooms))
	for id, rs := range in.m.Rooms {
		rooms[id] = RoomView{State: rs.State, Objects: rs.Objects, Spawns: rs.Spawns, PuzzleProgress: len(rs.PuzzleProgress)}
	}
	return Snapshot{InstanceID: in.id, DungeonID: in.dungeonID, Mode: in.mode, Status: in.m.Status, RoomID: in.m.RoomID,
		Checkpoint: in.m.Checkpoint, EventSeq: in.eventSeq, Rooms: rooms, Attempts: in.m.BossAttempts, Revision: rev}
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
