package dungeon

// Room / instance states (docs/contracts/dungeon-engine.md §2).
const (
	RoomLocked     = "LOCKED"
	RoomAvailable  = "AVAILABLE"
	RoomInProgress = "IN_PROGRESS"
	RoomCleared    = "CLEARED"

	StatusActive    = "ACTIVE"
	StatusBoss      = "BOSS_ACTIVE"
	StatusCompleted = "COMPLETED"
	StatusClosed    = "CLOSED"
	StatusExpired   = "EXPIRED"
	StatusAbandoned = "ABANDONED"
)

type RoomState struct {
	State          string          `json:"state"`
	Objects        map[string]bool `json:"objects"`
	Spawns         map[string]bool `json:"spawns"`
	PuzzleProgress []string        `json:"puzzle_progress,omitempty"`
	PuzzleSolved   bool            `json:"puzzle_solved,omitempty"`
}

// Machine is the pure, DB-free dungeon state: current room, statuses and per-room progress.
type Machine struct {
	Status       string
	RoomID       string
	Checkpoint   string
	BossAttempts int
	Rooms        map[string]*RoomState
}

type Event struct {
	Type    string
	Payload map[string]any
}

func newRoomState(r Room, state string) *RoomState {
	rs := &RoomState{State: state, Objects: map[string]bool{}, Spawns: map[string]bool{}}
	for _, o := range r.Objects {
		rs.Objects[o] = false
	}
	for _, o := range r.OptionalObjects {
		rs.Objects[o] = false
	}
	for _, s := range r.Spawns {
		rs.Spawns[s.ID] = false
	}
	return rs
}

// NewMachine starts at the first room (entrance), already in progress.
func NewMachine(def Definition) *Machine {
	m := &Machine{Status: StatusActive, RoomID: def.Rooms[0].ID, Checkpoint: def.Rooms[0].ID, Rooms: map[string]*RoomState{}}
	for i, r := range def.Rooms {
		st := RoomLocked
		if i == 0 {
			st = RoomInProgress
		}
		m.Rooms[r.ID] = newRoomState(r, st)
	}
	return m
}

func room(def Definition, id string) Room { return def.Rooms[def.RoomIndex(id)] }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// cleared evaluates the room's server-side clear condition (never a client claim).
func cleared(def Definition, r Room, rs *RoomState) bool {
	for _, o := range r.Clear.Objects {
		if !rs.Objects[o] {
			return false
		}
	}
	if r.Clear.SpawnsAll {
		for _, s := range r.Spawns {
			if !s.Optional && !rs.Spawns[s.ID] {
				return false
			}
		}
	}
	if r.Clear.Puzzle && !rs.PuzzleSolved {
		return false
	}
	return true
}

// advance clears the current room when its condition holds and opens the next one (atomic chain:
// a room with an empty condition, e.g. the exit, is cleared at once). Never skips a room.
func (m *Machine) advance(def Definition) []Event {
	var evs []Event
	for m.Status == StatusActive || m.Status == StatusBoss {
		idx := def.RoomIndex(m.RoomID)
		r := def.Rooms[idx]
		rs := m.Rooms[r.ID]
		if !cleared(def, r, rs) {
			break
		}
		rs.State = RoomCleared
		evs = append(evs, Event{"ROOM_CLEARED", map[string]any{"room_id": r.ID}})
		if r.Kind == "rest" || def.IsCheckpoint(r.ID) {
			m.Checkpoint = r.ID
			if r.Kind == "rest" {
				evs = append(evs, Event{"CHECKPOINT", map[string]any{"room_id": r.ID}})
			}
		}
		if idx+1 >= len(def.Rooms) {
			m.Status = StatusCompleted
			evs = append(evs, Event{"INSTANCE_COMPLETED", map[string]any{}})
			break
		}
		next := def.Rooms[idx+1]
		m.RoomID = next.ID
		m.Rooms[next.ID].State = RoomInProgress
		if next.Kind == "boss" {
			m.Status = StatusBoss
		} else if next.Kind == "exit" {
			// exit room has no condition: reaching it completes the run
			m.Status = StatusActive
		}
		evs = append(evs, Event{"ROOM_OPENED", map[string]any{"room_id": next.ID}})
	}
	return evs
}

func (m *Machine) live() error {
	if m.Status != StatusActive && m.Status != StatusBoss {
		return ErrStateConflict
	}
	return nil
}

// Interact applies an object interaction in the current room only.
func (m *Machine) Interact(def Definition, objectID string) ([]Event, error) {
	if err := m.live(); err != nil {
		return nil, err
	}
	r := room(def, m.RoomID)
	rs := m.Rooms[r.ID]
	if !contains(r.Objects, objectID) && !contains(r.OptionalObjects, objectID) {
		return nil, ErrInvalidTarget
	}
	var evs []Event
	if def.Puzzle.Room == r.ID && contains(def.Puzzle.Order, objectID) {
		if rs.PuzzleSolved || rs.Objects[objectID] {
			return nil, nil // idempotent
		}
		if def.Puzzle.Order[len(rs.PuzzleProgress)] == objectID {
			rs.PuzzleProgress = append(rs.PuzzleProgress, objectID)
			rs.Objects[objectID] = true
			evs = append(evs, Event{"PUZZLE_STEP", map[string]any{"object_id": objectID, "step": len(rs.PuzzleProgress)}})
			if len(rs.PuzzleProgress) == len(def.Puzzle.Order) {
				rs.PuzzleSolved = true
				evs = append(evs, Event{"PUZZLE_SOLVED", map[string]any{"room_id": r.ID}})
			}
		} else {
			// Wrong order: reset only the current sequence, never lock the door.
			for _, o := range def.Puzzle.Order {
				rs.Objects[o] = false
			}
			rs.PuzzleProgress = nil
			evs = append(evs, Event{"PUZZLE_RESET", map[string]any{"object_id": objectID}})
		}
		return append(evs, m.advance(def)...), nil
	}
	if rs.Objects[objectID] {
		return nil, nil // idempotent
	}
	rs.Objects[objectID] = true
	evs = append(evs, Event{"OBJECT_INTERACTED", map[string]any{"object_id": objectID}})
	return append(evs, m.advance(def)...), nil
}

// EnemyDefeated records a server-observed kill for a spawn in the current room.
func (m *Machine) EnemyDefeated(def Definition, spawnID string) ([]Event, error) {
	if err := m.live(); err != nil {
		return nil, err
	}
	r := room(def, m.RoomID)
	rs := m.Rooms[r.ID]
	dead, ok := rs.Spawns[spawnID]
	if !ok {
		return nil, ErrInvalidTarget
	}
	if dead {
		return nil, nil // idempotent
	}
	rs.Spawns[spawnID] = true
	evs := []Event{{"ENEMY_DEFEATED", map[string]any{"spawn_id": spawnID}}}
	return append(evs, m.advance(def)...), nil
}

// PlayerDefeated returns the run to the last checkpoint; rooms after it lose their progress.
func (m *Machine) PlayerDefeated(def Definition) ([]Event, error) {
	if err := m.live(); err != nil {
		return nil, err
	}
	if m.Status == StatusBoss {
		m.BossAttempts++
	}
	cp := def.RoomIndex(m.Checkpoint)
	cur := def.RoomIndex(m.RoomID)
	for i := cp + 1; i <= cur; i++ {
		r := def.Rooms[i]
		m.Rooms[r.ID] = newRoomState(r, RoomLocked)
	}
	// The checkpoint room itself is already cleared (or is the entrance): advance() opens the next room.
	m.RoomID = def.Rooms[cp].ID
	m.Status = StatusActive
	m.Rooms[m.RoomID].State = RoomInProgress
	evs := []Event{{"PLAYER_DEFEATED", map[string]any{"checkpoint": m.Checkpoint, "boss_attempts": m.BossAttempts}}}
	return append(evs, m.advance(def)...), nil
}
