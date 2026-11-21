// Package world is the server-authoritative simulation of a zone (docs/research/T-038-server-combat.md).
// Sim is pure and deterministic: no goroutines, no wall clock; Step advances exactly one tick.
// T-038b: players + static enemies, movement with a server-owned speed and world bounds, interest-filtered
// snapshots/deltas. Combat/AI arrive in T-038c.
package world

import (
	"math"
	"sort"
)

type Vec struct{ X, Y float64 }

type Rect struct{ MinX, MinY, MaxX, MaxY float64 }

func (r Rect) Clamp(v Vec) Vec {
	return Vec{math.Min(r.MaxX, math.Max(r.MinX, v.X)), math.Min(r.MaxY, math.Max(r.MinY, v.Y))}
}

type Kind string

const (
	KindPlayer Kind = "player"
	KindEnemy  Kind = "enemy"
)

type Config struct {
	TickRate       int
	PlayerSpeed    float64 // world units per second (server-owned; the client never sends a speed)
	Bounds         Rect
	InterestRadius float64
	PlayerSpawn    Vec
}

func (c Config) Dt() float64 { return 1 / float64(c.TickRate) }

type EntityID uint32

type Entity struct {
	ID      EntityID
	Kind    Kind
	Ref     string // player: character id; enemy: canon enemy id
	Spawn   string // enemy spawn id
	Name    string
	Level   int
	Pos     Vec
	Facing  int // 0..7 in the game's row order (0=E,1=SE,2=S,3=SW,4=W,5=NW,6=N,7=NE)
	Moving  bool
	HP      int
	MaxHP   int
	target  *Vec
	lastSeq uint64
	changed uint64 // tick of the last visible change
}

// View is the wire shape of an entity.
type View struct {
	ID     EntityID `json:"id"`
	Kind   Kind     `json:"kind"`
	Ref    string   `json:"ref"`
	Name   string   `json:"name,omitempty"`
	Level  int      `json:"level,omitempty"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	Facing int      `json:"facing"`
	Moving bool     `json:"moving"`
	HP     int      `json:"hp"`
	MaxHP  int      `json:"max_hp"`
}

func (e *Entity) view() View {
	return View{ID: e.ID, Kind: e.Kind, Ref: e.Ref, Name: e.Name, Level: e.Level,
		X: round2(e.Pos.X), Y: round2(e.Pos.Y), Facing: e.Facing, Moving: e.Moving, HP: e.HP, MaxHP: e.MaxHP}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

type Sim struct {
	cfg      Config
	tick     uint64
	next     EntityID
	entities map[EntityID]*Entity
	byRef    map[string]EntityID                // players by character id
	views    map[EntityID]map[EntityID]struct{} // per player: entities it currently knows
}

func New(cfg Config) *Sim {
	return &Sim{cfg: cfg, next: 1, entities: map[EntityID]*Entity{}, byRef: map[string]EntityID{}, views: map[EntityID]map[EntityID]struct{}{}}
}

func (s *Sim) Tick() uint64   { return s.tick }
func (s *Sim) Config() Config { return s.cfg }

func (s *Sim) alloc() EntityID { id := s.next; s.next++; return id }

func (s *Sim) AddEnemy(spawnID, enemyID, name string, level, maxHP int, pos Vec) EntityID {
	id := s.alloc()
	s.entities[id] = &Entity{ID: id, Kind: KindEnemy, Ref: enemyID, Spawn: spawnID, Name: name, Level: level,
		Pos: s.cfg.Bounds.Clamp(pos), Facing: 2, HP: maxHP, MaxHP: maxHP, changed: s.tick}
	return id
}

// AddPlayer places a character at the zone spawn; re-adding the same character returns the existing entity.
func (s *Sim) AddPlayer(ref, name string, level, maxHP int) EntityID {
	if id, ok := s.byRef[ref]; ok {
		return id
	}
	id := s.alloc()
	s.entities[id] = &Entity{ID: id, Kind: KindPlayer, Ref: ref, Name: name, Level: level,
		Pos: s.cfg.Bounds.Clamp(s.cfg.PlayerSpawn), Facing: 2, HP: maxHP, MaxHP: maxHP, changed: s.tick}
	s.byRef[ref] = id
	s.views[id] = map[EntityID]struct{}{}
	return id
}

func (s *Sim) RemovePlayer(id EntityID) {
	e, ok := s.entities[id]
	if !ok || e.Kind != KindPlayer {
		return
	}
	delete(s.byRef, e.Ref)
	delete(s.entities, id)
	delete(s.views, id)
	// Other viewers still list the id in their known set; DeltaFor diffs known vs visible and emits the removal.
}

func (s *Sim) Entity(id EntityID) (Entity, bool) {
	e, ok := s.entities[id]
	if !ok {
		return Entity{}, false
	}
	return *e, true
}

// Move sets the player's movement target. Older or repeated sequence numbers are ignored (reordering safe);
// the target is clamped into the zone bounds. The speed is NOT an input: it is cfg.PlayerSpeed.
// Returns false when the command was ignored.
func (s *Sim) Move(id EntityID, seq uint64, target Vec) bool {
	e, ok := s.entities[id]
	if !ok || e.Kind != KindPlayer || seq <= e.lastSeq || math.IsNaN(target.X) || math.IsNaN(target.Y) ||
		math.IsInf(target.X, 0) || math.IsInf(target.Y, 0) {
		return false
	}
	e.lastSeq = seq
	t := s.cfg.Bounds.Clamp(target)
	e.target = &t
	return true
}

// Stop clears the movement target (seq-ordered like Move).
func (s *Sim) Stop(id EntityID, seq uint64) bool {
	e, ok := s.entities[id]
	if !ok || e.Kind != KindPlayer || seq <= e.lastSeq {
		return false
	}
	e.lastSeq = seq
	e.target = nil
	if e.Moving {
		e.Moving = false
		e.changed = s.tick + 1
	}
	return true
}

func (s *Sim) AckSeq(id EntityID) uint64 {
	if e, ok := s.entities[id]; ok {
		return e.lastSeq
	}
	return 0
}

// Step advances one tick: movement toward targets at the server speed, then bookkeeping.
func (s *Sim) Step() {
	s.tick++
	step := s.cfg.PlayerSpeed * s.cfg.Dt()
	for _, e := range s.entities {
		if e.Kind != KindPlayer || e.target == nil {
			continue
		}
		dx, dy := e.target.X-e.Pos.X, e.target.Y-e.Pos.Y
		dist := math.Hypot(dx, dy)
		if dist <= 1e-9 {
			e.target, e.Moving = nil, false
			e.changed = s.tick
			continue
		}
		e.Facing = facing(dx, dy, e.Facing)
		if dist <= step {
			e.Pos = *e.target
			e.target, e.Moving = nil, false
		} else {
			e.Pos = Vec{e.Pos.X + dx/dist*step, e.Pos.Y + dy/dist*step}
			e.Moving = true
		}
		e.changed = s.tick
	}
}

// facing maps a direction to the game's 8 rows (angle = atan2(y, x), screen y down: 0=E, 2=S, 4=W, 6=N).
func facing(dx, dy float64, prev int) int {
	if math.Hypot(dx, dy) < 1e-6 {
		return prev
	}
	ang := math.Mod(math.Atan2(dy, dx)+2*math.Pi, 2*math.Pi)
	return int(math.Round(ang/(2*math.Pi/8))) % 8
}

func dist(a, b Vec) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// visible returns the sorted ids of entities within the viewer's interest radius (the viewer included).
func (s *Sim) visible(viewer *Entity) []EntityID {
	var out []EntityID
	for id, e := range s.entities {
		if id == viewer.ID || dist(viewer.Pos, e.Pos) <= s.cfg.InterestRadius {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

type Snapshot struct {
	Tick     uint64   `json:"tick"`
	TickMS   int      `json:"tick_ms"`
	You      EntityID `json:"you"`
	Bounds   Rect2    `json:"bounds"`
	Entities []View   `json:"entities"`
}

type Rect2 struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
}

// SnapshotFor builds a full interest-filtered snapshot and resets the viewer's known set to it.
func (s *Sim) SnapshotFor(id EntityID) (Snapshot, bool) {
	v, ok := s.entities[id]
	if !ok {
		return Snapshot{}, false
	}
	ids := s.visible(v)
	known := make(map[EntityID]struct{}, len(ids))
	views := make([]View, 0, len(ids))
	for _, eid := range ids {
		known[eid] = struct{}{}
		views = append(views, s.entities[eid].view())
	}
	s.views[id] = known
	b := s.cfg.Bounds
	return Snapshot{Tick: s.tick, TickMS: 1000 / s.cfg.TickRate, You: id, Bounds: Rect2{b.MinX, b.MinY, b.MaxX, b.MaxY}, Entities: views}, true
}

type Delta struct {
	Tick   uint64     `json:"tick"`
	AckSeq uint64     `json:"ack_seq"`
	Upsert []View     `json:"upsert,omitempty"`
	Remove []EntityID `json:"remove,omitempty"`
}

// DeltaFor returns what changed for this viewer in the current tick: entities that entered its interest set
// or changed, and ids that left it or disappeared. ok=false when there is nothing to send.
func (s *Sim) DeltaFor(id EntityID) (Delta, bool) {
	v, ok := s.entities[id]
	if !ok {
		return Delta{}, false
	}
	known := s.views[id]
	if known == nil {
		known = map[EntityID]struct{}{}
		s.views[id] = known
	}
	now := map[EntityID]struct{}{}
	var d Delta
	for _, eid := range s.visible(v) {
		now[eid] = struct{}{}
		e := s.entities[eid]
		if _, was := known[eid]; !was || e.changed >= s.tick {
			d.Upsert = append(d.Upsert, e.view())
		}
	}
	for eid := range known {
		if _, still := now[eid]; !still {
			d.Remove = append(d.Remove, eid)
		}
	}
	sort.Slice(d.Remove, func(i, j int) bool { return d.Remove[i] < d.Remove[j] })
	s.views[id] = now
	d.Tick, d.AckSeq = s.tick, v.lastSeq
	return d, len(d.Upsert) > 0 || len(d.Remove) > 0 || v.changed >= s.tick
}
