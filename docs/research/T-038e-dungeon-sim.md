# T-038e — Dungeon rooms on the world simulation

The owner approved the in-process Go approach at 10 Hz in T-038. This slice builds a private room simulation from the dungeon registry. The server uses the positions exported from the editable Godot scene, with the existing registry check detecting drift.

| Candidate | Findings |
|---|---|
| Existing `internal/world` and Go `time` | Already runs combat and snapshots for the Godot 4.7.2 client. Reuses the pinned Go toolchain and existing Android client. [Ticker documentation](https://pkg.go.dev/time#NewTicker). |
| Nakama | Apache-2.0. Upstream activity verified through the GitHub API on 2026-11-21. Authoritative matches are available, but integration would add its runtime and duplicate the current session and transport layer. [Upstream](https://github.com/heroiclabs/nakama). |
| Arche | MIT. GitHub reported last push 2025-04-01. An ECS could replace entity storage, but would leave room transitions, combat outcomes and Godot client integration to the project. [Upstream](https://github.com/mlange-42/arche). |
| Godot high-level multiplayer | Built into Godot 4.x, with Android network support. Its wire protocol would require a Godot server or replacing the existing Go WebSocket contract. [Official docs](https://docs.godotengine.org/en/stable/tutorials/networking/high_level_multiplayer.html). |

Checked the [Hysen Labs directory](https://hysenlabs.com/projects) for discovery and the [Godot Asset Library multiplayer listing](https://godotengine.org/asset-library/asset?filter=multiplayer) for client integrations. Reuse the existing simulator, room state machine and Godot client. No dependency or asset adoption is needed.
