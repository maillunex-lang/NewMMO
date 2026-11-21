extends SceneTree
## EX-W01 smoke: generated maps never create quest dead ends; Manual objects survive regen; a Manual wall is detected.

func _fail(msg: String) -> void:
	print("MAPGEN_SMOKE FAIL: ", msg)
	quit(1)

func _init() -> void:
	var packed: PackedScene = load("res://scenes/locations/dawn_valley.tscn")
	var root: Node = packed.instantiate()
	get_root().add_child(root)
	var v := LunexGen.verify(root)
	if not bool(v["ok"]) or not LunexGen.missing_quest_targets(root).is_empty():
		_fail("committed scene not clean: %s" % [v])
		return
	var manual_before := LunexGen.collect_blockers(root, true).size()
	# many seeds, dense settings: repair must always leave every anchor reachable
	for s in [1, 2, 3, 11, 42, 99, 1234, 777]:
		var rep := LunexGen.generate(root, {"seed": s, "trees": 400, "rocks": 120}, true)
		if not bool(rep["ok"]):
			_fail("seed %d left dead end: %s" % [s, rep])
			return
		if LunexGen.collect_blockers(root, true).size() != manual_before:
			_fail("manual blockers changed on seed %d" % s)
			return
	# same seed twice = same output
	var a := LunexGen.generate(root, {"seed": 5}, true)
	var names1: Array = []
	for c in root.get_node("Vegetation/Generated").get_children():
		names1.append("%s@%s" % [c.name, (c as Node2D).position])
	var b := LunexGen.generate(root, {"seed": 5}, true)
	var names2: Array = []
	for c in root.get_node("Vegetation/Generated").get_children():
		names2.append("%s@%s" % [c.name, (c as Node2D).position])
	if names1 != names2 or a["trees"] != b["trees"]:
		_fail("not deterministic")
		return
	# a Manual wall around an anchor must be reported as a dead end
	var wall := StaticBody2D.new()
	wall.name = "ManualWall"
	var cs := CollisionShape2D.new()
	var rs := RectangleShape2D.new()
	rs.size = Vector2(900, 700)
	cs.shape = rs
	wall.add_child(cs)
	# hollow it: ring of 4 rects around the Board anchor
	root.add_child(wall)
	wall.global_position = Vector2(620, 640)
	var bad := LunexGen.verify(root)
	if bool(bad["ok"]):
		_fail("Manual wall over anchors not detected")
		return
	print("MAPGEN_SMOKE OK anchors=%d trees=%s rocks=%s" % [v["anchors"], a["trees"], a["rocks"]])
	quit(0)
