package dungeon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

type Spawn struct {
	ID       string     `json:"id"`
	EnemyID  string     `json:"enemy_id"`
	Optional bool       `json:"optional"`
	Position *world.Vec `json:"position"`
}

type Clear struct {
	Objects   []string `json:"objects"`
	SpawnsAll bool     `json:"spawns_all"`
	Puzzle    bool     `json:"puzzle"`
}

type RoomLayout struct {
	Width  float64    `json:"width"`
	Height float64    `json:"height"`
	Entry  *world.Vec `json:"entry"`
}

type Room struct {
	ID              string      `json:"id"`
	Kind            string      `json:"kind"` // entrance | fight | puzzle | miniboss | rest | boss | exit
	Objects         []string    `json:"objects"`
	OptionalObjects []string    `json:"optional_objects"`
	Spawns          []Spawn     `json:"spawns"`
	Clear           Clear       `json:"clear"`
	Layout          *RoomLayout `json:"layout"`
}

type Definition struct {
	DungeonID          string   `json:"dungeon_id"`
	RegionID           string   `json:"region_id"`
	EntryLevel         int      `json:"entry_level"`
	Modes              []string `json:"modes"`
	BossID             string   `json:"boss_id"`
	MiniBossID         string   `json:"miniboss_id"`
	FirstClearMaterial string   `json:"first_clear_material_id"`
	CheckpointRooms    []string `json:"checkpoint_room_ids"`
	Flags              struct {
		FirstClear  string `json:"first_clear"`
		NormalClear string `json:"normal_clear"`
	} `json:"flags"`
	Puzzle struct {
		Room  string   `json:"room"`
		Order []string `json:"order"`
	} `json:"puzzle"`
	Rooms   []Room `json:"rooms"`
	Rewards struct {
		Kind   string   `json:"kind"`
		Tables []string `json:"tables"` // LOOT-ENM-* ids; mapped to enemies through the loot catalog
	} `json:"rewards"`
	IdleExpiryMinutes int `json:"idle_expiry_minutes"`
}

func (d Definition) HasMode(m string) bool {
	for _, x := range d.Modes {
		if x == m {
			return true
		}
	}
	return false
}

func (d Definition) RoomIndex(id string) int {
	for i, r := range d.Rooms {
		if r.ID == id {
			return i
		}
	}
	return -1
}

func (d Definition) IsCheckpoint(id string) bool {
	for _, c := range d.CheckpointRooms {
		if c == id {
			return true
		}
	}
	return false
}

// Validate checks the internal consistency of one definition (graph, puzzle, checkpoints).
func (d Definition) Validate() error {
	if d.DungeonID == "" || len(d.Rooms) < 2 {
		return fmt.Errorf("%s: id and at least 2 rooms required", d.DungeonID)
	}
	seen := map[string]bool{}
	objs := map[string]string{}
	for _, r := range d.Rooms {
		if r.ID == "" || seen[r.ID] {
			return fmt.Errorf("%s: duplicate or empty room id %q", d.DungeonID, r.ID)
		}
		seen[r.ID] = true
		if _, err := r.zoneDef(nil); err != nil {
			return err
		}
		for _, o := range append(append([]string{}, r.Objects...), r.OptionalObjects...) {
			if prev, dup := objs[o]; dup {
				return fmt.Errorf("%s: object %s in rooms %s and %s", d.DungeonID, o, prev, r.ID)
			}
			objs[o] = r.ID
		}
		for _, o := range r.Clear.Objects {
			if objs[o] != r.ID {
				return fmt.Errorf("%s: room %s clear object %s is not a required object of the room", d.DungeonID, r.ID, o)
			}
		}
	}
	if len(d.CheckpointRooms) == 0 || d.RoomIndex(d.CheckpointRooms[0]) != 0 {
		return fmt.Errorf("%s: first room must be a checkpoint", d.DungeonID)
	}
	for _, c := range d.CheckpointRooms {
		if d.RoomIndex(c) < 0 {
			return fmt.Errorf("%s: unknown checkpoint room %s", d.DungeonID, c)
		}
	}
	if d.Puzzle.Room != "" {
		pr := d.Rooms[d.RoomIndex(d.Puzzle.Room)]
		if len(d.Puzzle.Order) == 0 {
			return fmt.Errorf("%s: puzzle order empty", d.DungeonID)
		}
		for _, o := range d.Puzzle.Order {
			if objs[o] != pr.ID {
				return fmt.Errorf("%s: puzzle object %s not in room %s", d.DungeonID, o, pr.ID)
			}
		}
	}
	if d.Flags.FirstClear == "" || d.Flags.NormalClear == "" || len(d.Rewards.Tables) == 0 || d.IdleExpiryMinutes < 1 {
		return fmt.Errorf("%s: flags, reward tables and idle_expiry_minutes required", d.DungeonID)
	}
	return nil
}

type Registry struct{ byID map[string]Definition }

func (r *Registry) Get(id string) (Definition, bool) { d, ok := r.byID[id]; return d, ok }

func (r *Registry) IDs() []string {
	out := make([]string, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func NewRegistry(defs ...Definition) (*Registry, error) {
	r := &Registry{byID: map[string]Definition{}}
	for _, d := range defs {
		if err := d.Validate(); err != nil {
			return nil, err
		}
		r.byID[d.DungeonID] = d
	}
	return r, nil
}

// LoadDir loads every *.json definition in dir.
func LoadDir(dir string) (*Registry, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no dungeon definitions in %s", dir)
	}
	var defs []Definition
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var d Definition
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		defs = append(defs, d)
	}
	return NewRegistry(defs...)
}

// ResolveDir finds content/dungeons from the working directory upwards (tests, binary run from server/).
func ResolveDir(preferred string) string {
	for _, c := range []string{preferred, "content/dungeons", "../content/dungeons", "../../content/dungeons", "../../../content/dungeons"} {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return preferred
}
