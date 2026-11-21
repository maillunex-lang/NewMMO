package dungeon

import (
	"errors"
	"testing"
)

func loadDef(t *testing.T) Definition {
	t.Helper()
	reg, err := LoadDir(ResolveDir(""))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := reg.Get("DNG-01-001")
	if !ok {
		t.Fatal("DNG-01-001 missing")
	}
	return d
}

func mustInteract(t *testing.T, d Definition, m *Machine, obj string) {
	t.Helper()
	if _, err := m.Interact(d, obj); err != nil {
		t.Fatalf("interact %s: %v", obj, err)
	}
}

func mustKill(t *testing.T, d Definition, m *Machine, spawn string) {
	t.Helper()
	if _, err := m.EnemyDefeated(d, spawn); err != nil {
		t.Fatalf("kill %s: %v", spawn, err)
	}
}

// playToR04 clears R01..R03 and returns the machine standing in the mini-boss room.
func playToR04(t *testing.T, d Definition) *Machine {
	m := NewMachine(d)
	mustInteract(t, d, m, "OBJ-DNG-01-001-ENTRY")
	for _, s := range []string{"DNG-01-001-R02-E1", "DNG-01-001-R02-E2", "DNG-01-001-R02-E3"} {
		mustKill(t, d, m, s)
	}
	if m.RoomID != "DNG-01-001-R02" {
		t.Fatalf("R02 must stay locked until object A: room=%s", m.RoomID)
	}
	mustInteract(t, d, m, "OBJ-DNG-01-001-A")
	for _, o := range []string{"OBJ-DNG-01-001-B2", "OBJ-DNG-01-001-B1", "OBJ-DNG-01-001-B3"} {
		mustInteract(t, d, m, o)
	}
	return m
}

func TestDefinitionValid(t *testing.T) {
	d := loadDef(t)
	if len(d.Rooms) != 7 || d.Rooms[0].ID != "DNG-01-001-R01" || d.Rooms[6].Kind != "exit" {
		t.Fatalf("rooms: %d", len(d.Rooms))
	}
}

func TestValidateRejectsBadDefinitions(t *testing.T) {
	d := loadDef(t)
	bad := d
	bad.Puzzle.Order = []string{"OBJ-DNG-01-001-A"}
	if bad.Validate() == nil {
		t.Fatal("puzzle object outside puzzle room must be rejected")
	}
	bad = d
	bad.CheckpointRooms = []string{"DNG-01-001-R05"}
	if bad.Validate() == nil {
		t.Fatal("first room must be a checkpoint")
	}
}

func TestDoorsOpenOnlyOnServerCondition(t *testing.T) {
	d := loadDef(t)
	m := NewMachine(d)
	if _, err := m.Interact(d, "OBJ-DNG-01-001-A"); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("object of another room: %v", err)
	}
	if _, err := m.EnemyDefeated(d, "DNG-01-001-R02-E1"); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("spawn of another room: %v", err)
	}
	mustInteract(t, d, m, "OBJ-DNG-01-001-ENTRY")
	if m.RoomID != "DNG-01-001-R02" || m.Rooms["DNG-01-001-R01"].State != RoomCleared {
		t.Fatalf("after entry: %s", m.RoomID)
	}
	// Idempotent repeats.
	mustInteract(t, d, m, "OBJ-DNG-01-001-A")
	mustInteract(t, d, m, "OBJ-DNG-01-001-A")
	mustKill(t, d, m, "DNG-01-001-R02-E1")
	mustKill(t, d, m, "DNG-01-001-R02-E1")
	if m.RoomID != "DNG-01-001-R02" {
		t.Fatal("room opened before all spawns were defeated")
	}
}

func TestPuzzleOrderResetAndSolve(t *testing.T) {
	d := loadDef(t)
	m := playToR04(t, d) // R03 solved in the helper
	if m.RoomID != "DNG-01-001-R04" {
		t.Fatalf("expected R04, got %s", m.RoomID)
	}
	// Fresh run: wrong order resets only the sequence.
	m = NewMachine(d)
	mustInteract(t, d, m, "OBJ-DNG-01-001-ENTRY")
	mustInteract(t, d, m, "OBJ-DNG-01-001-A")
	for _, s := range []string{"DNG-01-001-R02-E1", "DNG-01-001-R02-E2", "DNG-01-001-R02-E3"} {
		mustKill(t, d, m, s)
	}
	mustInteract(t, d, m, "OBJ-DNG-01-001-B2")
	evs, _ := m.Interact(d, "OBJ-DNG-01-001-B3") // wrong: expected B1
	if len(evs) == 0 || evs[0].Type != "PUZZLE_RESET" {
		t.Fatalf("wrong order must reset: %#v", evs)
	}
	rs := m.Rooms["DNG-01-001-R03"]
	if len(rs.PuzzleProgress) != 0 || rs.Objects["OBJ-DNG-01-001-B2"] || m.RoomID != "DNG-01-001-R03" {
		t.Fatalf("reset state: %#v", rs)
	}
	for _, o := range []string{"OBJ-DNG-01-001-B2", "OBJ-DNG-01-001-B1"} {
		mustInteract(t, d, m, o)
	}
	if m.RoomID != "DNG-01-001-R03" {
		t.Fatal("door opened before the puzzle was solved")
	}
	mustInteract(t, d, m, "OBJ-DNG-01-001-B3")
	if m.RoomID != "DNG-01-001-R04" {
		t.Fatalf("puzzle solved must open R04, room=%s", m.RoomID)
	}
}

func TestFullRunCompletesAndCheckpointsApply(t *testing.T) {
	d := loadDef(t)
	m := playToR04(t, d)
	mustKill(t, d, m, "DNG-01-001-R04-MINI")
	if m.RoomID != "DNG-01-001-R05" {
		t.Fatalf("mini-boss kill must open R05: %s", m.RoomID)
	}
	if m.Checkpoint != "DNG-01-001-R01" {
		t.Fatalf("R05 checkpoint not active before its object: %s", m.Checkpoint)
	}
	mustInteract(t, d, m, "CP-DNG-01-001-02")
	if m.Checkpoint != "DNG-01-001-R05" || m.RoomID != "DNG-01-001-R06" || m.Status != StatusBoss {
		t.Fatalf("checkpoint/boss state: cp=%s room=%s status=%s", m.Checkpoint, m.RoomID, m.Status)
	}
	// Wipe on the final boss: back to R05 checkpoint, mini-boss room stays cleared, fresh R06.
	mustInteract(t, d, m, "OBJ-DNG-01-001-FINAL-1")
	if _, err := m.PlayerDefeated(d); err != nil {
		t.Fatal(err)
	}
	if m.BossAttempts != 1 || m.RoomID != "DNG-01-001-R06" || m.Rooms["DNG-01-001-R04"].State != RoomCleared ||
		m.Rooms["DNG-01-001-R06"].Objects["OBJ-DNG-01-001-FINAL-1"] {
		t.Fatalf("wipe state: attempts=%d room=%s", m.BossAttempts, m.RoomID)
	}
	for _, o := range []string{"OBJ-DNG-01-001-FINAL-1", "OBJ-DNG-01-001-FINAL-2", "OBJ-DNG-01-001-FINAL-3"} {
		mustInteract(t, d, m, o)
	}
	if m.Status == StatusCompleted {
		t.Fatal("boss still alive")
	}
	mustKill(t, d, m, "DNG-01-001-R06-BOSS")
	if m.Status != StatusCompleted || m.RoomID != "DNG-01-001-R07" {
		t.Fatalf("completion: status=%s room=%s", m.Status, m.RoomID)
	}
	if _, err := m.Interact(d, "EXIT-DNG-01-001"); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("no actions after completion: %v", err)
	}
}

func TestWipeBeforeMiniBossReturnsToEntrance(t *testing.T) {
	d := loadDef(t)
	m := playToR04(t, d)
	if _, err := m.PlayerDefeated(d); err != nil {
		t.Fatal(err)
	}
	// Checkpoint R01 (entry already done): R02..R04 are fresh, run continues at R02.
	if m.RoomID != "DNG-01-001-R02" || m.Rooms["DNG-01-001-R02"].Objects["OBJ-DNG-01-001-A"] || m.Rooms["DNG-01-001-R03"].PuzzleSolved {
		t.Fatalf("wipe reset: room=%s", m.RoomID)
	}
}
