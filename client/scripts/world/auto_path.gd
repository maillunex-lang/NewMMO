class_name AutoPath
extends RefCounted
## Client-side walkable grid + A* for tap-to-move and quest auto-travel (EX-UX autopath).
## Built once per location from the scene's StaticBody2D collision shapes (same occupancy rules as the
## location generator, so "reachable" here == "verified reachable" by the map verifier).
## Output is a short polyline of waypoints; the player walks it by feeding each waypoint to the normal
## movement intent, so the server (which only knows a straight-line target + speed cap) follows the same route.

var _grid: LunexGen.Grid = null
var _astar := AStarGrid2D.new()

func build(root: Node) -> void:
	var bounds := LunexGen.bounds_of(root)
	_grid = LunexGen.build_grid(bounds, LunexGen.collect_blockers(root, false))
	_astar = AStarGrid2D.new()
	_astar.region = Rect2i(0, 0, _grid.w, _grid.h)
	_astar.cell_size = Vector2(LunexGen.CELL, LunexGen.CELL)
	_astar.diagonal_mode = AStarGrid2D.DIAGONAL_MODE_ONLY_IF_NO_OBSTACLES
	_astar.update()
	for y in range(_grid.h):
		for x in range(_grid.w):
			if _grid.blocked[y * _grid.w + x] == 1:
				_astar.set_point_solid(Vector2i(x, y), true)

func is_ready() -> bool:
	return _grid != null

## Waypoints from `from` to `to` (start excluded, destination included). Empty when no route exists.
## A blocked destination (NPC standing next to a prop) is snapped to the nearest free cell.
func find_path(from: Vector2, to: Vector2) -> PackedVector2Array:
	var out := PackedVector2Array()
	if _grid == null:
		out.append(to)
		return out
	var c0 := _grid.nearest_free(_grid.cell_of(from), 8)
	var c1 := _grid.nearest_free(_grid.cell_of(to), 8)
	if c0.x < 0 or c1.x < 0:
		return out
	var ids := _astar.get_id_path(c0, c1)
	if ids.is_empty():
		return out
	var pts: Array[Vector2] = [from]
	for i in range(1, ids.size()):
		pts.append(_grid.center_of(ids[i]))
	var goal := to if not _blocked_at(to) else _grid.center_of(c1)
	if pts.size() > 1:
		pts[pts.size() - 1] = goal
	else:
		pts.append(goal)
	return _smooth(pts)

func _blocked_at(p: Vector2) -> bool:
	return _grid.is_blocked(_grid.cell_of(p))

## String pulling: keep a waypoint only when the straight segment to the next one would clip an obstacle.
func _smooth(pts: Array[Vector2]) -> PackedVector2Array:
	var out := PackedVector2Array()
	var anchor := 0
	while anchor < pts.size() - 1:
		var far := pts.size() - 1
		while far > anchor + 1 and not _line_clear(pts[anchor], pts[far]):
			far -= 1
		out.append(pts[far])
		anchor = far
	return out

func _line_clear(a: Vector2, b: Vector2) -> bool:
	var steps := int(ceil(a.distance_to(b) / (LunexGen.CELL * 0.5)))
	for i in range(1, steps + 1):
		if _blocked_at(a.lerp(b, float(i) / float(steps))):
			return false
	return true
