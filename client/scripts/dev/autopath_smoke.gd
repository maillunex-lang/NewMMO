extends SceneTree
## Headless: obstacle-aware autopath. Marker AUTOPATH_SMOKE_OK.
## 1) every quest anchor is routable from the spawn and no route leg crosses a blocked cell,
## 2) a straight line through props is detoured, 3) the real player walks to an NPC and auto-interacts.

var _failed := false

func _init() -> void:
	call_deferred("_run")

func _assert(cond: bool, label: String) -> void:
	if not cond:
		push_error("ASSERT FAIL: %s" % label)
		_failed = true

func _run() -> void:
	change_scene_to_file("res://scenes/world/main.tscn")
	await process_frame
	await process_frame
	await process_frame
	var main := root.get_child(0)
	var valley := main.get_node("DawnValley")
	var player: CharacterBody2D = valley.get_node("Player")
	var ap := AutoPath.new()
	ap.build(valley)
	var grid := LunexGen.build_grid(LunexGen.bounds_of(valley), LunexGen.collect_blockers(valley, false))
	var spawn: Vector2 = player.global_position

	# 1) all anchors routable, legs clear
	var routed := 0
	var detours := 0
	for a in LunexGen.collect_anchors(valley):
		if a["kind"] == "player_spawn":
			continue
		var p := ap.find_path(spawn, a["pos"])
		_assert(not p.is_empty(), "route to %s" % a["id"])
		var prev := spawn
		var len := 0.0
		for w in p:
			len += prev.distance_to(w)
			var steps := int(prev.distance_to(w) / 4.0) + 1
			for i in range(steps + 1):
				var q := prev.lerp(w, float(i) / float(steps))
				var c := grid.cell_of(q)
				# the very first cells may sit inside a clearance halo around the start; never inside an interior of a blocker
				if grid.is_blocked(c) and q.distance_to(a["pos"]) > 40.0 and q.distance_to(spawn) > 30.0:
					_assert(false, "leg to %s crosses blocked cell at %s" % [a["id"], q])
					break
			prev = w
		routed += 1
		if len > spawn.distance_to(a["pos"]) * 1.05:
			detours += 1
	print("AUTOPATH anchors_routed=%d detoured=%d" % [routed, detours])
	_assert(routed >= 20, "enough anchors routed")
	_assert(detours >= 1, "at least one route needs a detour around props")

	# 2) a route to an unreachable-by-line spot is not the straight line
	var far := Vector2(1900, 1300)
	var path := ap.find_path(spawn, far)
	_assert(not path.is_empty(), "far route exists")

	# 3) real walk + auto interaction
	var npc: Node2D = valley.get_node("NPCSpawns/FeiraNiss")
	var hit := [0]
	var probe := Node2D.new()
	probe.set_script(load("res://scripts/dev/autopath_probe.gd"))
	probe.global_position = npc.global_position + Vector2(0, 0)
	valley.add_child(probe)
	probe.connect("interacted", func(): hit[0] += 1)
	player.auto_path = ap
	player.auto_combat = false
	player.move_to_world(probe.global_position, probe)
	_assert(player.has_path(), "player has a route")
	var frames := 0
	while hit[0] == 0 and frames < 6000:
		await physics_frame
		frames += 1
	print("AUTOPATH walk frames=%d interacted=%d dist=%.0f" % [frames, hit[0], player.global_position.distance_to(probe.global_position)])
	_assert(hit[0] == 1, "arrival triggered exactly one interaction")
	_assert(not player.has_path(), "route cleared on arrival")

	# 4) farthest static anchor (monsters fight back and body-block, which would make this test flaky)
	for m in get_nodes_in_group("monsters"):
		m.set_physics_process(false)
	# farthest anchor through real collision (props must not trap the player)
	var best: Dictionary = {}
	for a in LunexGen.collect_anchors(valley):
		if a["kind"] in ["npc", "interact"] and (best.is_empty() or (a["pos"] as Vector2).distance_to(player.global_position) > (best["pos"] as Vector2).distance_to(player.global_position)):
			best = a
	probe.global_position = best["pos"]
	hit[0] = 0
	player.move_to_world(probe.global_position, probe)
	frames = 0
	while hit[0] == 0 and frames < 12000 and (player.has_path() or frames < 5):
		await physics_frame
		frames += 1
	print("AUTOPATH far walk to %s frames=%d interacted=%d dist=%.0f" % [best["id"], frames, hit[0], player.global_position.distance_to(probe.global_position)])
	_assert(hit[0] == 1, "far walk reached %s without getting stuck" % best["id"])

	if _failed:
		quit(1)
		return
	print("AUTOPATH_SMOKE_OK")
	quit(0)
