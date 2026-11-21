package world

import (
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
