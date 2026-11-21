package dialogue

import "testing"

func TestFilterOptionsByQuestState(t *testing.T) {
	opts := filterOptions("start", "AVAILABLE", "NPC-802", "NPC-802")
	if !optionAllowed(opts, "ACCEPT") || !optionAllowed(opts, "ASK") || !optionAllowed(opts, "LATER") {
		t.Fatalf("start available: %#v", opts)
	}
	opts = filterOptions("start", "AVAILABLE", "NPC-999", "NPC-802")
	if optionAllowed(opts, "ACCEPT") {
		t.Fatalf("wrong npc must not offer ACCEPT: %#v", opts)
	}
	opts = filterOptions("turnin", "READY_TO_TURN_IN", "NPC-802", "NPC-802")
	if !optionAllowed(opts, "TURN_IN") {
		t.Fatalf("turnin ready: %#v", opts)
	}
	opts = filterOptions("turnin", "ACTIVE", "NPC-802", "NPC-802")
	if optionAllowed(opts, "TURN_IN") {
		t.Fatalf("active must not offer TURN_IN: %#v", opts)
	}
	opts = filterOptions("progress", "ACTIVE", "NPC-102", "NPC-802")
	if !optionAllowed(opts, "CONTINUE") || optionAllowed(opts, "ACCEPT") {
		t.Fatalf("progress: %#v", opts)
	}
}

func TestDialogueFromNode(t *testing.T) {
	if got := dialogueFromNode("DIA.MQ-01-01.START.01"); got != "MQ-01-01" {
		t.Fatalf("got %q", got)
	}
	if got := dialogueFromNode("DIA.SQ-01-001.START.01"); got != "SQ-01-001" {
		t.Fatalf("got %q", got)
	}
	if got := dialogueFromNode("bad"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestContactFor(t *testing.T) {
	if contactFor("MQ-01-01") != "NPC-802" || contactFor("MQ-01-03") != "NPC-102" {
		t.Fatal("contact map wrong")
	}
}

func TestAdvanceLines(t *testing.T) {
	lines := []string{"a", "b", "c"}
	next := advanceLines(lines, "a")
	if len(next) != 2 || next[0] != "b" {
		t.Fatalf("%#v", next)
	}
	last := advanceLines(lines, "c")
	if len(last) != 1 || last[0] != "c" {
		t.Fatalf("%#v", last)
	}
}

func TestPickMode(t *testing.T) {
	flow := FlowDef{Start: []string{"s"}, Progress: []string{"p"}, TurnIn: []string{"t"}}
	mode, lines := pickMode(flow, "AVAILABLE")
	if mode != "start" || lines[0] != "s" {
		t.Fatalf("%s %#v", mode, lines)
	}
	mode, lines = pickMode(flow, "READY_TO_TURN_IN")
	if mode != "turnin" || lines[0] != "t" {
		t.Fatalf("%s %#v", mode, lines)
	}
}

func TestErrorCodes(t *testing.T) {
	if Code(ErrPrerequisiteNotMet) != "PREREQUISITE_NOT_MET" {
		t.Fatal(Code(ErrPrerequisiteNotMet))
	}
	if Code(ErrIdempotencyConflict) != "IDEMPOTENCY_CONFLICT" {
		t.Fatal(Code(ErrIdempotencyConflict))
	}
}
