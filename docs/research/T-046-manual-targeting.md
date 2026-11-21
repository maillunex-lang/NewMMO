# T-046 — Manual attack targeting

The project already has entity filtering in the Godot world controller, the local player controller and an authoritative server range check. Use these entry points and Godot's built-in distance calculation. The server supplies the effective radius in the world snapshot, including the private dungeon snapshot.

Checked the [official Godot Vector2 reference](https://docs.godotengine.org/en/stable/classes/class_vector2.html), [Godot demo repository](https://github.com/godotengine/godot-demo-projects), [Godot Asset Library](https://godotengine.org/asset-library/asset?filter=target) and [Hysen Labs directory](https://hysenlabs.com/projects). The current code already supplies all information needed for target selection. No external implementation is needed.
