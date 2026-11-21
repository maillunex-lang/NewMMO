extends SceneTree
## Headless T-038d: WorldController with a FakeTransport server. Marker WORLD_SMOKE_OK.
## join → snapshot (you + enemy + other player) → puppets, local monsters gone, HUD skills → cast request →
## events (damage number, telegraph) → movement intent → death state.

const FakeTransport = preload("res://scripts/net/fake_transport.gd")
const NetClient = preload("res://scripts/net/net_client.gd")

var _failed := false
var _rev := 0

func _init() -> void:
	call_deferred("_run")

func _assert(cond: bool, label: String) -> void:
	if not cond:
		push_error("ASSERT FAIL: %s" % label)
		_failed = true

func _s2c(type_name: String, payload: Dictionary, request_id: String = "") -> String:
	var msg := {
		"protocol_version": 1, "type": type_name, "session_id": "s1",
		"server_time": "2026-11-21T08:00:00.000Z", "revision": _rev, "trace_id": "t1", "payload": payload,
	}
	if request_id != "":
		msg["request_id"] = request_id
	return JSON.stringify(msg)

func _outbound(ft: RefCounted) -> Array:
	var res: Array = []
	for t in ft.pop_outbound():
		res.append(JSON.parse_string(t))
	return res

func _find(msgs: Array, type_name: String) -> Dictionary:
	for m in msgs:
		if str(m.get("type")) == type_name:
			return m
	return {}

func _run() -> void:
	if OS.get_environment("WORLD_SMOKE_FAIL") == "1":
		push_error("forced fail")
		quit(9)
		return
	var ft := FakeTransport.new()
	var net := NetClient.new()
	net.content_build_id = "build-test"
	net.configure(ft, func() -> String: return "ticket", "ws://127.0.0.1:18086/v1/ws", 42)
	_assert(net.connect_session() == OK, "connect")
	ft.pop_outbound()
	ft.inject_text(_s2c("session.ready", {"session_id": "s1", "character_id": "7", "heartbeat_ms": 25000, "content_build_id": "build-test"}))
	ft.inject_text(_s2c("state.snapshot", {"scopes": {}, "revision": 0}))
	net.poll(0.016)
	_assert(net.session_state == NetClient.SessionState.READY, "READY")

	_assert(change_scene_to_file("res://scenes/world/main.tscn") == OK, "load main")
	for i in range(40):
		await process_frame
	var main := root.get_child(root.get_child_count() - 1)
	var wc: Node = main.get_node_or_null("WorldController")
	_assert(wc != null, "WorldController in main.tscn")
	if wc == null:
		quit(1)
		return
	_assert(get_nodes_in_group("monsters").size() > 0, "offline monsters exist before the server joins")
	var session := Node.new()
	session.set_script(load("res://scripts/net/online_session.gd"))
	root.add_child(session)
	session.character = {"character_id": "7", "name": "Hero", "class_id": "CLS-01", "level": 1}
	wc.bind_net(net)
	var out := _outbound(ft)
	_assert(not _find(out, "world.join.request").is_empty(), "join request sent on bind")
	_assert(str(_find(out, "world.join.request").get("payload", {}).get("zone_id")) == "ZONE-01-DAWN-VALLEY", "zone id")

	var player: Node = main.get_node("DawnValley/Player")
	var px: float = player.global_position.x
	var py: float = player.global_position.y
	_rev = 1
	ft.inject_text(_s2c("world.snapshot", {
		"tick": 5, "tick_ms": 100, "you": 1, "zone_id": "ZONE-01-DAWN-VALLEY", "attack_reach": 80,
		"bounds": {"min_x": 0, "min_y": 0, "max_x": 2048, "max_y": 1400},
		"entities": [
			{"id": 1, "kind": "player", "ref": "7", "name": "Hero", "level": 1, "x": px, "y": py, "facing": 2, "moving": false, "hp": 400, "max_hp": 400},
			{"id": 2, "kind": "enemy", "ref": "ENM-01-004", "name": "Садовый шиповик", "level": 4, "x": px + 60, "y": py, "facing": 4, "moving": false, "hp": 300, "max_hp": 300},
			{"id": 3, "kind": "player", "ref": "9", "name": "Other", "level": 3, "x": px + 12, "y": py, "facing": 0, "moving": false, "hp": 380, "max_hp": 380},
			{"id": 4, "kind": "enemy", "ref": "ENM-01-004", "x": px + 79, "y": py, "hp": 80, "max_hp": 80},
			{"id": 5, "kind": "enemy", "ref": "ENM-01-004", "x": px + 5, "y": py, "hp": 0, "max_hp": 80, "dead": true},
		],
	}))
	net.poll(0.016)
	for i in range(4):
		await process_frame
	_assert(wc.is_active(), "world controller active after snapshot")
	_assert(get_nodes_in_group("monsters").is_empty(), "stage-1 local monsters removed online")
	_assert(get_nodes_in_group("server_enemies").size() == 3 and get_nodes_in_group("server_players").size() == 1, "puppets: 3 enemies, 1 other player")
	var hud := get_nodes_in_group("hud")
	_assert(hud.size() == 1, "HUD in group")
	var skills_box = hud[0].get_node("SafeArea/Chrome/Skills")
	var b0 := skills_box.get_child(0) as Button
	_assert(b0.text.begins_with("Удар бастиона") and not b0.disabled, "slot 1 = first supported Guard skill: %s" % b0.text)

	# Casting asks the server (target = nearest server enemy), never decides locally.
	player.use_skill(0)
	out = _outbound(ft)
	var cast := _find(out, "world.cast.request")
	_assert(not cast.is_empty(), "cast request sent")
	_assert(str(cast.get("payload", {}).get("skill_id")) == "CLS-01-SK01" and int(cast.get("payload", {}).get("target_id", 0)) == 2, "cast payload skill+target")
	_assert(str(cast.get("request_id", "")) != "", "cast carries request_id")

	# Updated server positions are outside the advertised radius even while the puppets interpolate.
	ft.inject_text(_s2c("world.delta", {"tick": 6, "ack_seq": 0, "upsert": [
		{"id": 2, "kind": "enemy", "x": px + 81, "y": py, "hp": 300},
		{"id": 4, "kind": "enemy", "x": px + 83, "y": py, "hp": 80},
	]}))
	net.poll(0.016)
	player.use_skill(0)
	_assert(_find(_outbound(ft), "world.cast.request").is_empty(), "manual cast requires a living enemy within server reach")

	# Server events: damage number + telegraph + self state (cooldown label).
	ft.inject_text(_s2c("world.event", {"tick": 6, "events": [
		{"type": "damage", "caster": 1, "target": 2, "skill": "CLS-01-SK01", "amount": 34, "hp": 266},
		{"type": "telegraph", "caster": 2, "skill": "AB-ENM-01-004-B", "amount": 800, "hp": 266, "aim_x": px + 60, "aim_y": py, "radius": 128},
	]}))
	ft.inject_text(_s2c("world.delta", {"tick": 6, "ack_seq": 0, "upsert": [
		{"id": 2, "kind": "enemy", "ref": "ENM-01-004", "x": px + 60, "y": py, "facing": 4, "moving": false, "hp": 266, "max_hp": 300}],
		"self": {"resource": 90, "resource_max": 100, "gcd_ms": 600, "cooldowns": {"CLS-01-SK01": 4200}}}))
	net.poll(0.016)
	for i in range(3):
		await process_frame
	wc._process(0.0)  # frame time in headless runs is not wall-clock: drive the label refresh explicitly
	_assert(get_nodes_in_group("damage_numbers").size() == 1, "damage number shown")
	_assert(get_nodes_in_group("telegraphs").size() == 1, "telegraph shown")
	_assert(abs(player.mp - 90.0) < 0.1, "resource bar from server")
	_assert(b0.text.contains("4.2"), "cooldown shown on the button: %s" % b0.text)

	# Movement intent: joystick → world.move.request with target + increasing seq (never a speed).
	ft.pop_outbound()
	player.joystick_dir = Vector2.RIGHT
	wc._process(0.11)  # one send interval
	out = _outbound(ft)
	var mv := _find(out, "world.move.request")
	_assert(not mv.is_empty(), "move intent sent")
	_assert(not mv.get("payload", {}).has("speed") and int(mv.get("payload", {}).get("seq", 0)) >= 1, "move payload has seq and no speed")
	player.joystick_dir = Vector2.ZERO
	wc._process(0.11)
	_assert(not _find(_outbound(ft), "world.stop.request").is_empty(), "stop sent when input ends")

	# Death comes from the server: movement frozen, hint, then respawn view.
	ft.inject_text(_s2c("world.delta", {"tick": 9, "ack_seq": 1, "upsert": [
		{"id": 1, "kind": "player", "ref": "7", "name": "Hero", "level": 1, "x": px, "y": py, "facing": 2, "moving": false, "hp": 0, "max_hp": 400, "dead": true}]}))
	net.poll(0.016)
	await process_frame
	_assert(bool(player.online_dead), "server death freezes the player")
	ft.inject_text(_s2c("world.delta", {"tick": 60, "ack_seq": 1, "upsert": [
		{"id": 1, "kind": "player", "ref": "7", "name": "Hero", "level": 1, "x": 620, "y": 720, "facing": 2, "moving": false, "hp": 400, "max_hp": 400}]}))
	net.poll(0.016)
	await process_frame
	_assert(not bool(player.online_dead) and player.hp == 400.0, "respawn restores the player")

	if _failed:
		quit(1)
		return
	print("WORLD_SMOKE_OK")
	quit(0)
