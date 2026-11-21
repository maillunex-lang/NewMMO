class_name Grass
extends Node3D
## Volumetric grass around the camera focus with a footstep "press map": stepped grass is held flat for HOLD_S
## (0.7 s), then recovers over RECOVER_S. Presentation only. CPU cost: one 64×64 RGB8 texture rebuild at 20 Hz.

const HOLD_S := 0.7
const RECOVER_S := 2.0
const LIFE_S := HOLD_S + RECOVER_S
const MAP_PX := 64
const STEP_M := 0.45          # an actor drops a footprint every STEP_M metres
const RADIUS_M := 1.15
const MAX_SAMPLES := 96

const PRESETS := {  # clumps per side, window size (m); 0 = no grass
	"low": [34, 15.0], "medium": [60, 21.0], "high": [96, 27.0], "ultra": [128, 32.0],
}

var _mat: ShaderMaterial = null
var _mm: MultiMeshInstance3D = null
var _area := 27.0
var _tex: ImageTexture = null
var _bytes := PackedByteArray()
var _samples: Array = []        # [x, z, born_seconds, strength]
var _last_step: Dictionary = {} # actor id -> Vector2
var _clock := 0.0
var _upd := 0.0
var _origin := Vector2.ZERO

func setup(preset: String, road_tex: Texture2D, map_size: Vector2) -> void:
	var cfg: Array = PRESETS.get(preset, PRESETS["high"])
	var n: int = cfg[0]
	_area = float(cfg[1])
	_mat = ShaderMaterial.new()
	_mat.shader = load("res://shaders/grass.gdshader")
	_mat.set_shader_parameter("area", _area)
	_mat.set_shader_parameter("press_size", _area)
	_mat.set_shader_parameter("road_mask", road_tex)
	_mat.set_shader_parameter("map_size", map_size)
	_bytes.resize(MAP_PX * MAP_PX * 3)
	var img := Image.create_from_data(MAP_PX, MAP_PX, false, Image.FORMAT_RGB8, _bytes)
	_tex = ImageTexture.create_from_image(img)
	_mat.set_shader_parameter("press_map", _tex)
	var mesh := _clump_mesh()
	mesh.surface_set_material(0, _mat)
	var mm := MultiMesh.new()
	mm.transform_format = MultiMesh.TRANSFORM_3D
	mm.mesh = mesh
	mm.instance_count = n * n
	var rng := RandomNumberGenerator.new()
	rng.seed = 11
	var cell := _area / float(n)
	for i in n:
		for j in n:
			var p := Vector3((i + rng.randf()) * cell, 0.0, (j + rng.randf()) * cell)
			mm.set_instance_transform(i * n + j, Transform3D(Basis(), p))
	_mm = MultiMeshInstance3D.new()
	_mm.multimesh = mm
	_mm.custom_aabb = AABB(Vector3(-2000, -1, -2000), Vector3(4000, 6, 4000))  # positions are wrapped in the shader
	_mm.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	add_child(_mm)

## One clump = 3 tapered blades fanned out; y in 0..1 (the shader scales the height).
func _clump_mesh() -> ArrayMesh:
	var st := SurfaceTool.new()
	st.begin(Mesh.PRIMITIVE_TRIANGLES)
	for b in 3:
		var a := b * TAU / 3.0 + 0.4
		var dx := Vector3(cos(a), 0, sin(a))
		var off := dx * 0.05
		var w := [0.035, 0.026, 0.014, 0.0]
		var ys := [0.0, 0.38, 0.72, 1.0]
		for k in 3:
			var l0: Vector3 = off - dx * w[k] + Vector3(0, ys[k], 0)
			var r0: Vector3 = off + dx * w[k] + Vector3(0, ys[k], 0)
			var l1: Vector3 = off - dx * w[k + 1] + Vector3(0, ys[k + 1], 0)
			var r1: Vector3 = off + dx * w[k + 1] + Vector3(0, ys[k + 1], 0)
			for v in [l0, r0, l1, r0, r1, l1]:
				st.add_vertex(v)
	return st.commit()

## focus: hero position (m). movers: [[id, Vector2 xz]] actors that may leave footprints.
func tick(delta: float, focus: Vector3, movers: Array) -> void:
	if _mat == null:
		return
	_clock += delta
	_mat.set_shader_parameter("focus", focus)
	_mat.set_shader_parameter("t_now", _clock)
	for m in movers:
		var id: int = m[0]
		var p: Vector2 = m[1]
		var last: Vector2 = _last_step.get(id, Vector2(1e9, 1e9))
		if p.distance_to(last) >= STEP_M:
			_last_step[id] = p
			_samples.append([p.x, p.y, _clock, 1.0])
			if _samples.size() > MAX_SAMPLES:
				_samples.pop_front()
	_upd -= delta
	if _upd <= 0.0:
		_upd = 0.05
		_rebuild(focus)

func _rebuild(focus: Vector3) -> void:
	var texel := _area / float(MAP_PX)
	_origin = Vector2(floorf(focus.x / texel) * texel - _area * 0.5, floorf(focus.z / texel) * texel - _area * 0.5)
	_mat.set_shader_parameter("press_origin", _origin)
	var press := PackedFloat32Array()
	press.resize(MAP_PX * MAP_PX)
	var dirx := PackedFloat32Array()
	dirx.resize(MAP_PX * MAP_PX)
	var dirz := PackedFloat32Array()
	dirz.resize(MAP_PX * MAP_PX)
	var alive: Array = []
	for s in _samples:
		var age: float = _clock - float(s[2])
		if age >= LIFE_S:
			continue
		alive.append(s)
		var k := 1.0 if age < HOLD_S else 1.0 - smoothstep(HOLD_S, LIFE_S, age)
		var r_px := int(ceil(RADIUS_M / texel))
		var cx := int((float(s[0]) - _origin.x) / texel)
		var cz := int((float(s[1]) - _origin.y) / texel)
		for dz in range(-r_px, r_px + 1):
			for dx in range(-r_px, r_px + 1):
				var x := cx + dx
				var z := cz + dz
				if x < 0 or z < 0 or x >= MAP_PX or z >= MAP_PX:
					continue
				var wx := _origin.x + (x + 0.5) * texel - float(s[0])
				var wz := _origin.y + (z + 0.5) * texel - float(s[1])
				var dist := sqrt(wx * wx + wz * wz)
				if dist > RADIUS_M:
					continue
				var v: float = k * (1.0 - smoothstep(RADIUS_M * 0.35, RADIUS_M, dist))
				var i := z * MAP_PX + x
				if v > press[i]:
					press[i] = v
					var inv := 1.0 / maxf(dist, 0.001)
					dirx[i] = wx * inv
					dirz[i] = wz * inv
	_samples = alive
	for i in MAP_PX * MAP_PX:
		_bytes[i * 3] = int(clampf(press[i], 0.0, 1.0) * 255.0)
		_bytes[i * 3 + 1] = int(clampf(0.5 + 0.5 * dirx[i] * press[i], 0.0, 1.0) * 255.0)
		_bytes[i * 3 + 2] = int(clampf(0.5 + 0.5 * dirz[i] * press[i], 0.0, 1.0) * 255.0)
	_tex.update(Image.create_from_data(MAP_PX, MAP_PX, false, Image.FORMAT_RGB8, _bytes))
