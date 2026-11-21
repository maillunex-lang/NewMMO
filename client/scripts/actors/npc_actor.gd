extends CharacterBody2D
## Canon NPC contact. Dialogue and quest progress go through QuestManager.

signal dialogue_requested(npc: Node)

@export var npc_id := "NPC-802"
@export var display_name := "Фейра Нисс"
@export var important_dialogue := true

@onready var animator: DirectionalAnimator = $DirectionalAnimator
@onready var prompt: Label = $Prompt

var _player_near := false

func _ready() -> void:
	add_to_group("npcs")
	if prompt:
		prompt.visible = false
		prompt.text = "Talk"
	if animator:
		animator.play_idle()

func _physics_process(_delta: float) -> void:
	z_index = int(global_position.y)
	var player := _get_player()
	if player == null:
		return
	_player_near = global_position.distance_to(player.global_position) < 70.0
	if prompt:
		prompt.visible = _player_near
	animator.set_direction_from_vector(player.global_position - global_position)
	animator.play_idle()

func on_interact(player: Node) -> void:
	if important_dialogue and player.has_method("pause_automation"):
		player.pause_automation(true)
	var quests := get_tree().get_first_node_in_group("quest_manager")
	if quests and quests.has_method("on_npc_talk"):
		quests.on_npc_talk(npc_id, player)
	else:
		dialogue_requested.emit(self)

func _get_player() -> Node2D:
	var nodes := get_tree().get_nodes_in_group("player")
	if nodes.is_empty():
		return null
	return nodes[0] as Node2D
