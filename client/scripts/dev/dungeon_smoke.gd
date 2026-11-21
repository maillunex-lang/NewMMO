extends SceneTree
## Headless dungeon controller + server combat stream with FakeTransport.

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

func _snap(room: String, status: String) -> Dictionary:
	var rooms := {}
	for i in range(1, 8):
		var rid := "DNG-01-001-R0%d" % i
		rooms[rid] = {"state": "IN_PROGRESS" if rid == room else "LOCKED", "objects": {}, "spawns": {}}
	var combat := {"you": 1, "encounter_id": room, "entities": [
		{"id": 1, "kind": "player", "x": 90, "y": 460, "hp": 200, "max_hp": 200},
	]}
	if room == "DNG-01-001-R02":
		for i in range(2, 5):
			combat["entities"].append({"id": i, "kind": "enemy", "ref": "ENM-01-001", "x": 420, "y": 260, "hp": 40, "max_hp": 40})
	return {
		"instance_id": "11111111-1111-4111-8111-111111111111", "dungeon_id": "DNG-01-001", "mode": "NORMAL",
		"status": status, "room_id": room, "checkpoint_room_id": "DNG-01-001-R01", "event_seq": _rev + 1,
		"rooms": rooms, "attempts": 0, "revision": _rev, "combat": combat,
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
	var wc: Node = main.get_node("WorldController")
	wc.bind_net(net)
	ft.pop_outbound()
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
	var spawned: Dictionary = wc.get_entities()
	_assert(spawned.size() == 3 and spawned.has(2), "R02 has three server puppets")
	if spawned.has(2):
		_assert(spawned[2].get_parent() == scene.get_node("Playground"), "puppet is in the dungeon")

	# A delayed update from the previous room must not move the player.
	ft.inject_text(_s2c("world.delta", {"encounter_id": "DNG-01-001-R01", "upsert": [{"id": 1, "x": 700, "y": 300}]}))
	# A server death hides the puppet and never generates a client report.
	ft.inject_text(_s2c("world.delta", {"encounter_id": "DNG-01-001-R02", "upsert": [{"id": 2, "dead": true, "hp": 0}]}))
	net.poll(0.016)
	_assert(player.global_position == Vector2(90, 460), "ignored previous room movement")
	_assert(spawned.has(2) and not spawned[2].is_alive(), "server death applied")
	_assert(_last_out(ft).is_empty(), "no client death report")
	_assert(int(player.gold) == 0, "dungeon gold waits for claim")

	# Phase 2 highlights cover and routes crystal interaction to the live stream.
	_rev = 2
	var arena := "DNG-01-001-R06"
	ft.inject_text(_s2c("dungeon.snapshot", _snap(arena, "BOSS_ACTIVE")))
	net.poll(0.016)
	await process_frame
	var crystal_id := "OBJ-DNG-01-001-CRYSTAL-1"
	var crystal: Area2D = scene.get_node("Rooms/R06/Objects/" + crystal_id)
	_assert(not crystal.monitorable, "crystal dormant before cast")
	var areas := [
		{"min_x": 0, "min_y": 0, "max_x": 380, "max_y": 280},
		{"min_x": 380, "min_y": 280, "max_x": 760, "max_y": 560},
	]
	ft.inject_text(_s2c("world.event", {"encounter_id": arena, "events": [{"type": "witness_telegraph", "amount": 2200, "safe_areas": areas}]}))
	net.poll(0.016)
	await process_frame
	_assert(crystal.monitorable and get_nodes_in_group("telegraphs").size() == 1, "cover and crystal active")
	crystal.on_interact(player)
	out = _last_out(ft)
	_assert(out.get("payload", {}).get("encounter_id") == arena, "crystal uses private stream")
	ft.inject_text(_s2c("world.event", {"encounter_id": arena, "events": [
		{"type": "witness_crystal", "object_id": crystal_id}, {"type": "witness_end"},
	]}))
	net.poll(0.016)
	await process_frame
	_assert(not crystal.visible and not crystal.monitorable, "broken crystal hidden")
	_assert(get_nodes_in_group("telegraphs").is_empty(), "interrupted telegraph removed")

	# Completed + reward → back to the overworld with the server gold.
	_rev = 3
	ft.inject_text(_s2c("dungeon.snapshot", _snap("DNG-01-001-R07", "COMPLETED")))
	net.poll(0.016)
	dc.claim()
	out = _last_out(ft)
	_assert(str(out.get("type")) == "dungeon.claim.request", "claim type")
	_rev = 4
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
	_assert(str(_last_out(ft).get("type")) == "world.join.request", "rejoin overworld simulation")

	if _failed:
		quit(1)
		return
	print("DUNGEON_SMOKE_OK")
	quit(0)
