extends Control
## Credits / attribution screen (CC-BY art must be credited). Text lives in res://data/credits.txt;
## tools/content/check_client_sync.py checks that every CC-BY source in assets/LICENSES.md appears there.

@onready var body: RichTextLabel = $Panel/VBox/Scroll/Body
@onready var title: Label = $Panel/VBox/Title
@onready var close_btn: Button = $Panel/VBox/Close

func _ready() -> void:
	add_to_group("credits_ui")
	var f := FileAccess.open("res://data/credits.txt", FileAccess.READ)
	body.text = f.get_as_text() if f != null else ""
	close_btn.pressed.connect(close)
	$Dim.gui_input.connect(_on_dim_input)
	_apply_labels()
	visible = false

func _notification(what: int) -> void:
	if what == NOTIFICATION_TRANSLATION_CHANGED and is_node_ready():
		_apply_labels()

func _apply_labels() -> void:
	title.text = tr("ui.credits")
	close_btn.text = tr("ui.close")

func open() -> void:
	visible = true

func close() -> void:
	visible = false

func toggle() -> void:
	visible = not visible

func _on_dim_input(event: InputEvent) -> void:
	if (event is InputEventMouseButton and event.pressed) or (event is InputEventScreenTouch and event.pressed):
		close()
