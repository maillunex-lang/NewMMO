package combatdata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maillunex-lang/NewMMO/server/internal/damage"
)

func TestLoadSyntheticAndValidate(t *testing.T) {
	dir := t.TempDir()
	lines := `{"id":"ENM-01-001","kind":"enemy","payload":{"name":"X","level":1,"threat":"обычный","ai":"melee"},"region_id":"REG-01"}
{"id":"AB-ENM-01-001-A","kind":"ability","payload":{"enemy":"ENM-01-001","slot":"A","coefficient":0.6,"scale":"AP","cooldown_s":2,"text":"a"},"region_id":"REG-01"}
{"id":"AB-ENM-01-001-B","kind":"ability","payload":{"enemy":"ENM-01-001","slot":"B","coefficient":1.05,"scale":"AP","cooldown_s":14,"telegraph_s":0.7,"text":"b"},"region_id":"REG-01"}
`
	if err := os.WriteFile(filepath.Join(dir, "entities.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadExport(dir)
	if err != nil {
		t.Fatal(err)
	}
	e := c.Enemies["ENM-01-001"]
	if e.Abilities["B"].TelegraphS != 0.7 || e.Abilities["A"].Coef != 0.6 {
		t.Fatalf("abilities: %#v", e.Abilities)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	st, ok := c.Stats("ENM-01-001", damage.NoScale)
	if !ok || st.MaxHP != 150 {
		t.Fatalf("stats: %+v", st)
	}
	// Unknown enemy in an ability is a load error; an ability without numbers fails validation.
	bad := lines + `{"id":"AB-ENM-09-999-A","kind":"ability","payload":{"enemy":"ENM-09-999","slot":"A","text":"x"},"region_id":null}` + "\n"
	_ = os.WriteFile(filepath.Join(dir, "entities.jsonl"), []byte(bad), 0o600)
	if _, err := LoadExport(dir); err == nil {
		t.Fatal("unknown enemy must be rejected")
	}
	noNum := `{"id":"ENM-01-001","kind":"enemy","payload":{"name":"X","level":1,"threat":"обычный","ai":"melee"},"region_id":"REG-01"}
{"id":"AB-ENM-01-001-A","kind":"ability","payload":{"enemy":"ENM-01-001","slot":"A","text":"a"},"region_id":"REG-01"}
`
	_ = os.WriteFile(filepath.Join(dir, "entities.jsonl"), []byte(noNum), 0o600)
	c2, _ := LoadExport(dir)
	if c2.Validate() == nil {
		t.Fatal("missing B / numbers must fail validation")
	}
}

// Real export: set by CI server-db (NEWMMO_CONTENT_EXPORT_DIR). Skipped without it.
func TestRealExport(t *testing.T) {
	dir := os.Getenv("NEWMMO_CONTENT_EXPORT_DIR")
	if dir == "" {
		t.Skip("NEWMMO_CONTENT_EXPORT_DIR not set")
	}
	c, err := LoadExport(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Enemies) != 96 || len(c.Skills) != 5 {
		t.Fatalf("enemies=%d classes=%d", len(c.Enemies), len(c.Skills))
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	// Canon spot checks (06 §5 ENM-01-001, 05 CLS-01-SK01 / SK02).
	e := c.Enemies["ENM-01-001"]
	a, b := e.Abilities["A"], e.Abilities["B"]
	if a.Coef != 0.60 || a.CooldownS != 2 || b.Coef != 1.05 || b.CooldownS != 14 || b.TelegraphS != 0.7 {
		t.Fatalf("ENM-01-001 abilities: %+v %+v", a, b)
	}
	boss := c.Enemies["ENM-01-301"]
	if d, ok := boss.Abilities["D"]; !ok || d.TelegraphS != 4 || d.Coef != 1.10 {
		t.Fatalf("boss D: %+v", boss.Abilities["D"])
	}
	sk := c.Skills["CLS-01"]
	if sk[0].ID != "CLS-01-SK01" || sk[0].Coef != 1.10 || sk[0].CooldownS != 5 || sk[0].EffectKind != "damage" || sk[0].Scale != "AP" {
		t.Fatalf("CLS-01-SK01: %+v", sk[0])
	}
	if sk[1].EffectKind != "shield" || sk[1].Scale != "MaxHP" || sk[1].Coef != 0.18 {
		t.Fatalf("CLS-01-SK02: %+v", sk[1])
	}
	// Stats of every enemy are computable and monotone with the category.
	for id := range c.Enemies {
		if _, ok := c.Stats(id, damage.NoScale); !ok {
			t.Fatalf("no stats for %s", id)
		}
	}
	elite, _ := c.Stats("ENM-01-101", damage.NoScale)
	normal, _ := c.Stats("ENM-01-008", damage.NoScale)
	if elite.MaxHP <= normal.MaxHP {
		t.Fatalf("elite L8 HP %d must exceed normal L8 HP %d", elite.MaxHP, normal.MaxHP)
	}
}
