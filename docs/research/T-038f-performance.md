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

## Follow-up: full interest snapshots (same VPS, 2026-11-21)

Command: `cd server && go test ./internal/world -run '^$' -bench 'BenchmarkZone(Snapshot|Tick)' -benchmem -benchtime=300ms -count=1`. Snapshot benchmark forces all 50 enemies and 1/10/50 players inside the observer's interest radius; each iteration rebuilds that observer's full snapshot.

| Synthetic snapshot | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| 1 player + 50 enemies | 5,659 | 7,368 | 13 |
| 10 players + 50 enemies | 7,504 | 9,992 | 13 |
| 50 players + 50 enemies | 16,741 | 14,600 | 14 |

Snapshot costs are **per full snapshot for one observer**, not per tick, all connected players, or encoded WebSocket message. The benchmark does not include JSON serialization, network fanout, live combat, database I/O, or retained heap. Tick measurements in this run: 361 / 636 / 861 ns/op for 1/10/50 players. Single-run numbers are indicative only, not capacity limits.

Director self-review: tests and `go vet ./internal/world` pass. This follow-up closes only the snapshot microbenchmark slice. **T-038f remains PARTIAL** pending realistic zone CPU/retained heap, JSON/WS fanout and physical Android measurements.

## Follow-up: JSON encoding, worst-case fanout, approximate retained heap

Same VPS, Go 1.26.11, 2026-11-21. Command: `cd server && go test ./internal/world -run '^$' -bench 'BenchmarkZone(SnapshotJSON|FanoutJSON)' -benchmem -benchtime=200ms -count=1`. Each iteration serializes a fresh full interest snapshot; fanout serializes once **for each player**, with all entities visible. These are deliberately expensive full snapshots, not normal incremental deltas.

| Players + 50 passive enemies | JSON one viewer (µs/op) | JSON full fanout (µs/op) | Fanout B/op |
|---|---:|---:|---:|
| 1 | 31.6 | 31.6 | 15,690 |
| 10 | 37.5 | 354 | 196,611 |
| 50 | 64.2 | 2,814 | 1,458,555 |

`go test ./internal/world -run '^TestZoneRetainedHeapApprox$' -v` (one diagnostic sample per size) reported **approximate live-heap deltas** of 43,896 / 54,128 / 89,472 bytes for 1/10/50 players with 50 passive enemies. These deltas include snapshot-known-set state; they are GC-sensitive and can be negative on other runs. Do not use them as capacity guarantees.

**Director review:** `go test ./internal/world` and `go vet ./internal/world` pass. Important scaling observation: 50-player all-to-all full JSON fanout costs about 2.8 ms on this VPS, before network I/O. Actual world updates use deltas and interest filtering; measure them with active AI/combat and real WebSocket traffic before any capacity claim. No new dependencies, no gameplay/canon changes. Android remains unmeasured; T-038f stays PARTIAL.

## VPS-only continuation: steady-state movement delta fanout (2026-11-21)

Command: `cd server && go test ./internal/world -run '^$' -bench BenchmarkZoneDeltaFanoutJSON -benchmem -benchtime=200ms -count=1`. All players share a large interest radius, alternate movement targets each iteration, and the benchmark runs `Step`, `DeltaFor` and JSON encoding for each observer. Passive enemies; no real network socket or active AI.

| Players + 50 passive enemies | Movement delta fanout (µs/op) | B/op |
|---|---:|---:|
| 1 | 10.6 | 2,208 |
| 10 | 175 | 84,831 |
| 50 | 2,261 | 1,374,806 |

**Review:** movement delta fanout still scales substantially when all moving players see one another. This is a synthetic upper-bound visibility pattern, not a typical populated map or a proven production bottleneck. Do not extrapolate throughput without active combat and actual WS delivery. First benchmark attempt exposed a test-fixture issue (movement targets were eventually reached), fixed by alternating server-authorized targets with monotonic sequence numbers. Final benchmark, `go test ./internal/world`, and `go vet ./internal/world` pass. Android and live WS profiling remain outstanding.

Owner's current deployment instruction: continue on VPS `main`, **do not push to GitHub until explicitly requested**.

## VPS-only continuation: actual Zone.Tick dispatch with JSON sink (2026-11-21)

`cd server && go test ./internal/world -run '^$' -bench BenchmarkZoneTickPushJSON -benchmem -benchtime=150ms -count=1`

This benchmark instantiates `NewZone` with the test combat catalog and 50 configured melee enemies, joins 1/10/50 player sessions, sends monotonic movement commands, executes the actual `Zone.Tick` dispatch loop, and JSON-encodes each pushed payload in an in-process sink. No TCP/WebSocket/TLS, PostgreSQL, physical Android, or realistic mixed combat load. All players have a deliberately large shared interest radius; enemies are mostly passive because they are distant from the spawn.

| Players + 50 enemies | Zone.Tick + JSON sink (µs/op) | B/op |
|---|---:|---:|
| 1 | 9.38 | 2,217 |
| 10 | 178 | 84,905 |
| 50 | 2,084 | 1,373,323 |

Director self-review: all world tests and `go vet ./internal/world` pass. These numbers isolate in-process push serialization, **not real WebSocket throughput**. Scaling in the worst-case all-visible pattern warrants follow-up on interest culling and actual combat traffic. T-038f stays PARTIAL until real transport and device tests.

## Real WebSocket load (loopback), 2026-11-21 — `server/internal/ws/load_test.go`

Opt-in: `cd server && NEWMMO_LOAD=1 go test ./internal/ws -run TestWSLoad -v -count=1` (`NEWMMO_LOAD_SECONDS`, default 5). Production `ws.Server` + `world.Handler` + `Zone.Run` over real TCP loopback; each client does `session.auth` → `world.join.request` → a new movement target at 4 Hz. Run in a 4-vCPU cloud container (not the VPS), Go 1.24.7; the process CPU figure includes **all simulated clients**, so it is an upper bound for the server.

| Players | deltas/s/client | KiB/s per client | KiB/s total | process CPU | GC cycles/5 s |
|---:|---:|---:|---:|---:|---:|
| 1 | 9.8 | 3.6 | 3.6 | 1% | 0 |
| 10 | 9.8 | 13.9 | 138 | 2% | 2 |
| 50 | 9.8 | 59.4 | 2,969 | 14% | 42 |

**Findings:** the 10 Hz tick holds with 50 connected sockets, no write errors, no dropped sessions. Bandwidth per client grows ~linearly with visible players (worst case: all 50 mutually in interest radius, all moving): ~59 KiB/s ≈ 475 kbit/s downlink per phone — acceptable on Wi-Fi/LTE but a battery/data concern; remedies to evaluate (not yet done): smaller interest radius, delta quantization, 5 Hz for far entities. Fixture limits: passive enemies, no abilities in flight, no PostgreSQL, no TLS, no latency. No capacity claim; T-038f stays PARTIAL (mixed combat traffic and Android remain).
