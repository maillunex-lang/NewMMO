package quest

import "testing"

func TestREG01SideQuestRegistryBound(t *testing.T) {
	cat := NewCatalog()
	if err := cat.LoadInteractables(ResolveInteractablesPath("")); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"SQO-01-001-01", "SQO-01-001-02", "SQO-01-001-03",
		"SQO-01-002-01", "SQO-01-002-02", "SQO-01-002-03",
		"SQO-01-003-01", "SQO-01-003-02", "SQO-01-003-03",
		"SQO-01-004-01", "SQO-01-004-02", "SQO-01-004-03",
		"SQO-01-005-01", "SQO-01-005-02", "SQO-01-005-03",
	}
	for _, id := range want {
		reg, ok := cat.Interactable(id)
		if !ok || reg.Event == "" || reg.TargetID == "" {
			t.Fatalf("missing binding for %s: %#v", id, reg)
		}
		if reg.Count > 1 && len(reg.Instances) < reg.Count {
			t.Fatalf("%s count=%d instances=%d", id, reg.Count, len(reg.Instances))
		}
	}
}
