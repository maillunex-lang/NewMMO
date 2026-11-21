extends SceneTree
## Headless T-037: dungeon controller + scene with a FakeTransport server. Marker DUNGEON_SMOKE_OK.
## Flow: enter → snapshot (R01) → interact request → snapshot (R02) spawns → kill report →
## player defeat report → reward → player back in the overworld.

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

func _snap(room: String, status: String, spawns_r02: Dictionary = {}) -> Dictionary:
	var rooms := {}
	for i in range(1, 8):
		var rid := "DNG-01-001-R0%d" % i
		rooms[rid] = {"state": "IN_PROGRESS" if rid == room else "LOCKED", "objects": {}, "spawns": {}}
	rooms["DNG-01-001-R02"]["spawns"] = spawns_r02
	return {
		"instance_id": "11111111-1111-4111-8111-111111111111", "dungeon_id": "DNG-01-001", "mode": "NORMAL",
		"status": status, "room_id": room, "checkpoint_room_id": "DNG-01-001-R01", "event_seq": 1,
		"rooms": rooms, "attempts": 0, "revision": _rev,
	}

func _last_out(ft: RefCounted) -> Dictionary:
	var out: Array = ft.pop_outbound()
	if out.is_empty():
		return {}
	return JSON.parse_string(out[out.size() - 1])

func _run() -> void:
	if OS.get_environment("DUNGEON_SMOKE_FAIL") == "1":
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
	var dc: Node = main.get_node_or_null("DungeonController")
	_assert(dc != null, "DungeonController node in main.tscn")
	if dc == null:
		quit(1)
		return
	dc.bind_net(net)
	var player: Node = main.get_node("DawnValley/Player")
	var home_parent: Node = player.get_parent()

	# Enter: request goes out; the entrance object in the valley exists.
	dc.enter("DNG-01-001")
	var out := _last_out(ft)
	_assert(str(out.get("type")) == "dungeon.enter.request", "enter request type")
	_assert(str(out.get("payload", {}).get("dungeon_id")) == "DNG-01-001", "enter dungeon_id")
	_assert(main.get_node_or_null("DawnValley/Gameplay/DungeonEntrance") != null, "entrance interactable")

	# Server snapshot R01 → dungeon scene active, player moved, only R01 visible.
	ft.inject_text(_s2c("dungeon.snapshot", _snap("DNG-01-001-R01", "ACTIVE"), str(out.get("request_id"))))
	net.poll(0.016)
	for i in range(3):
		await process_frame
	_assert(dc.is_inside(), "inside dungeon")
	var scene: Node = main.get_node_or_null("DungeonDng01001")
	_assert(scene != null, "dungeon scene instanced")
	_assert(player.get_parent() == scene.get_node("Playground"), "player reparented")
	_assert(scene.get_node("Rooms/R01").visible and not scene.get_node("Rooms/R02").visible, "only R01 visible")
	_assert(not home_parent.visible, "overworld hidden")

	# Interact with the entry seal → request carries the canon object id.
	dc.interact("OBJ-DNG-01-001-ENTRY")
	out = _last_out(ft)
	_assert(str(out.get("type")) == "dungeon.interact.request", "interact type")
	_assert(str(out.get("payload", {}).get("object_id")) == "OBJ-DNG-01-001-ENTRY", "interact object id")
	_assert(str(out.get("payload", {}).get("instance_id")) == "11111111-1111-4111-8111-111111111111", "interact instance id")

	# R02 opens: three enemies spawn from the registry spawn ids.
	_rev = 1
	ft.inject_text(_s2c("dungeon.snapshot", _snap("DNG-01-001-R02", "ACTIVE"), str(out.get("request_id"))))
	net.poll(0.016)
	for i in range(3):
		await process_frame
	var spawned: Array = []
	for m in get_nodes_in_group("monsters"):
		if str(m.get("dungeon_spawn_id")) != "":
			spawned.append(str(m.get("dungeon_spawn_id")))
	spawned.sort()
	_assert(spawned == ["DNG-01-001-R02-E1", "DNG-01-001-R02-E2", "DNG-01-001-R02-E3"], "R02 spawns: %s" % str(spawned))

	# Killing one reports its spawn id (no local loot).
	var victim: Node = null
	for m in get_nodes_in_group("monsters"):
		if str(m.get("dungeon_spawn_id")) == "DNG-01-001-R02-E1":
			victim = m
	_assert(victim != null, "victim found")
	if victim != null:
		victim._die(player)
		out = _last_out(ft)
		_assert(str(out.get("type")) == "dungeon.encounter.report", "report type")
		_assert(str(out.get("payload", {}).get("outcome")) == "enemy_defeated", "report outcome")
		_assert(str(out.get("payload", {}).get("spawn_id")) == "DNG-01-001-R02-E1", "report spawn id")
		_assert(int(player.gold) == 0, "no local gold from dungeon enemy")

	# Player defeat is reported, not decided.
	player.apply_damage(100000.0, null)
	out = _last_out(ft)
	_assert(str(out.get("payload", {}).get("outcome")) == "player_defeated", "defeat report")

	# Completed + reward → back to the overworld with the server gold.
	_rev = 2
	ft.inject_text(_s2c("dungeon.snapshot", _snap("DNG-01-001-R07", "COMPLETED")))
	net.poll(0.016)
	dc.claim()
	out = _last_out(ft)
	_assert(str(out.get("type")) == "dungeon.claim.request", "claim type")
	_rev = 3
	ft.inject_text(_s2c("dungeon.reward", {
		"instance_id": "11111111-1111-4111-8111-111111111111", "gold": 49, "gold_total": 49,
		"items": [{"item_id": "MAT-01-006", "quantity": 1}], "first_clear": true,
		"inventory_revision": 2, "changes": [{"container": "materials", "slot": 0, "item_id": "MAT-01-006", "quantity": 1}],
	}, str(out.get("request_id"))))
	net.poll(0.016)
	for i in range(4):
		await process_frame
	_assert(not dc.is_inside(), "left dungeon after reward")
	_assert(player.get_parent() == home_parent and home_parent.visible, "player back in the overworld")
	_assert(int(player.gold) == 49, "server gold applied")

	if _failed:
		quit(1)
		return
	print("DUNGEON_SMOKE_OK")
	quit(0)
