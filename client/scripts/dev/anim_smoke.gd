extends SceneTree
## Headless: 3D walk/idle decision must not flicker when the logic world moves on the 60 Hz physics tick but frames
## render at 144 Hz (raw per-frame speed alternates 0 / full step). Marker ANIM_SMOKE_OK.
var _failed := false
func _init() -> void:
	call_deferred("_run")
func _assert(c: bool, l: String) -> void:
	if not c:
		push_error("ASSERT FAIL: " + l)
		_failed = true
func _run() -> void:
	var ms := MoveState.new()
	var flips := 0
	var last := false
	var t := 0.0
	var dt := 1.0 / 144.0
	var started_at := -1.0
	# 3 s of steady 5.6 m/s motion sampled the way a 60 Hz tick under a 144 Hz renderer sees it
	for i in 432:
		var step_frame := (i % 12) in [0, 5]   # ~2 of every 4.8 frames see a position change
		var raw := (5.6 / 60.0) / dt if (i % 2 == 0 and step_frame or i % 5 == 0) else 0.0
		var m := ms.update(raw, dt)
		if m != last:
			flips += 1
			if m and started_at < 0.0:
				started_at = t
		last = m
		t += dt
	_assert(ms.moving, "still moving after steady motion")
	_assert(flips == 1, "exactly one idle->moving transition during steady motion, got %d" % flips)
	_assert(started_at < 0.3, "starts moving quickly (%.2f s)" % started_at)
	var stopped_after := -1.0
	var tt := 0.0
	for i in 144:
		ms.update(0.0, dt)
		tt += dt
		if not ms.moving and stopped_after < 0.0:
			stopped_after = tt
	_assert(stopped_after > 0.0 and stopped_after < 0.7, "goes idle shortly after stopping (%.2f s)" % stopped_after)
	_assert(not ms.moving, "idle at rest")
	if _failed:
		quit(1)
		return
	print("ANIM_SMOKE_OK")
	quit(0)
