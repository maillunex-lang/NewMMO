extends Node
## T-038d: client of the server-authoritative zone simulation (docs/contracts/world-sim.md, combat-engine.md).
## Online only. The server owns positions, HP, enemies, damage and loot; this node
##  - joins the zone and renders other entities as puppets (smoothed snapshots/deltas),
##  - sends movement INTENT (target + seq, never a speed) and keeps the local player predicted + reconciled,
##  - maps the HUD skill buttons to the class skills and sends world.cast.request,
##  - shows server events (damage numbers, telegraphs, deaths) and the cooldown/resource state.
## Offline (no session) nothing here runs and the stage-1 local combat stays as it was.

const NetClient = preload("res://scripts/net/net_client.gd")
const ZONE_ID := "ZONE-01-DAWN-VALLEY"
const PUPPET_SCENE := preload("res://scenes/world/server_puppet.tscn")
const TELEGRAPH := preload("res://scripts/world/telegraph.gd")
const SEND_INTERVAL := 0.1
const CAST_REACH := 420.0
# Compatibility with servers that predate the attack_reach snapshot field.
const LEGACY_MELEE_REACH := 112.0
const LEGACY_RANGED_REACH := 320.0
const SUPPORTED := {"damage": ["enemy"], "shield": ["self"], "heal": ["ally", "self"]}
# canon enemy id → [sheet, frame px, sprite offset y, hp bar y]
const ENEMY_ART := {
	"ENM-01-002": ["res://assets/monsters/goblin_sheet.png", 96, -41.0, -100.0],
	"ENM-01-201": ["res://assets/monsters/skeleton_sheet.png", 112, -35.0, -80.0],
	"ENM-01-301": ["res://assets/monsters/ogre_sheet.png", 144, -52.0, -130.0],
	"ENM-01-001": ["res://assets/monsters/bandit_sheet.png", 96, -41.0, -100.0],
	"ENM-01-003": ["res://assets/monsters/wolf_sheet.png", 96, -41.0, -100.0],
	"ENM-01-004": ["res://assets/monsters/slime_sheet.png", 96, -34.0, -62.0],
}
const DEFAULT_ENEMY_ART := ["res://assets/monsters/goblin_sheet.png", 96, -41.0, -100.0]
const PLAYER_ART := ["res://assets/characters/guardian_sheet.png", 96, -41.0, -100.0]

signal joined(zone_id: String)

var _net: RefCounted = null
var _active := false
var _encounter := ""
var _waiting_snapshot := false
var _boss_marker: Node2D = null
var _you := 0
var _entities: Dictionary = {}      # entity id -> ServerPuppet
var _player: Node2D = null
var _seq := 0
var _send_t := 0.0
var _last_target := Vector2.INF
var _sent_move := false
var _skills: Array = []             # slot -> skill dict (id, name, target, ...)
var _all_skills: Dictionary = {}
var _class_id := ""
var _level := 1
var _self_state: Dictionary = {}
var _server_position := Vector2.ZERO
var _attack_reach := 0.0
var _auto_cast_timer := 0.0

func _ready() -> void:
	add_to_group("world_controller")
	set_process(false)
	var f := FileAccess.open("res://data/skills.json", FileAccess.READ)
	if f != null:
		var parsed: Variant = JSON.parse_string(f.get_as_text())
		if typeof(parsed) == TYPE_DICTIONARY:
			_all_skills = (parsed as Dictionary).get("classes", {})

func is_active() -> bool:
	return _active

func get_entities() -> Dictionary:
	return _entities

func bind_net(net: RefCounted) -> void:
	_net = net
	for pair in [["world_snapshot", "_on_snapshot"], ["world_delta", "_on_delta"], ["world_event", "_on_event"],
			["world_cast_result", "_on_cast_result"], ["command_error", "_on_error"], ["session_ready", "_on_session_ready"]]:
		var sig := Signal(_net, pair[0])
		var cb := Callable(self, pair[1])
		if _net.has_signal(pair[0]) and not sig.is_connected(cb):
			sig.connect(cb)
	if int(_net.session_state) == NetClient.SessionState.READY:
		_join()

func _on_session_ready(_payload: Dictionary) -> void:
	_waiting_snapshot = true
	if _encounter == "":
		_join()

func _join() -> void:
	var session := get_tree().get_first_node_in_group("online_session")
	if session != null:
		_class_id = str(session.get("character").get("class_id", ""))
	_net.world_join(ZONE_ID)

# ----- server messages ---------------------------------------------------------

func _get_player() -> Node2D:
	if _player == null or not is_instance_valid(_player):
		var nodes := get_tree().get_nodes_in_group("player")
		_player = nodes[0] as Node2D if not nodes.is_empty() else null
	return _player

func enter_dungeon_combat(snapshot: Dictionary) -> void:
	_encounter = str(snapshot.get("encounter_id", ""))
	_reset_stream(snapshot)

func exit_dungeon_combat() -> void:
	_clear_boss_marker()
	_encounter = ""
	_waiting_snapshot = true
	for id in _entities.keys():
		_remove_puppet(id)
	if _net != null:
		_join()

func _on_snapshot(p: Dictionary) -> void:
	if _encounter == "":
		_reset_stream(p)

func _reset_stream(p: Dictionary) -> void:
	# The first snapshot may arrive before the initial session-ready callback.
	# Resolve the class from the persisted character before building skill slots.
	var session := get_tree().get_first_node_in_group("online_session")
	if session != null and typeof(session.get("character")) == TYPE_DICTIONARY:
		_class_id = str(session.get("character").get("class_id", _class_id))
	_clear_boss_marker()
	_you = int(p.get("you", 0))
	var legacy_reach := LEGACY_MELEE_REACH if _class_id in ["CLS-01", "CLS-02"] else LEGACY_RANGED_REACH
	_attack_reach = float(p.get("attack_reach", legacy_reach))
	_last_target = Vector2.INF
	_sent_move = false
	_self_state.clear()
	for id in _entities.keys():
		_remove_puppet(id)
	for group in ["monsters", "telegraphs", "damage_numbers"]:
		for n in get_tree().get_nodes_in_group(group):
			n.queue_free()
	for v in p.get("entities", []):
		if int(v.get("id", 0)) == _you:
			var player := _get_player()
			if player != null:
				player.global_position = Vector2(float(v["x"]), float(v["y"]))
				player.tap_active = false
				player.cancel_path()
		_apply_view(v)
	_active = true
	_waiting_snapshot = false
	set_process(true)
	_rebuild_skills()
	_auto_cast_timer = 0.0
	joined.emit(str(p.get("zone_id", ZONE_ID)))

func _on_delta(p: Dictionary) -> void:
	if not _active or _waiting_snapshot or str(p.get("encounter_id", "")) != _encounter:
		return
	for v in p.get("upsert", []):
		_apply_view(v)
	for id in p.get("remove", []):
		_remove_puppet(int(id))
	if p.has("self"):
		_apply_self(p["self"])

func _apply_view(v: Dictionary) -> void:
	var id := int(v.get("id", 0))
	if id == _you:
		_apply_you(v)
		return
	if _entities.has(id) and is_instance_valid(_entities[id]):
		_entities[id].apply_view(v)
		return
	var art: Array = PLAYER_ART if str(v.get("kind", "")) == "player" else ENEMY_ART.get(str(v.get("ref", "")), DEFAULT_ENEMY_ART)
	var tex: Texture2D = load(art[0])
	var pup = PUPPET_SCENE.instantiate()
	pup.setup(v, tex, int(art[1]), float(art[2]), float(art[3]))
	var parent := _get_player().get_parent() if _get_player() != null else get_parent()
	parent.add_child(pup)
	pup.apply_view(v)
	_entities[id] = pup

func _remove_puppet(id: int) -> void:
	if _entities.has(id):
		if is_instance_valid(_entities[id]):
			_entities[id].queue_free()
		_entities.erase(id)

func _apply_you(v: Dictionary) -> void:
	var pl := _get_player()
	if pl == null:
		return
	_level = int(v.get("level", _level))
	pl.max_hp = float(v.get("max_hp", pl.max_hp))
	pl.hp = float(v.get("hp", pl.hp))
	pl.hp_changed.emit(pl.hp, pl.max_hp)
	pl.set("online_dead", bool(v.get("dead", false)))
	_server_position = Vector2(float(v.get("x", pl.global_position.x)), float(v.get("y", pl.global_position.y)))
	var drift := pl.global_position.distance_to(_server_position)
	if drift > 160.0 or bool(v.get("dead", false)):
		pl.global_position = _server_position        # respawn / correction after a long stall
		pl.tap_active = false
		pl.cancel_path()
	elif drift > 40.0:
		pl.global_position = pl.global_position.lerp(_server_position, 0.15)

func _apply_self(s: Dictionary) -> void:
	_self_state = s
	var pl := _get_player()
	if pl != null:
		pl.max_mp = float(s.get("resource_max", 100))
		pl.mp = float(s.get("resource", 0))
		pl.mp_changed.emit(pl.mp, pl.max_mp)
	_refresh_buttons()

func _on_event(p: Dictionary) -> void:
	if _waiting_snapshot or str(p.get("encounter_id", "")) != _encounter:
		return
	for e in p.get("events", []):
		var t := str(e.get("type", ""))
		var target := int(e.get("target", 0))
		match t:
			"damage":
				_float_text(target, str(int(e.get("amount", 0))), Color(1, 0.85, 0.2) if bool(e.get("crit", false)) else (Color(1, 0.4, 0.4) if target == _you else Color.WHITE))
			"heal":
				_float_text(target, "+%d" % int(e.get("amount", 0)), Color(0.4, 1, 0.5))
			"shield":
				_float_text(target, "+%d" % int(e.get("amount", 0)), Color(0.5, 0.8, 1))
			"death":
				if target == _you:
					_hint(tr("world.hint.you_died"))
			"respawn":
				if target == _you:
					_hint(tr("world.hint.respawned"))
			"witness_telegraph", "sector_telegraph", "countdown_prepare":
				_clear_boss_marker()
				_boss_marker = TELEGRAPH.new()
				var seconds := float(e.get("amount", 0)) / 1000.0
				if t == "witness_telegraph":
					_boss_marker.setup_safe_areas(e.get("safe_areas", []), seconds)
				else:
					_boss_marker.setup_sectors(e.get("sectors", []), seconds, t == "countdown_prepare")
				var pl := _get_player()
				var parent := pl.get_parent() if pl != null else get_parent()
				parent.add_child(_boss_marker)
			"witness_end", "sector_end":
				_clear_boss_marker()
			"boss_vulnerable":
				_hint(tr("dungeon.countdown.vulnerable"))
			"telegraph":
				var tg = TELEGRAPH.new()
				tg.setup(float(e.get("radius", 64)), float(e.get("amount", 800)) / 1000.0)
				var parent := _get_player().get_parent() if _get_player() != null else get_parent()
				parent.add_child(tg)
				tg.global_position = Vector2(float(e.get("aim_x", 0)), float(e.get("aim_y", 0)))

func _clear_boss_marker() -> void:
	if is_instance_valid(_boss_marker):
		_boss_marker.queue_free()
	_boss_marker = null

func _on_cast_result(_p: Dictionary) -> void:
	pass  # damage/heal numbers come from world.event; cooldowns from the self state in world.delta

func _on_error(p: Dictionary) -> void:
	match str(p.get("code", "")):
		"OUT_OF_RANGE":
			_hint(tr("world.hint.out_of_range"))
		"NOT_ENOUGH_RESOURCE":
			_hint(tr("world.hint.no_resource"))
		"INVALID_TARGET":
			_hint(tr("world.hint.no_target"))
		"UNSUPPORTED_SKILL":
			_hint(tr("world.hint.unsupported"))
		"DEAD":
			_hint(tr("world.hint.you_died"))

# ----- movement intent -----------------------------------------------------------

func _process(delta: float) -> void:
	_auto_cast_timer = maxf(0.0, _auto_cast_timer - delta)
	if _waiting_snapshot:
		return
	_send_t += delta
	_refresh_cooldown_labels()
	if _send_t >= SEND_INTERVAL:
		_send_t = 0.0
		_send_move()

func _send_move() -> void:
	var pl := _get_player()
	if pl == null or _net == null or bool(pl.get("online_dead")):
		return
	var target := Vector2.INF
	if pl.joystick_dir.length() > 0.15:
		target = pl.global_position + pl.joystick_dir.normalized() * 400.0
	elif pl.tap_active:
		target = pl.tap_target
	if target != Vector2.INF:
		if _last_target == Vector2.INF or target.distance_to(_last_target) > 8.0:
			_seq += 1
			_net.world_move(_seq, target.x, target.y, _encounter)
			_last_target = target
			_sent_move = true
	elif _sent_move:
		_seq += 1
		_net.world_stop(_seq, _encounter)
		_sent_move = false
		_last_target = Vector2.INF

# ----- skills -------------------------------------------------------------------------

func _rebuild_skills() -> void:
	_skills = []
	for s in _all_skills.get(_class_id, []):
		var kinds: Array = SUPPORTED.get(str(s.get("effect_kind", "")), [])
		if str(s.get("target", "")) in kinds and int(s.get("level", 1)) <= _level and _skills.size() < 6:
			_skills.append(s)
	_refresh_buttons()

func _hud_buttons() -> Array:
	var hud := get_tree().get_first_node_in_group("hud")
	if hud == null:
		return []
	var box := hud.get_node_or_null("SafeArea/Chrome/Skills")
	return box.get_children() if box != null else []

func _refresh_buttons() -> void:
	var buttons := _hud_buttons()
	for i in range(buttons.size()):
		var b := buttons[i] as Button
		if b == null:
			continue
		if i < _skills.size():
			b.text = str(_skills[i].get("name", "?"))
			b.disabled = false
		else:
			b.text = "—"
			b.disabled = true

func _refresh_cooldown_labels() -> void:
	var cds: Dictionary = _self_state.get("cooldowns", {})
	var buttons := _hud_buttons()
	for i in range(min(buttons.size(), _skills.size())):
		var b := buttons[i] as Button
		var ms := int(cds.get(str(_skills[i].get("id", "")), 0))
		b.text = str(_skills[i].get("name", "?")) + ("\n%.1f" % (ms / 1000.0) if ms > 0 else "")

func nearest_enemy(max_dist: float = CAST_REACH, use_server_positions: bool = false):
	var pl := _get_player()
	if pl == null:
		return null
	var origin := _server_position if use_server_positions else pl.global_position
	var best = null
	var best_d := max_dist
	for id in _entities.keys():
		var pup = _entities[id]
		if not is_instance_valid(pup) or str(pup.get("kind")) != "enemy" or not pup.is_alive():
			continue
		var position: Vector2 = pup.target_pos if use_server_positions else pup.global_position
		var d := origin.distance_to(position)
		if d < best_d or (d == best_d and best == null):
			best_d = d
			best = pup
	return best

func cast_slot(slot: int) -> void:
	if not _active or _waiting_snapshot or _net == null or slot < 0 or slot >= _skills.size():
		return
	var pl := _get_player()
	if pl == null or bool(pl.get("online_dead")):
		return
	var skill: Dictionary = _skills[slot]
	var target_id := 0
	if str(skill.get("target", "")) == "enemy":
		var manual: bool = not pl.auto_combat
		var e = nearest_enemy(_attack_reach if manual else CAST_REACH, manual)
		if e == null:
			_hint(tr("world.hint.no_target"))
			return
		target_id = int(e.get("entity_id"))
		if manual:
			pl.animator.set_direction_from_vector(e.global_position - pl.global_position)
	_net.request_world_cast(str(skill.get("id", "")), target_id, _encounter)
	pl.animator.play_attack()  # visual only; the result is the server's

func auto_tick() -> void:
	if not _active or _waiting_snapshot or _skills.is_empty():
		return
	var pl := _get_player()
	if pl == null or bool(pl.get("online_dead")):
		return
	# Ranged classes should cast from their server-defined reach instead of
	# running into melee range. Movement and damage remain server-authoritative.
	var e = nearest_enemy(900.0, true)
	if e == null:
		pl.tap_active = false
		pl.cancel_path()
		return
	var dist := _server_position.distance_to(e.target_pos)
	var reach := maxf(65.0, _attack_reach - 35.0)
	if dist > reach:
		pl.move_to_world(e.target_pos)
		return
	pl.tap_active = false
	pl.cancel_path()
	if _auto_cast_timer > 0.0:
		return
	var cds: Dictionary = _self_state.get("cooldowns", {})
	for slot in range(_skills.size()):
		var skill: Dictionary = _skills[slot]
		if str(skill.get("target", "")) != "enemy":
			continue
		if int(cds.get(str(skill.get("id", "")), 0)) > 0:
			continue
		if float(_self_state.get("resource", 0)) < float(skill.get("cost", 0)):
			continue
		_auto_cast_timer = 0.5
		cast_slot(slot)
		return

# ----- helpers -------------------------------------------------------------------------

func _float_text(entity_id: int, text: String, color: Color) -> void:
	var node: Node2D = null
	if entity_id == _you:
		node = _get_player()
	elif _entities.has(entity_id) and is_instance_valid(_entities[entity_id]):
		node = _entities[entity_id]
	if node == null:
		return
	var l := Label.new()
	l.text = text
	l.modulate = color
	l.z_index = 4096
	l.add_theme_font_size_override("font_size", 18)
	l.add_to_group("damage_numbers")
	node.get_parent().add_child(l)
	l.global_position = node.global_position + Vector2(-12, -90)
	var tw := l.create_tween()
	tw.tween_property(l, "position:y", l.position.y - 36.0, 0.8)
	tw.parallel().tween_property(l, "modulate:a", 0.0, 0.8)
	tw.tween_callback(l.queue_free)

func _hint(text: String) -> void:
	var pl := _get_player()
	if pl != null and pl.has_method("show_hint"):
		pl.show_hint(text)
