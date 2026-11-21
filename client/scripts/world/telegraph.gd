extends Node2D
## T-038d: ground telegraph of an enemy signature attack (red circle that fills up until the hit lands).
## Duration/radius/centre come from the server's `telegraph` event; the hit itself is server-side.

var radius := 64.0
var duration := 0.8
var _t := 0.0
var _safe_areas: Array[Rect2] = []

func setup(p_radius: float, p_duration: float) -> void:
	radius = p_radius
	duration = maxf(0.1, p_duration)
	z_index = -500  # below actors, above the ground
	add_to_group("telegraphs")

func setup_safe_areas(areas: Array, seconds: float) -> void:
	setup(0.0, seconds)
	for area in areas:
		var top_left := Vector2(float(area["min_x"]), float(area["min_y"]))
		var bottom_right := Vector2(float(area["max_x"]), float(area["max_y"]))
		_safe_areas.append(Rect2(top_left, bottom_right - top_left))

func _process(delta: float) -> void:
	_t += delta
	queue_redraw()
	if _t >= duration + 0.2:
		queue_free()

func _draw() -> void:
	var k := clampf(_t / duration, 0.0, 1.0)
	if not _safe_areas.is_empty():
		for area in _safe_areas:
			draw_rect(area, Color(0.2, 0.85, 0.6, 0.24))
			draw_rect(area, Color(0.65, 1.0, 0.85), false, 3.0)
			var center := area.get_center()
			draw_polyline(PackedVector2Array([center + Vector2(-20, 0), center + Vector2(-6, 14),
				center + Vector2(23, -16)]), Color(0.9, 1.0, 0.92), 6.0, true)
			draw_line(area.position, area.position + Vector2(area.size.x * (1.0 - k), 0),
				Color(0.95, 1.0, 0.96), 5.0)
		return
	# True circle: the server hit area is a circle in world units; the fill grows with the wind-up.
	var pts := PackedVector2Array()
	for i in range(49):
		var a := TAU * float(i) / 48.0
		pts.append(Vector2(cos(a) * radius, sin(a) * radius))
	draw_colored_polygon(pts, Color(0.9, 0.12, 0.12, 0.12 + 0.38 * k))
	draw_polyline(pts, Color(1.0, 0.35, 0.3, 0.95), 3.0)
