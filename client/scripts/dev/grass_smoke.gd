extends SceneTree
## Headless: footstep press map. A step flattens grass at once, holds ~0.7 s, then recovers (~2 s), and only near the step.
var _failed := false
func _init() -> void:
	call_deferred("_run")
func _assert(c: bool, l: String) -> void:
	if not c:
		push_error("ASSERT FAIL: " + l)
		_failed = true
func _press_at(g: Grass, world: Vector2) -> int:
	var texel := g._area / float(Grass.MAP_PX)
	var x := int((world.x - g._origin.x) / texel)
	var z := int((world.y - g._origin.y) / texel)
	return g._bytes[(z * Grass.MAP_PX + x) * 3]
func _run() -> void:
	var g := Grass.new()
	root.add_child(g)
	g.setup("high", null, Vector2(64, 44))
	var focus := Vector3(20, 0, 20)
	g.tick(0.016, focus, [[1, Vector2(20.0, 20.0)]])       # first footprint
	_assert(_press_at(g, Vector2(20, 20)) >= 250, "fully pressed right after the step")
	_assert(_press_at(g, Vector2(20.0 + 4.0, 20.0)) == 0, "grass 4 m away is untouched")
	for i in 30:   # 0.5 s at 60 fps
		g.tick(0.016, focus, [])
	_assert(_press_at(g, Vector2(20, 20)) >= 250, "still held flat at 0.5 s")
	for i in 40:   # up to ~1.15 s
		g.tick(0.016, focus, [])
	var mid := _press_at(g, Vector2(20, 20))
	_assert(mid > 0 and mid < 250, "recovering after the hold (%d)" % mid)
	for i in 200:  # ~4.4 s total
		g.tick(0.016, focus, [])
	_assert(_press_at(g, Vector2(20, 20)) == 0, "fully recovered")
	# walking leaves a trail: 5 m of steps
	for i in 12:
		g.tick(0.016, focus, [[2, Vector2(20.0 + i * 0.5, 22.0)]])
	_assert(_press_at(g, Vector2(21.0, 22.0)) > 200 and _press_at(g, Vector2(24.0, 22.0)) > 200, "trail along the path")
	if _failed:
		quit(1)
		return
	print("GRASS_SMOKE_OK")
	quit(0)
