extends CharacterBody2D
## Stage 1 local monster with patrol, aggro, and loot table.

signal died(monster: Node)

@export var monster_id := "slime"
@export var display_name := "Forest Slime"
@export var max_hp := 40.0
@export var move_speed := 90.0
@export var aggro_radius := 160.0
@export var attack_range := 42.0
@export var attack_damage := 8.0
@export var attack_cooldown := 1.1
@export var gold_min := 1
@export var gold_max := 4
@export var loot_item := "slime_gel"
@export var loot_chance := 0.75

@onready var animator: DirectionalAnimator = $DirectionalAnimator
@onready var hp_bar: ProgressBar = $HpBar

var hp: float
var _cd := 0.0
var _home := Vector2.ZERO
var _patrol_target := Vector2.ZERO
var _alive := true

func _ready() -> void:
	hp = max_hp
	_home = global_position
	_patrol_target = _home + Vector2(randf_range(-80, 80), randf_range(-60, 60))
	add_to_group("monsters")
	if hp_bar:
		hp_bar.max_value = max_hp
		hp_bar.value = hp

func _physics_process(delta: float) -> void:
	z_index = int(global_position.y)
	if not _alive:
		return
	_cd = maxf(0.0, _cd - delta)
	var player := _get_player()
	var desired := Vector2.ZERO
	if player and global_position.distance_to(player.global_position) <= aggro_radius:
		var dist := global_position.distance_to(player.global_position)
		if dist > attack_range:
			desired = (player.global_position - global_position).normalized()
		elif _cd <= 0.0:
			_cd = attack_cooldown
			animator.set_direction_from_vector(player.global_position - global_position)
			animator.play_attack()
			if player.has_method("apply_damage"):
				player.apply_damage(attack_damage, self)
	else:
		if global_position.distance_to(_patrol_target) < 10.0:
			_patrol_target = _home + Vector2(randf_range(-90, 90), randf_range(-70, 70))
		desired = (_patrol_target - global_position).normalized() * 0.5

	velocity = desired * move_speed
	move_and_slide()
	if desired.length() > 0.05:
		animator.set_direction_from_vector(desired)
		animator.play_run()
	else:
		animator.play_idle()

func apply_damage(amount: float, from: Node = null) -> void:
	if not _alive:
		return
	hp = maxf(0.0, hp - amount)
	if hp_bar:
		hp_bar.value = hp
	if from is Node2D:
		animator.set_direction_from_vector((from as Node2D).global_position - global_position)
	if hp <= 0.0:
		_die(from)

func _die(killer: Node) -> void:
	_alive = false
	died.emit(self)
	if killer and killer.has_method("add_gold"):
		killer.add_gold(randi_range(gold_min, gold_max))
	if killer and killer.has_method("add_loot") and randf() <= loot_chance:
		killer.add_loot(loot_item, 1)
	var quests := get_tree().get_first_node_in_group("quest_manager")
	if quests and quests.has_method("on_monster_killed"):
		quests.on_monster_killed(monster_id)
	queue_free()

func _get_player() -> Node2D:
	var nodes := get_tree().get_nodes_in_group("player")
	if nodes.is_empty():
		return null
	return nodes[0] as Node2D
