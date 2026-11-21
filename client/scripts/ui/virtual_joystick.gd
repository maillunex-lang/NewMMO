extends Control
## On-screen joystick. Emits vector in [-1,1]. Touch or mouse.

signal direction_changed(direction: Vector2)

@export var knob_path: NodePath
@export var max_radius := 48.0

var _knob: Control
var _active := false
var _origin := Vector2.ZERO
var _dir := Vector2.ZERO
var _touch_index := -1

func _ready() -> void:
	_knob = get_node_or_null(knob_path)
	mouse_filter = Control.MOUSE_FILTER_STOP

func get_direction() -> Vector2:
	return _dir

func _gui_input(event: InputEvent) -> void:
	if event is InputEventScreenTouch:
		var st := event as InputEventScreenTouch
		if st.pressed and not _active:
			_active = true
			_touch_index = st.index
			_origin = st.position
			_update_knob(Vector2.ZERO)
			accept_event()
		elif not st.pressed and st.index == _touch_index:
			_reset()
			accept_event()
	elif event is InputEventScreenDrag and _active and event.index == _touch_index:
		_apply(event.position - _origin)
		accept_event()
	elif event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT:
		if event.pressed:
			_active = true
			_touch_index = -2
			_origin = event.position
			_update_knob(Vector2.ZERO)
			accept_event()
		elif _touch_index == -2:
			_reset()
			accept_event()
	elif event is InputEventMouseMotion and _active and _touch_index == -2:
		_apply(event.position - _origin)
		accept_event()

func _apply(delta: Vector2) -> void:
	var clamped := delta
	if clamped.length() > max_radius:
		clamped = clamped.normalized() * max_radius
	_dir = clamped / max_radius
	_update_knob(clamped)
	direction_changed.emit(_dir)

func _update_knob(local_offset: Vector2) -> void:
	if _knob:
		_knob.position = size * 0.5 + local_offset - _knob.size * 0.5

func _reset() -> void:
	_active = false
	_touch_index = -1
	_dir = Vector2.ZERO
	_update_knob(Vector2.ZERO)
	direction_changed.emit(_dir)
