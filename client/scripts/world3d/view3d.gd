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
const CHAR_SCALE := 1.3   # characters read larger than life, like most MMOs

var valley: Node2D = null
var player: Node2D = null
var camera: Camera3D = null
var sun: DirectionalLight3D = null
var environment: Environment = null
var preset := "high"
var zoom := 10.5
var fx: Fx3D = null
var grass: Grass = null
var _road_tex: Texture2D = null

var _static_nodes: Array = []      # props / dressing (freed on model reload)
var _road_segs: Array = []         # [Vector2 a, Vector2 b] in metres
var _fade_items: Array = []        # {meshes, pos, alpha} tall props that can hide the hero
var _fade_t := 0.0
var _movers: Array = []
var _views: Dictionary = {}        # instance_id -> {node, view, kind, last, tag}
var _scan_t := 0.0
var _cam_target := Vector3.ZERO
var _t := 0.0
var _name_font_size := 24

func setup(p_valley: Node2D, p_player: Node2D) -> void:
	valley = p_valley
	player = p_player
	preset = GfxQuality.load_preset()
	_build_environment()
	_build_road_mask()
	_build_ground()
	_collect_road_segments()
	_build_props()
	_build_dressing()
	grass = Grass.new()
	grass.name = "Grass"
	add_child(grass)
	if OS.get_environment("NEWMMO_GRASS") != "0":
		grass.setup(preset, _road_tex, MAP_SIZE / PX_PER_M)
	add_to_group("view3d")
	var packs := get_node_or_null("/root/AssetPacks")
	if packs != null and packs.has_signal("packs_ready"):
		packs.packs_ready.connect(reload_models)
	_build_camera()
	fx = Fx3D.new()
	fx.name = "Fx3D"
	fx.particle_scale = float(GfxQuality.settings(preset)["particles"])
	add_child(fx)
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
	environment.ambient_light_energy = 0.85
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
	sun.shadow_opacity = 0.55
	add_child(sun)

func _noise_tex() -> NoiseTexture2D:
	var noise := FastNoiseLite.new()
	noise.noise_type = FastNoiseLite.TYPE_SIMPLEX_SMOOTH
	noise.frequency = 0.02
	noise.fractal_octaves = 3
	var tex := NoiseTexture2D.new()
	tex.noise = noise
	tex.width = 512
	tex.height = 512
	tex.seamless = true
	return tex

func _ground_material() -> ShaderMaterial:
	var m := ShaderMaterial.new()
	m.shader = load("res://shaders/ground.gdshader")
	m.set_shader_parameter("noise_tex", _noise_tex())
	m.set_shader_parameter("road_mask", _road_tex)
	m.set_shader_parameter("map_size", MAP_SIZE / PX_PER_M)
	return m

func _build_ground() -> void:
	var plane := PlaneMesh.new()
	plane.size = Vector2(MAP_SIZE.x / PX_PER_M + 60.0, MAP_SIZE.y / PX_PER_M + 60.0)
	var mi := MeshInstance3D.new()
	mi.mesh = plane
	mi.material_override = _ground_material()
	mi.position = Vector3(MAP_SIZE.x / PX_PER_M / 2.0, 0, MAP_SIZE.y / PX_PER_M / 2.0)
	add_child(mi)

## Road mask: the location's Line2D paths drawn white-on-black into a small viewport (8 px per metre). The ground
## shader turns it into a dirt road and the grass shader keeps blades off it.
func _build_road_mask() -> void:
	var px_per_m := 8.0
	var vp := SubViewport.new()
	vp.size = Vector2i(int(MAP_SIZE.x / PX_PER_M * px_per_m), int(MAP_SIZE.y / PX_PER_M * px_per_m))
	vp.disable_3d = true
	vp.render_target_update_mode = SubViewport.UPDATE_ONCE
	var bg := ColorRect.new()
	bg.color = Color.BLACK
	bg.size = Vector2(vp.size)
	vp.add_child(bg)
	var k := px_per_m / PX_PER_M
	for n in valley.find_children("*", "Line2D", true, false):
		var line := n as Line2D
		if line.points.size() < 2:
			continue
		for pass_i in 2:
			var c := Line2D.new()
			var pts := PackedVector2Array()
			for p in line.points:
				pts.append(line.to_global(p) * k)
			c.points = pts
			c.width = (3.9 if pass_i == 0 else 2.7) * px_per_m
			c.default_color = Color(0.5, 0.5, 0.5) if pass_i == 0 else Color.WHITE
			c.joint_mode = Line2D.LINE_JOINT_ROUND
			c.begin_cap_mode = Line2D.LINE_CAP_ROUND
			c.end_cap_mode = Line2D.LINE_CAP_ROUND
			c.antialiased = true
			vp.add_child(c)
	add_child(vp)
	_road_tex = vp.get_texture()

func _collect_road_segments() -> void:
	_road_segs.clear()
	for n in valley.find_children("*", "Line2D", true, false):
		var line := n as Line2D
		for i in range(line.points.size() - 1):
			_road_segs.append([line.to_global(line.points[i]) / PX_PER_M, line.to_global(line.points[i + 1]) / PX_PER_M])

func _road_dist(p: Vector2) -> float:
	var best := 1e9
	for seg in _road_segs:
		var a: Vector2 = seg[0]
		var b: Vector2 = seg[1]
		var ab := b - a
		var t := clampf((p - a).dot(ab) / maxf(ab.length_squared(), 0.0001), 0.0, 1.0)
		best = minf(best, p.distance_to(a + ab * t))
	return best

## Variety: wild flowers, mushrooms, bushes and stumps scattered off the roads, plus a well/barrels/haystack by the cottage.
func _build_dressing() -> void:
	var d: float = float(GfxQuality.settings(preset)["vegetation"])
	var rng := RandomNumberGenerator.new()
	rng.seed = 99
	var blockers: Array = []
	for n in valley.find_children("*", "StaticBody2D", true, false):
		blockers.append((n as Node2D).global_position / PX_PER_M)
	var map_m := MAP_SIZE / PX_PER_M
	for spec in [["flowers", 170, 0.9], ["mushrooms", 36, 1.0], ["bush", 70, 1.2], ["stump", 12, 1.0]]:
		var placed := 0
		var tries := 0
		var want := int(float(spec[1]) * d)
		while placed < want and tries < want * 12:
			tries += 1
			var p := Vector2(rng.randf_range(2.0, map_m.x - 2.0), rng.randf_range(2.0, map_m.y - 2.0))
			if _road_dist(p) < 2.4:
				continue
			var near := false
			for b in blockers:
				if (b as Vector2).distance_to(p) < 1.8:
					near = true
					break
			if near:
				continue
			var m := ModelLibrary.dressing(str(spec[0]))
			if m == null:
				break
			m.position = Vector3(p.x, 0, p.y)
			m.rotation.y = rng.randf() * TAU
			var sc: float = float(spec[2]) * rng.randf_range(0.75, 1.3)
			m.scale = Vector3(sc, sc, sc)
			add_child(m)
			_static_nodes.append(m)
			placed += 1
	# cottage yard
	for n in valley.find_children("*", "StaticBody2D", true, false):
		if str(n.scene_file_path).get_file() == "cottage.tscn":
			var c := (n as Node2D).global_position / PX_PER_M
			for item in [["well", Vector2(-6.0, 1.2), 0.0], ["barrel", Vector2(-3.6, 2.6), 0.4], ["barrel", Vector2(-4.2, 3.2), 1.2], ["haystack", Vector2(5.6, -1.6), 0.8]]:
				var m := ModelLibrary.dressing(str(item[0]))
				if m != null:
					m.position = Vector3(c.x + (item[1] as Vector2).x, 0, c.y + (item[1] as Vector2).y)
					m.rotation.y = float(item[2])
					add_child(m)
					_static_nodes.append(m)

func reload_models() -> void:
	for n in _static_nodes:
		if is_instance_valid(n):
			n.queue_free()
	_static_nodes.clear()
	_fade_items.clear()
	for id in _views.keys():
		_views[id]["view"].queue_free()
	_views.clear()
	_build_props()
	_build_dressing()
	_scan_actors()

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
		var model: Node3D = null
		if sf.get_file() == "tree.tscn" and rng.randf() < 0.3:
			model = ModelLibrary.dressing("tree_birch")  # variety: birches among the oaks
		if model == null:
			model = ModelLibrary.prop(sf)
		if model == null:
			continue
		var p := (n as Node2D).global_position
		model.position = _to3(p)
		model.rotation.y = rng.randf() * TAU if is_tree or sf.get_file() == "rock.tscn" else 0.0
		var s := rng.randf_range(0.9, 1.25) if is_tree else 1.0
		model.scale = Vector3(s, s, s)
		add_child(model)
		_static_nodes.append(model)
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
			view = ModelLibrary.npc(str(n.get("npc_id")) if n.get("npc_id") != null else "")
			label = str(n.get("display_name"))
		"enemy":
			view = ModelLibrary.enemy(str(n.get("ref")) if n.get("ref") != null else _ref_of_local_monster(n))
			label = str(n.get("display_name")) if n.get("display_name") != null else ""
		"poi":
			view = _poi_marker()
			label = tr(str(n.get("display_name")))
	if view == null:
		return
	if kind != "poi":
		view.scale = Vector3.ONE * CHAR_SCALE
	add_child(view)
	var tag := Label3D.new()
	tag.text = label
	tag.font_size = _name_font_size
	tag.outline_size = 10
	tag.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	tag.no_depth_test = true
	tag.pixel_size = 0.006
	tag.position = Vector3(0, 2.7 if kind != "poi" else 1.6, 0) / (CHAR_SCALE if kind != "poi" else 1.0)
	tag.visible = label != "" and kind != "poi"
	view.add_child(tag)
	var bar: MeshInstance3D = null
	if kind == "enemy":
		bar = _hp_bar()
		bar.position = Vector3(0, 2.4, 0)
		view.add_child(bar)
	var ap := view.find_child("AnimationPlayer", true, false) as AnimationPlayer
	if ap != null and not ap.has_meta("humanoid"):
		ap = null
	_views[n.get_instance_id()] = {"node": n, "view": view, "kind": kind, "last": view.position, "tag": tag, "bar": bar, "bob": 0.0,
		"ap": ap, "anim": "", "lock": 0.0, "dead_t": 0.0}
	if ap != null:
		_play(_views[n.get_instance_id()], "idle", 1.0)
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
	q.size = Vector2(0.95, 0.075)
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
	if grass != null:
		grass.tick(delta, _cam_target, _movers)

func _sync_actors(delta: float) -> void:
	var movers: Array = []
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
		if moving and (kind == "hero" or kind == "enemy"):
			movers.append([id, Vector2(target.x, target.z)])
		if kind == "poi":
			(e["tag"] as Label3D).visible = id == closest_id
			v.rotation.y += delta * 1.2
			continue
		# facing (+Z is the model's front): from motion, else from the 2D node's 8-way direction index
		var yaw := v.rotation.y
		if moving:
			yaw = atan2(delta_pos.x, delta_pos.z)
		elif n.get("animator") != null and n.get("animator").get("_dir") != null:
			yaw = PI / 2.0 - float(n.get("animator").get("_dir")) * PI / 4.0
		v.rotation.y = lerp_angle(v.rotation.y, yaw, 1.0 - exp(-14.0 * delta))
		var speed := delta_pos.length() / maxf(delta, 0.001)
		var dead_now := bool(n.get("dead")) if n.get("dead") != null else bool(n.get("online_dead")) if n.get("online_dead") != null else false
		if e["ap"] != null:
			_drive_humanoid(e, moving, speed, dead_now, delta)
		var body := v.get_node_or_null("Body") as Node3D
		if body != null and e["ap"] == null and kind == "enemy" and str(n.get("ref")) == "ENM-01-004":
			e["bob"] = float(e["bob"]) + delta * (9.0 if moving else 3.0)
			var q := sin(float(e["bob"]))
			body.scale = Vector3(1.0 - 0.07 * q, 1.0 + 0.12 * q, 1.0 - 0.07 * q)
			body.position.y = maxf(0.0, q) * (0.25 if moving else 0.04)
		elif body != null and e["ap"] == null:
			e["bob"] = float(e["bob"]) + (delta * (11.0 if moving else 2.2))
			body.position.y = absf(sin(float(e["bob"]))) * (0.12 if moving else 0.03)
		# server puppet state
		var dead := bool(n.get("dead")) if n.get("dead") != null else false
		v.visible = not dead or (e["ap"] != null and float(e["dead_t"]) < 2.6)
		var bar := e["bar"] as MeshInstance3D
		if bar != null and n.get("max_hp") != null:
			var ratio := clampf(float(n.get("hp")) / maxf(1.0, float(n.get("max_hp"))), 0.0, 1.0)
			bar.scale = Vector3(maxf(ratio, 0.001), 1, 1)
		if kind == "hero" and n.has_method("is_attacking_now"):
			pass
	_movers = movers

func _play(e: Dictionary, anim: String, speed: float) -> void:
	var ap := e["ap"] as AnimationPlayer
	if ap == null or not ap.has_animation(anim):
		return
	if e["anim"] != anim:
		ap.play(anim, 0.15, speed)
		e["anim"] = anim

func _drive_humanoid(e: Dictionary, moving: bool, speed: float, dead_now: bool, delta: float) -> void:
	var now := Time.get_ticks_msec() / 1000.0
	if dead_now:
		e["dead_t"] = float(e["dead_t"]) + delta
		_play(e, "dead", 1.6)
		return
	e["dead_t"] = 0.0
	if e["anim"] == "dead":
		e["anim"] = ""
	if now < float(e["lock"]):
		return
	if moving:
		_play(e, "run" if speed > 3.6 else "walk", clampf(speed / 4.5, 0.7, 1.4))
	else:
		_play(e, "idle", 1.0)

## One-shot action clips (cast / attack / hit) fired by combat events; the state machine resumes afterwards.
func play_action(node: Node2D, anim: String) -> void:
	if node == null or not _views.has(node.get_instance_id()):
		return
	var e: Dictionary = _views[node.get_instance_id()]
	if e["ap"] == null or bool(e["dead_t"] > 0.0):
		return
	var ap := e["ap"] as AnimationPlayer
	if not ap.has_animation(anim):
		return
	var speed := 2.4 if anim != "hit" else 2.6
	e["anim"] = ""
	ap.play(anim, 0.08, speed)
	e["anim"] = anim
	e["lock"] = Time.get_ticks_msec() / 1000.0 + ap.get_animation(anim).length / speed * (0.55 if anim != "hit" else 0.4)

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
	if fx != null:
		fx.particle_scale = float(GfxQuality.settings(p)["particles"])
	GfxQuality.save_preset(p)
	GfxQuality.apply(p, get_viewport(), environment, sun)
