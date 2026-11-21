extends Node2D
## Stage 1 coordinator: input routing, NPC dialogue, local combat world.

@onready var player: CharacterBody2D = $DawnValley/Player
@onready var dialogue: Control = $CanvasLayer/Dialogue
@onready var quest_manager: Node = $QuestManager
@onready var camera: Camera2D = $DawnValley/Player/Camera2D

func _ready() -> void:
	for npc in get_tree().get_nodes_in_group("npcs"):
		if npc.has_signal("dialogue_requested"):
			npc.dialogue_requested.connect(_on_npc_dialogue)
	if dialogue and dialogue.has_signal("closed"):
		dialogue.closed.connect(_on_dialogue_closed)
	# Soft camera limits around dawn valley ground
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

func _on_npc_dialogue(npc: Node) -> void:
	var tex: Texture2D = load("res://assets/ui/portrait_mira.png")
	dialogue.open_dialogue(
		npc.npc_id,
		npc.display_name,
		npc.lines,
		npc.important_dialogue,
		tex
	)

func _on_dialogue_closed(npc_id: String) -> void:
	if quest_manager and quest_manager.has_method("on_dialogue_finished"):
		quest_manager.on_dialogue_finished(npc_id, player)
	if player and player.has_method("pause_automation"):
		# Re-enable automation control after important dialogue; auto stays off until toggled.
		player.pause_automation(false)
