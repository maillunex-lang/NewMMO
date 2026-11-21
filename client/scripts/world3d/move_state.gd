class_name MoveState
extends RefCounted
## Walk/idle decision with smoothing + hysteresis for 3D actors. The 2D logic world moves on the physics tick while the
## 3D view renders at the display rate (60/90/120 Hz), so raw per-frame displacement alternates between 0 and a full
## step; deciding "moving" from it flips run/idle every other frame and restarts the animation (visible jitter).

const START_SPEED := 0.9   # m/s (smoothed) to start moving
const STOP_SPEED := 0.35   # m/s (smoothed) below which we may stop
const STOP_HOLD_S := 0.18  # must stay slow this long before idle

var speed := 0.0           # smoothed m/s
var moving := false
var _slow_t := 0.0

## raw_speed: this frame's displacement / delta. Returns whether the actor is moving.
func update(raw_speed: float, delta: float) -> bool:
	speed = lerpf(speed, raw_speed, 1.0 - exp(-12.0 * delta))
	if moving:
		if speed < STOP_SPEED:
			_slow_t += delta
			if _slow_t >= STOP_HOLD_S:
				moving = false
				_slow_t = 0.0
		else:
			_slow_t = 0.0
	elif speed > START_SPEED:
		moving = true
		_slow_t = 0.0
	return moving
