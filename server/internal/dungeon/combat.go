package dungeon

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

// updateEncounter persists outcomes observed by the private simulation. The
// expected event sequence fences late ticks and makes a retry after a lost
// commit acknowledgment safe even when a wipe opens the same room again.
func (e *Engine) updateEncounter(ctx context.Context, accountID string, characterID int64, base Snapshot, outcome world.EncounterOutcome, expire bool) (Snapshot, error) {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return Snapshot{}, ErrInternal
	}
	defer tx.Rollback(ctx)
	revision, err := lockCombatCharacter(ctx, tx, accountID, characterID)
	if err != nil {
		return Snapshot{}, err
	}
	in, err := e.loadLive(ctx, tx, characterID)
	if err != nil {
		return Snapshot{}, err
	}
	if in.id != base.InstanceID {
		return Snapshot{}, ErrNotFound
	}
	def, ok := e.defs.Get(in.dungeonID)
	if !ok {
		return Snapshot{}, ErrInternal
	}
	if in.eventSeq != base.EventSeq || in.m.RoomID != base.RoomID {
		return e.snapshot(def, in, revision), nil
	}

	if expire && in.m.Status != StatusCompleted {
		if err := e.finish(ctx, tx, in, StatusExpired, "INSTANCE_EXPIRED"); err != nil {
			return Snapshot{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Snapshot{}, ErrInternal
		}
		return e.snapshot(def, in, revision), nil
	}
	var events []Event
	if outcome.PlayerDefeated {
		events, err = in.m.PlayerDefeated(def)
	} else {
		for _, kill := range outcome.Kills {
			if in.m.RoomID != base.RoomID {
				break
			}
			var applied []Event
			applied, err = in.m.EnemyDefeated(def, kill.Spawn)
			if err != nil {
				break
			}
			events = append(events, applied...)
		}
	}
	if err != nil {
		return Snapshot{}, err
	}
	if err = e.logEvents(ctx, tx, in, events); err != nil {
		return Snapshot{}, err
	}
	if err = e.save(ctx, tx, in); err != nil {
		return Snapshot{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Snapshot{}, ErrInternal
	}
	return e.snapshot(def, in, revision), nil
}

// resume resolves a replayed enter against the current persisted instance.
func (e *Engine) resume(ctx context.Context, accountID string, characterID int64, instanceID string) (Snapshot, error) {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return Snapshot{}, ErrInternal
	}
	defer tx.Rollback(ctx)
	revision, err := lockCombatCharacter(ctx, tx, accountID, characterID)
	if err != nil {
		return Snapshot{}, err
	}
	in, def, err := e.loadByID(ctx, tx, characterID, map[string]any{"instance_id": instanceID})
	if err != nil {
		return Snapshot{}, err
	}
	if err := e.touch(ctx, tx, in); err != nil {
		return Snapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Snapshot{}, ErrInternal
	}
	return e.snapshot(def, in, revision), nil
}

func lockCombatCharacter(ctx context.Context, tx pgx.Tx, accountID string, characterID int64) (int64, error) {
	var revision int64
	err := tx.QueryRow(ctx, `SELECT state_revision FROM public.characters
		WHERE account_id=$1 AND character_id=$2 FOR UPDATE`, accountID, characterID).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, ErrInternal
	}
	return revision, nil
}
