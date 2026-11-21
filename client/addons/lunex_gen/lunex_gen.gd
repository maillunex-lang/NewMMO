@tool
class_name LunexGen
extends RefCounted
## EX-W01 in-house location generator (docs/contracts/location-generator.md).
## Fills `<Layer>/Generated` (Paths, Vegetation, Decorations) of an ordinary editable scene and GUARANTEES that
## every gameplay anchor (player spawn, NPCs, quest interactables, monster spawns) stays reachable.
## Manual = everything that is NOT under a `Generated` node: it is never touched, only respected as an obstacle.
## Editor-only: runs from the "Lunex" tool menu or `cli.gd`; nothing here runs on the Android client at runtime.

const CELL := 16.0
const PLAYER_CLEAR := 22.0     # player collision radius 12 + margin, in px
const ANCHOR_CLEAR := 110.0    # nothing generated closer than this to an anchor
const PATH_WIDTH := 34.0
const PATH_CLEAR := 56.0       # generated props keep this far from a trail centre line
const PROP_SCENES := {
	"oak": "res://scenes/props/tree.tscn",
	"pine": "res://scenes/props/tree_pine.tscn",
	"rock": "res://scenes/props/rock.tscn",
}
const MIN_DIST := {"oak": 74.0, "pine": 66.0, "rock": 64.0}
const LAYER_OF := {"oak": "Vegetation", "pine": "Vegetation", "rock": "Decorations"}
const NAME_OF := {"oak": "Oak", "pine": "Pine", "rock": "Rock"}

# ----- parameters -----------------------------------------------------------------

static func default_params() -> Dictionary:
	return {"seed": 1, "trees": 150, "pine_ratio": 0.3, "rocks": 40, "forest_threshold": 0.35, "trails": true}

static func params_from_scene(root: Node) -> Dictionary:
	var p := default_params()
	if root.has_meta("lunex_gen"):
		for k in (root.get_meta("lunex_gen") as Dictionary).keys():
			p[k] = root.get_meta("lunex_gen")[k]
	return p

static func bounds_of(root: Node) -> Rect2:
	var r := Rect2(0, 0, 2048, 1400)
	if root.has_meta("lunex_bounds"):
		var b: Array = root.get_meta("lunex_bounds")
		r = Rect2(float(b[0]), float(b[1]), float(b[2]), float(b[3]))
	return r

# ----- scene inspection -----------------------------------------------------------

static func _is_generated(n: Node, root: Node) -> bool:
	var cur := n
	while cur != null and cur != root:
		if cur.name == "Generated":
			return true
		cur = cur.get_parent()
	return false

## Blocking circles/rects from the scene's collision shapes. `skip_generated` ignores everything under
## a `Generated` node (used when planning around Manual objects only).
static func collect_blockers(root: Node, skip_generated: bool) -> Array:
	var out: Array = []
	_walk_blockers(root, root, skip_generated, out)
	return out

static func _walk_blockers(n: Node, root: Node, skip_generated: bool, out: Array) -> void:
	if skip_generated and n != root and n.name == "Generated":
		return
	if n is StaticBody2D:
		var body := n as StaticBody2D
		var blocks: Variant = body.get("blocks_movement")
		if blocks == null or bool(blocks):
			for c in body.get_children():
				if c is CollisionShape2D and not (c as CollisionShape2D).disabled and (c as CollisionShape2D).shape != null:
					var cs := c as CollisionShape2D
					var sh := cs.shape
					var pos := cs.global_position
					if sh is CircleShape2D:
						out.append({"pos": pos, "r": (sh as CircleShape2D).radius * cs.global_scale.x, "node": body})
					elif sh is RectangleShape2D:
						var size := (sh as RectangleShape2D).size * cs.global_scale
						out.append({"pos": pos, "rect": Rect2(pos - size / 2.0, size), "node": body})
	for c in n.get_children():
		_walk_blockers(c, root, skip_generated, out)

## Anchors = positions the player must be able to reach.
static func collect_anchors(root: Node) -> Array:
	var out: Array = []
	if root.has_meta("lunex_spawn"):
		var sp: Array = root.get_meta("lunex_spawn")
		out.append({"name": "PlayerSpawn", "id": "PLAYER", "kind": "player_spawn", "pos": Vector2(float(sp[0]), float(sp[1]))})
	_walk_anchors(root, root, out)
	return out

static func _walk_anchors(n: Node, root: Node, out: Array) -> void:
	if n != root and n.name == "Generated":
		return
	if n is Node2D:
		var kind := ""
		var id := ""
		if n.get("interact_id") != null:
			kind = "interact"
			id = str(n.get("interact_id"))
			if str(n.get("dungeon_entry_id")) != "":
				kind = "dungeon_entrance"
		elif n.get("npc_id") != null:
			kind = "npc"
			id = str(n.get("npc_id"))
		elif n.is_in_group("player"):
			kind = "player_spawn"
			id = "PLAYER"
		elif n.get("monster_id") != null:
			kind = "monster_spawn"
			id = str(n.get("monster_id"))
		if kind != "":
			out.append({"name": str(n.name), "id": id, "kind": kind, "pos": (n as Node2D).global_position})
	for c in n.get_children():
		_walk_anchors(c, root, out)

# ----- occupancy grid -------------------------------------------------------------

class Grid:
	var w := 0
	var h := 0
	var origin := Vector2.ZERO
	var blocked := PackedByteArray()

	func _init(bounds: Rect2) -> void:
		origin = bounds.position
		w = int(ceil(bounds.size.x / LunexGen.CELL))
		h = int(ceil(bounds.size.y / LunexGen.CELL))
		blocked.resize(w * h)

	func cell_of(p: Vector2) -> Vector2i:
		return Vector2i(clampi(int((p.x - origin.x) / LunexGen.CELL), 0, w - 1), clampi(int((p.y - origin.y) / LunexGen.CELL), 0, h - 1))

	func center_of(c: Vector2i) -> Vector2:
		return origin + Vector2((c.x + 0.5) * LunexGen.CELL, (c.y + 0.5) * LunexGen.CELL)

	func stamp_circle(pos: Vector2, r: float) -> void:
		var c0 := cell_of(pos - Vector2(r, r))
		var c1 := cell_of(pos + Vector2(r, r))
		for y in range(c0.y, c1.y + 1):
			for x in range(c0.x, c1.x + 1):
				if center_of(Vector2i(x, y)).distance_to(pos) <= r:
					blocked[y * w + x] = 1

	func stamp_rect(rect: Rect2) -> void:
		var c0 := cell_of(rect.position)
		var c1 := cell_of(rect.end)
		for y in range(c0.y, c1.y + 1):
			for x in range(c0.x, c1.x + 1):
				if rect.has_point(center_of(Vector2i(x, y))):
					blocked[y * w + x] = 1

	func is_blocked(c: Vector2i) -> bool:
		return blocked[c.y * w + c.x] == 1

	## Nearest free cell within `max_cells` rings of `c` (anchors sit next to props).
	func nearest_free(c: Vector2i, max_cells: int = 6) -> Vector2i:
		if not is_blocked(c):
			return c
		for ring in range(1, max_cells + 1):
			for dy in range(-ring, ring + 1):
				for dx in range(-ring, ring + 1):
					if maxi(absi(dx), absi(dy)) != ring:
						continue
					var n := Vector2i(c.x + dx, c.y + dy)
					if n.x >= 0 and n.y >= 0 and n.x < w and n.y < h and not is_blocked(n):
						return n
		return Vector2i(-1, -1)

	## Flood fill (8-neighbour without cutting blocked corners) from `start`; returns visited flags.
	func flood(start: Vector2i) -> PackedByteArray:
		var seen := PackedByteArray()
		seen.resize(w * h)
		if start.x < 0 or is_blocked(start):
			return seen
		var queue: Array[Vector2i] = [start]
		seen[start.y * w + start.x] = 1
		var head := 0
		while head < queue.size():
			var cur: Vector2i = queue[head]
			head += 1
			for d in [Vector2i(1, 0), Vector2i(-1, 0), Vector2i(0, 1), Vector2i(0, -1), Vector2i(1, 1), Vector2i(1, -1), Vector2i(-1, 1), Vector2i(-1, -1)]:
				var n: Vector2i = cur + d
				if n.x < 0 or n.y < 0 or n.x >= w or n.y >= h or seen[n.y * w + n.x] == 1 or is_blocked(n):
					continue
				if d.x != 0 and d.y != 0 and (is_blocked(Vector2i(cur.x + d.x, cur.y)) or is_blocked(Vector2i(cur.x, cur.y + d.y))):
					continue
				seen[n.y * w + n.x] = 1
				queue.append(n)
		return seen

static func build_grid(bounds: Rect2, blockers: Array) -> Grid:
	var g := Grid.new(bounds)
	for b in blockers:
		if b.has("rect"):
			var rc: Rect2 = b["rect"]
			g.stamp_rect(rc.grow(PLAYER_CLEAR))
		else:
			g.stamp_circle(b["pos"], float(b["r"]) + PLAYER_CLEAR)
	return g

# ----- verification (the "no dead ends" guarantee) -------------------------------------

## Reports anchors the player cannot reach from the spawn. `ok` = all reachable (and a spawn exists).
static func verify(root: Node) -> Dictionary:
	var bounds := bounds_of(root)
	var anchors := collect_anchors(root)
	var grid := build_grid(bounds, collect_blockers(root, false))
	var spawn := {}
	for a in anchors:
		if a["kind"] == "player_spawn":
			spawn = a
	var report := {"ok": false, "anchors": anchors.size(), "unreachable": [], "has_spawn": not spawn.is_empty()}
	if spawn.is_empty():
		return report
	var start := grid.nearest_free(grid.cell_of(spawn["pos"]))
	var seen := grid.flood(start)
	for a in anchors:
		var c := grid.nearest_free(grid.cell_of(a["pos"]))
		if c.x < 0 or seen[c.y * grid.w + c.x] == 0:
			report["unreachable"].append("%s (%s)" % [a["name"], a["id"]])
	report["ok"] = (report["unreachable"] as Array).is_empty()
	return report

## Client quest targets (res://data/quests/*.json) that have no node in the scene = a quest dead end by design.
static func missing_quest_targets(root: Node) -> Array:
	var have := {}
	for a in collect_anchors(root):
		have[str(a["id"])] = true
	var missing: Array = []
	var dir := DirAccess.open("res://data/quests")
	if dir == null:
		return missing
	for f in dir.get_files():
		if not f.ends_with(".json"):
			continue
		var parsed: Variant = JSON.parse_string(FileAccess.get_file_as_string("res://data/quests/" + f))
		if typeof(parsed) != TYPE_DICTIONARY:
			continue
		for q in (parsed as Dictionary).get("quests", []):
			for o in q.get("objectives", []):
				var t := str(o.get("target", ""))
				if t != "" and not have.has(t) and not missing.has(t):
					missing.append(t)
	return missing

# ----- generation --------------------------------------------------------------------------

static func _layer_generated(root: Node, layer: String) -> Node2D:
	var lay := root.get_node_or_null(layer)
	if lay == null:
		lay = Node2D.new()
		lay.name = layer
		root.add_child(lay)
		lay.owner = root
	var gen := lay.get_node_or_null("Generated")
	if gen == null:
		gen = Node2D.new()
		gen.name = "Generated"
		gen.y_sort_enabled = true
		lay.add_child(gen)
		gen.owner = root
	return gen as Node2D

static func _clear_generated(root: Node) -> int:
	var n := 0
	for layer in ["Paths", "Vegetation", "Decorations"]:
		var lay := root.get_node_or_null(layer)
		if lay == null:
			continue
		var gen := lay.get_node_or_null("Generated")
		if gen == null:
			continue
		for c in gen.get_children():
			gen.remove_child(c)
			c.queue_free()
			n += 1
	return n

static func _dist_to_polyline(p: Vector2, pts: PackedVector2Array) -> float:
	var best := 1e9
	for i in range(pts.size() - 1):
		var cp := Geometry2D.get_closest_point_to_segment(p, pts[i], pts[i + 1])
		best = minf(best, p.distance_to(cp))
	return best

static func _rdp(pts: PackedVector2Array, eps: float) -> PackedVector2Array:
	if pts.size() < 3:
		return pts
	var idx := 0
	var dmax := 0.0
	for i in range(1, pts.size() - 1):
		var d := pts[i].distance_to(Geometry2D.get_closest_point_to_segment(pts[i], pts[0], pts[pts.size() - 1]))
		if d > dmax:
			dmax = d
			idx = i
	if dmax <= eps:
		return PackedVector2Array([pts[0], pts[pts.size() - 1]])
	var left := _rdp(pts.slice(0, idx + 1), eps)
	var right := _rdp(pts.slice(idx), eps)
	left.resize(left.size() - 1)
	left.append_array(right)
	return left

## Generates trails, trees/pines and rocks into the `Generated` nodes and repairs any dead end it created.
## `root` must be inside a SceneTree (global transforms). Returns a report; apply=false builds on a throw-away copy.
static func generate(root: Node, params: Dictionary, apply: bool = true) -> Dictionary:
	var work := root
	var temp: Node = null
	if not apply:
		temp = root.duplicate()
		root.get_parent().add_child(temp) if root.get_parent() != null else root.get_tree().root.add_child(temp)
		temp.visible = false
		work = temp
	var report := _generate_in(work, params)
	if temp != null:
		temp.get_parent().remove_child(temp)
		temp.queue_free()
	report["applied"] = apply
	return report

static func _generate_in(root: Node, params: Dictionary) -> Dictionary:
	var p := default_params()
	for k in params.keys():
		p[k] = params[k]
	var bounds := bounds_of(root)
	var rep := {"seed": int(p["seed"]), "previous_generated": 0, "paths": 0, "trees": 0, "rocks": 0, "removed_by_repair": 0,
			"manual_blockers": 0, "unreachable_manual": [], "ok": false}
	rep["previous_generated"] = _clear_generated(root)
	var rng := RandomNumberGenerator.new()
	rng.seed = int(p["seed"])
	var anchors := collect_anchors(root)
	var manual := collect_blockers(root, true)
	rep["manual_blockers"] = manual.size()
	var mgrid := build_grid(bounds, manual)

	# 1) trails: minimum spanning tree over the anchors, routed around Manual obstacles
	var trails: Array = []
	if bool(p["trails"]) and anchors.size() > 1:
		trails = _plan_trails(anchors, mgrid, bounds)
	var paths_gen := _layer_generated(root, "Paths")
	for i in range(trails.size()):
		var line := Line2D.new()
		line.name = "Gen_Path_%03d" % i
		line.width = PATH_WIDTH
		line.default_color = Color(0.5, 0.37, 0.25, 0.92)
		line.joint_mode = Line2D.LINE_JOINT_ROUND
		line.begin_cap_mode = Line2D.LINE_CAP_ROUND
		line.end_cap_mode = Line2D.LINE_CAP_ROUND
		line.antialiased = true
		line.z_index = -400
		line.points = trails[i]
		line.set_meta("gen_id", "path-%d" % i)
		paths_gen.add_child(line)
		line.owner = root
	rep["paths"] = trails.size()

	# 2) scatter props with clearance from anchors, trails and Manual obstacles
	var noise := FastNoiseLite.new()
	noise.seed = int(p["seed"])
	noise.frequency = 0.0035
	var accepted: Array = []   # {type, pos}
	var targets := [["oak", int(round(int(p["trees"]) * (1.0 - float(p["pine_ratio"]))))], ["pine", int(round(int(p["trees"]) * float(p["pine_ratio"])))], ["rock", int(p["rocks"])]]
	for t in targets:
		var type: String = t[0]
		var want: int = t[1]
		var got := 0
		var tries := want * 60 + 200
		while got < want and tries > 0:
			tries -= 1
			var pos := Vector2(rng.randf_range(bounds.position.x + 60.0, bounds.end.x - 60.0), rng.randf_range(bounds.position.y + 80.0, bounds.end.y - 40.0))
			if type != "rock":
				var density := clampf((noise.get_noise_2dv(pos) * 0.5 + 0.5 - float(p["forest_threshold"])) * 3.0, 0.0, 1.0)
				if rng.randf() > density:
					continue
			if not _free_spot(pos, type, anchors, trails, manual, accepted):
				continue
			accepted.append({"type": type, "pos": pos})
			got += 1
	var counters := {"oak": 0, "pine": 0, "rock": 0}
	for a in accepted:
		var type: String = a["type"]
		counters[type] += 1
		var scene: PackedScene = load(PROP_SCENES[type])
		var node: Node2D = scene.instantiate()
		node.name = "Gen_%s_%04d" % [NAME_OF[type], counters[type]]
		node.position = a["pos"]
		node.set_meta("gen_id", "%s-%d" % [type, counters[type]])
		_layer_generated(root, LAYER_OF[type]).add_child(node)
		node.owner = root
		# instance children keep the packed scene's own ownership: nothing else to own
	rep["trees"] = counters["oak"] + counters["pine"]
	rep["rocks"] = counters["rock"]

	# 3) repair: no generated blocker may cut an anchor off; Manual blockers are never touched
	var guard := 0
	while guard < 400:
		guard += 1
		var v := verify(root)
		if bool(v["ok"]):
			rep["ok"] = true
			break
		if not _remove_nearest_generated(root, anchors, v):
			rep["unreachable_manual"] = v["unreachable"]
			break
		rep["removed_by_repair"] += 1
	return rep

static func _free_spot(pos: Vector2, type: String, anchors: Array, trails: Array, manual: Array, accepted: Array) -> bool:
	for a in anchors:
		if pos.distance_to(a["pos"]) < ANCHOR_CLEAR:
			return false
	for tr in trails:
		if _dist_to_polyline(pos, tr) < PATH_CLEAR:
			return false
	for m in manual:
		if m.has("rect"):
			if (m["rect"] as Rect2).grow(50.0).has_point(pos):
				return false
		elif pos.distance_to(m["pos"]) < float(m["r"]) + 46.0:
			return false
	var md: float = MIN_DIST[type]
	for o in accepted:
		if pos.distance_to(o["pos"]) < maxf(md, MIN_DIST[o["type"]]):
			return false
	return true

## Removes the generated blocker closest to the first cut-off anchor; false when none is left (a Manual dead end).
static func _remove_nearest_generated(root: Node, anchors: Array, v: Dictionary) -> bool:
	var first := str((v["unreachable"] as Array)[0])
	var target := Vector2.INF
	for a in anchors:
		if first.begins_with(str(a["name"]) + " ("):
			target = a["pos"]
	if target == Vector2.INF:
		return false
	var best: Node = null
	var best_d := 700.0
	for layer in ["Vegetation", "Decorations"]:
		var gen := root.get_node_or_null(layer + "/Generated")
		if gen == null:
			continue
		for c in gen.get_children():
			if c is Node2D:
				var d := (c as Node2D).global_position.distance_to(target)
				if d < best_d:
					best_d = d
					best = c
	if best == null:
		return false
	best.get_parent().remove_child(best)
	best.queue_free()
	return true

static func _plan_trails(anchors: Array, mgrid: Grid, bounds: Rect2) -> Array:
	var astar := AStarGrid2D.new()
	astar.region = Rect2i(0, 0, mgrid.w, mgrid.h)
	astar.cell_size = Vector2(CELL, CELL)
	astar.diagonal_mode = AStarGrid2D.DIAGONAL_MODE_ONLY_IF_NO_OBSTACLES
	astar.update()
	for y in range(mgrid.h):
		for x in range(mgrid.w):
			if mgrid.blocked[y * mgrid.w + x] == 1:
				astar.set_point_solid(Vector2i(x, y), true)
	# Prim over anchors (Euclidean), routing each edge with A*.
	var inside: Array = [0]
	var out: Array = []
	var remaining: Array = range(1, anchors.size())
	while not remaining.is_empty():
		var best_i := -1
		var best_j := -1
		var best_d := 1e18
		for i in inside:
			for j in remaining:
				var d: float = (anchors[i]["pos"] as Vector2).distance_squared_to(anchors[j]["pos"])
				if d < best_d:
					best_d = d
					best_i = i
					best_j = j
		remaining.erase(best_j)
		inside.append(best_j)
		var c0 := mgrid.nearest_free(mgrid.cell_of(anchors[best_i]["pos"]))
		var c1 := mgrid.nearest_free(mgrid.cell_of(anchors[best_j]["pos"]))
		if c0.x < 0 or c1.x < 0:
			continue
		var ids := astar.get_id_path(c0, c1)
		if ids.is_empty():
			continue
		var pts := PackedVector2Array()
		pts.append(anchors[best_i]["pos"])
		for id in ids:
			pts.append(mgrid.center_of(id))
		pts.append(anchors[best_j]["pos"])
		out.append(_rdp(pts, 10.0))
	return out

# ----- export for the server (W5 stub) ---------------------------------------------------------

## Static blockers as plain data (circles `[x, y, r]`, rects `[x, y, w, h]`) for server-side movement checks.
static func export_obstacles(root: Node) -> Dictionary:
	var circles: Array = []
	var rects: Array = []
	for b in collect_blockers(root, false):
		if b.has("rect"):
			var rc: Rect2 = b["rect"]
			rects.append([snappedf(rc.position.x, 0.1), snappedf(rc.position.y, 0.1), snappedf(rc.size.x, 0.1), snappedf(rc.size.y, 0.1)])
		else:
			circles.append([snappedf((b["pos"] as Vector2).x, 0.1), snappedf((b["pos"] as Vector2).y, 0.1), snappedf(float(b["r"]), 0.1)])
	var bounds := bounds_of(root)
	return {"version": 1, "bounds": [bounds.position.x, bounds.position.y, bounds.size.x, bounds.size.y],
			"player_clearance": PLAYER_CLEAR, "circles": circles, "rects": rects}
