package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

type peer struct {
	ctx       context.Context
	cancel    context.CancelFunc
	work      context.Context
	stopWork  context.CancelFunc
	conn      *websocket.Conn
	raw       net.Conn
	cfg       Config
	logger    *slog.Logger
	id        string
	character int64 // guarded by Server.mu

	queue   chan packet
	closing chan struct{}
	done    chan struct{}
	once    sync.Once
	workers sync.WaitGroup
}

type packet struct {
	data       []byte
	closeAfter websocket.StatusCode
}

func newPeer(ctx context.Context, conn *websocket.Conn, raw net.Conn, cfg Config, logger *slog.Logger) *peer {
	ctx, cancel := context.WithCancel(ctx)
	work, stopWork := context.WithCancel(ctx)
	return &peer{
		ctx: ctx, cancel: cancel, work: work, stopWork: stopWork, conn: conn, raw: raw, cfg: cfg, logger: logger,
		queue: make(chan packet, cfg.Queue), closing: make(chan struct{}), done: make(chan struct{}),
	}
}

func (p *peer) close(code websocket.StatusCode) {
	p.once.Do(func() {
		close(p.closing)
		p.stopWork()
		p.logger.Debug("websocket closing", "close_code", int(code))
		go func() {
			_ = p.conn.Close(code, "")
			_ = p.raw.Close()
			close(p.done)
		}()
	})
}

func (p *peer) finish() {
	p.once.Do(func() {
		close(p.closing)
		p.stopWork()
		_ = p.raw.Close()
		close(p.done)
	})
	<-p.done
	p.cancel()
	p.workers.Wait()
}

func (p *peer) encode(typ, requestID string, revision int64, payload any) []byte {
	data, err := json.Marshal(outbound{
		Version: 1, Type: typ, SessionID: p.id, RequestID: requestID, ServerTime: timestamp(),
		Revision: revision, TraceID: sessionID(), Payload: payload,
	})
	if err != nil || len(data) > outputLimit {
		p.close(websocket.StatusInternalError)
		return nil
	}
	return data
}

func (p *peer) enqueue(typ, requestID string, revision int64, payload any, closeAfter websocket.StatusCode) bool {
	data := p.encode(typ, requestID, revision, payload)
	if data == nil {
		return false
	}
	select {
	case <-p.closing:
		return false
	default:
	}
	select {
	case p.queue <- packet{data, closeAfter}:
		return true
	default:
		p.close(websocket.StatusTryAgainLater)
		return false
	}
}

func (p *peer) write(data []byte) bool {
	if data == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(p.ctx, p.cfg.WriteTimeout)
	defer cancel()
	if err := p.conn.Write(ctx, websocket.MessageText, data); err != nil {
		p.close(websocket.StatusTryAgainLater)
		return false
	}
	return true
}

func (p *peer) startWriter() {
	p.workers.Go(func() {
		for {
			select {
			case <-p.closing:
				return
			case next := <-p.queue:
				select {
				case <-p.closing:
					return
				default:
				}
				if !p.write(next.data) {
					return
				}
				if next.closeAfter != 0 {
					p.close(next.closeAfter)
					return
				}
			}
		}
	})
}

func (p *peer) startHeartbeat() {
	p.workers.Go(func() {
		ticker := time.NewTicker(p.cfg.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-p.closing:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(p.ctx, p.cfg.PongTimeout)
				err := p.conn.Ping(ctx)
				cancel()
				if err != nil {
					p.close(websocket.StatusGoingAway)
					return
				}
			}
		}
	})
}

func (p *peer) read() (inbound, error) {
	mt, data, err := p.conn.Read(p.ctx)
	if err != nil {
		return inbound{}, err
	}
	if mt != websocket.MessageText {
		p.close(websocket.StatusUnsupportedData)
		return inbound{}, errMalformed
	}
	m, err := decodeMessage(data)
	switch {
	case errors.Is(err, errMalformed):
		p.close(websocket.StatusPolicyViolation)
	case errors.Is(err, errVersion):
		p.close(closeProtocol)
	}
	if m.ignored {
		p.logger.Debug("ignored client-owned copies of server fields", "size", len(data))
	}
	return m, err
}

type bucket struct {
	rate, capacity, tokens float64
	stamp                  time.Time
}

func newBucket(rate, capacity int) bucket {
	return bucket{rate: float64(rate), capacity: float64(capacity), tokens: float64(capacity)}
}

func (b *bucket) refresh(now time.Time) {
	if !b.stamp.IsZero() {
		b.tokens = min(b.capacity, b.tokens+now.Sub(b.stamp).Seconds()*b.rate)
	}
	b.stamp = now
}

func (b *bucket) delayMS() int64 {
	if b.tokens >= 1 {
		return 0
	}
	return int64(math.Ceil((1 - b.tokens) * 1000 / b.rate))
}

func (s *Server) commands(p *peer, owner string, id, revision int64) {
	commands := newBucket(s.cfg.Rate, s.cfg.Burst)
	mutations := newBucket(s.cfg.MutationRate, s.cfg.MutationRate)
	violations := 0
	for {
		m, err := p.read()
		if err != nil && !errors.Is(err, errEnvelope) {
			return
		}
		now := time.Now()
		commands.refresh(now)
		mutations.refresh(now)
		delay := commands.delayMS()
		mutation := isMutation(m.typ)
		if mutation {
			delay = max(delay, mutations.delayMS())
		}
		if delay > 0 {
			violations++
			var closeAfter websocket.StatusCode
			if violations >= s.cfg.Violations {
				closeAfter = closeRate
				s.logger.Warn("websocket repeated rate violations", "character_id", id, "close_code", int(closeRate))
			}
			if !p.sendError(m.requestID, revision, "RATE_LIMITED", delay, closeAfter) {
				return
			}
			if closeAfter != 0 {
				<-p.done
				return
			}
			continue
		}
		commands.tokens--
		if mutation {
			mutations.tokens--
		}
		violations = 0

		switch {
		case err != nil || (mutation && m.requestID == ""):
			if !p.sendError(m.requestID, revision, "BAD_REQUEST", 0, 0) {
				return
			}
		case m.typ == "state.snapshot.request":
			scopes, valid := snapshotScopes(m.payload)
			if !valid {
				if !p.sendError(m.requestID, revision, "BAD_REQUEST", 0, 0) {
					return
				}
				continue
			}
			ctx, cancel := context.WithTimeout(p.work, s.cfg.WriteTimeout)
			state, err := s.store.LoadWSState(ctx, owner, id)
			cancel()
			if err != nil || !validState(state) {
				if !p.sendError(m.requestID, revision, "INTERNAL", 0, 0) {
					return
				}
				continue
			}
			revision = state.Revision
			if !p.enqueue("state.snapshot", m.requestID, revision, s.snapshot(p.work, owner, id, state, scopes), 0) {
				return
			}
		case m.typ == "quest.accept.request" || m.typ == "world.interact.request" || m.typ == "quest.turnin.request":
			if s.quests == nil {
				if !p.sendError(m.requestID, revision, "UNKNOWN_TYPE", 0, 0) {
					return
				}
				continue
			}
			payload := rawPayload(m.payload)
			ctx, cancel := context.WithTimeout(p.work, s.cfg.WriteTimeout)
			var (
				result any
				err    error
				reply  string
			)
			switch m.typ {
			case "quest.accept.request":
				result, err = s.quests.Accept(ctx, owner, id, m.typ, m.requestID, payload)
				reply = "quest.accept.result"
			case "world.interact.request":
				result, err = s.quests.Interact(ctx, owner, id, m.typ, m.requestID, payload)
				reply = "quest.progress.delta"
			case "quest.turnin.request":
				result, err = s.quests.TurnIn(ctx, owner, id, m.typ, m.requestID, payload)
				reply = "quest.reward.granted"
			}
			cancel()
			if err != nil {
				if !p.sendError(m.requestID, revision, questErrorCode(err), 0, 0) {
					return
				}
				continue
			}
			revision = revisionFromResult(result, revision)
			if !p.enqueue(reply, m.requestID, revision, result, 0) {
				return
			}
		case m.typ == "dialogue.open.request" || m.typ == "dialogue.choice.request":
			if s.dialogue == nil {
				if !p.sendError(m.requestID, revision, "UNKNOWN_TYPE", 0, 0) {
					return
				}
				continue
			}
			payload := rawPayload(m.payload)
			ctx, cancel := context.WithTimeout(p.work, s.cfg.WriteTimeout)
			var (
				result any
				err    error
			)
			switch m.typ {
			case "dialogue.open.request":
				result, err = s.dialogue.Open(ctx, owner, id, m.typ, m.requestID, payload)
			case "dialogue.choice.request":
				result, err = s.dialogue.Choice(ctx, owner, id, m.typ, m.requestID, payload)
			}
			cancel()
			if err != nil {
				if !p.sendError(m.requestID, revision, questErrorCode(err), 0, 0) {
					return
				}
				continue
			}
			revision = revisionFromResult(result, revision)
			if !p.enqueue("dialogue.node.state", m.requestID, revision, result, 0) {
				return
			}
		case m.typ == "inventory.move.request":
			if s.inv == nil {
				if !p.sendError(m.requestID, revision, "UNKNOWN_TYPE", 0, 0) {
					return
				}
				continue
			}
			payload := rawPayload(m.payload)
			ctx, cancel := context.WithTimeout(p.work, s.cfg.WriteTimeout)
			result, err := s.inv.Move(ctx, owner, id, m.typ, m.requestID, payload)
			cancel()
			if err != nil {
				if !p.sendError(m.requestID, revision, questErrorCode(err), 0, 0) {
					return
				}
				continue
			}
			revision = revisionFromResult(result, revision)
			if !p.enqueue("inventory.delta", m.requestID, revision, result, 0) {
				return
			}
		case m.typ == "combat.loot.request":
			if s.loot == nil {
				if !p.sendError(m.requestID, revision, "UNKNOWN_TYPE", 0, 0) {
					return
				}
				continue
			}
			payload := rawPayload(m.payload)
			ctx, cancel := context.WithTimeout(p.work, s.cfg.WriteTimeout)
			result, err := s.loot.Claim(ctx, owner, id, m.typ, m.requestID, payload)
			cancel()
			if err != nil {
				if !p.sendError(m.requestID, revision, questErrorCode(err), 0, 0) {
					return
				}
				continue
			}
			revision = revisionFromResult(result, revision)
			if !p.enqueue("combat.loot.result", m.requestID, revision, result, 0) {
				return
			}
		case isDungeonCommand(m.typ):
			if s.dungeon == nil {
				if !p.sendError(m.requestID, revision, "UNKNOWN_TYPE", 0, 0) {
					return
				}
				continue
			}
			payload := rawPayload(m.payload)
			ctx, cancel := context.WithTimeout(p.work, s.cfg.WriteTimeout)
			replyType, result, err := s.dungeon.Handle(ctx, owner, id, m.typ, m.requestID, payload)
			cancel()
			if err != nil {
				if !p.sendError(m.requestID, revision, questErrorCode(err), 0, 0) {
					return
				}
				continue
			}
			revision = revisionFromResult(result, revision)
			if !p.enqueue(replyType, m.requestID, revision, result, 0) {
				return
			}
		default:
			if !p.sendError(m.requestID, revision, "UNKNOWN_TYPE", 0, 0) {
				return
			}
		}
	}
}

func (p *peer) sendError(id string, revision int64, code string, delay int64, closeAfter websocket.StatusCode) bool {
	errorPayload := struct {
		RequestID    string `json:"request_id"`
		Code         string `json:"code"`
		MessageKey   string `json:"message_key"`
		RetryAfterMS int64  `json:"retry_after_ms,omitempty"`
	}{id, code, "error." + strings.ToLower(code), delay}
	return p.enqueue("command.error", id, revision, errorPayload, closeAfter)
}

func validState(s auth.WSState) bool {
	return s.Revision >= 0 && s.Revision <= maxJSONInteger && s.XP >= 0 && s.XP <= maxJSONInteger &&
		s.Gold >= 0 && s.Gold <= maxJSONInteger
}

func snapshotScopes(payload map[string]json.RawMessage) ([]string, bool) {
	raw, present := payload["scopes"]
	if !present {
		return nil, true
	}
	var scopes []string
	if json.Unmarshal(raw, &scopes) != nil || scopes == nil {
		return nil, false
	}
	for _, scope := range scopes {
		if scope == "" {
			return nil, false
		}
	}
	return scopes, true
}

func (s *Server) snapshot(ctx context.Context, owner string, id int64, state auth.WSState, requested []string) map[string]any {
	scopes := make(map[string]any, 2)
	if len(requested) == 0 {
		requested = []string{"character", "quests"}
	}
	for _, name := range requested {
		switch name {
		case "character":
			scopes[name] = struct {
				auth.Character
				XP         int64          `json:"xp"`
				Gold       int64          `json:"gold"`
				Reputation map[string]int `json:"reputation"`
				RestedXP   int64          `json:"rested_xp"`
			}{Character: state.Character, XP: state.XP, Gold: state.Gold, Reputation: map[string]int{}, RestedXP: 0}
		case "quests":
			quests := any(map[string]any{"active": []any{}, "completed": []string{}, "available": []string{}})
			if s.quests != nil {
				if q, err := s.quests.SnapshotQuests(ctx, owner, id); err == nil {
					quests = q
				}
			}
			scopes[name] = quests
		case "inventory":
			inv := any(map[string]any{"inventory_revision": 0, "bag": []any{}, "materials": []any{}, "quest_keys": []string{}})
			if s.inv != nil {
				if snap, err := s.inv.SnapshotInventory(ctx, owner, id); err == nil {
					inv = snap
				}
			}
			scopes[name] = inv
		}
	}
	return map[string]any{"scopes": scopes, "revision": state.Revision}
}

func rawPayload(payload map[string]json.RawMessage) map[string]any {
	out := make(map[string]any, len(payload))
	for k, raw := range payload {
		var v any
		if json.Unmarshal(raw, &v) == nil {
			out[k] = v
		}
	}
	return out
}

func questErrorCode(err error) string {
	if err == nil {
		return "INTERNAL"
	}
	msg := err.Error()
	switch msg {
	case "NOT_FOUND", "STATE_CONFLICT", "PREREQUISITE_NOT_MET", "LEVEL_TOO_LOW",
		"UNSUPPORTED_EVENT", "INVALID_TARGET", "CONTENT_MISMATCH", "IDEMPOTENCY_CONFLICT",
		"REGION_CLOSED", "RATE_LIMITED", "BAD_REQUEST", "INTERNAL":
		return msg
	default:
		if u := errors.Unwrap(err); u != nil {
			return questErrorCode(u)
		}
		return "INTERNAL"
	}
}

func revisionFromResult(result any, fallback int64) int64 {
	switch v := result.(type) {
	case interface{ GetRevision() int64 }:
		return v.GetRevision()
	default:
		raw, err := json.Marshal(result)
		if err != nil {
			return fallback
		}
		var probe struct {
			Revision          int64 `json:"revision"`
			InventoryRevision int64 `json:"inventory_revision"`
		}
		if json.Unmarshal(raw, &probe) == nil {
			if probe.Revision > 0 {
				return probe.Revision
			}
			if probe.InventoryRevision > 0 {
				return probe.InventoryRevision
			}
		}
		return fallback
	}
}

func isDungeonCommand(typ string) bool {
	switch typ {
	case "dungeon.enter.request", "dungeon.interact.request", "dungeon.encounter.report",
		"dungeon.claim.request", "dungeon.leave.request":
		return true
	}
	return false
}
