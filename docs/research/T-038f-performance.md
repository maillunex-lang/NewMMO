# T-038f — initial server simulation baseline (2026-11-21)

Environment: VPS Lunex, Linux amd64, AMD EPYC-Genoa, Go 1.26.11. Command: `cd server && go test ./internal/world -run '^$' -bench BenchmarkZoneTick -benchmem -benchtime=500ms -count=2`.

| Synthetic scenario | ns/tick (two runs) | allocs/tick |
|---|---:|---:|
| 1 player, 50 passive enemies | 358 / 353 | 0 |
| 10 players, 50 passive enemies | 479 / 482 | 0 |
| 50 players, 50 passive enemies | 694 / 676 | 0 |

**Limitations:** These are synthetic `Sim.Step` microbenchmarks, **not** full zone CPU/RAM measurements. Enemies have no combat AI definitions; no WS snapshots, PostgreSQL, rendering, or active ability casts. `0 B/op` means steady-state allocations per tick, **not** zone RAM. Do not extrapolate these figures to production capacity. Next: realistic zone profiles with combat catalog, AI, snapshots, pprof and retained heap per zone.

Android: requires physical phone for frame pacing, GPU, thermal throttling, memory/PSS, battery and 5-minute play session. Existing `tools/android/measure_device.sh` collects cold-start/meminfo/gfxinfo but still references old Godot 4.3 in its comment. No Android device connected to this VPS; CT-020 / R-DEVICE remains open. Do not report FPS without device evidence.

No dependencies added. Baseline uses Go's standard `testing` benchmark facility; alternative built-in profilers: `runtime/pprof` (CPU/heap) and `runtime/metrics` (process metrics). Third-party profiling libraries are unnecessary for this first slice.
