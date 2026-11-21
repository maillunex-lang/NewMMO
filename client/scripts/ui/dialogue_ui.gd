extends Control

signal closed(npc_id: String)
signal action_chosen(action: String, npc_id: String)

@onready var panel: PanelContainer = $SafeArea/Chrome/Panel
@onready var portrait: TextureRect = $SafeArea/Chrome/Panel/Margin/VBox/Header/Portrait
@onready var speaker: Label = $SafeArea/Chrome/Panel/Margin/VBox/Header/Speaker
@onready var body: Label = $SafeArea/Chrome/Panel/Margin/VBox/Body
@onready var next_btn: Button = $SafeArea/Chrome/Panel/Margin/VBox/Next
@onready var choices: HBoxContainer = $SafeArea/Chrome/Panel/Margin/VBox/Choices
@onready var choice_accept: Button = $SafeArea/Chrome/Panel/Margin/VBox/Choices/ChoiceAccept
@onready var choice_ask: Button = $SafeArea/Chrome/Panel/Margin/VBox/Choices/ChoiceAsk
@onready var choice_turn_in: Button = $SafeArea/Chrome/Panel/Margin/VBox/Choices/ChoiceTurnIn

var _npc_id := ""
var _important := true
var _flow: Array = []
var _index := 0
var _quest: Node = null
var _pending_action := ""
var _mode := ""
var _server_options: Array = []
var _server_driven := false

func _ready() -> void:
	visible = false
	mouse_filter = Control.MOUSE_FILTER_STOP
	next_btn.pressed.connect(_on_next)
	if choice_accept:
		choice_accept.pressed.connect(func(): _choose("ACCEPT"))
	if choice_ask:
		choice_ask.pressed.connect(func(): _choose("ASK"))
	if choice_turn_in:
		choice_turn_in.pressed.connect(func(): _choose("TURN_IN"))
	_hide_choice_buttons()
	_apply_chrome_labels()

func _notification(what: int) -> void:
	if what == NOTIFICATION_TRANSLATION_CHANGED:
		_apply_chrome_labels()

func _apply_chrome_labels() -> void:
	if choice_accept:
		choice_accept.text = tr("ui.accept")
	if choice_ask:
		choice_ask.text = tr("ui.ask")
	if choice_turn_in:
		choice_turn_in.text = tr("ui.turn_in")
	if next_btn and next_btn.visible:
		# Keep next/close dynamic via _show_current; only ensure base key when idle.
		pass

func open_flow(npc_id: String, display_name: String, flow_ids: PackedStringArray, portrait_tex: Texture2D = null, mode: String = "") -> void:
	_npc_id = npc_id
	_mode = mode
	_server_driven = false
	_server_options.clear()
	_flow.clear()
	_index = 0
	_pending_action = ""
	_quest = get_tree().get_first_node_in_group("quest_manager")
	speaker.text = display_name
	if portrait_tex:
		portrait.texture = portrait_tex
	for id in flow_ids:
		var line: Dictionary = {}
		if _quest and _quest.has_method("get_line"):
			line = _quest.get_line(str(id))
		var act := str(line.get("action", ""))
		if act in ["ACCEPT", "TURN_IN", "ASK"]:
			continue
		_flow.append(str(id))
	visible = true
	_show_current()

## Server-authoritative node (EX-P05 / T-025). Options list drives visible buttons.
func open_server_node(npc_id: String, display_name: String, node: Dictionary, portrait_tex: Texture2D = null) -> void:
	_npc_id = npc_id
	_server_driven = true
	_mode = ""
	_flow.clear()
	_index = 0
	_quest = get_tree().get_first_node_in_group("quest_manager")
	if portrait_tex:
		portrait.texture = portrait_tex
	var sp := str(node.get("speaker", ""))
	if sp == "PLAYER":
		speaker.text = tr("ui.you")
	elif display_name != "":
		speaker.text = display_name
	else:
		speaker.text = sp if sp != "" else npc_id
	body.text = str(node.get("text", ""))
	_server_options = node.get("options", [])
	if typeof(_server_options) != TYPE_ARRAY:
		_server_options = []
	visible = true
	_show_server_options()

func _show_server_options() -> void:
	_hide_choice_buttons()
	var ids: PackedStringArray = PackedStringArray()
	for opt in _server_options:
		if typeof(opt) != TYPE_DICTIONARY:
			continue
		ids.append(str(opt.get("option_id", "")))
	var has_accept := "ACCEPT" in ids
	var has_ask := "ASK" in ids
	var has_turn_in := "TURN_IN" in ids
	var has_continue := "CONTINUE" in ids or "LATER" in ids
	if choice_accept:
		choice_accept.visible = has_accept
	if choice_ask:
		choice_ask.visible = has_ask
	if choice_turn_in:
		choice_turn_in.visible = has_turn_in
	if has_accept or has_ask or has_turn_in:
		next_btn.visible = false
	elif has_continue:
		next_btn.visible = true
		next_btn.text = tr("ui.close")
	else:
		next_btn.visible = true
		next_btn.text = tr("ui.close")

func _show_current() -> void:
	_hide_choice_buttons()
	if _index >= _flow.size():
		_show_mode_actions_or_finish()
		return
	var dia_id: String = str(_flow[_index])
	var line: Dictionary = {"speaker": "SYSTEM", "text": dia_id}
	if _quest and _quest.has_method("get_line"):
		line = _quest.get_line(dia_id)
	var sp: String = str(line.get("speaker", ""))
	if sp == "PLAYER":
		speaker.text = tr("ui.you")
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
		next_btn.text = tr("ui.next") if not last else tr("ui.close")

func _show_mode_actions_or_finish() -> void:
	_hide_choice_buttons()
	if _mode == "start":
		next_btn.visible = false
		if choice_accept:
			choice_accept.visible = true
		if choice_ask:
			choice_ask.visible = true
		return
	if _mode == "turnin":
		next_btn.visible = false
		if choice_turn_in:
			choice_turn_in.visible = true
		return
	_finish()

func _hide_choice_buttons() -> void:
	if choice_accept:
		choice_accept.visible = false
	if choice_ask:
		choice_ask.visible = false
	if choice_turn_in:
		choice_turn_in.visible = false

func _choose(action: String) -> void:
	if _server_driven:
		action_chosen.emit(action, _npc_id)
		if action == "LATER" or action == "CONTINUE":
			_finish()
			return
		# Wait for next dialogue.node.state; keep panel open for ASK/ACCEPT/TURN_IN follow-up text.
		_hide_choice_buttons()
		next_btn.visible = true
		next_btn.text = tr("ui.close")
		return
	if action == "ASK":
		var qid := ""
		if _quest and _quest.has_method("get_active_quest_id"):
			qid = _quest.get_active_quest_id()
		var ask_id := "DIA.%s.START.03" % qid
		var line: Dictionary = _quest.get_line(ask_id) if _quest else {}
		speaker.text = _quest.npc_display_name(_npc_id) if _quest else _npc_id
		body.text = str(line.get("text", ""))
		_hide_choice_buttons()
		next_btn.visible = true
		next_btn.text = tr("ui.close")
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
		body.text = str(done_line.get("text", ""))
		_hide_choice_buttons()
		next_btn.visible = true
		next_btn.text = tr("ui.close")
		_mode = ""
		_index = _flow.size()
		return
	_finish()

func _on_next() -> void:
	if _server_driven:
		_finish()
		return
	_index += 1
	_show_current()

func _finish() -> void:
	visible = false
	_server_driven = false
	_server_options.clear()
	_hide_choice_buttons()
	closed.emit(_npc_id)

func _unhandled_input(event: InputEvent) -> void:
	if not visible:
		return
	if _important and event is InputEventMouseButton and event.pressed:
		get_viewport().set_input_as_handled()
