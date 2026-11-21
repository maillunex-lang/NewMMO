class_name View3D
extends Node3D
## Lunex Fantasy 3D: renders the (unchanged) 2D logic world as a real-time 3D scene.
## The 2D scene keeps all gameplay (physics, autopath, quest nodes, server wiring); this node mirrors it:
## 2D (x, y) px  →  3D (x/32, 0, y/32) m. It never decides anything and is replaced/removed without touching gameplay.

const PX_PER_M := 32.0
const PITCH_DEG := 48.0
const ZOOM_MIN := 7.0
const ZOOM_MAX := 30.0
const MAP_SIZE := Vector2(2048, 1400)

var valley: Node2D = null
var player: Node2D = null
var camera: Camera3D = null
var sun: DirectionalLight3D = null
var environment: Environment = null
var preset := "high"
var zoom := 13.0

var _fade_items: Array = []        # {meshes, pos, alpha} tall props that can hide the hero
var _fade_t := 0.0
var _views: Dictionary = {}        # instance_id -> {node, view, kind, last, tag}
var _scan_t := 0.0
var _cam_target := Vector3.ZERO
var _t := 0.0
var _name_font_size := 44

func setup(p_valley: Node2D, p_player: Node2D) -> void:
	valley = p_valley
	player = p_player
	preset = GfxQuality.load_preset()
	_build_environment()
	_build_ground()
	_build_paths()
	_build_props()
	_build_camera()
	GfxQuality.apply(preset, get_viewport(), environment, sun)
	valley.visible = false  # 2D visuals off; physics, areas and groups keep working
	_scan_actors()
	if player != null:
		_cam_target = _to3(player.global_position)

static func _to3(p: Vector2) -> Vector3:
	return Vector3(p.x / PX_PER_M, 0.0, p.y / PX_PER_M)

func to2(p: Vector3) -> Vector2:
	return Vector2(p.x * PX_PER_M, p.z * PX_PER_M)

# ---------------------------------------------------------------- world

func _build_environment() -> void:
	var sky_mat := ProceduralSkyMaterial.new()
	sky_mat.sky_top_color = Color(0.22, 0.5, 0.95)
	sky_mat.sky_horizon_color = Color(0.72, 0.86, 1.0)
	sky_mat.ground_horizon_color = Color(0.72, 0.86, 1.0)
	sky_mat.ground_bottom_color = Color(0.35, 0.5, 0.4)
	sky_mat.sun_angle_max = 25.0
	var sky := Sky.new()
	sky.sky_material = sky_mat
	environment = Environment.new()
	environment.background_mode = Environment.BG_SKY
	environment.sky = sky
	environment.ambient_light_source = Environment.AMBIENT_SOURCE_SKY
	environment.ambient_light_energy = 0.55
	environment.tonemap_mode = Environment.TONE_MAPPER_FILMIC
	environment.tonemap_exposure = 0.85
	environment.adjustment_enabled = true
	environment.adjustment_saturation = 1.15      # bright, saturated look
	environment.adjustment_contrast = 1.06
	environment.fog_enabled = true
	environment.fog_light_color = Color(0.78, 0.88, 1.0)
	environment.fog_density = 0.0015
	environment.glow_intensity = 0.6
	environment.glow_bloom = 0.08
	var we := WorldEnvironment.new()
	we.environment = environment
	add_child(we)
	sun = DirectionalLight3D.new()
	sun.rotation_degrees = Vector3(-52.0, -35.0, 0.0)
	sun.light_color = Color(1.0, 0.95, 0.82)
	sun.light_energy = 1.15
	sun.shadow_bias = 0.04
	add_child(sun)

func _ground_material() -> StandardMaterial3D:
	var noise := FastNoiseLite.new()
	noise.frequency = 0.012
	noise.fractal_octaves = 3
	var tex := NoiseTexture2D.new()
	tex.noise = noise
	tex.width = 256
	tex.height = 256
	tex.seamless = true
	var grad := Gradient.new()
	grad.colors = PackedColorArray([Color(0.16, 0.5, 0.14), Color(0.32, 0.68, 0.2)])
	tex.color_ramp = grad
	var m := StandardMaterial3D.new()
	m.albedo_texture = tex
	m.uv1_triplanar = true
	m.uv1_scale = Vector3(0.05, 0.05, 0.05)
	m.roughness = 0.95
	return m

func _build_ground() -> void:
	var plane := PlaneMesh.new()
	plane.size = Vector2(MAP_SIZE.x / PX_PER_M + 60.0, MAP_SIZE.y / PX_PER_M + 60.0)
	var mi := MeshInstance3D.new()
	mi.mesh = plane
	mi.material_override = _ground_material()
	mi.position = Vector3(MAP_SIZE.x / PX_PER_M / 2.0, 0, MAP_SIZE.y / PX_PER_M / 2.0)
	add_child(mi)

## Dirt roads: ribbons along the generated/manual Line2D paths.
func _build_paths() -> void:
	var mat := StandardMaterial3D.new()
	mat.albedo_color = Color(0.82, 0.62, 0.36)
	mat.roughness = 1.0
	var st := SurfaceTool.new()
	st.begin(Mesh.PRIMITIVE_TRIANGLES)
	var any := false
	for n in valley.find_children("*", "Line2D", true, false):
		var line := n as Line2D
		var pts: PackedVector2Array = line.points
		if pts.size() < 2:
			continue
		var half := maxf(0.7, line.width / PX_PER_M / 2.0)
		var left: Array[Vector3] = []
		var right: Array[Vector3] = []
		for i in range(pts.size()):
			var a := _to3(line.to_global(pts[i]))
			var b := _to3(line.to_global(pts[min(i + 1, pts.size() - 1)]))
			var prev := _to3(line.to_global(pts[max(i - 1, 0)]))
			var dir := (b - prev)
			dir.y = 0
			if dir.length() < 0.001:
				dir = Vector3.RIGHT
			dir = dir.normalized()
			var nrm := Vector3(-dir.z, 0, dir.x) * half
			left.append(a + nrm + Vector3(0, 0.03, 0))
			right.append(a - nrm + Vector3(0, 0.03, 0))
		for i in range(left.size() - 1):
			for v in [left[i], right[i], left[i + 1], right[i], right[i + 1], left[i + 1]]:
				st.set_normal(Vector3.UP)
				st.add_vertex(v)
			any = true
	if any:
		var mi := MeshInstance3D.new()
		mi.mesh = st.commit()
		mi.material_override = mat
		add_child(mi)

func _build_props() -> void:
	var rng := RandomNumberGenerator.new()
	rng.seed = 7
	var density: float = float(GfxQuality.settings(preset)["vegetation"])
	for n in valley.find_children("*", "StaticBody2D", true, false):
		var sf := str(n.scene_file_path)
		if sf == "":
			continue
		var is_tree := sf.get_file().begins_with("tree")
		if is_tree and density < 1.0 and rng.randf() > density:
			continue
		var model := ModelLibrary.prop(sf)
		if model == null:
			continue
		var p := (n as Node2D).global_position
		model.position = _to3(p)
		model.rotation.y = rng.randf() * TAU if is_tree or sf.get_file() == "rock.tscn" else 0.0
		var s := rng.randf_range(0.9, 1.25) if is_tree else 1.0
		model.scale = Vector3(s, s, s)
		add_child(model)
		if is_tree or sf.get_file() == "cottage.tscn":
			_fade_items.append({"meshes": model.find_children("*", "MeshInstance3D", true, false), "pos": model.position, "alpha": 0.0})

func _build_camera() -> void:
	camera = Camera3D.new()
	camera.fov = 45.0
	camera.near = 0.3
	camera.far = 260.0
	camera.current = true
	add_child(camera)

# ---------------------------------------------------------------- actors

func _scan_actors() -> void:
	var groups := {"player": "hero", "npcs": "npc", "monsters": "enemy", "server_enemies": "enemy", "server_players": "hero", "interactables": "poi"}
	var seen := {}
	for g in groups.keys():
		for n in get_tree().get_nodes_in_group(g):
			var id: int = n.get_instance_id()
			if seen.has(id):
				continue
			seen[id] = true
			if not _views.has(id) and is_instance_valid(n):
				_make_view(n, groups[g])
	for id in _views.keys():
		if not seen.has(id) or not is_instance_valid(_views[id]["node"]):
			_views[id]["view"].queue_free()
			_views.erase(id)

func _make_view(n: Node2D, kind: String) -> void:
	var view: Node3D = null
	var label := ""
	match kind:
		"hero":
			view = ModelLibrary.hero(_class_of(n))
		"npc":
			view = ModelLibrary.npc()
			label = str(n.get("display_name"))
		"enemy":
			view = ModelLibrary.enemy(str(n.get("ref")) if n.get("ref") != null else _ref_of_local_monster(n))
			label = str(n.get("display_name")) if n.get("display_name") != null else ""
		"poi":
			view = _poi_marker()
			label = tr(str(n.get("display_name")))
	if view == null:
		return
	add_child(view)
	var tag := Label3D.new()
	tag.text = label
	tag.font_size = _name_font_size
	tag.outline_size = 14
	tag.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	tag.no_depth_test = true
	tag.pixel_size = 0.006
	tag.position = Vector3(0, 2.7 if kind != "poi" else 1.6, 0)
	tag.visible = label != "" and kind != "poi"
	view.add_child(tag)
	var bar: MeshInstance3D = null
	if kind == "enemy":
		bar = _hp_bar()
		bar.position = Vector3(0, 2.4, 0)
		view.add_child(bar)
	_views[n.get_instance_id()] = {"node": n, "view": view, "kind": kind, "last": view.position, "tag": tag, "bar": bar, "bob": 0.0}
	view.position = _to3(n.global_position)

func _class_of(n: Node) -> String:
	if n.has_meta("class_art"):
		return str(n.get_meta("class_art"))
	var wc := get_tree().get_first_node_in_group("world_controller")
	if n.is_in_group("player") and wc != null:
		return str(wc.get("_class_id"))
	return "CLS-01"

func _ref_of_local_monster(n: Node) -> String:
	match str(n.get("monster_id")):
		"bandit": return "ENM-01-001"
		"goblin": return "ENM-01-002"
		"wolf": return "ENM-01-003"
		_: return "ENM-01-004"

func _poi_marker() -> Node3D:
	var root := Node3D.new()
	var ring := MeshInstance3D.new()
	var tor := TorusMesh.new()
	tor.inner_radius = 0.28
	tor.outer_radius = 0.4
	ring.mesh = tor
	var m := StandardMaterial3D.new()
	m.albedo_color = Color(0.4, 0.95, 1.0)
	m.emission_enabled = true
	m.emission = Color(0.3, 0.9, 1.0)
	m.emission_energy_multiplier = 2.0
	ring.material_override = m
	ring.position = Vector3(0, 0.08, 0)
	root.add_child(ring)
	return root

func _hp_bar() -> MeshInstance3D:
	var mi := MeshInstance3D.new()
	var q := QuadMesh.new()
	q.size = Vector2(1.2, 0.14)
	mi.mesh = q
	var m := StandardMaterial3D.new()
	m.albedo_color = Color(0.9, 0.2, 0.2)
	m.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	m.billboard_mode = BaseMaterial3D.BILLBOARD_ENABLED
	m.no_depth_test = true
	mi.material_override = m
	return mi

func _process(delta: float) -> void:
	_t += delta
	_scan_t -= delta
	if _scan_t <= 0.0:
		_scan_t = 0.25
		_scan_actors()
	_sync_actors(delta)
	_update_camera(delta)
	_update_fades(delta)

func _sync_actors(delta: float) -> void:
	var pp := player.global_position if player != null else Vector2.ZERO
	# closest interactable name
	var closest_id := -1
	var closest_d := 4.0 * PX_PER_M
	for id in _views.keys():
		var e: Dictionary = _views[id]
		if e["kind"] == "poi" and is_instance_valid(e["node"]):
			var d: float = (e["node"] as Node2D).global_position.distance_to(pp)
			if d < closest_d:
				closest_d = d
				closest_id = id
	for id in _views.keys():
		var e: Dictionary = _views[id]
		if not is_instance_valid(e["node"]):
			continue
		var n: Node2D = e["node"]
		var v: Node3D = e["view"]
		var target := _to3(n.global_position)
		var delta_pos := target - v.position
		v.position = target
		var moving := delta_pos.length() > 0.004
		var kind: String = e["kind"]
		if kind == "poi":
			(e["tag"] as Label3D).visible = id == closest_id
			v.rotation.y += delta * 1.2
			continue
		# facing from motion, else from the 2D animator's direction index
		if moving:
			v.rotation.y = lerp_angle(v.rotation.y, -(atan2(delta_pos.z, delta_pos.x) + PI / 2.0), 1.0 - exp(-14.0 * delta))
		var body := v.get_node_or_null("Body") as Node3D
		if body != null:
			e["bob"] = float(e["bob"]) + (delta * (11.0 if moving else 2.2))
			body.position.y = absf(sin(float(e["bob"]))) * (0.12 if moving else 0.03)
		# server puppet state
		var dead := bool(n.get("dead")) if n.get("dead") != null else false
		v.visible = not dead
		var bar := e["bar"] as MeshInstance3D
		if bar != null and n.get("max_hp") != null:
			var ratio := clampf(float(n.get("hp")) / maxf(1.0, float(n.get("max_hp"))), 0.0, 1.0)
			bar.scale = Vector3(maxf(ratio, 0.001), 1, 1)
		if kind == "hero" and n.has_method("is_attacking_now"):
			pass

func _update_camera(delta: float) -> void:
	if camera == null:
		return
	if player != null:
		var goal := _to3(player.global_position)
		_cam_target = _cam_target.lerp(goal, 1.0 - exp(-8.0 * delta))
	var pitch := deg_to_rad(PITCH_DEG)
	var off := Vector3(0, sin(pitch), cos(pitch)) * zoom
	camera.global_position = _cam_target + Vector3(0, 1.0, 0) + off
	camera.look_at(_cam_target + Vector3(0, 1.0, 0), Vector3.UP)

## Tall props between the camera and the hero fade out so the hero is never hidden by a canopy.
func _update_fades(delta: float) -> void:
	if camera == null or player == null:
		return
	var hero := _to3(player.global_position)
	var cam_dir := Vector3(camera.global_position.x - hero.x, 0, camera.global_position.z - hero.z)
	if cam_dir.length() < 0.01:
		return
	cam_dir = cam_dir.normalized()
	for it in _fade_items:
		var rel: Vector3 = (it["pos"] as Vector3) - hero
		rel.y = 0
		var along := rel.dot(cam_dir)
		var side := (rel - cam_dir * along).length()
		var want := 0.0
		if along > -1.0 and along < 14.0 and side < 3.6:
			want = 0.82
		var a: float = lerpf(float(it["alpha"]), want, 1.0 - exp(-9.0 * delta))
		if absf(a - float(it["alpha"])) > 0.004:
			it["alpha"] = a
			for m in it["meshes"]:
				if is_instance_valid(m):
					(m as GeometryInstance3D).transparency = a

# ---------------------------------------------------------------- input / API

func set_zoom(z: float) -> void:
	zoom = clampf(z, ZOOM_MIN, ZOOM_MAX)

func _input(event: InputEvent) -> void:
	if event is InputEventMouseButton and event.pressed:
		if event.button_index == MOUSE_BUTTON_WHEEL_UP:
			set_zoom(zoom - 1.2)
		elif event.button_index == MOUSE_BUTTON_WHEEL_DOWN:
			set_zoom(zoom + 1.2)
	elif event is InputEventMagnifyGesture:
		set_zoom(zoom / maxf(0.2, event.factor))

## Screen position → 2D world position on the ground plane (y=0).
func pick_ground(screen_pos: Vector2) -> Vector2:
	if camera == null:
		return Vector2.ZERO
	var from := camera.project_ray_origin(screen_pos)
	var dir := camera.project_ray_normal(screen_pos)
	if absf(dir.y) < 0.0001:
		return Vector2.ZERO
	var t := -from.y / dir.y
	return to2(from + dir * t)

func set_preset(p: String) -> void:
	if not GfxQuality.DATA.has(p):
		return
	preset = p
	GfxQuality.save_preset(p)
	GfxQuality.apply(p, get_viewport(), environment, sun)
