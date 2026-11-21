extends Control

signal closed(npc_id: String)
signal action_chosen(action: String, npc_id: String)

@onready var panel: PanelContainer = $Panel
@onready var portrait: TextureRect = $Panel/Margin/VBox/Header/Portrait
@onready var speaker: Label = $Panel/Margin/VBox/Header/Speaker
@onready var body: Label = $Panel/Margin/VBox/Body
@onready var next_btn: Button = $Panel/Margin/VBox/Next
@onready var choices: HBoxContainer = $Panel/Margin/VBox/Choices

var _npc_id := ""
var _important := true
var _flow: Array = []
var _index := 0
var _quest: Node = null
var _pending_action := ""
var _mode := ""

func _ready() -> void:
	visible = false
	mouse_filter = Control.MOUSE_FILTER_STOP
	next_btn.pressed.connect(_on_next)
	if choices:
		for c in choices.get_children():
			c.queue_free()

func open_flow(npc_id: String, display_name: String, flow_ids: PackedStringArray, portrait_tex: Texture2D = null, mode: String = "") -> void:
	_npc_id = npc_id
	_mode = mode
	_flow.clear()
	_index = 0
	_pending_action = ""
	_quest = get_tree().get_first_node_in_group("quest_manager")
	speaker.text = display_name
	if portrait_tex:
		portrait.texture = portrait_tex
	for id in flow_ids:
		# Skip PLAYER ACCEPT/TURN_IN/ASK lines; those become buttons
		var line: Dictionary = {}
		if _quest and _quest.has_method("get_line"):
			line = _quest.get_line(str(id))
		var act := str(line.get("action", ""))
		if act in ["ACCEPT", "TURN_IN", "ASK"]:
			continue
		_flow.append(str(id))
	visible = true
	_show_current()

func _show_current() -> void:
	_clear_choices()
	if _index >= _flow.size():
		_show_mode_actions_or_finish()
		return
	var dia_id: String = str(_flow[_index])
	var line: Dictionary = {"speaker": "SYSTEM", "text": dia_id}
	if _quest and _quest.has_method("get_line"):
		line = _quest.get_line(dia_id)
	var sp: String = str(line.get("speaker", ""))
	if sp == "PLAYER":
		speaker.text = "Вы"
	elif sp.begins_with("NPC-") and _quest and _quest.has_method("npc_display_name"):
		speaker.text = _quest.npc_display_name(sp)
	elif sp != "SYSTEM":
		speaker.text = sp
	body.text = str(line.get("text", ""))
	_pending_action = ""
	var last := _index >= _flow.size() - 1
	if last and (_mode == "start" or _mode == "turnin"):
		next_btn.visible = false
		_show_mode_actions_or_finish()
	else:
		next_btn.visible = true
		next_btn.text = "Далее" if not last else "Закрыть"

func _show_mode_actions_or_finish() -> void:
	_clear_choices()
	if _mode == "start":
		next_btn.visible = false
		_add_choice("ACCEPT", "Принять")
		_add_choice("ASK", "Спросить")
		return
	if _mode == "turnin":
		next_btn.visible = false
		_add_choice("TURN_IN", "Сдать")
		return
	_finish()

func _label_for_action(action: String) -> String:
	match action:
		"ACCEPT":
			return "Принять"
		"TURN_IN":
			return "Сдать"
		"ASK":
			return "Спросить"
		_:
			return action

func _add_choice(action: String, label: String) -> void:
	if choices == null:
		return
	var btn := Button.new()
	btn.text = label
	btn.pressed.connect(func(): _choose(action))
	choices.add_child(btn)

func _clear_choices() -> void:
	if choices == null:
		return
	for c in choices.get_children():
		c.queue_free()

func _choose(action: String) -> void:
	if action == "ASK":
		# Show canon ASK response, do not accept quest
		var qid := ""
		if _quest and _quest.has_method("get_active_quest_id"):
			qid = _quest.get_active_quest_id()
		var ask_id := "DIA.%s.START.03" % qid
		var line: Dictionary = _quest.get_line(ask_id) if _quest else {}
		speaker.text = _quest.npc_display_name(_npc_id) if _quest else _npc_id
		body.text = str(line.get("text", "Доказательство важнее удобного объяснения."))
		_clear_choices()
		next_btn.visible = true
		next_btn.text = "Закрыть"
		_mode = ""
		_index = _flow.size()
		return
	action_chosen.emit(action, _npc_id)
	if action == "TURN_IN":
		var qid2 := ""
		if _quest and _quest.has_method("get_active_quest_id"):
			qid2 = _quest.get_active_quest_id()
		var done_id := "DIA.%s.DONE.01" % qid2
		var done_line: Dictionary = _quest.get_line(done_id) if _quest else {}
		speaker.text = _quest.npc_display_name(_npc_id) if _quest else _npc_id
		body.text = str(done_line.get("text", "Задание завершено."))
		_clear_choices()
		next_btn.visible = true
		next_btn.text = "Закрыть"
		_mode = ""
		_index = _flow.size()
		return
	_finish()

func _on_next() -> void:
	_index += 1
	_show_current()

func _finish() -> void:
	visible = false
	_clear_choices()
	closed.emit(_npc_id)

func _unhandled_input(event: InputEvent) -> void:
	if not visible:
		return
	if _important and event is InputEventMouseButton and event.pressed:
		get_viewport().set_input_as_handled()
