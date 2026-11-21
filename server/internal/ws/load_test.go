package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestWSLoad drives N real WebSocket clients (loopback TCP, full session.auth + world.join + movement)
// through the production ws.Server and world.Handler and reports delivered traffic and CPU.
// Opt-in: NEWMMO_LOAD=1 go test ./internal/ws -run TestWSLoad -v. Optional NEWMMO_LOAD_SECONDS (default 5).
// Loopback only: no TLS, no real network latency, passive enemies in the fixture zone, no PostgreSQL.
func TestWSLoad(t *testing.T) {
	if os.Getenv("NEWMMO_LOAD") != "1" {
		t.Skip("set NEWMMO_LOAD=1 to run the WebSocket load measurement")
	}
	secs := 5
	if v, err := strconv.Atoi(os.Getenv("NEWMMO_LOAD_SECONDS")); err == nil && v > 0 {
		secs = v
	}
	for _, n := range []int{1, 10, 50} {
		t.Run(fmt.Sprintf("players_%d", n), func(t *testing.T) { runWSLoad(t, n, time.Duration(secs)*time.Second) })
	}
}

func runWSLoad(t *testing.T, n int, dur time.Duration) {
	cfg := DefaultConfig()
	cfg.Handshakes, cfg.Pending = 200, 200
	store := newMemoryStore()
	socket, url, _ := testServer(t, cfg, store, nil)
	zone, handler := worldFixture(t)
	socket.SetWorld(handler)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	var msgs, bytesIn, deltas, errs atomic.Int64
	lat := make([]int64, 0, 4096)
	var latMu sync.Mutex
	ready := make(chan struct{}, n)

	for i := 0; i < n; i++ {
		c := dial(t, url, nil)
		login(t, c, store.ticket(t, int64(1000+i)), "0.1.0", fixtureBuild)
		expectInitial(t, c, strconv.Itoa(1000+i), "ok")
		send(t, c, "world.join.request", "", map[string]any{"zone_id": "ZONE-01-DAWN-VALLEY"})
		expect(t, c, "world.snapshot")
		wg.Add(2)
		go func(i int) { // reader
			defer wg.Done()
			ready <- struct{}{}
			for {
				_, data, err := c.Read(ctx)
				if err != nil {
					return
				}
				msgs.Add(1)
				bytesIn.Add(int64(len(data)))
				var m struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(data, &m) == nil && m.Type == "world.delta" {
					deltas.Add(1)
				}
			}
		}(i)
		go func(i int) { // mover: new server-validated target at 4 Hz, alternating directions
			defer wg.Done()
			tk := time.NewTicker(250 * time.Millisecond)
			defer tk.Stop()
			seq := uint64(1)
			for {
				select {
				case <-ctx.Done():
					return
				case <-tk.C:
					x := 800.0 + float64((int(seq)+i)%2)*300 + float64(i%10)*20
					b, _ := json.Marshal(map[string]any{"protocol_version": 1, "type": "world.move.request",
						"payload": map[string]any{"seq": seq, "x": x, "y": 700 + float64(i%7)*15}})
					seq++
					start := time.Now()
					wctx, wc := context.WithTimeout(ctx, 2*time.Second)
					if err := c.Write(wctx, websocket.MessageText, b); err != nil && ctx.Err() == nil {
						errs.Add(1)
					}
					wc()
					latMu.Lock()
					lat = append(lat, time.Since(start).Microseconds())
					latMu.Unlock()
				}
			}
		}(i)
	}
	for i := 0; i < n; i++ {
		<-ready
	}
	if zone.PlayerCount() != n {
		t.Fatalf("players in zone = %d, want %d", zone.PlayerCount(), n)
	}

	var ms0, ms1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms0)
	cpu0 := processCPU()
	start := time.Now()
	time.Sleep(dur)
	elapsed := time.Since(start)
	cpu1 := processCPU()
	runtime.ReadMemStats(&ms1)
	m1, b1, d1, e1 := msgs.Load(), bytesIn.Load(), deltas.Load(), errs.Load()
	cancel()
	wg.Wait()

	// CPU figure covers server + all n simulated clients in this one process (upper bound for the server).
	t.Logf("players=%d dur=%.1fs msgs/s=%.0f deltas/s/client=%.1f KiB/s total=%.1f KiB/s/client=%.2f process-CPU=%.0f%% heapAlloc=%.1fMiB gc=%d writeErrs=%d",
		n, elapsed.Seconds(), float64(m1)/elapsed.Seconds(), float64(d1)/elapsed.Seconds()/float64(n),
		float64(b1)/1024/elapsed.Seconds(), float64(b1)/1024/elapsed.Seconds()/float64(n),
		(cpu1-cpu0).Seconds()/elapsed.Seconds()*100, float64(ms1.HeapAlloc)/(1<<20), ms1.NumGC-ms0.NumGC, e1)
	if d1 == 0 {
		t.Fatal("no deltas received over real WebSocket")
	}
}
