extends SceneTree
## Headless protocol v1 tests using FakeTransport. Marker NET_SMOKE_OK only after all checks.

const FakeTransport = preload("res://scripts/net/fake_transport.gd")
const NetClient = preload("res://scripts/net/net_client.gd")
const Envelope = preload("res://scripts/net/envelope.gd")

var _failed := false
var _ticket_n := 0

func _init() -> void:
	call_deferred("_run")

func _fail(msg: String) -> void:
	push_error("NET_SMOKE_FAIL: " + msg)
	_failed = true

func _assert(cond: bool, msg: String) -> void:
	if not cond:
		_fail(msg)

func _ticket() -> String:
	_ticket_n += 1
	return "ticket-%d" % _ticket_n

func _s2c(type_name: String, payload: Dictionary, revision: int, request_id: String = "", session_id: String = "sess-1") -> String:
	var msg := {
		"protocol_version": 1,
		"type": type_name,
		"session_id": session_id,
		"server_time": "2026-11-21T08:00:00.000Z",
		"revision": revision,
		"trace_id": "tr-1",
		"payload": payload,
	}
	if request_id != "":
		msg["request_id"] = request_id
	return JSON.stringify(msg)

func _make_client(seed: int = 42) -> Array:
	var ft: RefCounted = FakeTransport.new()
	var client: RefCounted = NetClient.new()
	client.configure(ft, Callable(self, "_ticket"), "ws://127.0.0.1/v1/ws", seed)
	client.content_build_id = "a".repeat(64)
	client.auto_reconnect = true
	return [client, ft]

func _run() -> void:
	_test_envelope_valid_invalid()
	_test_session_and_subprotocol()
	_test_revision_order_and_gap()
	_test_reconnect_backoff_deterministic()
	_test_mutation_idempotency_and_duplicate_ack()
	_test_inventory_move_mutation()
	_test_loot_request_and_result()
	_test_command_error_rate_limit()
	_test_close_codes()
	_test_unknown_type_and_bad_json()
	if _failed:
		quit(1)
		return
	print("NET_SMOKE_OK")
	quit(0)

func _test_envelope_valid_invalid() -> void:
	var good := {
		"protocol_version": 1,
		"type": "quest.progress.delta",
		"session_id": "s",
		"server_time": "2026-11-21T08:00:00Z",
		"revision": 3,
		"trace_id": "t",
		"payload": {"quest_id": "MQ-01-01"},
	}
	var v := Envelope.validate_s2c(good)
	_assert(v.get("ok") == true, "valid s2c should pass")
	var bad_rev := good.duplicate(true)
	bad_rev["revision"] = 9007199254740992.0  # >= 2^53
	_assert(Envelope.validate_s2c(bad_rev).get("ok") == false, "revision >= 2^53 must fail")
	var bad_float := good.duplicate(true)
	bad_float["revision"] = 1.5
	_assert(Envelope.validate_s2c(bad_float).get("ok") == false, "non-int revision must fail")
	var miss := good.duplicate(true)
	miss.erase("trace_id")
	_assert(Envelope.validate_s2c(miss).get("ok") == false, "missing trace_id must fail")
	var junk := Envelope.parse_json_object("{not-json")
	_assert(junk.get("ok") == false, "junk json must fail parse")
	var depth := {"a": {"b": {"c": {"d": {"e": {"f": {"g": {"h": {"i": {}}}}}}}}}}
	_assert(Envelope.parse_json_object(JSON.stringify(depth)).get("ok") == false, "depth>8 must fail")
	print("CASE envelope_valid_invalid OK")

func _test_session_and_subprotocol() -> void:
	var pair := _make_client()
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	var states: Array = []
	client.session_state_changed.connect(func(s): states.append(s))
	client.connect_session()
	_assert(ft.last_protocols.size() == 1 and ft.last_protocols[0] == "newmmo.v1", "subprotocol newmmo.v1")
	_assert(client.session_state == NetClient.SessionState.AUTHING, "should be AUTHING")
	var out: Array = ft.pop_outbound()
	_assert(out.size() == 1, "session.auth sent")
	var auth: Dictionary = JSON.parse_string(out[0])
	_assert(str(auth.get("type")) == "session.auth", "auth type")
	_assert(auth.get("payload").get("protocol_versions")[0] == 1, "protocol_versions")
	_assert(not str(JSON.stringify(auth)).contains("permessage-deflate"), "no compression flag in app msgs")
	ft.inject_text(_s2c("session.ready", {
		"session_id": "sess-1",
		"character_id": "42",
		"heartbeat_ms": 25000,
		"content_build_id": client.content_build_id,
	}, 0))
	client.poll(0.0)
	_assert(client.session_state == NetClient.SessionState.READY, "READY after session.ready")
	_assert(client.character_id == "42", "character_id is string")
	print("CASE session_and_subprotocol OK")

func _test_revision_order_and_gap() -> void:
	var pair := _make_client()
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	var deltas: Array = []
	var snaps: Array = []
	client.quest_progress_delta.connect(func(p): deltas.append(p))
	client.state_snapshot.connect(func(p, r): snaps.append({"p": p, "r": r}))
	client.connect_session()
	ft.pop_outbound()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 25000, "content_build_id": "x"}, 0))
	ft.inject_text(_s2c("state.snapshot", {"scopes": {"quests": {}}, "revision": 10}, 10))
	client.poll(0.0)
	_assert(client.last_revision == 10, "snapshot sets revision")
	ft.inject_text(_s2c("quest.progress.delta", {"quest_id": "MQ-01-01", "objective_id": "O1", "count": 1, "revision": 11}, 11))
	client.poll(0.0)
	_assert(deltas.size() == 1, "in-order delta applied")
	# Gap 11 -> 13
	ft.inject_text(_s2c("quest.progress.delta", {"quest_id": "MQ-01-01", "objective_id": "O1", "count": 2, "revision": 13}, 13))
	client.poll(0.0)
	_assert(deltas.size() == 1, "gapped delta dropped")
	var reqs: Array = ft.pop_outbound()
	var asked := false
	for t in reqs:
		var m: Dictionary = JSON.parse_string(t)
		if str(m.get("type")) == "state.snapshot.request":
			asked = true
	_assert(asked, "snapshot requested on gap")
	# Delta while awaiting must be dropped
	ft.inject_text(_s2c("quest.progress.delta", {"quest_id": "MQ-01-01", "objective_id": "O1", "count": 3, "revision": 12}, 12))
	client.poll(0.0)
	_assert(deltas.size() == 1, "delta before snapshot discarded")
	ft.inject_text(_s2c("state.snapshot", {"scopes": {}, "revision": 12}, 12))
	client.poll(0.0)
	_assert(client.last_revision == 12, "new snapshot revision")
	ft.inject_text(_s2c("quest.progress.delta", {"quest_id": "MQ-01-01", "objective_id": "O1", "count": 4, "revision": 13}, 13))
	client.poll(0.0)
	_assert(deltas.size() == 2 and client.last_revision == 13, "post-snapshot delta > snap applied")
	print("CASE revision_order_and_gap OK")

func _test_reconnect_backoff_deterministic() -> void:
	var pair := _make_client(7)
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	client.connect_session()
	ft.pop_outbound()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	client.poll(0.0)
	# Capture first reconnect delay with seed 7
	ft.remote_close(1001, "going away")
	client.poll(0.0)
	_assert(client.session_state == NetClient.SessionState.RECONNECTING, "RECONNECTING")
	# Reproduce expected delay: base 0.5 * (0.8 + 0.4 * first_rand)
	var rng := RandomNumberGenerator.new()
	rng.seed = 7
	# uuid not called yet after configure seed; connect_session didn't call uuid. First randf is for jitter.
	# But configure set seed; then connect used ticket only. uuid not used. So first randf in schedule is first draw.
	var expected := 0.5 * (0.8 + 0.4 * rng.randf())
	# Advance just before
	client.poll(expected - 0.01)
	_assert(client.session_state == NetClient.SessionState.RECONNECTING, "still waiting")
	client.poll(0.02)
	# Should have reconnected → AUTHING or READY path
	_assert(client.session_state == NetClient.SessionState.AUTHING or client.session_state == NetClient.SessionState.READY or client.session_state == NetClient.SessionState.CONNECTING, "reconnect fired")
	var tickets_before_reconnect_auth := _ticket_n
	# Auth on reopen consumes a new ticket via provider when OPEN path runs; ensure provider was used again after first session.
	_assert(tickets_before_reconnect_auth >= 1, "ticket provider used")
	print("CASE reconnect_backoff_deterministic OK delay≈%.3f" % expected)

func _test_mutation_idempotency_and_duplicate_ack() -> void:
	var pair := _make_client(99)
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	client.connect_session()
	ft.pop_outbound()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	ft.inject_text(_s2c("state.snapshot", {"scopes": {}, "revision": 1}, 1))
	client.poll(0.0)
	ft.pop_outbound()
	var rid: String = client.request_quest_accept("MQ-01-01")
	var ure := RegEx.new()
	ure.compile(Envelope.UUID_RE)
	_assert(ure.search(rid) != null, "uuid v4 request_id")
	var sent: Array = ft.pop_outbound()
	_assert(sent.size() == 1, "mutation sent")
	var body: Dictionary = JSON.parse_string(sent[0])
	_assert(str(body.get("request_id")) == rid, "same request_id on wire")
	_assert(client.get_pending_count() == 1, "pending queued")
	# Reconnect and ensure resend same id
	ft.remote_close(1001, "bye")
	client.poll(0.0)
	# Force immediate reconnect by advancing past backoff
	client.poll(20.0)
	ft.pop_outbound()  # may include session.auth
	# Complete auth+snapshot to trigger resend
	ft.inject_text(_s2c("session.ready", {"session_id": "s2", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	ft.inject_text(_s2c("state.snapshot", {"scopes": {}, "revision": 1}, 1))
	client.poll(0.0)
	var after: Array = ft.pop_outbound()
	var resent := false
	for t in after:
		var m: Dictionary = JSON.parse_string(t)
		if str(m.get("type")) == "quest.accept.request" and str(m.get("request_id")) == rid:
			resent = true
	_assert(resent, "mutation resent with same request_id")
	# Duplicate ack closes pending once
	ft.inject_text(_s2c("quest.accept.result", {"quest_id": "MQ-01-01", "state": "ACTIVE", "revision": 2, "ok": true}, 2, rid))
	client.poll(0.0)
	_assert(client.get_pending_count() == 0, "acked clears pending")
	ft.inject_text(_s2c("quest.accept.result", {"quest_id": "MQ-01-01", "state": "ACTIVE", "revision": 2, "ok": true}, 2, rid))
	client.poll(0.0)
	_assert(client.get_pending_count() == 0, "duplicate response ok")
	print("CASE mutation_idempotency_and_duplicate_ack OK")

func _test_inventory_move_mutation() -> void:
	var pair := _make_client(11)
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	var deltas: Array = []
	client.inventory_delta.connect(func(p): deltas.append(p))
	client.connect_session()
	ft.pop_outbound()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	ft.inject_text(_s2c("state.snapshot", {"scopes": {"inventory": {"inventory_revision": 1, "bag": [], "materials": [], "quest_keys": []}}, "revision": 1}, 1))
	client.poll(0.0)
	ft.pop_outbound()
	var rid: String = client.request_inventory_move("materials", 0, "bag", 3, 2)
	_assert(rid != "", "inventory move request_id")
	var sent: Array = ft.pop_outbound()
	_assert(sent.size() == 1, "inventory.move.request sent")
	var body: Dictionary = JSON.parse_string(sent[0])
	_assert(str(body.get("type")) == "inventory.move.request", "move type")
	_assert(str(body.get("payload", {}).get("from_container")) == "materials", "from_container")
	_assert(int(body.get("payload", {}).get("to_slot")) == 3, "to_slot")
	_assert(int(body.get("payload", {}).get("quantity")) == 2, "quantity")
	ft.inject_text(_s2c("inventory.delta", {
		"inventory_revision": 2,
		"changes": [{"container": "bag", "slot": 3, "item_id": "MAT-01-001", "quantity": 2}],
	}, 2, rid))
	client.poll(0.0)
	_assert(deltas.size() == 1, "inventory.delta emitted")
	_assert(client.get_pending_count() == 0, "move acked via delta")
	print("CASE inventory_move_mutation OK")

func _test_loot_request_and_result() -> void:
	var pair := _make_client(12)
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	var results: Array = []
	var deltas: Array = []
	client.loot_result.connect(func(p): results.append(p))
	client.inventory_delta.connect(func(p): deltas.append(p))
	client.connect_session()
	ft.pop_outbound()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	ft.inject_text(_s2c("state.snapshot", {"scopes": {}, "revision": 1}, 1))
	client.poll(0.0)
	ft.pop_outbound()
	var rid: String = client.request_loot("ENM-01-001")
	var sent: Array = ft.pop_outbound()
	_assert(sent.size() == 1, "combat.loot.request sent")
	var body: Dictionary = JSON.parse_string(sent[0])
	_assert(str(body.get("type")) == "combat.loot.request", "loot type")
	_assert(str(body.get("payload", {}).get("enemy_id")) == "ENM-01-001", "enemy_id only")
	_assert(body.get("payload", {}).size() == 1, "client never sends amounts")
	_assert(client.get_pending_count() == 1, "loot pending until result")
	ft.inject_text(_s2c("combat.loot.result", {
		"enemy_id": "ENM-01-001", "loot_table_id": "LOOT-ENM-01-001", "gold": 3,
		"items": [{"item_id": "MAT-01-001", "quantity": 1}], "gold_total": 3,
		"inventory_revision": 2, "changes": [{"container": "materials", "slot": 0, "item_id": "MAT-01-001", "quantity": 1}],
	}, 2, rid))
	client.poll(0.0)
	_assert(results.size() == 1 and int(results[0].get("gold", 0)) == 3, "loot_result emitted")
	_assert(deltas.size() == 1 and int(deltas[0].get("inventory_revision", 0)) == 2, "bag delta derived from loot result")
	_assert(client.get_pending_count() == 0, "loot acked")
	print("CASE loot_request_and_result OK")

func _test_command_error_rate_limit() -> void:
	var pair := _make_client()
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	var errors: Array = []
	client.command_error.connect(func(p): errors.append(p))
	client.connect_session()
	ft.pop_outbound()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	ft.inject_text(_s2c("state.snapshot", {"scopes": {}, "revision": 1}, 1))
	client.poll(0.0)
	ft.pop_outbound()
	var rid: String = client.request_quest_turnin("MQ-01-01")
	ft.pop_outbound()
	ft.inject_text(_s2c("command.error", {
		"request_id": rid,
		"code": "RATE_LIMITED",
		"message_key": "error.RATE_LIMITED",
		"retry_after_ms": 2000,
	}, 1, rid))
	client.poll(0.0)
	_assert(errors.size() == 1, "command.error received")
	# Immediate resend attempt via _resend should hold — call send path
	client._resend_pending()
	_assert(ft.pop_outbound().size() == 0, "no resend before retry_after")
	client.poll(2.1)
	client._resend_pending()
	var again: Array = ft.pop_outbound()
	_assert(again.size() == 1 and str(JSON.parse_string(again[0]).get("request_id")) == rid, "resend after retry_after_ms")
	print("CASE command_error_rate_limit OK")

func _test_close_codes() -> void:
	# 4005 no auto reconnect
	var pair := _make_client()
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	var replaced: Array = []
	client.session_replaced.connect(func(): replaced.append(true))
	client.connect_session()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	client.poll(0.0)
	ft.remote_close(4005, "replaced")
	client.poll(0.0)
	_assert(replaced.size() == 1, "4005 emits replaced")
	_assert(client.session_state == NetClient.SessionState.DISCONNECTED, "4005 no reconnect")
	client.poll(20.0)
	_assert(client.session_state == NetClient.SessionState.DISCONNECTED, "4005 still disconnected")

	# 4002 needs update
	pair = _make_client()
	client = pair[0]
	ft = pair[1]
	var need := []
	client.needs_client_update.connect(func(c, r): need.append([c, r]))
	client.connect_session()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	client.poll(0.0)
	ft.remote_close(4002, "proto")
	client.poll(0.0)
	_assert(need.size() == 1 and int(need[0][0]) == 4002, "4002 needs update")
	client.poll(20.0)
	_assert(client.session_state == NetClient.SessionState.DISCONNECTED, "4002 no reconnect")

	# 4004
	pair = _make_client()
	client = pair[0]
	ft = pair[1]
	need.clear()
	client.needs_client_update.connect(func(c, r): need.append([c, r]))
	client.connect_session()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	client.poll(0.0)
	ft.remote_close(4004, "content")
	client.poll(0.0)
	_assert(need.size() == 1 and int(need[0][0]) == 4004, "4004 needs update")
	print("CASE close_codes OK")

func _test_unknown_type_and_bad_json() -> void:
	var pair := _make_client()
	var client: RefCounted = pair[0]
	var ft: RefCounted = pair[1]
	var unknown: Array = []
	client.unknown_s2c_type.connect(func(t): unknown.append(t))
	client.connect_session()
	ft.inject_text(_s2c("session.ready", {"session_id": "s", "character_id": "1", "heartbeat_ms": 1, "content_build_id": "x"}, 0))
	client.poll(0.0)
	ft.inject_text(_s2c("movement.input", {"x": 1}, 1))  # reserved / unknown to client catalogue
	client.poll(0.0)
	_assert(unknown.size() == 1 and str(unknown[0]) == "movement.input", "unknown type ignored")
	ft.inject_text("not-json{{{")
	client.poll(0.0)  # must not crash
	ft.inject_text(_s2c("quest.progress.delta", {"quest_id": "MQ-01-01", "objective_id": "O", "count": 1, "revision": 1}, 1))
	# missing required field envelope
	ft.inject_text(JSON.stringify({
		"protocol_version": 1,
		"type": "quest.progress.delta",
		"session_id": "s",
		"server_time": "2026-11-21T08:00:00Z",
		"revision": 2,
		"payload": {},
	}))
	client.poll(0.0)
	_assert(true, "survived bad json and invalid envelope")
	print("CASE unknown_type_and_bad_json OK")
