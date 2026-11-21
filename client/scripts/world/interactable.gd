extends Area2D
## World / POI interactable for quest objectives (canon target IDs).

@export var interact_id := "POI"
@export var display_name := "Объект"
@export var event_type := "world_interact"  # world_interact | npc_interact | escort_checkpoint
@export var instance_id := ""  # required when registry count>1 (e.g. POI-CLUE#1)
@export var important := false

@onready var label: Label = $Label

func _ready() -> void:
	monitoring = false
	monitorable = true
	collision_layer = 1
	collision_mask = 0
	if label:
		label.text = display_name
	add_to_group("interactables")

func on_interact(player: Node) -> void:
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
