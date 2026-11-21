package ws

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
)

// Real zone definition + a synthetic combat catalog containing exactly the enemies the zone spawns.
func worldFixture(t *testing.T) (*world.Zone, *world.Handler) {
	t.Helper()
	defs, err := world.LoadZoneDefs(world.ResolveZonesDir(""))
	if err != nil {
		t.Fatal(err)
	}
	def := defs["ZONE-01-DAWN-VALLEY"]
	var sb strings.Builder
	for _, e := range []struct{ id, name string }{{"ENM-01-001", "A"}, {"ENM-01-002", "D"}, {"ENM-01-003", "B"}, {"ENM-01-004", "C"}} {
		sb.WriteString(`{"id":"` + e.id + `","kind":"enemy","payload":{"name":"` + e.name + `","level":3,"threat":"обычный","ai":"melee"},"region_id":"REG-01"}` + "\n")
		for _, slot := range []string{"A", "B"} {
			sb.WriteString(`{"id":"AB-` + e.id + `-` + slot + `","kind":"ability","payload":{"enemy":"` + e.id + `","slot":"` + slot + `","coefficient":1,"scale":"AP","cooldown_s":2,"text":"x"},"region_id":"REG-01"}` + "\n")
		}
	}
	sb.WriteString(`{"id":"CLS-01-SK01","kind":"skill","payload":{"class_id":"CLS-01","name":"Удар","level":1,"target":"enemy","cost":0,"cooldown_s":5,"effect_raw":"1.10*AP","effect_kind":"damage","coefficient":1.1,"scale":"AP","rule":"Базовый удар; +10 стойкости"},"region_id":null}` + "\n")
	sb.WriteString(`{"id":"CLS-01-SK10","kind":"skill","payload":{"class_id":"CLS-01","name":"Снятие","level":1,"target":"self","cost":0,"cooldown_s":1,"effect_raw":"cleanse:1","effect_kind":"cleanse","coefficient":1,"rule":"x"},"region_id":null}` + "\n")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "entities.jsonl"), []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	cat, err := combatdata.LoadExport(dir)
	if err != nil {
		t.Fatal(err)
	}
	z, err := world.NewZone(def, cat)
	if err != nil {
		t.Fatal(err)
	}
	h := world.NewHandler([]*world.Zone{z}, func(ctx context.Context, account string, id int64) (world.PlayerInfo, error) {
		return world.PlayerInfo{Name: "Hero", ClassID: "CLS-01", Level: 1}, nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go z.Run(ctx)
	return z, h
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWorldJoinMoveDeltaAndDisconnect(t *testing.T) {
	store := newMemoryStore()
	socket, url, log := testServer(t, DefaultConfig(), store, nil)
	zone, handler := worldFixture(t)
	socket.SetWorld(handler)

	c := dial(t, url, nil)
	login(t, c, store.ticket(t, 77), "0.1.0", fixtureBuild)
	expectInitial(t, c, "77", "ok")

	// The retired outcome report is rejected before it can reach dungeon progression.
	send(t, c, "dungeon.encounter.report", requestID, map[string]any{"outcome": "enemy_defeated"})
	assertCode(t, c, "UNKNOWN_TYPE")

	// Moving before joining is a state conflict, not a crash.
	send(t, c, "world.move.request", "", map[string]any{"seq": 1, "x": 10, "y": 10})
	assertCode(t, c, "STATE_CONFLICT")

	send(t, c, "world.join.request", "", map[string]any{"zone_id": "ZONE-01-DAWN-VALLEY"})
	snap := expect(t, c, "world.snapshot")
	if got := string(snap.Payload["attack_reach"]); got != "112" {
		t.Fatalf("Guard effective attack reach = %s, want 112", got)
	}
	var entities []struct {
		Kind string  `json:"kind"`
		X    float64 `json:"x"`
	}
	if err := json.Unmarshal(snap.Payload["entities"], &entities); err != nil || len(entities) == 0 {
		t.Fatalf("snapshot entities: %v %v", entities, err)
	}
	players := 0
	for _, e := range entities {
		if e.Kind == "player" {
			players++
		}
	}
	if players != 1 || string(snap.Payload["tick_ms"]) != "100" {
		t.Fatalf("snapshot: players=%d tick_ms=%s", players, snap.Payload["tick_ms"])
	}

	// Authoritative movement: a client-supplied speed is rejected, a valid target produces deltas with ack_seq.
	send(t, c, "world.move.request", "", map[string]any{"seq": 1, "x": 900, "y": 720, "speed": 9999})
	assertCode(t, c, "BAD_REQUEST")
	send(t, c, "world.move.request", "", map[string]any{"seq": 2, "x": 900, "y": 720})
	var last struct {
		AckSeq uint64 `json:"ack_seq"`
		Upsert []struct {
			Kind   string  `json:"kind"`
			X      float64 `json:"x"`
			Moving bool    `json:"moving"`
		} `json:"upsert"`
	}
	waitForDelta := func(cond func() bool) {
		t.Helper()
		for i := 0; i < 40; i++ {
			d := expect(t, c, "world.delta")
			last.AckSeq, last.Upsert = 0, nil
			if err := json.Unmarshal(d.Payload["ack_seq"], &last.AckSeq); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(d.Payload["upsert"], &last.Upsert); err != nil {
				t.Fatal(err)
			}
			if cond() {
				return
			}
		}
		t.Fatal("expected delta not received")
	}
	waitForDelta(func() bool {
		for _, u := range last.Upsert {
			if u.Kind == "player" && u.Moving && u.X > 620 && last.AckSeq == 2 {
				return true
			}
		}
		return false
	})

	// Unknown world message types fall through to UNKNOWN_TYPE.
	send(t, c, "world.fly.request", requestID, map[string]any{})
	assertCode(t, c, "UNKNOWN_TYPE")

	// Newest session wins: the old socket is closed with 4005 and the entity stays (owned by the new session).
	c2 := dial(t, url, nil)
	login(t, c2, store.ticket(t, 77), "0.1.0", fixtureBuild)
	expectInitial(t, c2, "77", "ok")
	send(t, c2, "world.join.request", "", map[string]any{"zone_id": "ZONE-01-DAWN-VALLEY"})
	expect(t, c2, "world.snapshot")
	for {
		_, _, err := c.Read(t.Context())
		if err != nil {
			if websocket.CloseStatus(err) != 4005 {
				t.Fatalf("old session close status = %d", websocket.CloseStatus(err))
			}
			break
		}
	}
	time.Sleep(50 * time.Millisecond)
	if zone.PlayerCount() != 1 {
		t.Fatalf("player count after takeover = %d, want 1", zone.PlayerCount())
	}

	// Closing the current session removes the player; logs never contain world coordinates or tickets.
	_ = c2.Close(websocket.StatusNormalClosure, "")
	waitFor(t, "player removal after disconnect", func() bool { return zone.PlayerCount() == 0 })
	if logged := log.String(); strings.Contains(logged, `"x":900`) {
		t.Fatal("world payloads must not be logged")
	}
}

func TestWorldCastOverWebSocket(t *testing.T) {
	store := newMemoryStore()
	socket, url, _ := testServer(t, DefaultConfig(), store, nil)
	_, handler := worldFixture(t)
	socket.SetWorld(handler)
	c := dial(t, url, nil)
	login(t, c, store.ticket(t, 88), "0.1.0", fixtureBuild)
	expectInitial(t, c, "88", "ok")
	send(t, c, "world.join.request", "", map[string]any{"zone_id": "ZONE-01-DAWN-VALLEY"})
	snap := expect(t, c, "world.snapshot")
	var entities []struct {
		ID   int    `json:"id"`
		Kind string `json:"kind"`
		Ref  string `json:"ref"`
		X    float64
	}
	if err := json.Unmarshal(snap.Payload["entities"], &entities); err != nil {
		t.Fatal(err)
	}
	var enemy int
	for _, e := range entities {
		if e.Kind == "enemy" && e.Ref == "ENM-01-002" && (enemy == 0 || e.ID < enemy) {
			enemy = e.ID // ZS-01-001 at (1240,780), the first slime
		}
	}
	if enemy == 0 {
		t.Fatal("no slime in the snapshot")
	}
	const rid = "5b5a7114-59f0-4ab6-b34d-5938cdfbb425"
	// Too far from the slime at spawn → OUT_OF_RANGE; strict payloads; unsupported and locked skills.
	send(t, c, "world.cast.request", rid, map[string]any{"skill_id": "CLS-01-SK01", "target_id": enemy})
	assertCode(t, c, "OUT_OF_RANGE")
	send(t, c, "world.cast.request", rid, map[string]any{"skill_id": "CLS-01-SK01", "target_id": enemy, "damage": 9999})
	assertCode(t, c, "BAD_REQUEST")
	send(t, c, "world.cast.request", "", map[string]any{"skill_id": "CLS-01-SK01", "target_id": enemy})
	assertCode(t, c, "BAD_REQUEST") // mutation without request_id
	send(t, c, "world.cast.request", rid, map[string]any{"skill_id": "CLS-01-SK10"})
	assertCode(t, c, "UNSUPPORTED_SKILL")
	send(t, c, "world.cast.request", rid, map[string]any{"skill_id": "CLS-03-SK01", "target_id": enemy})
	assertCode(t, c, "SKILL_LOCKED")

	// Walk next to the slime (1240,780), then cast: result + damage event + self cooldown state.
	send(t, c, "world.move.request", "", map[string]any{"seq": 1, "x": 1200, "y": 780})
	deadline := time.Now().Add(8 * time.Second)
	for {
		d, err := read(t, c)
		if err != nil {
			t.Fatal(err)
		}
		if d.Type == "world.delta" {
			var up []struct {
				Kind string  `json:"kind"`
				X    float64 `json:"x"`
			}
			_ = json.Unmarshal(d.Payload["upsert"], &up)
			arrived := false
			for _, u := range up {
				if u.Kind == "player" && u.X >= 1195 {
					arrived = true
				}
			}
			if arrived {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("never arrived next to the slime")
		}
	}
	send(t, c, "world.cast.request", rid, map[string]any{"skill_id": "CLS-01-SK01", "target_id": enemy})
	var result, event, self bool
	for i := 0; i < 60 && !(result && event && self); i++ {
		m, err := read(t, c)
		if err != nil {
			t.Fatal(err)
		}
		switch m.Type {
		case "world.cast.result":
			if m.Request != rid || string(m.Payload["skill_id"]) != `"CLS-01-SK01"` || string(m.Payload["amount"]) == "" {
				t.Fatalf("cast result: %+v", m.Payload)
			}
			result = true
		case "world.event":
			event = event || strings.Contains(string(m.Payload["events"]), `"type":"damage"`)
		case "world.delta":
			self = self || strings.Contains(string(m.Payload["self"]), "CLS-01-SK01")
		case "command.error":
			t.Fatalf("unexpected error: %s", m.Payload["code"])
		}
	}
	if !result || !event || !self {
		t.Fatalf("result=%v event=%v self=%v", result, event, self)
	}
	// The same request id again is replayed (idempotent), a different cast on cooldown is refused.
	send(t, c, "world.cast.request", rid, map[string]any{"skill_id": "CLS-01-SK01", "target_id": enemy})
	for {
		m, err := read(t, c)
		if err != nil {
			t.Fatal(err)
		}
		if m.Type == "world.cast.result" {
			break
		}
		if m.Type == "command.error" {
			t.Fatalf("replay must succeed, got %s", m.Payload["code"])
		}
	}
	send(t, c, "world.cast.request", "6b5a7114-59f0-4ab6-b34d-5938cdfbb426", map[string]any{"skill_id": "CLS-01-SK01", "target_id": enemy})
	for {
		m, err := read(t, c)
		if err != nil {
			t.Fatal(err)
		}
		if m.Type == "command.error" {
			code := string(m.Payload["code"])
			if code != `"GCD"` && code != `"ON_COOLDOWN"` {
				t.Fatalf("code %s", code)
			}
			break
		}
	}
}
