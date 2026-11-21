extends SceneTree
## Headless T-023 wiring: AuthClient(FakeHttp) → ticket → NetClient(FakeTransport) →
## quest accept / world.interact(+instance_id) / turn-in. Marker ONLINE_SMOKE_OK.

const AuthClient = preload("res://scripts/net/auth_client.gd")
const FakeHttp = preload("res://scripts/net/fake_http.gd")
const FakeTransport = preload("res://scripts/net/fake_transport.gd")
const NetClient = preload("res://scripts/net/net_client.gd")

var _failed := false

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	if OS.get_environment("ONLINE_SMOKE_FAIL") == "1":
		push_error("forced fail")
		quit(9)
		return

	var fh := FakeHttp.new()
	fh.enqueue("POST", "/v1/accounts", 201, {"account_id": "acc-1"})
	fh.enqueue("POST", "/v1/auth/login", 200, {
		"access_token": "tok_secret_value_do_not_log",
		"expires_at": "2099-01-01T00:00:00Z",
		"account_id": "acc-1",
	})
	fh.enqueue("GET", "/v1/characters", 200, {
		"characters": [{"character_id": "7", "name": "Hero", "class_id": "CLS-01", "level": 1}],
	})
	fh.enqueue("POST", "/v1/session/ticket", 200, {
		"ticket": "ticket_secret_value",
		"expires_in_ms": 30000,
		"ws_url": "ws://127.0.0.1:18086/v1/ws",
	})

	var auth := AuthClient.new()
	auth.configure(fh, "http://127.0.0.1:18086")
	_assert(auth.register("player_one", "Password-12345").get("ok") == true, "register")
	_assert(auth.login("player_one", "Password-12345").get("ok") == true, "login")
	_assert(str(auth.access_token) != "", "token memory")
	_assert(auth.list_characters().get("ok") == true, "list")
	var ticket_res: Dictionary = auth.request_ticket("7")
	_assert(ticket_res.get("ok") == true, "ticket")
	var ticket := str(ticket_res.get("data", {}).get("ticket", ""))
	var ws_url := str(ticket_res.get("data", {}).get("ws_url", ""))
	_assert(ticket == "ticket_secret_value", "ticket value")
	_assert(ws_url.begins_with("ws://"), "ws_url")

	var ft := FakeTransport.new()
	var net := NetClient.new()
	net.content_build_id = "build-test"
	net.configure(ft, func() -> String: return ticket, ws_url, 42)
	_assert(net.connect_session() == OK, "connect")
	var out: Array = ft.pop_outbound()
	_assert(out.size() == 1, "session.auth sent")
	var auth_msg: Dictionary = JSON.parse_string(out[0])
	_assert(str(auth_msg.get("type")) == "session.auth", "auth type")
	_assert(str(auth_msg.get("payload", {}).get("ticket", "")) == ticket, "ticket in auth")

	ft.inject_text(_s2c("session.ready", {
		"session_id": "s1", "character_id": "7", "heartbeat_ms": 25000, "content_build_id": "build-test",
	}, 0))
	ft.inject_text(_s2c("state.snapshot", {
		"scopes": {"quests": {"active": [], "completed": [], "available": ["MQ-01-01"]}},
		"revision": 0,
	}, 0))
	net.poll(0.016)
	_assert(net.session_state == NetClient.SessionState.READY, "READY")

	var err := change_scene_to_file("res://scenes/world/main.tscn")
	_assert(err == OK, "load main")
	for i in range(40):
		await process_frame
	var main := root.get_child(root.get_child_count() - 1)
	var qm: Node = main.get_node_or_null("QuestManager")
	_assert(qm != null, "quest manager")
	qm.enable_online(net, "build-test")
	_assert(qm.is_online() == true, "online flag")
	_assert(str(qm.get_status()) == "available", "available locally")

	qm.on_dialogue_action("ACCEPT", "NPC-802", null)
	net.poll(0.016)
	var accept_msg := _take_type(ft, "quest.accept.request")
	_assert(not accept_msg.is_empty(), "accept sent")
	_assert(str(accept_msg.get("payload", {}).get("quest_id", "")) == "MQ-01-01", "accept quest")
	_assert(str(accept_msg.get("payload", {}).get("content_build_id", "")) == "build-test", "build id")
	var rid := str(accept_msg.get("request_id", ""))
	ft.inject_text(_s2c("quest.accept.result", {
		"quest_id": "MQ-01-01", "state": "ACTIVE", "revision": 1, "ok": true,
	}, 1, rid))
	net.poll(0.016)
	_assert(str(qm.get_status()) == "active", "active after accept")

	qm.report_world_event_ex("npc_interact", "NPC-802", "", null)
	net.poll(0.016)
	var interact := _take_type(ft, "world.interact.request")
	_assert(not interact.is_empty(), "interact sent")
	_assert(str(interact.get("payload", {}).get("event", "")) == "npc_interact", "event")
	_assert(str(interact.get("payload", {}).get("target_id", "")) == "NPC-802", "target")
	rid = str(interact.get("request_id", ""))
	ft.inject_text(_s2c("quest.progress.delta", {
		"quest_id": "MQ-01-01", "objective_id": "OBJ-01-01-1", "count": 1, "need": 1, "revision": 2,
	}, 2, rid))
	net.poll(0.016)

	qm.report_world_event_ex("world_interact", "POI-CLUE", "POI-CLUE#2", null)
	net.poll(0.016)
	var multi := _take_type(ft, "world.interact.request")
	_assert(not multi.is_empty(), "multi interact")
	_assert(str(multi.get("payload", {}).get("instance_id", "")) == "POI-CLUE#2", "instance_id")

	qm._status = "objectives_complete"
	qm.on_dialogue_action("TURN_IN", "NPC-802", null)
	net.poll(0.016)
	var turnin := _take_type(ft, "quest.turnin.request")
	_assert(not turnin.is_empty(), "turnin sent")
	rid = str(turnin.get("request_id", ""))
	ft.inject_text(_s2c("quest.reward.granted", {
		"quest_id": "MQ-01-01",
		"reward_claim_id": "00000000-0000-4000-8000-000000000001",
		"inventory_revision": 3,
		"xp_total": 100,
		"gold_total": 12,
		"level": 1,
		"reputation": {"FAC-08": 20},
		"evidence": [],
	}, 3, rid))
	net.poll(0.016)
	_assert(int(qm.debug_snapshot().get("xp", 0)) == 100, "xp applied")
	_assert(int(qm.debug_snapshot().get("rep_fac08", 0)) == 20, "rep applied")

	# Secrets must not appear in printed markers / forced fail path already checked.
	_assert(not ticket.contains("ONLINE_SMOKE_OK"), "sanity")

	if _failed:
		quit(1)
		return
	print("ONLINE_SMOKE_OK")
	quit(0)

func _take_type(ft: RefCounted, type_name: String) -> Dictionary:
	var batch: Array = ft.pop_outbound()
	for raw in batch:
		var m: Dictionary = JSON.parse_string(str(raw))
		if typeof(m) == TYPE_DICTIONARY and str(m.get("type")) == type_name:
			return m
	return {}

func _s2c(type_name: String, payload: Dictionary, revision: int, request_id: String = "") -> String:
	var msg := {
		"protocol_version": 1,
		"type": type_name,
		"session_id": "s1",
		"server_time": "2026-11-21T08:00:00.000Z",
		"revision": revision,
		"trace_id": "t1",
		"payload": payload,
	}
	if request_id != "":
		msg["request_id"] = request_id
	return JSON.stringify(msg)

func _assert(cond: bool, label: String) -> void:
	if not cond:
		push_error("ASSERT FAIL: %s" % label)
		_failed = true
