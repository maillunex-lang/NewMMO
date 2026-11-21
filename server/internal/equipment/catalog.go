package equipment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Template is one canon EQ-CC-RR-SS-NN row (11 §6).
type Template struct {
	ID      string
	ClassID string // CLS-0x
	Region  string // REG-0x
	Slot    string
	Name    string
	Level   int
}

type Catalog struct {
	mu    sync.RWMutex
	build string
	byID  map[string]Template
	// byKey: class|region|slot -> template ids (sorted)
	byKey map[string][]string
}

func NewCatalog() *Catalog {
	return &Catalog{byID: map[string]Template{}, byKey: map[string][]string{}}
}

func key(class, region, slot string) string { return class + "|" + region + "|" + slot }

func (c *Catalog) Put(t Template) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID[t.ID] = t
	k := key(t.ClassID, t.Region, t.Slot)
	c.byKey[k] = append(c.byKey[k], t.ID)
	sort.Strings(c.byKey[k])
}

func (c *Catalog) Get(id string) (Template, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.byID[id]
	return t, ok
}

// Candidates lists template ids a class may drop in a region for one slot.
func (c *Catalog) Candidates(class, region, slot string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string(nil), c.byKey[key(class, region, slot)]...)
}

func (c *Catalog) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.byID)
}

// classFromID maps the class number inside EQ-CC-… to CLS-CC.
func classFromID(id string) string {
	if len(id) < 5 {
		return ""
	}
	return "CLS-" + id[3:5]
}

// LoadFromDB reads kind=equipment entities of one content build.
func (c *Catalog) LoadFromDB(ctx context.Context, pool *pgxpool.Pool, buildID string) error {
	rows, err := pool.Query(ctx, `
		SELECT entity_id, coalesce(region_id, ''), payload
		FROM public.content_entities
		WHERE build_id = $1 AND kind = 'equipment'
		ORDER BY entity_id`, buildID)
	if err != nil {
		return err
	}
	defer rows.Close()
	n := NewCatalog()
	for rows.Next() {
		var id, region string
		var payload []byte
		if err := rows.Scan(&id, &region, &payload); err != nil {
			return err
		}
		var f struct {
			Slot  string `json:"slot"`
			Name  string `json:"name"`
			Level int    `json:"level"`
		}
		if err := json.Unmarshal(payload, &f); err != nil {
			return err
		}
		cls := classFromID(id)
		if !ValidSlot(f.Slot) || classShares[cls] == nil || region == "" || f.Level < 1 {
			return fmt.Errorf("equipment %s: invalid slot/class/region/level", id)
		}
		n.Put(Template{ID: id, ClassID: cls, Region: region, Slot: f.Slot, Name: f.Name, Level: f.Level})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.build, c.byID, c.byKey = buildID, n.byID, n.byKey
	c.mu.Unlock()
	return nil
}
