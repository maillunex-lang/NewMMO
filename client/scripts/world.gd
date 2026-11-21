extends Node2D

@onready var player: Node2D = $Player
var target: Vector2
var moving := false
const SPEED := 180.0

func _ready() -> void:
    target = player.position

func _unhandled_input(event: InputEvent) -> void:
    if event is InputEventScreenTouch and event.pressed:
        target = get_global_mouse_position()
        moving = true
    elif event is InputEventMouseButton and event.pressed and event.button_index == MOUSE_BUTTON_LEFT:
        target = get_global_mouse_position()
        moving = true

func _physics_process(delta: float) -> void:
    if moving:
        player.position = player.position.move_toward(target, SPEED * delta)
        if player.position.distance_to(target) < 2.0:
            moving = false
