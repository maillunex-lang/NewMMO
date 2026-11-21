package dungeon

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

// RoomSnapshot changes the combat stream when a character enters or resets a room.
type RoomSnapshot struct {
	world.Snapshot
	Encounter string `json:"encounter_id"`
}

type roomDelta struct {
	world.Delta
	Encounter string `json:"encounter_id"`
}

// Session owns the private simulation attached to one authenticated connection.
// The database retains room outcomes across connections. Live HP and positions
// are rebuilt on re-entry.
type Session struct {
	mu          sync.Mutex
	ctx         context.Context
	engine      *Engine
	catalog     *combatdata.Catalog
	loader      world.PlayerLoader
	accountID   string
	characterID int64
	ref         string
	push        world.Pusher
	critical    world.CriticalPusher
	logger      *slog.Logger

	snapshot  Snapshot
	encounter string
	zone      *world.Zone
	handler   *world.Handler
	stop      context.CancelFunc
	pending   world.EncounterOutcome
	idle      time.Duration
	touched   time.Time
}

func (e *Engine) NewSession(ctx context.Context, catalog *combatdata.Catalog, loader world.PlayerLoader,
	accountID string, characterID int64, push world.Pusher, critical world.CriticalPusher, logger *slog.Logger) *Session {
	return &Session{ctx: ctx, engine: e, catalog: catalog, loader: loader,
		accountID: accountID, characterID: characterID, ref: strconv.FormatInt(characterID, 10),
		push: push, critical: critical, logger: logger}
}

func (s *Session) Handle(ctx context.Context, typ, requestID string, payload map[string]any) (string, any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.flushCombat(ctx); err != nil {
		return "", nil, err
	}
	replyType, result, err := s.engine.Handle(ctx, s.accountID, s.characterID, typ, requestID, payload)
	if err != nil {
		return "", nil, err
	}
	if reward, ok := result.(Reward); ok && reward.InstanceID == s.snapshot.InstanceID {
		s.clear()
	}
	if replyType != "dungeon.snapshot" {
		return replyType, result, nil
	}
	snapshot, ok := result.(Snapshot)
	if !ok {
		raw, _ := json.Marshal(result)
		var stored Snapshot
		if json.Unmarshal(raw, &stored) != nil {
			return "", nil, ErrInternal
		}
		switch stored.Status {
		case StatusClosed, StatusAbandoned, StatusExpired:
			if stored.InstanceID == s.snapshot.InstanceID {
				s.clear()
			}
			return replyType, stored, nil
		}
		if s.zone != nil && stored.InstanceID == s.snapshot.InstanceID && stored.EventSeq <= s.snapshot.EventSeq {
			return replyType, s.snapshot, nil
		}
		snapshot, err = s.engine.resume(ctx, s.accountID, s.characterID, stored.InstanceID)
		if err != nil {
			return "", nil, err
		}
	}
	snapshot, err = s.applySnapshot(ctx, snapshot, false)
	return replyType, snapshot, err
}

// HandleWorld claims combat commands only when a room is attached or when a
// stale client command carries a prior room's stream id.
func (s *Session) HandleWorld(ctx context.Context, typ, requestID string, payload map[string]any) (string, any, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stream, scoped := payload["encounter_id"]
	if s.zone == nil && !scoped {
		return "", nil, false, nil
	}
	if s.zone == nil || stream != s.encounter || typ == "world.join.request" || typ == "world.leave.request" {
		return "", nil, true, ErrStateConflict
	}
	if hasOutcome(s.pending) {
		return "", nil, true, ErrStateConflict
	}
	delete(payload, "encounter_id")
	reply, result, err := s.handler.Handle(ctx, s.accountID, s.characterID, typ, requestID, payload, s, s.push, s.critical)
	if err == nil && s.snapshot.Status != StatusCompleted && s.engine.now().Sub(s.touched) >= min(time.Minute, s.idle/2) {
		snapshot, touchErr := s.engine.resume(ctx, s.accountID, s.characterID, s.snapshot.InstanceID)
		if touchErr != nil {
			return "", nil, true, touchErr
		}
		s.snapshot, s.touched = snapshot, s.engine.now()
	}
	return reply, result, true, err
}

func (s *Session) applySnapshot(ctx context.Context, snapshot Snapshot, reset bool) (Snapshot, error) {
	switch snapshot.Status {
	case StatusClosed, StatusAbandoned, StatusExpired:
		if snapshot.InstanceID == s.snapshot.InstanceID {
			s.clear()
		}
		return snapshot, nil
	}

	if !reset && s.zone != nil && s.snapshot.InstanceID == snapshot.InstanceID && s.snapshot.RoomID == snapshot.RoomID {
		s.snapshot, s.touched = snapshot, s.engine.now()
		return snapshot, nil
	}
	def, ok := s.engine.defs.Get(snapshot.DungeonID)
	if !ok || def.RoomIndex(snapshot.RoomID) < 0 {
		return Snapshot{}, ErrInternal
	}
	room := def.Rooms[def.RoomIndex(snapshot.RoomID)]
	z, err := room.NewRoomZone(snapshot.Rooms[room.ID], s.catalog)
	if err != nil {
		s.logger.Error("build dungeon room", "room_id", room.ID, "error", err)
		return Snapshot{}, ErrInternal
	}
	handler := world.NewHandler([]*world.Zone{z}, s.loader)
	encounter := rand.Text()
	push := func(typ string, payload any) bool {
		switch data := payload.(type) {
		case world.Delta:
			payload = roomDelta{Delta: data, Encounter: encounter}
		case map[string]any:
			data["encounter_id"] = encounter
		}
		return s.push(typ, payload)
	}
	_, initial, err := handler.Handle(ctx, s.accountID, s.characterID, "world.join.request", "",
		map[string]any{"zone_id": room.ID}, s, push, s.critical)
	if err != nil {
		return Snapshot{}, err
	}

	s.zone, s.handler, s.encounter, s.snapshot = z, handler, encounter, snapshot
	s.idle, s.touched = s.engine.idle(def), s.engine.now()
	snapshot.Combat = &RoomSnapshot{Snapshot: initial.(world.Snapshot), Encounter: encounter}
	if s.stop == nil {
		runCtx, stop := context.WithCancel(s.ctx)
		s.stop = stop
		go s.run(runCtx, time.Duration(snapshot.Combat.TickMS)*time.Millisecond)
	}
	return snapshot, nil
}

func (s *Session) clear() {
	if s.stop != nil {
		s.stop()
		s.stop = nil
	}
	s.zone, s.handler, s.encounter, s.snapshot = nil, nil, "", Snapshot{}
	s.pending = world.EncounterOutcome{}
}

func (s *Session) run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			if ctx.Err() == nil && s.zone != nil {
				expired := s.snapshot.Status != StatusCompleted && s.engine.now().Sub(s.touched) > s.idle
				if !expired && !hasOutcome(s.pending) {
					s.pending = s.zone.TickEncounter(s.ref)
				}
				if expired || hasOutcome(s.pending) {
					dbCtx, cancel := context.WithTimeout(ctx, time.Second)
					if err := s.flushEncounter(dbCtx, expired); err != nil && ctx.Err() == nil {
						s.logger.Warn("persist dungeon combat", "character_id", s.characterID, "error", err)
					}
					cancel()
				}
			}
			s.mu.Unlock()
		}
	}
}

func hasOutcome(outcome world.EncounterOutcome) bool {
	return outcome.PlayerDefeated || len(outcome.Kills) != 0
}

func (s *Session) flushCombat(ctx context.Context) error {
	return s.flushEncounter(ctx, false)
}

func (s *Session) flushEncounter(ctx context.Context, expired bool) error {
	if !expired && !hasOutcome(s.pending) {
		return nil
	}
	previous := s.snapshot
	snapshot, err := s.engine.updateEncounter(ctx, s.accountID, s.characterID, previous, s.pending, expired)
	if errors.Is(err, ErrNotFound) {
		s.clear()
		return ErrStateConflict
	}
	if err != nil {
		return err
	}
	snapshot, err = s.applySnapshot(ctx, snapshot, s.pending.PlayerDefeated)
	if err != nil {
		return err
	}
	s.pending = world.EncounterOutcome{}
	s.critical("dungeon.snapshot", snapshot.Revision, snapshot)
	return nil
}
