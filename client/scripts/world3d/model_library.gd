class_name ModelLibrary
extends RefCounted
## Maps game things to 3D models. Real Meshy GLBs live in res://assets/models/<name>.glb and win automatically;
## until a model exists a bright procedural placeholder is built (so the 3D slice is playable before art arrives).
## World scale: 1 unit = 1 metre; server/2D logic uses 32 px per metre.

const MODEL_DIR := "res://assets/models/"
const CLASS_COLORS := {
	"CLS-01": Color(0.35, 0.55, 0.95), "CLS-02": Color(0.9, 0.25, 0.2), "CLS-03": Color(0.62, 0.38, 0.98),
	"CLS-04": Color(0.25, 0.8, 0.35), "CLS-05": Color(1.0, 0.85, 0.35),
}
const ENEMY_COLORS := {
	"ENM-01-001": Color(0.85, 0.4, 0.2), "ENM-01-002": Color(0.9, 0.7, 0.2), "ENM-01-003": Color(0.6, 0.6, 0.7),
	"ENM-01-004": Color(0.3, 0.9, 0.35),
}
# 2D scene file → model name
const PROP_NAMES := {
	"tree.tscn": "tree_oak", "tree_pine.tscn": "tree_pine", "rock.tscn": "rock", "cottage.tscn": "cottage",
	"fence.tscn": "fence", "lantern.tscn": "lantern", "crystal.tscn": "crystal",
}

static func _mat(c: Color, rough := 0.85, emissive := 0.0) -> StandardMaterial3D:
	var m := StandardMaterial3D.new()
	m.albedo_color = c
	m.roughness = rough
	if emissive > 0.0:
		m.emission_enabled = true
		m.emission = c
		m.emission_energy_multiplier = emissive
	return m

static func _mesh(parent: Node3D, mesh: Mesh, mat: Material, pos := Vector3.ZERO, scl := Vector3.ONE) -> MeshInstance3D:
	var mi := MeshInstance3D.new()
	mi.mesh = mesh
	mi.material_override = mat
	mi.position = pos
	mi.scale = scl
	parent.add_child(mi)
	return mi

static func _cyl(top: float, bottom: float, h: float, seg := 10) -> CylinderMesh:
	var c := CylinderMesh.new()
	c.top_radius = top
	c.bottom_radius = bottom
	c.height = h
	c.radial_segments = seg
	c.rings = 1
	return c

static func _sphere(r: float, seg := 12) -> SphereMesh:
	var s := SphereMesh.new()
	s.radius = r
	s.height = r * 2.0
	s.radial_segments = seg
	s.rings = seg / 2
	return s

## Target heights in metres for imported models (Meshy output has arbitrary scale/pivot).
const HEIGHTS := {
	"tree_oak": 5.2, "tree_pine": 6.0, "rock": 1.3, "cottage": 5.2, "fence": 1.2, "lantern": 3.0, "crystal": 2.2,
	"npc_generic": 1.85, "bush": 1.1, "flowers": 0.6, "mushrooms": 0.5, "stump": 0.8, "tree_birch": 5.6, "well": 2.4, "barrel": 1.0, "haystack": 1.3,
	"hero_cls-01": 1.95, "hero_cls-02": 1.95, "hero_cls-04": 1.85, "hero_cls-05": 1.9, "enemy_enm-01-002": 1.5,
	"npc_elder_mira": 1.65, "npc_npc-802": 1.8, "npc_npc-102": 1.85, "hero_cls-03": 1.9, "enemy_enm-01-001": 1.85, "enemy_enm-01-003": 1.1, "enemy_enm-01-004": 1.2,
}

static func _aabb_of(node: Node3D) -> AABB:
	# Skinned (rigged) models: the mesh AABB is in bind space and can be off by the armature scale (Meshy: cm bones, 0.01
	# armature). Measure the skeleton instead and extend the span to the full body height (bones end at the head centre).
	var skels := node.find_children("*", "Skeleton3D", true, false)
	if not skels.is_empty():
		var sk := skels[0] as Skeleton3D
		var chain := Transform3D.IDENTITY
		var p: Node = sk
		while p != null and p != node:
			if p is Node3D:
				chain = (p as Node3D).transform * chain
			p = p.get_parent()
		if node is Node3D and p == node:
			chain = (node as Node3D).transform * chain if false else chain
		var lo := Vector3(1e9, 1e9, 1e9)
		var hi := Vector3(-1e9, -1e9, -1e9)
		for i in sk.get_bone_count():
			var pos: Vector3 = chain * sk.get_bone_global_pose(i).origin
			lo = lo.min(pos)
			hi = hi.max(pos)
		var span := hi.y - lo.y
		var full := span / 0.87
		return AABB(Vector3(lo.x, lo.y - 0.02 * full, lo.z), Vector3(hi.x - lo.x, full, hi.z - lo.z))
	var acc := [AABB(), false]
	_aabb_walk(node, Transform3D.IDENTITY, acc)
	return acc[0]

## Works on nodes that are not in the tree yet (global_transform would be invalid there).
static func _aabb_walk(n: Node, xf: Transform3D, acc: Array) -> void:
	var cur := xf
	if n is Node3D:
		cur = xf * (n as Node3D).transform
	if n is MeshInstance3D:
		var a: AABB = cur * (n as MeshInstance3D).get_aabb()
		acc[0] = a if not acc[1] else (acc[0] as AABB).merge(a)
		acc[1] = true
	for c in n.get_children():
		_aabb_walk(c, cur, acc)

## Try a real GLB first. Returns null when absent. The model is scaled to `height` metres, centred on x/z and
## standing on y=0, inside a node named "Body" (animated bob / future skeleton animation target).
static func load_glb(name: String, height := 0.0) -> Node3D:
	var path := MODEL_DIR + name + ".glb"
	if not ResourceLoader.exists(path):
		return null
	var ps := load(path) as PackedScene
	if ps == null:
		return null
	var inst := ps.instantiate() as Node3D
	var h: float = height if height > 0.0 else float(HEIGHTS.get(name, 1.8 if name.begins_with("hero_") or name.begins_with("enemy_") else 2.0))
	var holder := Node3D.new()
	var body := Node3D.new()
	body.name = "Body"
	holder.add_child(body)
	body.add_child(inst)
	_attach_animations(inst)
	var box := _aabb_of(inst)  # inst itself has identity transform here
	if box.size.y > 0.001:
		var k := h / box.size.y
		inst.scale = Vector3.ONE * k
		inst.position = Vector3(-box.get_center().x * k, -box.position.y * k, -box.get_center().z * k)
	return holder

static var _anim_cache: Dictionary = {}

## Meshy humanoids share one 24-bone skeleton, so one baked animation set (res://assets/anims/humanoid/*.res) drives all.
static func _attach_animations(inst: Node) -> void:
	var skel := inst.find_children("*", "Skeleton3D", true, false)
	var players := inst.find_children("*", "AnimationPlayer", true, false)
	if skel.is_empty() or players.is_empty():
		return
	if _anim_cache.is_empty():
		for n in ["idle", "walk", "run", "attack", "cast", "hit", "dead"]:
			var path := "res://assets/anims/humanoid/%s.res" % n
			if ResourceLoader.exists(path):
				_anim_cache[n] = load(path)
	if _anim_cache.is_empty():
		return
	var ap := players[0] as AnimationPlayer
	for old in ap.get_animation_library_list():
		ap.remove_animation_library(old)
	var lib := AnimationLibrary.new()
	for k in _anim_cache.keys():
		lib.add_animation(k, _anim_cache[k])
	ap.add_animation_library("", lib)
	ap.set_meta("humanoid", true)

static func prop(scene_file: String) -> Node3D:
	var base := scene_file.get_file()
	var name: String = PROP_NAMES.get(base, "")
	if name == "":
		return null
	var real := load_glb(name)
	if real != null:
		return real
	var root := Node3D.new()
	match name:
		"tree_oak":
			_mesh(root, _cyl(0.25, 0.4, 2.6), _mat(Color(0.5, 0.3, 0.14)), Vector3(0, 1.3, 0))
			_mesh(root, _sphere(1.9), _mat(Color(0.2, 0.72, 0.2)), Vector3(0, 3.6, 0), Vector3(1, 0.9, 1))
			_mesh(root, _sphere(1.3), _mat(Color(0.3, 0.8, 0.25)), Vector3(0.9, 4.4, 0.3))
		"tree_pine":
			_mesh(root, _cyl(0.2, 0.35, 2.0), _mat(Color(0.45, 0.28, 0.14)), Vector3(0, 1.0, 0))
			for i in 4:
				_mesh(root, _cyl(0.05, 1.7 - i * 0.32, 1.7, 8), _mat(Color(0.1, 0.55 + i * 0.05, 0.3)), Vector3(0, 2.0 + i * 1.05, 0))
		"rock":
			_mesh(root, _sphere(0.9, 8), _mat(Color(0.62, 0.64, 0.72), 0.95), Vector3(0, 0.45, 0), Vector3(1.2, 0.7, 1.0))
		"cottage":
			_mesh(root, BoxMesh.new(), _mat(Color(0.95, 0.88, 0.72)), Vector3(0, 1.4, 0), Vector3(4.0, 2.8, 3.4))
			var roof := PrismMesh.new()
			_mesh(root, roof, _mat(Color(0.85, 0.28, 0.2)), Vector3(0, 3.5, 0), Vector3(4.8, 1.8, 4.0))
			_mesh(root, BoxMesh.new(), _mat(Color(0.4, 0.26, 0.12)), Vector3(0, 0.8, 1.72), Vector3(0.9, 1.6, 0.1))
		"fence":
			for i in 3:
				_mesh(root, BoxMesh.new(), _mat(Color(0.7, 0.5, 0.28)), Vector3(-0.8 + i * 0.8, 0.5, 0), Vector3(0.12, 1.0, 0.12))
			_mesh(root, BoxMesh.new(), _mat(Color(0.72, 0.52, 0.3)), Vector3(0, 0.75, 0), Vector3(2.0, 0.12, 0.08))
			_mesh(root, BoxMesh.new(), _mat(Color(0.72, 0.52, 0.3)), Vector3(0, 0.4, 0), Vector3(2.0, 0.12, 0.08))
		"lantern":
			_mesh(root, _cyl(0.06, 0.08, 2.2, 6), _mat(Color(0.25, 0.25, 0.3)), Vector3(0, 1.1, 0))
			_mesh(root, _sphere(0.28, 8), _mat(Color(1.0, 0.85, 0.4), 0.3, 2.5), Vector3(0, 2.3, 0))
			var l := OmniLight3D.new()
			l.light_color = Color(1.0, 0.8, 0.45)
			l.omni_range = 7.0
			l.light_energy = 1.3
			l.position = Vector3(0, 2.3, 0)
			root.add_child(l)
		"crystal":
			_mesh(root, PrismMesh.new(), _mat(Color(0.35, 0.8, 1.0), 0.2, 1.6), Vector3(0, 1.0, 0), Vector3(0.8, 2.0, 0.8))
	return root

static func hero(class_id: String) -> Node3D:
	var real := load_glb("hero_" + class_id.to_lower())
	if real != null:
		return real
	var c: Color = CLASS_COLORS.get(class_id, Color(0.8, 0.8, 0.8))
	var root := Node3D.new()
	var body := Node3D.new()
	body.name = "Body"
	root.add_child(body)
	_mesh(body, CapsuleMesh.new(), _mat(c, 0.6), Vector3(0, 0.9, 0), Vector3(0.7, 0.9, 0.7))
	_mesh(body, _sphere(0.28), _mat(Color(1.0, 0.85, 0.7), 0.7), Vector3(0, 1.85, 0))
	_mesh(body, _cyl(0.0, 0.34, 0.5, 8), _mat(c.darkened(0.25)), Vector3(0, 2.2, 0))  # hat / hood
	_mesh(body, _cyl(0.04, 0.04, 1.5, 6), _mat(Color(0.5, 0.32, 0.15)), Vector3(0.5, 0.95, 0.15))  # staff
	_mesh(body, _sphere(0.12, 8), _mat(Color(1.0, 0.8, 0.3), 0.3, 2.0), Vector3(0.5, 1.75, 0.15))
	return root

static func enemy(ref: String) -> Node3D:
	var real := load_glb("enemy_" + ref.to_lower())
	if real != null:
		return real
	var c: Color = ENEMY_COLORS.get(ref, Color(0.8, 0.3, 0.3))
	var root := Node3D.new()
	var body := Node3D.new()
	body.name = "Body"
	root.add_child(body)
	_mesh(body, _sphere(0.75, 14), _mat(c, 0.5), Vector3(0, 0.75, 0), Vector3(1, 0.85, 1))
	for sx in [-1.0, 1.0]:
		_mesh(body, _sphere(0.16, 8), _mat(Color.WHITE, 0.3), Vector3(0.28 * sx, 1.0, 0.55))
		_mesh(body, _sphere(0.08, 8), _mat(Color(0.1, 0.1, 0.1), 0.3), Vector3(0.28 * sx, 1.0, 0.68))
	return root

static func npc(npc_id := "") -> Node3D:
	var real := load_glb("npc_" + npc_id.to_lower()) if npc_id != "" else null
	if real == null:
		real = load_glb("npc_generic")
	if real != null:
		return real
	var root := Node3D.new()
	var body := Node3D.new()
	body.name = "Body"
	root.add_child(body)
	_mesh(body, CapsuleMesh.new(), _mat(Color(0.95, 0.55, 0.2), 0.7), Vector3(0, 0.9, 0), Vector3(0.7, 0.9, 0.7))
	_mesh(body, _sphere(0.28), _mat(Color(1.0, 0.85, 0.7), 0.7), Vector3(0, 1.85, 0))
	return root

## Dressing models used by View3D's scatter (no 2D counterpart): real GLB or null.
static func dressing(name: String) -> Node3D:
	return load_glb(name)
