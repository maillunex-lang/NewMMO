package dialogue

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LineDef struct {
	ID      string
	Speaker string
	Text    string
}

type FlowDef struct {
	Start    []string
	Progress []string
	TurnIn   []string
}

type Catalog struct {
	mu    sync.RWMutex
	build string
	lines map[string]LineDef
	flows map[string]FlowDef
}

func NewCatalog() *Catalog {
	return &Catalog{lines: map[string]LineDef{}, flows: map[string]FlowDef{}}
}

func (c *Catalog) BuildID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.build
}

func (c *Catalog) Line(id string) (LineDef, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	l, ok := c.lines[id]
	return l, ok
}

func (c *Catalog) Flow(questID string) (FlowDef, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	f, ok := c.flows[questID]
	return f, ok
}

func (c *Catalog) LoadFlows(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var file struct {
		Flows map[string]struct {
			Start    []string `json:"start"`
			Progress any      `json:"progress"`
			TurnIn   []string `json:"turnin"`
		} `json:"flows"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	flows := map[string]FlowDef{}
	for id, f := range file.Flows {
		progress := []string{}
		switch v := f.Progress.(type) {
		case []any:
			for _, x := range v {
				progress = append(progress, str(x))
			}
		case map[string]any:
			// Variant map keyed by completed-objective count — flatten unique ids for EX-P05.
			seen := map[string]struct{}{}
			for _, arr := range v {
				list, _ := arr.([]any)
				for _, x := range list {
					s := str(x)
					if _, ok := seen[s]; ok {
						continue
					}
					seen[s] = struct{}{}
					progress = append(progress, s)
				}
			}
		}
		flows[id] = FlowDef{Start: f.Start, Progress: progress, TurnIn: f.TurnIn}
	}
	c.mu.Lock()
	c.flows = flows
	c.mu.Unlock()
	return nil
}

func (c *Catalog) LoadLinesFromDB(ctx context.Context, pool *pgxpool.Pool, buildID string) error {
	rows, err := pool.Query(ctx, `
		SELECT entity_id, payload FROM public.content_entities
		WHERE build_id = $1 AND kind = 'dialogue_line'`, buildID)
	if err != nil {
		return err
	}
	defer rows.Close()
	lines := map[string]LineDef{}
	for rows.Next() {
		var id string
		var payload []byte
		if err := rows.Scan(&id, &payload); err != nil {
			return err
		}
		var fields struct {
			Speaker string `json:"speaker"`
			Text    string `json:"text"`
		}
		_ = json.Unmarshal(payload, &fields)
		lines[id] = LineDef{ID: id, Speaker: fields.Speaker, Text: fields.Text}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.build = buildID
	c.lines = lines
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

func ResolveFlowsPath(preferred string) string {
	if preferred != "" {
		if _, err := os.Stat(preferred); err == nil {
			return preferred
		}
	}
	for _, c := range []string{
		"content/dialogues/flows_reg01.json",
		"../content/dialogues/flows_reg01.json",
		"../../content/dialogues/flows_reg01.json",
		"../../../content/dialogues/flows_reg01.json",
	} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return preferred
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
