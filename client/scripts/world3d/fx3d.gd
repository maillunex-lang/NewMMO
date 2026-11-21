class_name Fx3D
extends Node3D
## Lunex Fantasy 3D spell & combat effects. Pure presentation, built from GPU particles + glowing meshes (no textures to
## download): cast runes, projectiles with trails, impact bursts + shockwave rings, heal sparkles, shield bubble, ground
## telegraphs and floating damage numbers. Particle amounts scale with the graphics preset (`GfxQuality`).
## world_controller → group "fx3d" → on_event(type, caster_node2d, target_node2d, event). 2D px → 3D m is /32.

const PX := 32.0
var particle_scale := 1.0
var _glow: Texture2D = null
var _rune: Texture2D = null
var _skills: Dictionary = {}

const STYLE_COLORS := {
	"arcane": [Color(0.7, 0.45, 1.0), Color(0.35, 0.7, 1.0)],
	"fire": [Color(1.0, 0.55, 0.15), Color(1.0, 0.9, 0.35)],
	"ice": [Color(0.45, 0.85, 1.0), Color(0.9, 1.0, 1.0)],
	"lightning": [Color(1.0, 0.95, 0.4), Color(0.6, 0.8, 1.0)],
	"nature": [Color(0.5, 1.0, 0.4), Color(0.9, 1.0, 0.5)],
	"holy": [Color(1.0, 0.9, 0.5), Color(1.0, 1.0, 0.85)],
	"physical": [Color(1.0, 0.85, 0.7), Color(1.0, 0.45, 0.25)],
	"blood": [Color(1.0, 0.25, 0.25), Color(1.0, 0.6, 0.4)],
}
const CLASS_STYLE := {"CLS-01": "physical", "CLS-02": "blood", "CLS-03": "arcane", "CLS-04": "nature", "CLS-05": "holy"}
const ELEMENT_WORDS := [
	[["огн", "пламя", "метеор", "жар", "печать"], "fire"], [["лед", "холод", "мороз"], "ice"],
	[["молни", "шторм", "гром"], "lightning"], [["яд", "корн", "лоз", "шип"], "nature"],
]

func _ready() -> void:
	add_to_group("fx3d")
	_glow = _make_glow()
	_rune = _make_rune()
	var f := FileAccess.open("res://data/skills.json", FileAccess.READ)
	if f != null:
		var parsed: Variant = JSON.parse_string(f.get_as_text())
		if typeof(parsed) == TYPE_DICTIONARY:
			for cls in (parsed as Dictionary).get("classes", {}).keys():
				for s in parsed["classes"][cls]:
					_skills[str(s.get("id", ""))] = s

func style_for(skill_id: String) -> String:
	var s: Dictionary = _skills.get(skill_id, {})
	var name := str(s.get("name", "")).to_lower()
	for pair in ELEMENT_WORDS:
		for w in pair[0]:
			if name.contains(w):
				return pair[1]
	return CLASS_STYLE.get(skill_id.substr(0, 6), "arcane")

# ----------------------------------------------------------------- procedural textures

func _make_glow() -> Texture2D:
	var g := Gradient.new()
	g.colors = PackedColorArray([Color(1, 1, 1, 1), Color(1, 1, 1, 0.55), Color(1, 1, 1, 0)])
	g.offsets = PackedFloat32Array([0.0, 0.35, 1.0])
	var t := GradientTexture2D.new()
	t.gradient = g
	t.fill = GradientTexture2D.FILL_RADIAL
	t.fill_from = Vector2(0.5, 0.5)
	t.fill_to = Vector2(1.0, 0.5)
	t.width = 128
	t.height = 128
	return t

func _make_rune() -> Texture2D:
	var n := 256
	var img := Image.create(n, n, false, Image.FORMAT_RGBA8)
	var c := Vector2(n / 2.0, n / 2.0)
	for y in n:
		for x in n:
			var d := Vector2(x, y).distance_to(c) / (n / 2.0)
			var a := 0.0
			for ring in [[0.96, 0.025], [0.80, 0.02], [0.52, 0.018]]:
				a = maxf(a, 1.0 - clampf(absf(d - ring[0]) / ring[1], 0.0, 1.0))
			var ang := atan2(y - c.y, x - c.x)
			# 8 rune ticks between the outer rings and a star inside
			var tick := absf(fposmod(ang, TAU / 8.0) - TAU / 16.0) < 0.05 and d > 0.52 and d < 0.8
			if tick:
				a = maxf(a, 0.9)
			var star := absf(fposmod(ang, TAU / 5.0) - TAU / 10.0) < 0.035 and d < 0.5
			if star:
				a = maxf(a, 0.85)
			img.set_pixel(x, y, Color(1, 1, 1, clampf(a, 0.0, 1.0)))
	return ImageTexture.create_from_image(img)

# ----------------------------------------------------------------- helpers

func _additive(color: Color, tex: Texture2D = null, energy := 2.0) -> StandardMaterial3D:
	var m := StandardMaterial3D.new()
	m.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	m.blend_mode = BaseMaterial3D.BLEND_MODE_ADD
	m.albedo_color = Color(color.r * energy, color.g * energy, color.b * energy, 1.0)
	m.vertex_color_use_as_albedo = true
	m.billboard_mode = BaseMaterial3D.BILLBOARD_PARTICLES
	m.billboard_keep_scale = true
	m.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	m.no_depth_test = false
	if tex != null:
		m.albedo_texture = tex
	return m

func _particles(pos: Vector3, color: Color, amount: int, life: float, one_shot := true) -> GPUParticles3D:
	var p := GPUParticles3D.new()
	p.amount = maxi(4, int(amount * particle_scale))
	p.lifetime = life
	p.one_shot = one_shot
	p.explosiveness = 0.9 if one_shot else 0.0
	p.local_coords = false
	p.visibility_aabb = AABB(Vector3(-6, -3, -6), Vector3(12, 8, 12))
	var q := QuadMesh.new()
	q.size = Vector2(0.34, 0.34)
	q.material = _additive(color, _glow, 1.25)
	p.draw_pass_1 = q
	p.position = pos
	add_child(p)
	return p

func _free_later(n: Node, secs: float) -> void:
	get_tree().create_timer(secs).timeout.connect(func(): if is_instance_valid(n): n.queue_free())

func _col(style: String, i := 0) -> Color:
	return (STYLE_COLORS.get(style, STYLE_COLORS["arcane"]) as Array)[i]

func _to3(p: Vector2, h := 0.0) -> Vector3:
	return Vector3(p.x / PX, h, p.y / PX)

# ----------------------------------------------------------------- effects

## Burst of glowing sparks + expanding shock ring + short light flash.
func impact(pos: Vector3, style: String, crit := false) -> void:
	var c := _col(style, 0)
	var p := _particles(pos, c, 46 if crit else 28, 0.7)
	var pm := ParticleProcessMaterial.new()
	pm.direction = Vector3.UP
	pm.spread = 180.0
	pm.initial_velocity_min = 2.5
	pm.initial_velocity_max = 7.5 if crit else 5.5
	pm.gravity = Vector3(0, -6.0, 0)
	pm.damping_min = 2.0
	pm.damping_max = 4.0
	pm.scale_min = 0.5
	pm.scale_max = 1.6 if crit else 1.1
	var ramp := Gradient.new()
	ramp.colors = PackedColorArray([_col(style, 1), c, Color(c.r, c.g, c.b, 0)])
	var rt := GradientTexture1D.new()
	rt.gradient = ramp
	pm.color_ramp = rt
	p.process_material = pm
	p.emitting = true
	ring(pos + Vector3(0, 0.05, 0), c, 2.6 if crit else 1.8, 0.45)
	flash(pos, _col(style, 1), 3.5 if crit else 2.2, 0.18)
	_free_later(p, 1.2)

func ring(pos: Vector3, color: Color, radius: float, secs: float) -> void:
	var mi := MeshInstance3D.new()
	var q := QuadMesh.new()
	q.size = Vector2(1, 1)
	q.orientation = PlaneMesh.FACE_Y
	mi.mesh = q
	var m := _additive(color, _rune, 1.8)
	m.billboard_mode = BaseMaterial3D.BILLBOARD_DISABLED
	mi.material_override = m
	mi.position = pos
	mi.scale = Vector3.ONE * 0.2
	add_child(mi)
	var tw := create_tween()
	tw.tween_property(mi, "scale", Vector3.ONE * radius * 2.0, secs).set_ease(Tween.EASE_OUT).set_trans(Tween.TRANS_CUBIC)
	tw.parallel().tween_property(m, "albedo_color:a", 0.0, secs)
	tw.tween_callback(mi.queue_free)

func flash(pos: Vector3, color: Color, energy: float, secs: float) -> void:
	var l := OmniLight3D.new()
	l.light_color = color
	l.light_energy = energy
	l.omni_range = 6.0
	l.position = pos
	add_child(l)
	var tw := create_tween()
	tw.tween_property(l, "light_energy", 0.0, secs)
	tw.tween_callback(l.queue_free)

## Rotating rune circle under the caster + rising swirl.
func cast_circle(pos: Vector3, style: String, secs := 0.55) -> void:
	var c := _col(style, 0)
	var mi := MeshInstance3D.new()
	var q := QuadMesh.new()
	q.size = Vector2(2.6, 2.6)
	q.orientation = PlaneMesh.FACE_Y
	mi.mesh = q
	var m := _additive(c, _rune, 2.2)
	m.billboard_mode = BaseMaterial3D.BILLBOARD_DISABLED
	mi.material_override = m
	mi.position = pos + Vector3(0, 0.06, 0)
	mi.scale = Vector3.ONE * 0.3
	add_child(mi)
	var tw := create_tween()
	tw.tween_property(mi, "scale", Vector3.ONE, 0.18).set_ease(Tween.EASE_OUT).set_trans(Tween.TRANS_BACK)
	tw.parallel().tween_property(mi, "rotation:y", 2.2, secs)
	tw.tween_property(m, "albedo_color:a", 0.0, 0.22)
	tw.tween_callback(mi.queue_free)
	var p := _particles(pos + Vector3(0, 0.2, 0), c, 22, 0.8)
	var pm := ParticleProcessMaterial.new()
	pm.emission_shape = ParticleProcessMaterial.EMISSION_SHAPE_RING
	pm.emission_ring_radius = 1.0
	pm.emission_ring_inner_radius = 0.8
	pm.emission_ring_height = 0.1
	pm.emission_ring_axis = Vector3.UP
	pm.direction = Vector3.UP
	pm.spread = 8.0
	pm.initial_velocity_min = 1.5
	pm.initial_velocity_max = 3.0
	pm.gravity = Vector3.ZERO
	pm.scale_min = 0.3
	pm.scale_max = 0.7
	p.process_material = pm
	p.emitting = true
	_free_later(p, 1.4)

## Glowing orb with a particle trail flying from `from` to `to`; calls `on_hit` on arrival.
func projectile(from: Vector3, to: Vector3, style: String, on_hit: Callable) -> void:
	var root := Node3D.new()
	root.position = from
	add_child(root)
	var orb := MeshInstance3D.new()
	var sm := SphereMesh.new()
	sm.radius = 0.2
	sm.height = 0.4
	orb.mesh = sm
	var om := StandardMaterial3D.new()
	om.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	om.albedo_color = _col(style, 1) * 2.0
	orb.material_override = om
	root.add_child(orb)
	var l := OmniLight3D.new()
	l.light_color = _col(style, 0)
	l.light_energy = 2.0
	l.omni_range = 5.0
	root.add_child(l)
	var trail := _particles(Vector3.ZERO, _col(style, 0), 36, 0.45, false)
	remove_child(trail)
	root.add_child(trail)
	trail.local_coords = false
	var pm := ParticleProcessMaterial.new()
	pm.direction = Vector3.UP
	pm.spread = 40.0
	pm.initial_velocity_min = 0.2
	pm.initial_velocity_max = 1.0
	pm.gravity = Vector3.ZERO
	pm.scale_min = 0.5
	pm.scale_max = 1.0
	var ramp := Gradient.new()
	ramp.colors = PackedColorArray([_col(style, 1), _col(style, 0), Color(0, 0, 0, 0)])
	var rt := GradientTexture1D.new()
	rt.gradient = ramp
	pm.color_ramp = rt
	trail.process_material = pm
	trail.emitting = true
	var dur := clampf(from.distance_to(to) / 22.0, 0.12, 0.5)
	var tw := create_tween()
	tw.tween_property(root, "position", to, dur).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_IN)
	tw.tween_callback(func():
		trail.emitting = false
		orb.visible = false
		on_hit.call())
	tw.tween_interval(0.5)
	tw.tween_callback(root.queue_free)

func slash(pos: Vector3, yaw: float, style: String) -> void:
	var mi := MeshInstance3D.new()
	var q := QuadMesh.new()
	q.size = Vector2(2.4, 2.4)
	mi.mesh = q
	var m := _additive(_col(style, 1), _crescent(), 2.4)
	m.billboard_mode = BaseMaterial3D.BILLBOARD_DISABLED
	mi.material_override = m
	mi.position = pos + Vector3(0, 1.1, 0)
	mi.rotation = Vector3(deg_to_rad(-80), yaw, 0)
	mi.scale = Vector3(0.4, 0.4, 0.4)
	add_child(mi)
	var tw := create_tween()
	tw.tween_property(mi, "scale", Vector3.ONE, 0.14).set_ease(Tween.EASE_OUT)
	tw.parallel().tween_property(m, "albedo_color:a", 0.0, 0.22)
	tw.tween_callback(mi.queue_free)

var _cres: Texture2D = null
func _crescent() -> Texture2D:
	if _cres != null:
		return _cres
	var n := 128
	var img := Image.create(n, n, false, Image.FORMAT_RGBA8)
	var c := Vector2(n * 0.5, n * 0.62)
	for y in n:
		for x in n:
			var d := Vector2(x, y).distance_to(c) / (n * 0.5)
			var d2 := Vector2(x, y).distance_to(c + Vector2(0, n * 0.16)) / (n * 0.5)
			var a := clampf(1.0 - absf(d - 0.78) / 0.2, 0.0, 1.0) * (1.0 if d2 > 0.78 else 0.0)
			img.set_pixel(x, y, Color(1, 1, 1, a))
	_cres = ImageTexture.create_from_image(img)
	return _cres

func heal_fx(pos: Vector3) -> void:
	var c := _col("nature", 0)
	var p := _particles(pos + Vector3(0, 0.1, 0), c, 26, 1.1)
	var pm := ParticleProcessMaterial.new()
	pm.emission_shape = ParticleProcessMaterial.EMISSION_SHAPE_SPHERE
	pm.emission_sphere_radius = 0.7
	pm.direction = Vector3.UP
	pm.spread = 10.0
	pm.initial_velocity_min = 1.2
	pm.initial_velocity_max = 2.6
	pm.gravity = Vector3.ZERO
	pm.scale_min = 0.3
	pm.scale_max = 0.8
	p.process_material = pm
	p.emitting = true
	ring(pos + Vector3(0, 0.05, 0), c, 1.4, 0.7)
	flash(pos + Vector3(0, 1, 0), c, 1.6, 0.35)
	_free_later(p, 1.6)

func shield_fx(node: Node3D, style := "holy") -> void:
	var mi := MeshInstance3D.new()
	var sm := SphereMesh.new()
	sm.radius = 1.25
	sm.height = 2.5
	mi.mesh = sm
	var m := StandardMaterial3D.new()
	m.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	m.blend_mode = BaseMaterial3D.BLEND_MODE_ADD
	m.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	m.albedo_color = Color(_col(style, 0), 0.5)
	m.cull_mode = BaseMaterial3D.CULL_FRONT
	mi.material_override = m
	mi.position = Vector3(0, 1.0, 0)
	mi.scale = Vector3.ONE * 0.3
	node.add_child(mi)
	var tw := create_tween()
	tw.tween_property(mi, "scale", Vector3.ONE, 0.25).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	tw.tween_property(m, "albedo_color:a", 0.0, 0.9).set_delay(0.5)
	tw.tween_callback(mi.queue_free)
	flash(node.global_position + Vector3(0, 1, 0), _col(style, 1), 1.8, 0.4)

## Danger zone: pulsing red disc that fills up until the hit lands.
func telegraph(pos: Vector3, radius: float, secs: float) -> void:
	var disc := MeshInstance3D.new()
	var q := QuadMesh.new()
	q.size = Vector2(1, 1)
	q.orientation = PlaneMesh.FACE_Y
	disc.mesh = q
	var m := StandardMaterial3D.new()
	m.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	m.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	m.albedo_texture = _glow
	m.albedo_color = Color(1.0, 0.18, 0.12, 0.55)
	disc.material_override = m
	disc.position = pos + Vector3(0, 0.05, 0)
	disc.scale = Vector3.ONE * radius * 2.0
	add_child(disc)
	var fill := MeshInstance3D.new()
	fill.mesh = q
	var fm := _additive(Color(1.0, 0.3, 0.15), _glow, 1.6)
	fm.billboard_mode = BaseMaterial3D.BILLBOARD_DISABLED
	fill.material_override = fm
	fill.position = pos + Vector3(0, 0.08, 0)
	fill.scale = Vector3.ONE * 0.1
	add_child(fill)
	ring(pos + Vector3(0, 0.1, 0), Color(1, 0.25, 0.2), radius, secs)
	var tw := create_tween()
	tw.tween_property(fill, "scale", Vector3.ONE * radius * 2.0, secs)
	tw.tween_callback(func():
		fill.queue_free()
		disc.queue_free()
		impact(pos, "fire", true))

func damage_number(pos: Vector3, text: String, color: Color, crit := false) -> void:
	var l := Label3D.new()
	l.text = text
	l.modulate = color
	l.outline_size = 14
	l.font_size = 72 if crit else 54
	l.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	l.no_depth_test = true
	l.pixel_size = 0.0075
	l.position = pos + Vector3(randf_range(-0.3, 0.3), 2.2, 0)
	add_child(l)
	var tw := create_tween()
	tw.tween_property(l, "position:y", l.position.y + 1.6, 0.9).set_ease(Tween.EASE_OUT)
	tw.parallel().tween_property(l, "scale", Vector3.ONE * (1.35 if crit else 1.0), 0.15).from(Vector3.ONE * 0.4)
	tw.parallel().tween_property(l, "modulate:a", 0.0, 0.4).set_delay(0.55)
	tw.tween_callback(l.queue_free)

# ----------------------------------------------------------------- event router

var _last_skill_style := {}  # caster id -> style of the last cast

func on_event(type: String, caster: Node2D, target: Node2D, e: Dictionary) -> void:
	var cpos := _to3(caster.global_position) if caster != null and is_instance_valid(caster) else Vector3.INF
	var tpos := _to3(target.global_position) if target != null and is_instance_valid(target) else Vector3.INF
	match type:
		"cast":
			var style := style_for(str(e.get("skill", "")))
			_last_skill_style[int(e.get("caster", 0))] = style
			var ranged_cls := str(e.get("skill", "")).substr(0, 6) in ["CLS-03", "CLS-04", "CLS-05"]
			get_tree().call_group("view3d", "play_action", caster, "cast" if ranged_cls else "attack")
			if cpos != Vector3.INF:
				cast_circle(cpos, style)
				var skill: Dictionary = _skills.get(str(e.get("skill", "")), {})
				var ranged := str(e.get("skill", "")).substr(0, 6) in ["CLS-03", "CLS-04", "CLS-05"]
				if tpos != Vector3.INF and str(skill.get("target", "")) == "enemy":
					if ranged:
						projectile(cpos + Vector3(0, 1.5, 0), tpos + Vector3(0, 1.0, 0), style, func(): pass)
					else:
						var d := tpos - cpos
						slash(cpos, -(atan2(d.z, d.x) + PI / 2.0), style)
		"damage":
			if tpos == Vector3.INF:
				return
			var style := str(_last_skill_style.get(int(e.get("caster", 0)), "physical"))
			var crit := bool(e.get("crit", false))
			impact(tpos + Vector3(0, 1.0, 0), style, crit)
			get_tree().call_group("view3d", "play_action", target, "hit")
			var mine := target != null and target.is_in_group("player")
			damage_number(tpos, str(int(e.get("amount", 0))) + ("!" if crit else ""),
				Color(1.0, 0.85, 0.2) if crit else (Color(1.0, 0.4, 0.4) if mine else Color.WHITE), crit)
		"heal":
			if tpos != Vector3.INF:
				heal_fx(tpos)
				damage_number(tpos, "+%d" % int(e.get("amount", 0)), Color(0.5, 1.0, 0.55))
		"shield":
			if tpos != Vector3.INF:
				var holder := Node3D.new()
				holder.position = tpos
				add_child(holder)
				shield_fx(holder)
				_free_later(holder, 1.8)
				damage_number(tpos, "+%d" % int(e.get("amount", 0)), Color(0.6, 0.85, 1.0))
		"death":
			if tpos != Vector3.INF:
				impact(tpos + Vector3(0, 0.8, 0), "blood", true)
		"telegraph":
			telegraph(Vector3(float(e.get("aim_x", 0)) / PX, 0, float(e.get("aim_y", 0)) / PX), float(e.get("radius", 64)) / PX, float(e.get("amount", 800)) / 1000.0)
