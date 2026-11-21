extends Node
## T-037: client side of the dungeon engine (docs/contracts/dungeon-engine.md).
## The server owns every fact (doors, puzzle, rewards). This node only
##  - asks (enter / interact / report / claim / leave),
##  - shows the server snapshot: current room visible, finished objects dimmed, spawns for live enemies,
##  - moves the existing Player node into the dungeon scene and back.
## Enemy deaths are *reported* (interim R-KILL-DNG until the server combat engine).

signal dungeon_entered(dungeon_id: String)
signal dungeon_left(reason: String)
signal room_changed(room_id: String)

const SCENES := {
	"DNG-01-001": "res://scenes/dungeon/dng_01_001.tscn",
}
const ROOM_HINTS := {
	"entrance": "dungeon.hint.entrance",
	"fight": "dungeon.hint.fight",
	"puzzle": "dungeon.hint.puzzle",
	"miniboss": "dungeon.hint.miniboss",
	"rest": "dungeon.hint.rest",
	"boss": "dungeon.hint.boss",
	"exit": "dungeon.hint.exit",
}

var _net: RefCounted = null
var _snapshot: Dictionary = {}
var _scene: Node2D = null
var _player: Node2D = null
var _home_parent: Node = null
var _home_pos := Vector2.ZERO
var _home_limits := Rect2i()
var _world_root: Node = null
var _spawned: Dictionary = {}   # spawn_id -> Node
var _last_room := ""
var _claiming := false

func _ready() -> void:
	add_to_group("dungeon_controller")

func is_inside() -> bool:
	return _scene != null

func get_snapshot() -> Dictionary:
	return _snapshot

# ----- requests --------------------------------------------------------------

func enter(dungeon_id: String) -> void:
	if not _bind_net():
		_hint(tr("dungeon.hint.need_online"))
		return
	if not SCENES.has(dungeon_id):
		_hint(tr("dungeon.hint.unavailable"))
		return
	_net.request_dungeon_enter(dungeon_id, "NORMAL")

func interact(object_id: String) -> void:
	if _snapshot.is_empty() or _net == null:
		return
	if object_id.begins_with("EXIT-"):
		claim()
		return
	_net.request_dungeon_interact(str(_snapshot.get("instance_id", "")), object_id)

func report_enemy_defeated(spawn_id: String) -> void:
	if _snapshot.is_empty() or _net == null:
		return
	_spawned.erase(spawn_id)
	_net.request_dungeon_enemy_defeated(str(_snapshot.get("instance_id", "")), spawn_id)

func claim() -> void:
	if _snapshot.is_empty() or _net == null or _claiming:
		return
	if str(_snapshot.get("status", "")) != "COMPLETED":
		_hint(tr("dungeon.hint.not_done"))
		return
	_claiming = true
	_net.request_dungeon_claim(str(_snapshot.get("instance_id", "")))

func leave() -> void:
	if _snapshot.is_empty() or _net == null:
		return
	_net.request_dungeon_leave(str(_snapshot.get("instance_id", "")))

# ----- server messages -------------------------------------------------------

func _bind_net() -> bool:
	if _net != null:
		return true
	var session := get_tree().get_first_node_in_group("online_session")
	if session == null or not session.has_method("get_net"):
		return false
	var net = session.get_net()
	if net == null:
		return false
	bind_net(net)
	return true

func bind_net(net: RefCounted) -> void:
	_net = net
	if not _net.dungeon_snapshot.is_connected(_on_snapshot):
		_net.dungeon_snapshot.connect(_on_snapshot)
	if not _net.dungeon_reward.is_connected(_on_reward):
		_net.dungeon_reward.connect(_on_reward)
	if _net.has_signal("command_error") and not _net.command_error.is_connected(_on_error):
		_net.command_error.connect(_on_error)

func _on_snapshot(snap: Dictionary) -> void:
	_snapshot = snap
	var status := str(snap.get("status", ""))
	if status in ["CLOSED", "EXPIRED", "ABANDONED"]:
		_exit_scene(status)
		return
	if _scene == null:
		_enter_scene(snap)
	_apply(snap)

func _on_reward(reward: Dictionary) -> void:
	_claiming = false
	var player := _get_player()
	if player != null and player.has_method("apply_server_loot"):
		player.apply_server_loot(int(reward.get("gold", 0)), int(reward.get("gold_total", 0)), reward.get("items", []))
	_snapshot = {}
	_exit_scene("REWARD_CLAIMED")

func _on_error(payload: Dictionary) -> void:
	_claiming = false
	var code := str(payload.get("code", ""))
	match code:
		"LEVEL_TOO_LOW":
			_hint(tr("dungeon.hint.level_low"))
		"REGION_CLOSED":
			_hint(tr("dungeon.hint.region_closed"))
		"STATE_CONFLICT":
			if _scene != null:
				_hint(tr("dungeon.hint.conflict"))

# ----- scene handling --------------------------------------------------------

func _get_player() -> Node2D:
	var nodes := get_tree().get_nodes_in_group("player")
	return nodes[0] as Node2D if not nodes.is_empty() else null

func _enter_scene(snap: Dictionary) -> void:
	var path: String = SCENES.get(str(snap.get("dungeon_id", "")), "")
	if path == "":
		return
	_player = _get_player()
	var packed: PackedScene = load(path)
	if _player == null or packed == null:
		return
	_scene = packed.instantiate() as Node2D
	_world_root = get_parent()
	_world_root.add_child(_scene)
	_home_parent = _player.get_parent()
	_home_pos = _player.global_position
	var cam := _player.get_node_or_null("Camera2D") as Camera2D
	if cam != null:
		_home_limits = Rect2i(cam.limit_left, cam.limit_top, cam.limit_right - cam.limit_left, cam.limit_bottom - cam.limit_top)
		cam.limit_left = -40
		cam.limit_top = -40
		cam.limit_right = 800
		cam.limit_bottom = 600
	_player.reparent(_scene.get_node("Playground"), false)
	if not _player.died.is_connected(_on_player_died):
		_player.died.connect(_on_player_died)
	# Pause the overworld while inside (its Player child is already moved out).
	_home_parent.process_mode = Node.PROCESS_MODE_DISABLED
	_home_parent.visible = false
	_last_room = ""
	dungeon_entered.emit(str(snap.get("dungeon_id", "")))

func _exit_scene(reason: String) -> void:
	_snapshot = {}
	_spawned.clear()
	_last_room = ""
	if _scene == null:
		return
	if _player != null and is_instance_valid(_player) and _home_parent != null:
		if _player.died.is_connected(_on_player_died):
			_player.died.disconnect(_on_player_died)
		_player.reparent(_home_parent, false)
		_player.global_position = _home_pos
		var cam := _player.get_node_or_null("Camera2D") as Camera2D
		if cam != null and _home_limits.size != Vector2i.ZERO:
			cam.limit_left = _home_limits.position.x
			cam.limit_top = _home_limits.position.y
			cam.limit_right = _home_limits.position.x + _home_limits.size.x
			cam.limit_bottom = _home_limits.position.y + _home_limits.size.y
	if _home_parent != null:
		_home_parent.process_mode = Node.PROCESS_MODE_INHERIT
		_home_parent.visible = true
	_scene.queue_free()
	_scene = null
	dungeon_left.emit(reason)

func _apply(snap: Dictionary) -> void:
	if _scene == null:
		return
	var rooms: Dictionary = snap.get("rooms", {})
	var current := str(snap.get("room_id", ""))
	for room_id in rooms.keys():
		var node := _scene.get_node_or_null("Rooms/%s" % str(room_id).get_slice("-", 3)) as Node2D
		if node == null:
			continue
		var active: bool = str(room_id) == current
		node.visible = active
		node.process_mode = Node.PROCESS_MODE_INHERIT if active else Node.PROCESS_MODE_DISABLED
		var rv: Dictionary = rooms[room_id]
		for o in node.get_node("Objects").get_children():
			var oid := str(o.get("dungeon_object_id"))
			var done: bool = bool(rv.get("objects", {}).get(oid, false))
			o.modulate = Color(1, 1, 1, 0.35) if done else Color(1, 1, 1, 1)
		if active:
			_sync_spawns(node, rv.get("spawns", {}))
	if current != _last_room:
		_last_room = current
		_enter_room(current)
		room_changed.emit(current)
	if str(snap.get("status", "")) == "COMPLETED":
		_hint(tr("dungeon.hint.done"))

func _enter_room(room_id: String) -> void:
	var node := _scene.get_node_or_null("Rooms/%s" % room_id.get_slice("-", 3)) as Node2D
	if node == null:
		return
	# A new room (or a respawn at a checkpoint): clear leftovers, place the player at the room entry.
	for sid in _spawned.keys():
		if is_instance_valid(_spawned[sid]):
			_spawned[sid].queue_free()
	_spawned.clear()
	var entry := node.get_node_or_null("Entry") as Node2D
	if entry != null and _player != null:
		_player.global_position = entry.global_position
		if _player.has_method("heal") and "max_hp" in _player:
			_player.heal(_player.max_hp)
	var kind := str(node.get_meta("kind", ""))
	if ROOM_HINTS.has(kind):
		_hint(tr(ROOM_HINTS[kind]))

func _sync_spawns(room_node: Node2D, spawns: Dictionary) -> void:
	for marker in room_node.get_node("Spawns").get_children():
		var sid := str(marker.name)
		if bool(spawns.get(sid, false)) or _spawned.has(sid):
			continue
		var path := str(marker.get_meta("monster_scene", ""))
		if path == "":
			continue
		var packed: PackedScene = load(path)
		if packed == null:
			continue
		var m: Node2D = packed.instantiate()
		m.set("dungeon_spawn_id", sid)
		for key in marker.get_meta("overrides", {}).keys():
			m.set(key, marker.get_meta("overrides")[key])
		_scene.get_node("Playground").add_child(m)
		m.global_position = marker.global_position
		_spawned[sid] = m

func _on_player_died() -> void:
	if _snapshot.is_empty() or _net == null:
		return
	_net.request_dungeon_player_defeated(str(_snapshot.get("instance_id", "")))

func _hint(text: String) -> void:
	var player := _get_player()
	if player != null and player.has_method("show_hint"):
		player.show_hint(text)
