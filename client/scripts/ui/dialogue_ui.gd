extends Control

signal closed(npc_id: String)

@onready var panel: PanelContainer = $Panel
@onready var portrait: TextureRect = $Panel/Margin/VBox/Header/Portrait
@onready var speaker: Label = $Panel/Margin/VBox/Header/Speaker
@onready var body: Label = $Panel/Margin/VBox/Body
@onready var next_btn: Button = $Panel/Margin/VBox/Next

var _lines: PackedStringArray = PackedStringArray()
var _index := 0
var _npc_id := ""
var _important := false

func _ready() -> void:
	visible = false
	mouse_filter = Control.MOUSE_FILTER_STOP
	next_btn.pressed.connect(_on_next)

func open_dialogue(npc_id: String, display_name: String, lines: PackedStringArray, important: bool, portrait_tex: Texture2D = null) -> void:
	_npc_id = npc_id
	_lines = lines
	_index = 0
	_important = important
	speaker.text = display_name
	if portrait_tex:
		portrait.texture = portrait_tex
	visible = true
	_show_line()

func _show_line() -> void:
	if _index >= _lines.size():
		_finish()
		return
	body.text = _lines[_index]
	next_btn.text = "Continue" if _index < _lines.size() - 1 else "Accept"

func _on_next() -> void:
	_index += 1
	if _index >= _lines.size():
		_finish()
	else:
		_show_line()

func _finish() -> void:
	visible = false
	closed.emit(_npc_id)

func _unhandled_input(event: InputEvent) -> void:
	# Important dialogues are not skipped by tap elsewhere; only Next advances.
	if not visible:
		return
	if _important and event is InputEventMouseButton and event.pressed:
		# Swallow world taps while dialogue is open.
		get_viewport().set_input_as_handled()
