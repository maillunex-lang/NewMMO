package quest

import (
	"testing"
	"time"
)

func TestRegionOpen(t *testing.T) {
	cfg := RegionConfig{
		OpenAt: map[string]time.Time{
			"REG-02": time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		EarlyAccessAccounts: map[string]struct{}{"early": {}},
	}
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	if !cfg.RegionOpen("REG-01", "x", now) {
		t.Fatal("missing open_at means open")
	}
	if cfg.RegionOpen("REG-02", "x", now) {
		t.Fatal("future open_at must close region")
	}
	if !cfg.RegionOpen("REG-02", "early", now) {
		t.Fatal("early access bypasses gate")
	}
}

func TestFlagName(t *testing.T) {
	if FlagName("MQ-01-01", "START") != "QF-MQ-01-01-START" {
		t.Fatal(FlagName("MQ-01-01", "START"))
	}
	if FlagName("SQ-01-001", "START") != "SQF-01-001-ACCEPTED" {
		t.Fatal(FlagName("SQ-01-001", "START"))
	}
	if FlagName("SQ-01-001", "REWARD") != "SQR-01-001-PAID" {
		t.Fatal(FlagName("SQ-01-001", "REWARD"))
	}
	if FlagName("SQ-01-001", "DONE") != "SQF-01-001-DONE" {
		t.Fatal(FlagName("SQ-01-001", "DONE"))
	}
}

func TestParseUUID(t *testing.T) {
	ok := "550e8400-e29b-41d4-a716-446655440000"
	if _, err := parseUUID(ok); err != nil {
		t.Fatal(err)
	}
	if _, err := parseUUID("not-a-uuid"); err == nil {
		t.Fatal("expected bad uuid")
	}
}

func TestCurrentObjectiveOrdering(t *testing.T) {
	def := QuestDef{Objectives: []ObjectiveDef{
		{ID: "OBJ-1", Need: 1},
		{ID: "OBJ-2", Need: 1},
	}}
	cur, ok := currentObjective(def, map[string]map[string]int{})
	if !ok || cur.ID != "OBJ-1" {
		t.Fatalf("want OBJ-1 got %#v", cur)
	}
	cur, ok = currentObjective(def, map[string]map[string]int{"OBJ-1": {"have": 1}})
	if !ok || cur.ID != "OBJ-2" {
		t.Fatalf("want OBJ-2 got %#v", cur)
	}
	_, ok = currentObjective(def, map[string]map[string]int{"OBJ-1": {"have": 1}, "OBJ-2": {"have": 1}})
	if ok {
		t.Fatal("all complete")
	}
}

func TestLoadInteractablesFile(t *testing.T) {
	path := ResolveInteractablesPath("../../content/interactables/reg-01.json")
	cat := NewCatalog()
	if err := cat.LoadInteractables(path); err != nil {
		t.Skip("interactables file unavailable:", err)
	}
	entry, ok := cat.Interactable("OBJ-01-05-1")
	if !ok || entry.Count != 3 || len(entry.Instances) != 3 {
		t.Fatalf("OBJ-01-05-1 registry %#v", entry)
	}
	entry, ok = cat.Interactable("OBJ-01-01-1")
	if !ok || entry.Event != "npc_interact" || entry.TargetID != "NPC-802" {
		t.Fatalf("OBJ-01-01-1 %#v", entry)
	}
}

func TestCodeMapping(t *testing.T) {
	if Code(ErrRegionClosed) != "REGION_CLOSED" {
		t.Fatal(Code(ErrRegionClosed))
	}
}

func TestQuestForBuildUsesCachedActive(t *testing.T) {
	cat := NewCatalog()
	cat.mu.Lock()
	cat.build = "build-active"
	cat.byID["MQ-01-01"] = QuestDef{ID: "MQ-01-01", Reward: RewardDef{XP: 100, Gold: 12}}
	cat.mu.Unlock()
	def, err := cat.QuestForBuild(t.Context(), nil, "build-active", "MQ-01-01")
	if err != nil || def.Reward.XP != 100 {
		t.Fatalf("%#v %v", def, err)
	}
	if _, err := cat.QuestForBuild(t.Context(), nil, "build-active", "missing"); err != ErrNotFound {
		t.Fatal(err)
	}
}
