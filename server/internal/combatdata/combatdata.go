// Package combatdata loads enemies, enemy abilities and class skills from a content export
// (kinds `enemy`, `ability`, `skill`, `class`) for the combat simulation (T-038b/c).
// Numbers come from canon; free-text mechanics stay as Text for hand-written boss scripts.
package combatdata

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/NewMMO/server/internal/damage"
)

var durationRe = regexp.MustCompile(`(\d+)\s*с(?:\s|[,;.]|$)`)

var gainRe = regexp.MustCompile(`\+(\d+)\s+(?:стойкости|ярости|фокуса|света|маны)`)

type Ability struct {
	ID         string
	EnemyID    string
	Slot       string // A basic attack, B signature, C/D elite & boss mechanics
	Coef       float64
	HasCoef    bool
	Scale      string // AP | SP | AP|SP
	CooldownS  float64
	HasCD      bool // A: attack interval
	TelegraphS float64
	Text       string
}

type Enemy struct {
	ID        string
	Name      string
	Level     int
	Category  string // exported `threat`: обычный | элита | мини-босс | босс DNG | мировой босс
	AI        string
	Element   string
	RegionID  string
	Abilities map[string]Ability // by slot
}

type Skill struct {
	ID         string
	ClassID    string
	Name       string
	Level      int
	Target     string
	Cost       int
	CooldownS  float64
	EffectKind string
	Coef       float64
	Scale      string
	Rule       string
	Gain       int // resource gained on cast, parsed from the rule text ("+10 стойкости")
	DurationS  int // first "N с" in the rule text (shield/buff duration), 0 if absent
}

type Catalog struct {
	Enemies map[string]Enemy
	Skills  map[string][]Skill // by class id, ordered by unlock level
}

type row struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	RegionID *string         `json:"region_id"`
	Payload  json.RawMessage `json:"payload"`
}

// LoadExport reads <dir>/entities.jsonl.
func LoadExport(dir string) (*Catalog, error) {
	f, err := os.Open(filepath.Join(dir, "entities.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := newBuilder()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var r row
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, err
		}
		if err := b.add(r); err != nil {
			return nil, err
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return b.finish()
}

// LoadFromDB reads the combat kinds of one published content build from content_entities.
func LoadFromDB(ctx context.Context, pool *pgxpool.Pool, buildID string) (*Catalog, error) {
	rows, err := pool.Query(ctx, `
		SELECT entity_id, kind, region_id, payload FROM public.content_entities
		WHERE build_id = $1 AND kind IN ('enemy', 'ability', 'skill')
		ORDER BY kind, entity_id`, buildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	b := newBuilder()
	for rows.Next() {
		var r row
		var payload []byte
		if err := rows.Scan(&r.ID, &r.Kind, &r.RegionID, &payload); err != nil {
			return nil, err
		}
		r.Payload = payload
		if err := b.add(r); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return b.finish()
}

type builder struct {
	c         *Catalog
	abilities []Ability
}

func newBuilder() *builder {
	return &builder{c: &Catalog{Enemies: map[string]Enemy{}, Skills: map[string][]Skill{}}}
}

func (b *builder) add(r row) error {
	c := b.c
	switch r.Kind {
	case "enemy":
		var p struct {
			Name    string `json:"name"`
			Level   int    `json:"level"`
			Threat  string `json:"threat"`
			AI      string `json:"ai"`
			Element string `json:"element"`
		}
		if err := json.Unmarshal(r.Payload, &p); err != nil {
			return err
		}
		e := Enemy{ID: r.ID, Name: p.Name, Level: p.Level, Category: p.Threat, AI: p.AI, Element: p.Element, Abilities: map[string]Ability{}}
		if r.RegionID != nil {
			e.RegionID = *r.RegionID
		}
		c.Enemies[r.ID] = e
	case "ability":
		var p struct {
			Enemy     string   `json:"enemy"`
			Slot      string   `json:"slot"`
			Coef      *float64 `json:"coefficient"`
			Scale     string   `json:"scale"`
			Cooldown  *float64 `json:"cooldown_s"`
			Telegraph *float64 `json:"telegraph_s"`
			Text      string   `json:"text"`
		}
		if err := json.Unmarshal(r.Payload, &p); err != nil {
			return err
		}
		a := Ability{ID: r.ID, EnemyID: p.Enemy, Slot: p.Slot, Scale: p.Scale, Text: p.Text}
		if p.Coef != nil {
			a.Coef, a.HasCoef = *p.Coef, true
		}
		if p.Cooldown != nil {
			a.CooldownS, a.HasCD = *p.Cooldown, true
		}
		if p.Telegraph != nil {
			a.TelegraphS = *p.Telegraph
		}
		b.abilities = append(b.abilities, a)
	case "skill":
		var p struct {
			ClassID string   `json:"class_id"`
			Name    string   `json:"name"`
			Level   int      `json:"level"`
			Target  string   `json:"target"`
			Cost    int      `json:"cost"`
			CD      float64  `json:"cooldown_s"`
			Kind    string   `json:"effect_kind"`
			Coef    *float64 `json:"coefficient"`
			Scale   string   `json:"scale"`
			Rule    string   `json:"rule"`
		}
		if err := json.Unmarshal(r.Payload, &p); err != nil {
			return err
		}
		s := Skill{ID: r.ID, ClassID: p.ClassID, Name: p.Name, Level: p.Level, Target: p.Target, Cost: p.Cost,
			CooldownS: p.CD, EffectKind: p.Kind, Scale: p.Scale, Rule: p.Rule}
		if p.Coef != nil {
			s.Coef = *p.Coef
		}
		if m := durationRe.FindStringSubmatch(p.Rule); m != nil {
			s.DurationS, _ = strconv.Atoi(m[1])
		}
		if m := gainRe.FindStringSubmatch(p.Rule); m != nil {
			s.Gain, _ = strconv.Atoi(m[1])
		}
		c.Skills[p.ClassID] = append(c.Skills[p.ClassID], s)
	}
	return nil
}

func (b *builder) finish() (*Catalog, error) {
	c := b.c
	for _, a := range b.abilities {
		e, ok := c.Enemies[a.EnemyID]
		if !ok {
			return nil, fmt.Errorf("ability %s: unknown enemy %s", a.ID, a.EnemyID)
		}
		e.Abilities[a.Slot] = a
	}
	for _, list := range c.Skills {
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	}
	return c, nil
}

// Validate checks what the simulation relies on: known categories, attack A and signature B with numbers.
func (c *Catalog) Validate() error {
	for _, e := range c.Enemies {
		if _, ok := damage.Categories[e.Category]; !ok {
			return fmt.Errorf("%s: unknown category %q", e.ID, e.Category)
		}
		for _, slot := range []string{"A", "B"} {
			a, ok := e.Abilities[slot]
			if !ok || !a.HasCoef || !a.HasCD {
				return fmt.Errorf("%s: ability %s missing or without coefficient/cooldown", e.ID, slot)
			}
		}
	}
	for id, list := range c.Skills {
		if len(list) != 12 {
			return fmt.Errorf("class %s: %d skills, want 12", id, len(list))
		}
	}
	return nil
}

// Stats returns the canon stats of an enemy for the given group scale.
func (c *Catalog) Stats(enemyID string, s damage.Scale) (damage.EnemyStatsOut, bool) {
	e, ok := c.Enemies[enemyID]
	if !ok {
		return damage.EnemyStatsOut{}, false
	}
	return damage.EnemyStats(e.Level, e.Category, s)
}
