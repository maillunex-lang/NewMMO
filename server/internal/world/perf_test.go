package world

import (
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
)

// BenchmarkZoneTick measures the deterministic simulation, not networking or rendering.
// Run: go test ./internal/world -run '^$' -bench BenchmarkZoneTick -benchmem -count=3
func BenchmarkZoneTick(b *testing.B) {
	for _, players := range []int{1, 10, 50} {
		b.Run(fmt.Sprintf("players_%d_enemies_50", players), func(b *testing.B) {
			cfg := testCfg()
			cfg.Bounds = Rect{0, 0, 10000, 10000}
			sim := New(cfg)
			for i := 0; i < 50; i++ {
				sim.AddEnemy(fmt.Sprintf("spawn-%d", i), "ENM-01-001", "Enemy", 1, 1000, Vec{X: float64(i%10)*200 + 100, Y: float64(i/10)*200 + 100})
			}
			for i := 0; i < players; i++ {
				id := sim.AddPlayer(fmt.Sprintf("player-%d", i), "Player", 60, 2000)
				sim.Move(id, 1, Vec{X: 9000, Y: 9000})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sim.Step()
			}
			b.StopTimer()
			runtime.KeepAlive(sim)
		})
	}
}

// BenchmarkZoneSnapshot isolates interest-filtered full snapshots from tick cost.
// All entities are deliberately inside the observer's interest radius.
func BenchmarkZoneSnapshot(b *testing.B) {
	for _, players := range []int{1, 10, 50} {
		b.Run(fmt.Sprintf("players_%d_enemies_50", players), func(b *testing.B) {
			cfg := testCfg()
			cfg.Bounds = Rect{0, 0, 10000, 10000}
			cfg.InterestRadius = 10000
			sim := New(cfg)
			for i := 0; i < 50; i++ {
				sim.AddEnemy(fmt.Sprintf("spawn-%d", i), "ENM-01-001", "Enemy", 1, 1000, Vec{X: float64(i%10)*100 + 100, Y: float64(i/10)*100 + 100})
			}
			var observer EntityID
			for i := 0; i < players; i++ {
				id := sim.AddPlayer(fmt.Sprintf("player-%d", i), "Player", 60, 2000)
				if i == 0 {
					observer = id
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, ok := sim.SnapshotFor(observer)
				if !ok {
					b.Fatal("observer missing")
				}
			}
			b.StopTimer()
			runtime.KeepAlive(sim)
		})
	}
}

// BenchmarkZoneSnapshotJSON measures the snapshot plus JSON encoding for one
// observer. It excludes WebSocket transport, fanout, TLS and compression.
func BenchmarkZoneSnapshotJSON(b *testing.B) {
	for _, players := range []int{1, 10, 50} {
		b.Run(fmt.Sprintf("players_%d_enemies_50", players), func(b *testing.B) {
			sim, observer := perfZone(players)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				snapshot, ok := sim.SnapshotFor(observer)
				if !ok {
					b.Fatal("observer missing")
				}
				data, err := json.Marshal(snapshot)
				if err != nil {
					b.Fatal(err)
				}
				runtime.KeepAlive(data)
			}
			runtime.KeepAlive(sim)
		})
	}
}

// BenchmarkZoneFanoutJSON measures full-snapshot serialization for every
// player, deliberately representing a worst-case full-snapshot fanout.
func BenchmarkZoneFanoutJSON(b *testing.B) {
	for _, players := range []int{1, 10, 50} {
		b.Run(fmt.Sprintf("players_%d_enemies_50", players), func(b *testing.B) {
			sim, _ := perfZone(players)
			ids := make([]EntityID, 0, players)
			for i := 0; i < players; i++ {
				ids = append(ids, sim.byRef[fmt.Sprintf("player-%d", i)])
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, id := range ids {
					snapshot, ok := sim.SnapshotFor(id)
					if !ok {
						b.Fatal("observer missing")
					}
					data, err := json.Marshal(snapshot)
					if err != nil {
						b.Fatal(err)
					}
					runtime.KeepAlive(data)
				}
			}
			runtime.KeepAlive(sim)
		})
	}
}

func perfZone(players int) (*Sim, EntityID) {
	cfg := testCfg()
	cfg.Bounds = Rect{0, 0, 10000, 10000}
	cfg.InterestRadius = 10000
	sim := New(cfg)
	for i := 0; i < 50; i++ {
		sim.AddEnemy(fmt.Sprintf("spawn-%d", i), "ENM-01-001", "Enemy", 1, 1000, Vec{X: float64(i%10)*100 + 100, Y: float64(i/10)*100 + 100})
	}
	var observer EntityID
	for i := 0; i < players; i++ {
		id := sim.AddPlayer(fmt.Sprintf("player-%d", i), "Player", 60, 2000)
		if i == 0 {
			observer = id
		}
	}
	return sim, observer
}

// TestZoneRetainedHeapApprox reports the live heap delta for synthetic zones.
// Heap allocations are noisy; this is a diagnostic, not a hard assertion.
func TestZoneRetainedHeapApprox(t *testing.T) {
	if testing.Short() {
		t.Skip("diagnostic")
	}
	for _, players := range []int{1, 10, 50} {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		sim, observer := perfZone(players)
		if _, ok := sim.SnapshotFor(observer); !ok {
			t.Fatal("observer missing")
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		t.Logf("players=%d enemies=50 retained_heap_delta=%d bytes heap_objects_delta=%d", players, int64(after.HeapAlloc)-int64(before.HeapAlloc), int64(after.HeapObjects)-int64(before.HeapObjects))
		runtime.KeepAlive(sim)
	}
}
