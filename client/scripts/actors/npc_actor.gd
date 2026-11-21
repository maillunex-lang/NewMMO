extends CharacterBody2D
## Interactable story NPC. Important dialogue pauses player automation.

signal dialogue_requested(npc: Node)

@export var npc_id := "elder_mira"
@export var display_name := "Elder Mira"
@export var important_dialogue := true
@export_multiline var lines: PackedStringArray = PackedStringArray([
	"Welcome to Dawn Valley, traveler.",
	"Shadows gather in the Whispering woods.",
	"Clear the path of slimes, then return to me."
])

@onready var animator: DirectionalAnimator = $DirectionalAnimator
@onready var prompt: Label = $Prompt

var _player_near := false

func _ready() -> void:
	add_to_group("npcs")
	if prompt:
		prompt.visible = false
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
	if player:
		animator.set_direction_from_vector(player.global_position - global_position)
		animator.play_idle()

func on_interact(player: Node) -> void:
	if important_dialogue and player.has_method("pause_automation"):
		player.pause_automation(true)
	dialogue_requested.emit(self)

func _get_player() -> Node2D:
	var nodes := get_tree().get_nodes_in_group("player")
	if nodes.is_empty():
		return null
	return nodes[0] as Node2D
