package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/damage"
)

var (
	ErrBadRequest    = errors.New("BAD_REQUEST")
	ErrNotFound      = errors.New("NOT_FOUND")
	ErrStateConflict = errors.New("STATE_CONFLICT")
	ErrInternal      = errors.New("INTERNAL")
)

// ZoneDef is content/zones/*.json.
type ZoneDef struct {
	ZoneID         string  `json:"zone_id"`
	RegionID       string  `json:"region_id"`
	TickRate       int     `json:"tick_rate"`
	PlayerSpeed    float64 `json:"player_speed"`
	InterestRadius float64 `json:"interest_radius"`
	Bounds         struct {
		MinX float64 `json:"min_x"`
		MinY float64 `json:"min_y"`
		MaxX float64 `json:"max_x"`
		MaxY float64 `json:"max_y"`
	} `json:"bounds"`
	PlayerSpawn struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"player_spawn"`
	Enemies []struct {
		SpawnID string  `json:"spawn_id"`
		EnemyID string  `json:"enemy_id"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
	} `json:"enemies"`
}

func (d ZoneDef) Config() Config {
	return Config{TickRate: d.TickRate, PlayerSpeed: d.PlayerSpeed, InterestRadius: d.InterestRadius,
		Bounds:      Rect{d.Bounds.MinX, d.Bounds.MinY, d.Bounds.MaxX, d.Bounds.MaxY},
		PlayerSpawn: Vec{d.PlayerSpawn.X, d.PlayerSpawn.Y}}
}

func (d ZoneDef) Validate() error {
	if d.ZoneID == "" || d.TickRate < 5 || d.TickRate > 30 || d.PlayerSpeed <= 0 || d.InterestRadius <= 0 ||
		d.Bounds.MaxX <= d.Bounds.MinX || d.Bounds.MaxY <= d.Bounds.MinY {
		return fmt.Errorf("zone %q: invalid tick rate, speed, interest radius or bounds", d.ZoneID)
	}
	seen := map[string]bool{}
	for _, e := range d.Enemies {
		if e.SpawnID == "" || seen[e.SpawnID] || e.EnemyID == "" {
			return fmt.Errorf("zone %s: bad or duplicate spawn %q", d.ZoneID, e.SpawnID)
		}
		seen[e.SpawnID] = true
		if e.X < d.Bounds.MinX || e.X > d.Bounds.MaxX || e.Y < d.Bounds.MinY || e.Y > d.Bounds.MaxY {
			return fmt.Errorf("zone %s: spawn %s outside bounds", d.ZoneID, e.SpawnID)
		}
	}
	return nil
}

func LoadZoneDefs(dir string) (map[string]ZoneDef, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no zone definitions in %s", dir)
	}
	out := map[string]ZoneDef{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var d ZoneDef
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if err := d.Validate(); err != nil {
			return nil, err
		}
		out[d.ZoneID] = d
	}
	return out, nil
}

// ResolveZonesDir finds content/zones from the working directory upwards.
func ResolveZonesDir(preferred string) string {
	for _, c := range []string{preferred, "content/zones", "../content/zones", "../../content/zones", "../../../content/zones"} {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return preferred
}

// Pusher delivers a server→client message to one session; false = the session is gone or overloaded.
type Pusher = func(typ string, payload any) bool

// CriticalPusher delivers a message that must not be dropped (loot results); false = session gone/overloaded.
type CriticalPusher = func(typ string, revision int64, payload any) bool

type PlayerInfo struct {
	Name      string
	ClassID   string
	Level     int
	AccountID string
	Character int64
}

type session struct {
	owner  any
	entity EntityID
	push   Pusher
	crit   CriticalPusher
	info   PlayerInfo
	recent []castRecord // idempotency window for cast request ids
}

type castRecord struct {
	requestID string
	skill     string
	target    EntityID
	out       CastOutcome
}

// Contributor of a kill with what the loot hook needs to reach the player.
type KillContributor struct {
	Character int64
	AccountID string
	Push      CriticalPusher
}

type KillEvent struct {
	EnemyID      string
	Spawn        string
	Contributors []KillContributor
}

// KillHook is called (outside the zone lock, in its own goroutine) for every enemy the server saw die.
type KillHook func(KillEvent)

// Zone owns one Sim and its sessions. A mutex serialises commands and ticks (equivalent to a command inbox
// at this scale); Tick is exported so tests drive time explicitly.
type Zone struct {
	mu       sync.Mutex
	def      ZoneDef
	sim      *Sim
	sessions map[string]*session // by character id
	onKill   KillHook
	units    float64 // world units per metre
}

// SetKillHook installs the callback for server-observed kills (loot, later quests).
func (z *Zone) SetKillHook(h KillHook) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.onKill = h
}

func NewZone(def ZoneDef, cat *combatdata.Catalog) (*Zone, error) {
	if err := def.Validate(); err != nil {
		return nil, err
	}
	z := &Zone{def: def, sim: New(def.Config()), sessions: map[string]*session{}}
	z.sim.SetCombat(NewCombat(cat))
	for _, e := range def.Enemies {
		en, ok := cat.Enemies[e.EnemyID]
		if !ok {
			return nil, fmt.Errorf("zone %s spawn %s: unknown enemy %s", def.ZoneID, e.SpawnID, e.EnemyID)
		}
		st, ok := cat.Stats(e.EnemyID, damage.NoScale)
		if !ok {
			return nil, fmt.Errorf("zone %s spawn %s: no stats for %s", def.ZoneID, e.SpawnID, e.EnemyID)
		}
		id := z.sim.AddEnemy(e.SpawnID, e.EnemyID, en.Name, en.Level, st.MaxHP, Vec{e.X, e.Y})
		z.sim.SetEnemyStats(id, st)
		z.sim.SetEnemyDef(id, EnemyDef{AI: en.AI, Element: en.Element, A: en.Abilities["A"], B: en.Abilities["B"]})
	}
	return z, nil
}

func (z *Zone) ID() string { return z.def.ZoneID }

func (z *Zone) PlayerCount() int {
	z.mu.Lock()
	defer z.mu.Unlock()
	return len(z.sessions)
}

// Join adds (or re-attaches) a character; a newer session replaces the older one's pusher.
func (z *Zone) Join(characterID string, owner any, info PlayerInfo, push Pusher, crit CriticalPusher) (Snapshot, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	stats := damage.PlayerStats(info.ClassID, info.Level, damage.StartAttributes, damage.Gear{})
	id := z.sim.AddPlayer(characterID, info.Name, info.Level, stats.MaxHP)
	z.sim.SetPlayerCombat(id, info.ClassID, stats)
	z.sessions[characterID] = &session{owner: owner, entity: id, push: push, crit: crit, info: info}
	snap, ok := z.sim.SnapshotFor(id)
	if !ok {
		return Snapshot{}, ErrInternal
	}
	return snap, nil
}

func (z *Zone) lookup(characterID string, owner any) (*session, error) {
	s, ok := z.sessions[characterID]
	if !ok || s.owner != owner {
		return nil, ErrStateConflict // not joined (or a newer session owns the character)
	}
	return s, nil
}

func (z *Zone) Move(characterID string, owner any, seq uint64, target Vec) error {
	z.mu.Lock()
	defer z.mu.Unlock()
	s, err := z.lookup(characterID, owner)
	if err != nil {
		return err
	}
	z.sim.Move(s.entity, seq, target)
	return nil
}

func (z *Zone) Stop(characterID string, owner any, seq uint64) error {
	z.mu.Lock()
	defer z.mu.Unlock()
	s, err := z.lookup(characterID, owner)
	if err != nil {
		return err
	}
	z.sim.Stop(s.entity, seq)
	return nil
}

// Leave removes the character if `owner` is still the session that joined.
func (z *Zone) Leave(characterID string, owner any) {
	z.mu.Lock()
	defer z.mu.Unlock()
	if s, ok := z.sessions[characterID]; ok && s.owner == owner {
		z.sim.RemovePlayer(s.entity)
		delete(z.sessions, characterID)
	}
}

// Cast applies a skill for the owning session; a repeated requestID inside the window returns the stored
// outcome without applying it again (idempotent resend after reconnect).
func (z *Zone) Cast(characterID string, owner any, requestID, skillID string, target EntityID) (CastOutcome, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	s, err := z.lookup(characterID, owner)
	if err != nil {
		return CastOutcome{}, err
	}
	for _, r := range s.recent {
		if r.requestID == requestID {
			if r.skill != skillID || r.target != target {
				return CastOutcome{}, ErrStateConflict // same id, different content
			}
			return r.out, nil
		}
	}
	out, err := z.sim.Cast(s.entity, skillID, target)
	if err != nil {
		return CastOutcome{}, err
	}
	s.recent = append(s.recent, castRecord{requestID, skillID, target, out})
	if len(s.recent) > 32 {
		s.recent = s.recent[1:]
	}
	return out, nil
}

// Tick steps the simulation once and pushes each session its delta and visible combat events.
func (z *Zone) Tick() {
	z.mu.Lock()
	z.tickLocked()
	hook, kills := z.onKill, z.collectKillsLocked()
	z.mu.Unlock()
	if hook != nil {
		for _, k := range kills {
			go hook(k)
		}
	}
}

func (z *Zone) collectKillsLocked() []KillEvent {
	var out []KillEvent
	for _, k := range z.sim.DrainKills() {
		ev := KillEvent{EnemyID: k.EnemyID, Spawn: k.Spawn}
		for _, c := range k.Contributors {
			if s, ok := z.sessions[c.Ref]; ok {
				ev.Contributors = append(ev.Contributors, KillContributor{Character: s.info.Character, AccountID: s.info.AccountID, Push: s.crit})
			}
		}
		out = append(out, ev)
	}
	return out
}

func (z *Zone) tickLocked() {
	z.sim.Step()
	events := z.sim.DrainEvents()
	for _, s := range z.sessions {
		if mine := z.sim.EventsFor(s.entity, events); len(mine) > 0 {
			s.push("world.event", map[string]any{"tick": z.sim.Tick(), "events": mine})
		}
	}
	for ref, s := range z.sessions {
		d, send := z.sim.DeltaFor(s.entity)
		if !send {
			continue
		}
		if !s.push("world.delta", d) {
			z.sim.RemovePlayer(s.entity)
			delete(z.sessions, ref)
		}
	}
}

// Run ticks at the zone's rate until ctx is done.
func (z *Zone) Run(ctx context.Context) {
	t := time.NewTicker(time.Second / time.Duration(z.def.TickRate))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			z.Tick()
		}
	}
}
