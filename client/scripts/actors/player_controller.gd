extends CharacterBody2D
## Stage 1 local player: joystick + tap move, skills, auto-combat interrupt.

signal died
signal loot_gained(item_id: String, amount: int)
signal hp_changed(hp: float, max_hp: float)
signal mp_changed(mp: float, max_mp: float)

@export var move_speed := 180.0
@export var max_hp := 100.0
@export var max_mp := 80.0
@export var attack_range := 70.0
@export var attack_damage := 18.0
@export var attack_cooldown := 0.55

@onready var animator: DirectionalAnimator = $DirectionalAnimator
@onready var interact_area: Area2D = $InteractArea

var hp: float
var mp: float
var auto_combat := false
var automation_paused := false
var tap_target := Vector2.ZERO
var tap_active := false
var joystick_dir := Vector2.ZERO
var _attack_cd := 0.0
var _auto_busy := false
var inventory: Dictionary = {}  # item_id -> amount
var gold := 0

func _ready() -> void:
	hp = max_hp
	mp = max_mp
	add_to_group("player")
	hp_changed.emit(hp, max_hp)
	mp_changed.emit(mp, max_mp)

func set_joystick(dir: Vector2) -> void:
	joystick_dir = dir
	if dir.length() > 0.15:
		tap_active = false  # manual override

func move_to_world(pos: Vector2) -> void:
	tap_target = pos
	tap_active = true

func pause_automation(paused: bool) -> void:
	automation_paused = paused
	if paused:
		auto_combat = false

func toggle_auto_combat() -> bool:
	if automation_paused:
		return false
	auto_combat = not auto_combat
	return auto_combat

func _physics_process(delta: float) -> void:
	z_index = int(global_position.y)
	_attack_cd = maxf(0.0, _attack_cd - delta)
	var desired := Vector2.ZERO
	if joystick_dir.length() > 0.15:
		desired = joystick_dir
		tap_active = false
	elif tap_active:
		var to := tap_target - global_position
		if to.length() < 6.0:
			tap_active = false
		else:
			desired = to.normalized()

	velocity = desired * move_speed
	move_and_slide()

	if desired.length() > 0.1:
		animator.set_direction_from_vector(desired)
		animator.play_run()
	else:
		animator.play_idle()

	if auto_combat and not automation_paused:
		_auto_combat_tick()

func _auto_combat_tick() -> void:
	if _auto_busy:
		return
	var target := _nearest_monster()
	if target == null:
		return
	var dist := global_position.distance_to(target.global_position)
	if dist > attack_range * 0.9:
		tap_target = target.global_position
		tap_active = true
	elif _attack_cd <= 0.0:
		_auto_busy = true
		await use_skill(0)
		_auto_busy = false

func _nearest_monster() -> Node2D:
	var best: Node2D = null
	var best_d := 99999.0
	for n in get_tree().get_nodes_in_group("monsters"):
		if not is_instance_valid(n) or n.get("hp") != null and float(n.hp) <= 0.0:
			continue
		var d: float = global_position.distance_to(n.global_position)
		if d < best_d:
			best_d = d
			best = n
	return best

func use_skill(slot: int) -> void:
	if hp <= 0.0 or _attack_cd > 0.0 or animator.is_attacking():
		return
	var costs := [0.0, 8.0, 12.0, 10.0, 14.0, 18.0]
	var mults := [1.0, 1.35, 1.6, 1.25, 1.8, 2.1]
	var cost: float = costs[clamp(slot, 0, 5)]
	if mp < cost:
		return
	mp -= cost
	mp_changed.emit(mp, max_mp)
	_attack_cd = attack_cooldown
	var facing := Vector2.RIGHT.rotated(_dir_to_angle())
	animator.set_direction_from_vector(facing)
	await animator.play_attack()
	var dmg: float = attack_damage * float(mults[clamp(slot, 0, 5)])
	_deal_damage_in_front(dmg, attack_range + float(slot) * 4.0)

func _dir_to_angle() -> float:
	# Match DirectionalAnimator dir index if exposed; approximate from velocity/joystick.
	var v := joystick_dir
	if v.length() < 0.1 and tap_active:
		v = (tap_target - global_position).normalized()
	if v.length() < 0.1:
		v = Vector2.DOWN
	return v.angle()

func _deal_damage_in_front(amount: float, reach: float) -> void:
	var origin := global_position
	for n in get_tree().get_nodes_in_group("monsters"):
		if not is_instance_valid(n):
			continue
		if origin.distance_to(n.global_position) <= reach:
			if n.has_method("apply_damage"):
				n.apply_damage(amount, self)

func apply_damage(amount: float, _from: Node = null) -> void:
	if hp <= 0.0:
		return
	hp = maxf(0.0, hp - amount)
	hp_changed.emit(hp, max_hp)
	if hp <= 0.0:
		died.emit()

func heal(amount: float) -> void:
	hp = minf(max_hp, hp + amount)
	hp_changed.emit(hp, max_hp)

func add_loot(item_id: String, amount: int = 1) -> void:
	inventory[item_id] = int(inventory.get(item_id, 0)) + amount
	loot_gained.emit(item_id, amount)

func add_gold(amount: int) -> void:
	gold += amount
	loot_gained.emit("gold", amount)

func try_interact() -> void:
	for a in interact_area.get_overlapping_areas():
		if a.has_method("on_interact"):
			a.on_interact(self)
			return
		var host := a.get_parent()
		if host and host.has_method("on_interact"):
			host.on_interact(self)
			return
	for b in interact_area.get_overlapping_bodies():
		if b.has_method("on_interact"):
			b.on_interact(self)
			return
