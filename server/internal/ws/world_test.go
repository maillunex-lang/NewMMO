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
	for _, e := range []struct{ id, name string }{{"ENM-01-001", "A"}, {"ENM-01-003", "B"}, {"ENM-01-004", "C"}} {
		sb.WriteString(`{"id":"` + e.id + `","kind":"enemy","payload":{"name":"` + e.name + `","level":3,"threat":"обычный","ai":"melee"},"region_id":"REG-01"}` + "\n")
		for _, slot := range []string{"A", "B"} {
			sb.WriteString(`{"id":"AB-` + e.id + `-` + slot + `","kind":"ability","payload":{"enemy":"` + e.id + `","slot":"` + slot + `","coefficient":1,"scale":"AP","cooldown_s":2,"text":"x"},"region_id":"REG-01"}` + "\n")
		}
	}
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

	// Moving before joining is a state conflict, not a crash.
	send(t, c, "world.move.request", "", map[string]any{"seq": 1, "x": 10, "y": 10})
	assertCode(t, c, "STATE_CONFLICT")

	send(t, c, "world.join.request", "", map[string]any{"zone_id": "ZONE-01-DAWN-VALLEY"})
	snap := expect(t, c, "world.snapshot")
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
