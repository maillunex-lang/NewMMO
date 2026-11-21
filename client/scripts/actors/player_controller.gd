extends CharacterBody2D
## Stage 1 local player: joystick + tap move, skills, auto-combat interrupt.

signal died
signal loot_gained(item_id: String, amount: int)
signal hint(text: String)
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
var online_dead := false  # T-038d: server says this character is dead (movement frozen until respawn)
## Autopath: obstacle-aware routing for taps / quest travel. `tap_target` is always the CURRENT waypoint, so
## the online movement intent (sent from tap_target) makes the server walk the same polyline.
var auto_path: AutoPath = null
var _path := PackedVector2Array()
var _path_i := 0
var _path_goal := Vector2.INF
var _arrive_target: Node2D = null
var _stuck_t := 0.0
var _replans := 0
const WAYPOINT_REACH := 10.0
const ARRIVE_INTERACT_DIST := 120.0
const REPLAN_GOAL_SHIFT := 48.0

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
		cancel_path()

## Walk to `pos`, routing around obstacles. `interact_with` is interacted with automatically on arrival.
## Calling it again with (almost) the same goal keeps the current route, so it is cheap to call every tick.
func move_to_world(pos: Vector2, interact_with: Node2D = null) -> void:
	if tap_active and _path.size() > 0 and _path_goal != Vector2.INF and pos.distance_to(_path_goal) < REPLAN_GOAL_SHIFT:
		if interact_with != null:
			_arrive_target = interact_with
		return
	_arrive_target = interact_with
	_replans = 0
	_plan_to(pos)

func _plan_to(pos: Vector2) -> void:
	_path_goal = pos
	_path_i = 0
	_stuck_t = 0.0
	_path = auto_path.find_path(global_position, pos) if auto_path != null and auto_path.is_ready() else PackedVector2Array([pos])
	if _path.is_empty():
		_path_goal = Vector2.INF
		tap_active = false
		show_hint("Путь не найден")
		return
	tap_target = _path[0]
	tap_active = true

func cancel_path() -> void:
	_path = PackedVector2Array()
	_path_i = 0
	_path_goal = Vector2.INF
	_arrive_target = null

func has_path() -> bool:
	return _path.size() > 0

func _finish_path() -> void:
	var target := _arrive_target
	cancel_path()
	tap_active = false
	if target != null and is_instance_valid(target) and target.has_method("on_interact") \
			and global_position.distance_to(target.global_position) <= ARRIVE_INTERACT_DIST:
		target.on_interact(self)

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
	if online_dead:
		velocity = Vector2.ZERO
		move_and_slide()
		animator.play_idle()
		return
	_attack_cd = maxf(0.0, _attack_cd - delta)
	var desired := Vector2.ZERO
	if joystick_dir.length() > 0.15:
		desired = joystick_dir
		tap_active = false
		cancel_path()
	elif tap_active:
		var to := tap_target - global_position
		if _path.size() > 0 and to.length() < WAYPOINT_REACH:
			_path_i += 1
			if _path_i >= _path.size():
				_finish_path()
				to = Vector2.ZERO
			else:
				tap_target = _path[_path_i]
				to = tap_target - global_position
		if to.length() < 6.0:
			if _path.size() == 0:
				tap_active = false
		else:
			desired = to.normalized()

	var before := global_position
	velocity = desired * move_speed
	move_and_slide()
	_check_stuck(before, desired, delta)

	if desired.length() > 0.1:
		animator.set_direction_from_vector(desired)
		animator.play_run()
	else:
		animator.play_idle()

	if auto_combat and not automation_paused:
		_auto_combat_tick()

## Blocked by something the grid did not predict (NPC, monster): re-plan twice, then give up visibly.
func _check_stuck(before: Vector2, desired: Vector2, delta: float) -> void:
	if _path.size() == 0 or desired.length() < 0.1:
		_stuck_t = 0.0
		return
	if global_position.distance_to(before) < move_speed * delta * 0.25:
		_stuck_t += delta
	else:
		_stuck_t = 0.0
	if _stuck_t < 0.6:
		return
	_stuck_t = 0.0
	if _path_goal != Vector2.INF and global_position.distance_to(_path_goal) <= ARRIVE_INTERACT_DIST * 0.6:
		_finish_path()  # pressed against a prop right next to the goal: close enough
		return
	if _replans >= 2:
		cancel_path()
		tap_active = false
		show_hint("Путь заблокирован")
		return
	_replans += 1
	var goal := _path_goal
	var target := _arrive_target
	_plan_to(goal)
	_arrive_target = target

func _auto_combat_tick() -> void:
	var wc := get_tree().get_first_node_in_group("world_controller")
	if wc != null and wc.is_active():
		wc.auto_tick()  # online: the server decides, we only choose where to go and what to cast
		return
	if _auto_busy:
		return
	var target := _nearest_monster()
	if target == null:
		return
	var dist := global_position.distance_to(target.global_position)
	if dist > attack_range * 0.9:
		move_to_world(target.global_position)
	elif _attack_cd <= 0.0:
		_auto_busy = true
		await use_skill(0)
		_auto_busy = false

func _nearest_monster(reach: float = INF) -> Node2D:
	var best: Node2D = null
	var best_d := reach
	for n in get_tree().get_nodes_in_group("monsters"):
		if not is_instance_valid(n) or n.is_queued_for_deletion() or n.get("hp") != null and float(n.hp) <= 0.0:
			continue
		var d: float = global_position.distance_to(n.global_position)
		if d < best_d or (d == best_d and best == null):
			best_d = d
			best = n
	return best

func use_skill(slot: int) -> void:
	var wc := get_tree().get_first_node_in_group("world_controller")
	if wc != null and wc.is_active():
		wc.cast_slot(slot)
		return
	if hp <= 0.0 or _attack_cd > 0.0 or animator.is_attacking():
		return
	var costs := [0.0, 8.0, 12.0, 10.0, 14.0, 18.0]
	var mults := [1.0, 1.35, 1.6, 1.25, 1.8, 2.1]
	var skill := clampi(slot, 0, 5)
	var cost: float = costs[skill]
	if mp < cost:
		return
	var reach := attack_range + float(skill) * 4.0
	var manual := not auto_combat
	var target: Node2D = null
	if manual:
		target = _nearest_monster(reach)
		if target == null:
			show_hint(tr("world.hint.no_target"))
			return
	mp -= cost
	mp_changed.emit(mp, max_mp)
	_attack_cd = attack_cooldown
	var facing := (target.global_position - global_position) if manual else Vector2.RIGHT.rotated(_dir_to_angle())
	animator.set_direction_from_vector(facing)
	await animator.play_attack()
	var dmg := attack_damage * float(mults[skill])
	if not manual:
		_deal_damage_in_front(dmg, reach)
	elif is_instance_valid(target) and not target.is_queued_for_deletion() and global_position.distance_to(target.global_position) <= reach:
		target.apply_damage(dmg, self)

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

## T-034: apply a server-rolled drop. Gold total is authoritative; item stacks arrive via inventory.delta.
func apply_server_loot(gold_gained: int, gold_total: int, items: Array) -> void:
	gold = gold_total
	if gold_gained > 0:
		loot_gained.emit("gold", gold_gained)
	for it in items:
		if typeof(it) == TYPE_DICTIONARY:
			loot_gained.emit(str(it.get("item_id", "")), int(it.get("quantity", 1)))

func add_gold(amount: int) -> void:
	gold += amount
	loot_gained.emit("gold", amount)

## Short player-facing message (HUD toast); used when an interaction has no effect.
func show_hint(text: String) -> void:
	hint.emit(text)

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
	show_hint(tr("ui.hint_nothing_here"))
