extends MarginContainer
## Applies DisplayServer safe-area insets on mobile; keeps a base margin on all platforms.
## Pattern: native Godot only (see docs/research/T-004-client-ui-research.md). Do not vendor addons.

@export var base_margin := 16

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	var win := get_window()
	if win:
		win.size_changed.connect(_update_margins)
	_update_margins()

func _update_margins() -> void:
	var left := base_margin
	var top := base_margin
	var right := base_margin
	var bottom := base_margin
	# Safe-area API is meaningful on handhelds; desktop usable_rect can be odd for windowed.
	if OS.has_feature("mobile") or OS.has_feature("android") or OS.has_feature("ios"):
		var safe: Rect2i = DisplayServer.get_display_safe_area()
		var screen: Vector2i = DisplayServer.screen_get_size()
		if screen.x > 0 and screen.y > 0 and size.x > 0.0 and size.y > 0.0:
			var sx := size.x / float(screen.x)
			var sy := size.y / float(screen.y)
			left = maxi(left, int(safe.position.x * sx))
			top = maxi(top, int(safe.position.y * sy))
			right = maxi(right, int((screen.x - safe.end.x) * sx))
			bottom = maxi(bottom, int((screen.y - safe.end.y) * sy))
	add_theme_constant_override("margin_left", left)
	add_theme_constant_override("margin_top", top)
	add_theme_constant_override("margin_right", right)
	add_theme_constant_override("margin_bottom", bottom)
