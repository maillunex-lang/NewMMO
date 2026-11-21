extends Node2D
## T-038d: ground telegraph of an enemy signature attack (red circle that fills up until the hit lands).
## Duration/radius/centre come from the server's `telegraph` event; the hit itself is server-side.

var radius := 64.0
var duration := 0.8
var _t := 0.0

func setup(p_radius: float, p_duration: float) -> void:
	radius = p_radius
	duration = maxf(0.1, p_duration)
	z_index = -500  # below actors, above the ground
	add_to_group("telegraphs")

func _process(delta: float) -> void:
	_t += delta
	queue_redraw()
	if _t >= duration + 0.2:
		queue_free()

func _draw() -> void:
	var k := clampf(_t / duration, 0.0, 1.0)
	# True circle: the server hit area is a circle in world units; the fill grows with the wind-up.
	var pts := PackedVector2Array()
	for i in range(49):
		var a := TAU * float(i) / 48.0
		pts.append(Vector2(cos(a) * radius, sin(a) * radius))
	draw_colored_polygon(pts, Color(0.9, 0.12, 0.12, 0.12 + 0.38 * k))
	draw_polyline(pts, Color(1.0, 0.35, 0.3, 0.95), 3.0)
