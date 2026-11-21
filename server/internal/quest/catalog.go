package quest

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ObjectiveDef struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Need  int    // from registry count; default 1
	Event string
	Target string
	Instances []string
}

type RewardDef struct {
	XP         int64 `json:"xp"`
	Gold       int64 `json:"gold"`
	Evidence   []string `json:"evidence"`
	Reputation []struct {
		Faction string `json:"faction"`
		Delta   int    `json:"delta"`
	} `json:"reputation"`
}

type QuestDef struct {
	ID        string
	Kind      string // quest_main | quest_side
	RegionID  string
	Title     string
	MinLevel  int
	Prereqs   []string
	Flags     []string
	Objectives []ObjectiveDef
	Reward    RewardDef
}

type Interactable struct {
	ObjectiveID string   `json:"objective_id"`
	QuestID     string   `json:"quest_id"`
	Event       string   `json:"event"`
	TargetID    string   `json:"target_id"`
	Count       int      `json:"count"`
	Instances   []string `json:"-"`
}

type Catalog struct {
	mu    sync.RWMutex
	build string
	byID  map[string]QuestDef
	byObj map[string]Interactable // objective_id → registry
}

func NewCatalog() *Catalog {
	return &Catalog{byID: map[string]QuestDef{}, byObj: map[string]Interactable{}}
}

func (c *Catalog) BuildID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.build
}

func (c *Catalog) Quest(id string) (QuestDef, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	q, ok := c.byID[id]
	return q, ok
}

func (c *Catalog) All() []QuestDef {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]QuestDef, 0, len(c.byID))
	for _, q := range c.byID {
		out = append(out, q)
	}
	return out
}

func (c *Catalog) Interactable(objectiveID string) (Interactable, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	i, ok := c.byObj[objectiveID]
	return i, ok
}

func (c *Catalog) LoadInteractables(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var file struct {
		Interactables []struct {
			ObjectiveID string          `json:"objective_id"`
			QuestID     string          `json:"quest_id"`
			Event       string          `json:"event"`
			TargetID    string          `json:"target_id"`
			Count       int             `json:"count"`
			Instances   json.RawMessage `json:"instances"`
		} `json:"interactables"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	byObj := make(map[string]Interactable, len(file.Interactables))
	for _, row := range file.Interactables {
		count := row.Count
		if count < 1 {
			count = 1
		}
		var instances []string
		if len(row.Instances) > 0 && row.Instances[0] == '[' {
			_ = json.Unmarshal(row.Instances, &instances)
		}
		byObj[row.ObjectiveID] = Interactable{
			ObjectiveID: row.ObjectiveID,
			QuestID:     row.QuestID,
			Event:       row.Event,
			TargetID:    row.TargetID,
			Count:       count,
			Instances:   instances,
		}
	}
	c.mu.Lock()
	c.byObj = byObj
	c.mu.Unlock()
	return nil
}

func (c *Catalog) LoadFromDB(ctx context.Context, pool *pgxpool.Pool, buildID string) error {
	rows, err := pool.Query(ctx, `
		SELECT entity_id, kind, coalesce(region_id, ''), payload
		FROM public.content_entities
		WHERE build_id = $1 AND kind IN ('quest_main', 'quest_side')
		ORDER BY entity_id`, buildID)
	if err != nil {
		return err
	}
	defer rows.Close()

	byID := map[string]QuestDef{}
	for rows.Next() {
		var id, kind, region string
		var payload []byte
		if err := rows.Scan(&id, &kind, &region, &payload); err != nil {
			return err
		}
		var fields struct {
			Title      string         `json:"title"`
			MinLevel   int            `json:"min_level"`
			Prereqs    []string       `json:"prereqs"`
			Flags      []string       `json:"flags"`
			Objectives []ObjectiveDef `json:"objectives"`
			Reward     RewardDef      `json:"reward"`
		}
		if err := json.Unmarshal(payload, &fields); err != nil {
			return err
		}
		if fields.MinLevel < 1 {
			fields.MinLevel = 1
		}
		if fields.Prereqs == nil {
			fields.Prereqs = []string{}
		}
		if region == "" && strings.HasPrefix(id, "MQ-") {
			region = "REG-" + id[3:5]
		}
		q := QuestDef{
			ID: id, Kind: kind, RegionID: region, Title: fields.Title,
			MinLevel: fields.MinLevel, Prereqs: fields.Prereqs, Flags: fields.Flags,
			Objectives: fields.Objectives, Reward: fields.Reward,
		}
		byID[id] = q
	}
	if err := rows.Err(); err != nil {
		return err
	}

	c.mu.RLock()
	registry := c.byObj
	c.mu.RUnlock()

	for id, q := range byID {
		objs := make([]ObjectiveDef, len(q.Objectives))
		copy(objs, q.Objectives)
		for i := range objs {
			if reg, ok := registry[objs[i].ID]; ok {
				objs[i].Need = reg.Count
				objs[i].Event = reg.Event
				objs[i].Target = reg.TargetID
				objs[i].Instances = append([]string{}, reg.Instances...)
			} else if objs[i].Need < 1 {
				objs[i].Need = 1
			}
		}
		q.Objectives = objs
		byID[id] = q
	}

	c.mu.Lock()
	c.build = buildID
	c.byID = byID
	c.mu.Unlock()
	return nil
}

func (c *Catalog) ActiveBuildID(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		SELECT b.build_id FROM public.content_active a
		JOIN public.content_builds b ON b.build_id = a.build_id AND b.status = 'published'
		WHERE a.id = 1`).Scan(&id)
	return id, err
}

// QuestForBuild returns the quest definition for a specific content build.
// When buildID matches the active catalog build, returns the cached def.
// Otherwise loads the pinned payload from content_entities (S-01 / T-021).
func (c *Catalog) QuestForBuild(ctx context.Context, pool *pgxpool.Pool, buildID, questID string) (QuestDef, error) {
	if buildID == "" || questID == "" {
		return QuestDef{}, ErrNotFound
	}
	c.mu.RLock()
	if buildID == c.build {
		q, ok := c.byID[questID]
		c.mu.RUnlock()
		if !ok {
			return QuestDef{}, ErrNotFound
		}
		return q, nil
	}
	registry := c.byObj
	c.mu.RUnlock()

	var kind, region string
	var payload []byte
	err := pool.QueryRow(ctx, `
		SELECT kind, coalesce(region_id, ''), payload
		FROM public.content_entities
		WHERE build_id = $1 AND entity_id = $2 AND kind IN ('quest_main', 'quest_side')`,
		buildID, questID).Scan(&kind, &region, &payload)
	if err != nil {
		return QuestDef{}, ErrContentMismatch
	}
	var fields struct {
		Title      string         `json:"title"`
		MinLevel   int            `json:"min_level"`
		Prereqs    []string       `json:"prereqs"`
		Flags      []string       `json:"flags"`
		Objectives []ObjectiveDef `json:"objectives"`
		Reward     RewardDef      `json:"reward"`
	}
	if json.Unmarshal(payload, &fields) != nil {
		return QuestDef{}, ErrInternal
	}
	if fields.MinLevel < 1 {
		fields.MinLevel = 1
	}
	if fields.Prereqs == nil {
		fields.Prereqs = []string{}
	}
	if region == "" && strings.HasPrefix(questID, "MQ-") && len(questID) >= 5 {
		region = "REG-" + questID[3:5]
	}
	objs := make([]ObjectiveDef, len(fields.Objectives))
	copy(objs, fields.Objectives)
	for i := range objs {
		if reg, ok := registry[objs[i].ID]; ok {
			objs[i].Need = reg.Count
			objs[i].Event = reg.Event
			objs[i].Target = reg.TargetID
			objs[i].Instances = append([]string{}, reg.Instances...)
		} else if objs[i].Need < 1 {
			objs[i].Need = 1
		}
	}
	return QuestDef{
		ID: questID, Kind: kind, RegionID: region, Title: fields.Title,
		MinLevel: fields.MinLevel, Prereqs: fields.Prereqs, Flags: fields.Flags,
		Objectives: objs, Reward: fields.Reward,
	}, nil
}

func FlagName(questID, suffix string) string {
	if strings.HasPrefix(questID, "MQ-") {
		return "QF-" + questID + "-" + suffix
	}
	if strings.HasPrefix(questID, "SQ-") {
		if suffix == "START" {
			return "SQF-" + questID + "-ACCEPTED"
		}
		return "SQF-" + questID + "-" + suffix
	}
	return questID + "-" + suffix
}
