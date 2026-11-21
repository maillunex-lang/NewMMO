# T-031 / T-032 — Location generator research


## Update (T-032): decision
In-house generator chosen (grid BFS + MST/A* + Poisson-style scatter on Godot built-ins `AStarGrid2D`, `FastNoiseLite`). No third-party code or assets added, so no owner gate. Reason: the hard requirement (no quest dead ends, Manual survives regen) is project-specific and small enough to own. See `docs/contracts/location-generator.md`.
