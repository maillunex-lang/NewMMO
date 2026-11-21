package world

import (
	"context"
	"math"
	"strconv"
	"sync"
)

// PlayerLoader returns the persistent facts the zone needs about a character (name, class, level).
type PlayerLoader func(ctx context.Context, accountID string, characterID int64) (PlayerInfo, error)

// Handler adapts zones to the WebSocket layer (ws.WorldHandler).
type Handler struct {
	zones  map[string]*Zone
	loader PlayerLoader
	byChar sync.Map // characterID(string) -> *Zone currently joined
}

func NewHandler(zones []*Zone, loader PlayerLoader) *Handler {
	h := &Handler{zones: map[string]*Zone{}, loader: loader}
	for _, z := range zones {
		h.zones[z.ID()] = z
	}
	return h
}

// Handle serves world.join/move/stop/cast/leave requests.
// The returned type/payload (if any) is sent as the reply; deltas flow later through push.
func (h *Handler) Handle(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any,
	owner any, push Pusher, pushCritical CriticalPusher) (string, any, error) {
	ref := strconv.FormatInt(characterID, 10)
	switch typ {
	case "world.join.request":
		id, ok := payload["zone_id"].(string)
		z := h.zones[id]
		if !ok || len(payload) != 1 {
			return "", nil, ErrBadRequest
		}
		if z == nil {
			return "", nil, ErrNotFound
		}
		info, err := h.loader(ctx, accountID, characterID)
		if err != nil {
			return "", nil, ErrNotFound
		}
		if prev, ok := h.byChar.Load(ref); ok && prev.(*Zone) != z {
			prev.(*Zone).Leave(ref, owner)
		}
		info.AccountID, info.Character = accountID, characterID
		snap, err := z.Join(ref, owner, info, push, pushCritical)
		if err != nil {
			return "", nil, err
		}
		h.byChar.Store(ref, z)
		return "world.snapshot", snap, nil
	case "world.move.request":
		seq, x, y, ok := moveArgs(payload)
		if !ok {
			return "", nil, ErrBadRequest
		}
		z, err := h.joined(ref)
		if err != nil {
			return "", nil, err
		}
		return "", nil, z.Move(ref, owner, seq, Vec{x, y})
	case "world.cast.request":
		skill, ok := payload["skill_id"].(string)
		var target EntityID
		switch len(payload) {
		case 1:
		case 2:
			tf, okt := payload["target_id"].(float64)
			if !okt || tf < 1 || tf > 4294967295 || tf != math.Trunc(tf) {
				return "", nil, ErrBadRequest
			}
			target = EntityID(tf)
		default:
			return "", nil, ErrBadRequest
		}
		if !ok || skill == "" || requestID == "" {
			return "", nil, ErrBadRequest
		}
		z, err := h.joined(ref)
		if err != nil {
			return "", nil, err
		}
		out, err := z.Cast(ref, owner, requestID, skill, target)
		if err != nil {
			return "", nil, err
		}
		return "world.cast.result", out, nil
	case "world.stop.request":
		seq, ok := seqArg(payload["seq"])
		if !ok || len(payload) != 1 {
			return "", nil, ErrBadRequest
		}
		z, err := h.joined(ref)
		if err != nil {
			return "", nil, err
		}
		return "", nil, z.Stop(ref, owner, seq)
	case "world.leave.request":
		if len(payload) != 0 {
			return "", nil, ErrBadRequest
		}
		h.Disconnect(characterID, owner)
		return "", nil, nil
	}
	return "", nil, ErrBadRequest
}

// Disconnect removes the character when `owner` is still its session (a newer session keeps the entity).
func (h *Handler) Disconnect(characterID int64, owner any) {
	ref := strconv.FormatInt(characterID, 10)
	if z, ok := h.byChar.Load(ref); ok {
		z.(*Zone).Leave(ref, owner)
		if z.(*Zone).hasNoSession(ref) {
			h.byChar.Delete(ref)
		}
	}
}

func (h *Handler) joined(ref string) (*Zone, error) {
	z, ok := h.byChar.Load(ref)
	if !ok {
		return nil, ErrStateConflict
	}
	return z.(*Zone), nil
}

func (z *Zone) hasNoSession(ref string) bool {
	z.mu.Lock()
	defer z.mu.Unlock()
	_, ok := z.sessions[ref]
	return !ok
}

func seqArg(v any) (uint64, bool) {
	f, ok := v.(float64)
	if !ok || f < 1 || f > 9007199254740991 || f != math.Trunc(f) {
		return 0, false
	}
	return uint64(f), true
}

func moveArgs(p map[string]any) (seq uint64, x, y float64, ok bool) {
	if len(p) != 3 {
		return 0, 0, 0, false
	}
	seq, ok = seqArg(p["seq"])
	xf, okx := p["x"].(float64)
	yf, oky := p["y"].(float64)
	if !ok || !okx || !oky || math.IsNaN(xf) || math.IsInf(xf, 0) || math.IsNaN(yf) || math.IsInf(yf, 0) {
		return 0, 0, 0, false
	}
	return seq, xf, yf, true
}
