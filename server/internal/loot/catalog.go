package loot

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Table is one LOOT-ENM-* row exported from canon 11 §9 (percentages as in canon, 0..100).
type Table struct {
	ID          string
	EnemyID     string
	RegionID    string
	Category    string // обычный | элита | мини-босс | босс DNG | мировой босс
	GoldMin     int64
	GoldMax     int64
	Mat1Pct     float64
	Mat3Pct     float64
	SpecialItem string
	SpecialPct  float64
}

// Claimable reports whether an open-world kill report may be rolled for this table (EX-P08 interim).
// Dungeon and world bosses wait for EX-P09/P10 (instance/weekly rules).
func (t Table) Claimable() bool {
	switch t.Category {
	case "обычный", "элита", "мини-босс":
		return true
	}
	return false
}

type Catalog struct {
	mu      sync.RWMutex
	build   string
	byEnemy map[string]Table
}

func NewCatalog() *Catalog { return &Catalog{byEnemy: map[string]Table{}} }

func (c *Catalog) Put(t Table) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byEnemy[t.EnemyID] = t
}

func (c *Catalog) ForEnemy(enemyID string) (Table, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.byEnemy[enemyID]
	return t, ok
}

func (c *Catalog) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.byEnemy)
}

func (c *Catalog) BuildID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.build
}

// LoadFromDB reads kind=loot_table entities of one content build (same source as the quest catalog).
func (c *Catalog) LoadFromDB(ctx context.Context, pool *pgxpool.Pool, buildID string) error {
	rows, err := pool.Query(ctx, `
		SELECT entity_id, coalesce(region_id, ''), payload
		FROM public.content_entities
		WHERE build_id = $1 AND kind = 'loot_table'
		ORDER BY entity_id`, buildID)
	if err != nil {
		return err
	}
	defer rows.Close()
	byEnemy := map[string]Table{}
	for rows.Next() {
		var id, region string
		var payload []byte
		if err := rows.Scan(&id, &region, &payload); err != nil {
			return err
		}
		var f struct {
			Enemy       string  `json:"enemy"`
			Category    string  `json:"category"`
			GoldMin     int64   `json:"gold_min"`
			GoldMax     int64   `json:"gold_max"`
			Mat1Pct     float64 `json:"mat1_pct"`
			Mat3Pct     float64 `json:"mat3_pct"`
			SpecialItem string  `json:"special_item"`
			SpecialPct  float64 `json:"special_pct"`
		}
		if err := json.Unmarshal(payload, &f); err != nil {
			return err
		}
		if f.Enemy == "" || f.GoldMax < f.GoldMin {
			continue
		}
		byEnemy[f.Enemy] = Table{ID: id, EnemyID: f.Enemy, RegionID: region, Category: f.Category,
			GoldMin: f.GoldMin, GoldMax: f.GoldMax, Mat1Pct: f.Mat1Pct, Mat3Pct: f.Mat3Pct,
			SpecialItem: f.SpecialItem, SpecialPct: f.SpecialPct}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.build, c.byEnemy = buildID, byEnemy
	c.mu.Unlock()
	return nil
}
