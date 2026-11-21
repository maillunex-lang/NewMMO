extends Node2D
## Test double for an interactable: counts on_interact calls.
signal interacted
func on_interact(_player: Node) -> void:
	interacted.emit()
