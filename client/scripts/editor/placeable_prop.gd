@tool
extends Node2D
## Editable in the Godot Inspector. Props are saved as scene instances.
@export var blocks_movement := false
@export var server_object_id := ""
@export var display_name := "Decoration"
@export var depth_from_y := true

func _process(_delta: float) -> void:
	if depth_from_y and not Engine.is_editor_hint():
		z_index = int(global_position.y)
