extends Node2D
## Stage 1 + MQ-01-01…05 coordinator.

@onready var player: CharacterBody2D = $DawnValley/Player
@onready var dialogue: Control = $CanvasLayer/Dialogue
@onready var quest_journal: Control = $CanvasLayer/QuestJournal
@onready var quest_manager: Node = $QuestManager
@onready var camera: Camera2D = $DawnValley/Player/Camera2D

func _ready() -> void:
	# Honor project.godot [internationalization] locale/locale (OS locale otherwise wins).
	var forced := str(ProjectSettings.get_setting("internationalization/locale/locale", ""))
	if forced != "":
		TranslationServer.set_locale(forced)
	_try_attach_online_session()
	if quest_manager and quest_manager.has_signal("dialogue_flow_requested"):
		quest_manager.dialogue_flow_requested.connect(_on_flow)
	if dialogue and dialogue.has_signal("closed"):
		dialogue.closed.connect(_on_dialogue_closed)
	if dialogue and dialogue.has_signal("action_chosen"):
		dialogue.action_chosen.connect(_on_dialogue_action)
	if camera:
		camera.limit_left = -80
		camera.limit_top = -80
		camera.limit_right = 2120
		camera.limit_bottom = 1480
		camera.position_smoothing_enabled = true
		camera.position_smoothing_speed = 6.0

func _unhandled_input(event: InputEvent) -> void:
	if dialogue and dialogue.visible:
		return
	if quest_journal and quest_journal.visible:
		return
	if event is InputEventKey and event.pressed and not event.echo:
		match event.keycode:
			KEY_1, KEY_2, KEY_3, KEY_4, KEY_5, KEY_6:
				if player and player.has_method("use_skill"):
					player.use_skill(event.keycode - KEY_1)
			KEY_E:
				if player and player.has_method("try_interact"):
					player.try_interact()
			KEY_T:
				if player and player.has_method("toggle_auto_combat"):
					player.toggle_auto_combat()
		return
	var tap := false
	var pos := Vector2.ZERO
	if event is InputEventScreenTouch and event.pressed:
		tap = true
		pos = get_global_mouse_position()
	elif event is InputEventMouseButton and event.pressed and event.button_index == MOUSE_BUTTON_LEFT:
		tap = true
		pos = get_global_mouse_position()
	if tap and player and player.has_method("move_to_world"):
		player.move_to_world(pos)

func _on_flow(npc_id: String, flow_ids: PackedStringArray, mode: String) -> void:
	var tex: Texture2D = load("res://assets/ui/portrait_mira.png") if npc_id == "NPC-102" else load("res://assets/ui/portrait_player.png")
	var name := npc_id
	if quest_manager and quest_manager.has_method("npc_display_name"):
		name = quest_manager.npc_display_name(npc_id)
	dialogue.open_flow(npc_id, name, flow_ids, tex, mode)

func _on_dialogue_action(action: String, npc_id: String) -> void:
	if quest_manager and quest_manager.has_method("on_dialogue_action"):
		quest_manager.on_dialogue_action(action, npc_id, player)

func _on_dialogue_closed(_npc_id: String) -> void:
	if player and player.has_method("pause_automation"):
		player.pause_automation(false)

func _try_attach_online_session() -> void:
	var session := get_tree().get_first_node_in_group("online_session")
	if session == null or quest_manager == null:
		return
	if not session.has_method("get_net") or not session.has_method("is_online"):
		return
	# Attach even while AUTH/CONNECTING — enable_online wires signals; READY may arrive next frames.
	var net = session.get_net()
	if net == null:
		return
	var build := ""
	if str(net.content_build_id) != "":
		build = str(net.content_build_id)
	quest_manager.enable_online(net, build)
