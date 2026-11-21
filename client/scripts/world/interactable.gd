extends Area2D
## World / POI interactable for quest objectives (canon target IDs).

@export var interact_id := "POI"
@export var display_name := "Объект"
@export var event_type := "world_interact"  # world_interact | npc_interact | escort_checkpoint
@export var instance_id := ""  # required when registry count>1 (e.g. POI-CLUE#1)
@export var important := false
## T-037: canon dungeon object id (OBJ-DNG-…) — interaction goes to the dungeon controller, not the quest manager.
@export var dungeon_object_id := ""
## T-037: dungeon entrance (DNG-…) — interaction asks the server to enter the dungeon.
@export var dungeon_entry_id := ""

@onready var label: Label = $Label

func _ready() -> void:
	monitoring = false
	monitorable = true
	collision_layer = 1
	collision_mask = 0
	if label:
		label.text = tr(display_name)
		label.visible = false  # shown only for the closest interactable near the player (no map clutter)
		label.add_theme_color_override("font_outline_color", Color(0, 0, 0, 0.95))
		label.add_theme_constant_override("outline_size", 6)
		label.add_theme_font_size_override("font_size", 15)
	add_to_group("interactables")

const LABEL_SHOW_RADIUS := 120.0
var _label_timer := 0.0

func _process(delta: float) -> void:
	_label_timer -= delta
	if _label_timer > 0.0 or label == null:
		return
	_label_timer = 0.15
	var nodes := get_tree().get_nodes_in_group("player")
	if nodes.is_empty():
		label.visible = false
		return
	var pp: Vector2 = (nodes[0] as Node2D).global_position
	var mine := global_position.distance_to(pp)
	if mine >= LABEL_SHOW_RADIUS:
		label.visible = false
		return
	# Only the closest interactable shows its name, so labels never pile up.
	var closest := true
	for other in get_tree().get_nodes_in_group("interactables"):
		if other != self and (other as Node2D).global_position.distance_to(pp) < mine:
			closest = false
			break
	label.visible = closest

func on_interact(player: Node) -> void:
	if dungeon_entry_id != "" or dungeon_object_id != "":
		var dc := get_tree().get_first_node_in_group("dungeon_controller")
		if dc == null:
			return
		if dungeon_entry_id != "":
			dc.enter(dungeon_entry_id)
		else:
			dc.interact(dungeon_object_id)
		return
	var quests := get_tree().get_first_node_in_group("quest_manager")
	if quests and quests.has_method("report_world_event"):
		if important and player and player.has_method("pause_automation"):
			player.pause_automation(true)
			# Brief pause for important story interacts; resume after event
			player.pause_automation(false)
		if quests.has_method("report_world_event_ex"):
			quests.report_world_event_ex(event_type, interact_id, instance_id, player)
		else:
			quests.report_world_event(event_type, interact_id, player)
