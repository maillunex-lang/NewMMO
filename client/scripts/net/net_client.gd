extends RefCounted
## Protocol v1 session client (D-003). Not wired into the game loop — call poll(delta) from tests/host.
## Transport and ticket provider are injectable for FakeTransport tests.

const Envelope = preload("res://scripts/net/envelope.gd")
const TransportBase = preload("res://scripts/net/transport.gd")

signal session_state_changed(state: String)
signal session_ready(payload: Dictionary)
signal content_version(payload: Dictionary)
signal state_snapshot(payload: Dictionary, revision: int)
signal quest_accept_result(payload: Dictionary, request_id: String)
signal quest_progress_delta(payload: Dictionary)
signal quest_reward_granted(payload: Dictionary)
signal dialogue_node_state(payload: Dictionary)
signal inventory_delta(payload: Dictionary)
signal loot_result(payload: Dictionary)
signal dungeon_snapshot(payload: Dictionary)
signal world_snapshot(payload: Dictionary)
signal world_delta(payload: Dictionary)
signal world_event(payload: Dictionary)
signal world_cast_result(payload: Dictionary)
signal dungeon_reward(payload: Dictionary)
signal command_error(payload: Dictionary)
signal needs_client_update(code: int, reason: String)
signal session_replaced()
signal unknown_s2c_type(type_name: String)
signal protocol_log(level: String, message: String)

enum SessionState {
	DISCONNECTED,
	CONNECTING,
	AUTHING,
	READY,
	RECONNECTING,
}

const SUBPROTOCOL := "newmmo.v1"
const BACKOFF_SEC := [0.5, 1.0, 2.0, 4.0, 8.0, 15.0]
const AUTH_TIMEOUT_SEC := 5.0
const KNOWN_S2C := {
	"session.ready": true,
	"content.version": true,
	"state.snapshot": true,
	"quest.accept.result": true,
	"quest.progress.delta": true,
	"quest.reward.granted": true,
	"dialogue.node.state": true,
	"dungeon.state.delta": true,
	"event.snapshot": true,
	"event.contribution": true,
	"inventory.delta": true,
	"combat.loot.result": true,
	"dungeon.snapshot": true,
	"world.snapshot": true,
	"world.delta": true,
	"world.event": true,
	"world.cast.result": true,
	"dungeon.reward": true,
	"command.error": true,
}
const DELTA_TYPES := {
	"quest.progress.delta": true,
	"dungeon.state.delta": true,
	"inventory.delta": true,
}

var transport: RefCounted = null
var ticket_provider: Callable = Callable()  # () -> String
var url: String = "ws://127.0.0.1/v1/ws"
var client_version: String = "0.0.1"
var content_build_id: String = ""
var auto_reconnect: bool = true
var rng: RandomNumberGenerator = RandomNumberGenerator.new()

var session_state: int = SessionState.DISCONNECTED
var session_id: String = ""
var character_id: String = ""
var last_revision: int = -1
var awaiting_snapshot: bool = false

var _auth_deadline: float = -1.0
var _reconnect_at: float = -1.0
var _backoff_index: int = 0
var _clock: float = 0.0
var _pending: Dictionary = {}  # request_id -> {type, payload, sent_at, retry_after_until}
var _drop_until_snapshot: bool = false
var _halt_reconnect: bool = false
var _update_required: bool = false
var _transport_live: bool = false

func configure(p_transport: RefCounted, p_ticket_provider: Callable, p_url: String = "", p_rng_seed: int = 1) -> void:
	transport = p_transport
	ticket_provider = p_ticket_provider
	if p_url != "":
		url = p_url
	rng.seed = p_rng_seed

func connect_session() -> Error:
	_halt_reconnect = false
	_update_required = false
	return _begin_connect(false)

func disconnect_session(code: int = 1000, reason: String = "") -> void:
	_halt_reconnect = true
	auto_reconnect = false
	if transport:
		transport.close(code, reason)
	_set_state(SessionState.DISCONNECTED)

func poll(delta: float) -> void:
	_clock += delta
	if transport == null:
		return
	transport.poll()
	_drain_inbound()
	_check_auth_timeout()
	_check_reconnect_timer()
	_handle_transport_closed()

func send_mutation(type_name: String, payload: Dictionary, request_id: String = "") -> String:
	var rid := request_id
	if rid == "":
		rid = Envelope.uuid_v4(rng)
	_pending[rid] = {
		"type": type_name,
		"payload": payload.duplicate(true),
		"sent_at": _clock,
		"retry_after_until": 0.0,
	}
	_send_c2s(type_name, payload, rid)
	return rid

func request_quest_accept(quest_id: String, build_id: String = "") -> String:
	var bid := build_id if build_id != "" else content_build_id
	return send_mutation("quest.accept.request", {"quest_id": quest_id, "content_build_id": bid})

func request_quest_turnin(quest_id: String) -> String:
	return send_mutation("quest.turnin.request", {"quest_id": quest_id})

func request_world_interact(event: String, target_id: String, instance_id: String = "") -> String:
	var payload := {"event": event, "target_id": target_id}
	if instance_id != "":
		payload["instance_id"] = instance_id
	return send_mutation("world.interact.request", payload)

func request_dialogue_open(npc_id: String, dialogue_id: String) -> String:
	return send_mutation("dialogue.open.request", {"npc_id": npc_id, "dialogue_id": dialogue_id})

func request_dialogue_choice(node_id: String, option_id: String, dialogue_id: String = "", npc_id: String = "") -> String:
	var payload := {"node_id": node_id, "option_id": option_id}
	if dialogue_id != "":
		payload["dialogue_id"] = dialogue_id
	if npc_id != "":
		payload["npc_id"] = npc_id
	return send_mutation("dialogue.choice.request", payload)

func request_inventory_move(from_container: String, from_slot: int, to_container: String, to_slot: int, quantity: int = 0) -> String:
	var payload := {
		"from_container": from_container,
		"from_slot": from_slot,
		"to_container": to_container,
		"to_slot": to_slot,
	}
	if quantity > 0:
		payload["quantity"] = quantity
	return send_mutation("inventory.move.request", payload)

## T-034: report a kill of a canon enemy; the server rolls the drop (loot-engine.md). Never sends amounts.
func request_loot(enemy_id: String) -> String:
	return send_mutation("combat.loot.request", {"enemy_id": enemy_id})

## Dungeon requests (dungeon-engine.md §4). Combat uses the world stream.
func request_dungeon_enter(dungeon_id: String, mode: String = "NORMAL") -> String:
	return send_mutation("dungeon.enter.request", {"dungeon_id": dungeon_id, "mode": mode})

func request_dungeon_interact(instance_id: String, object_id: String, encounter: String = "") -> String:
	var payload := {"instance_id": instance_id, "object_id": object_id}
	if encounter != "":
		payload["encounter_id"] = encounter
	return send_mutation("dungeon.interact.request", payload)

func request_dungeon_claim(instance_id: String) -> String:
	return send_mutation("dungeon.claim.request", {"instance_id": instance_id})

func request_dungeon_leave(instance_id: String) -> String:
	return send_mutation("dungeon.leave.request", {"instance_id": instance_id})

## T-038d: server-authoritative world (world-sim.md, combat-engine.md). Movement carries only a target + seq;
## the speed is the server's. Casting is a mutation (request_id, resent after reconnect).
func world_join(zone_id: String) -> void:
	_send_c2s("world.join.request", {"zone_id": zone_id}, "")

func world_move(seq: int, x: float, y: float, encounter: String = "") -> void:
	var payload := {"seq": seq, "x": x, "y": y}
	if encounter != "":
		payload["encounter_id"] = encounter
	_send_c2s("world.move.request", payload, "")

func world_stop(seq: int, encounter: String = "") -> void:
	var payload := {"seq": seq}
	if encounter != "":
		payload["encounter_id"] = encounter
	_send_c2s("world.stop.request", payload, "")

func request_world_cast(skill_id: String, target_id: int = 0, encounter: String = "") -> String:
	var payload := {"skill_id": skill_id}
	if target_id > 0:
		payload["target_id"] = target_id
	if encounter != "":
		payload["encounter_id"] = encounter
	return send_mutation("world.cast.request", payload)

func request_snapshot(scopes: Array = []) -> void:
	var payload := {}
	if scopes.size() > 0:
		payload["scopes"] = scopes
	_send_c2s("state.snapshot.request", payload, "")
	awaiting_snapshot = true
	_drop_until_snapshot = true

func get_pending_count() -> int:
	return _pending.size()

func get_pending_ids() -> PackedStringArray:
	var out := PackedStringArray()
	for k in _pending.keys():
		out.append(str(k))
	return out

func _begin_connect(is_reconnect: bool) -> Error:
	if transport == null:
		return ERR_UNCONFIGURED
	session_id = ""
	character_id = ""
	_drop_until_snapshot = false
	awaiting_snapshot = false
	_auth_deadline = -1.0
	_set_state(SessionState.CONNECTING if not is_reconnect else SessionState.RECONNECTING)
	var err: Error = transport.connect_to(url, PackedStringArray([SUBPROTOCOL]))
	if err != OK:
		_transport_live = false
		_log("error", "connect failed: %s" % error_string(err))
		if auto_reconnect and not _halt_reconnect:
			_schedule_reconnect()
		else:
			_set_state(SessionState.DISCONNECTED)
		return err
	_transport_live = true
	# FakeTransport may already be OPEN.
	if transport.get_state() == TransportBase.State.OPEN:
		_on_transport_open()
	return OK

func _on_transport_open() -> void:
	_transport_live = true
	_set_state(SessionState.AUTHING)
	_auth_deadline = _clock + AUTH_TIMEOUT_SEC
	var ticket := ""
	if ticket_provider.is_valid():
		ticket = str(ticket_provider.call())
	var payload := {
		"ticket": ticket,
		"client_version": client_version,
		"protocol_versions": [1],
	}
	if content_build_id != "":
		payload["content_build_id"] = content_build_id
	_send_c2s("session.auth", payload, "")

func _send_c2s(type_name: String, payload: Dictionary, request_id: String) -> void:
	if transport == null or transport.get_state() != TransportBase.State.OPEN:
		_log("warn", "send skipped; not open: %s" % type_name)
		return
	if request_id != "" and _pending.has(request_id):
		var until: float = float(_pending[request_id].get("retry_after_until", 0.0))
		if until > _clock:
			_log("info", "RATE_LIMITED hold %s until %s" % [request_id, until])
			return
	var msg: Dictionary = Envelope.build_c2s(type_name, payload, request_id)
	var text: String = Envelope.encode_c2s(msg)
	if text.to_utf8_buffer().size() > Envelope.MAX_C2S_BYTES:
		_log("error", "c2s oversize %s" % type_name)
		return
	var err: Error = transport.send_text(text)
	if err != OK:
		_log("error", "send_text failed: %s" % error_string(err))

func _drain_inbound() -> void:
	if transport == null:
		return
	# Transition CONNECTING → open
	if session_state == SessionState.CONNECTING or session_state == SessionState.RECONNECTING:
		if transport.get_state() == TransportBase.State.OPEN and session_state != SessionState.AUTHING:
			# Only if we haven't already sent auth (AUTHING set in _on_transport_open).
			if session_id == "" and _auth_deadline < 0.0:
				_on_transport_open()
	for text in transport.take_messages():
		_handle_raw_s2c(str(text))

func _handle_raw_s2c(text: String) -> void:
	var parsed: Dictionary = Envelope.parse_json_object(text)
	if not parsed.get("ok", false):
		_log("warn", "bad json: %s" % str(parsed.get("error", "")))
		return
	var msg: Dictionary = parsed.get("value")
	var val: Dictionary = Envelope.validate_s2c(msg)
	if not val.get("ok", false):
		_log("warn", "invalid s2c envelope: %s" % str(val.get("error", "")))
		return
	var type_name: String = str(msg.get("type"))
	if not KNOWN_S2C.has(type_name):
		unknown_s2c_type.emit(type_name)
		_log("info", "ignore unknown s2c type: %s" % type_name)
		return
	var rev: int = Envelope.as_revision(msg.get("revision"))
	var payload: Dictionary = msg.get("payload")
	var request_id: String = str(msg.get("request_id", ""))

	if type_name == "state.snapshot":
		_apply_snapshot(payload, rev)
		return

	if _drop_until_snapshot or awaiting_snapshot:
		# Drop everything until snapshot (contract §6).
		return

	if DELTA_TYPES.has(type_name):
		if last_revision >= 0 and rev != last_revision + 1:
			_log("warn", "revision gap delta got=%d last=%d" % [rev, last_revision])
			request_snapshot()
			return
		_dispatch_typed(type_name, payload, request_id)
		last_revision = rev
		return

	# Non-delta: still track revision monotonicity loosely; gap ⇒ snapshot.
	if last_revision >= 0 and rev > last_revision + 1:
		_log("warn", "revision gap non-delta got=%d last=%d" % [rev, last_revision])
		request_snapshot()
		return

	_dispatch_typed(type_name, payload, request_id)
	if rev > last_revision:
		last_revision = rev

func _apply_snapshot(payload: Dictionary, rev: int) -> void:
	awaiting_snapshot = false
	_drop_until_snapshot = false
	last_revision = rev
	state_snapshot.emit(payload, rev)
	# After reconnect snapshot, resend pending mutations with same request_id.
	_resend_pending()

func _dispatch_typed(type_name: String, payload: Dictionary, request_id: String) -> void:
	match type_name:
		"session.ready":
			session_id = str(payload.get("session_id", ""))
			character_id = str(payload.get("character_id", ""))
			_set_state(SessionState.READY)
			_backoff_index = 0
			_auth_deadline = -1.0
			session_ready.emit(payload)
		"content.version":
			var compat := str(payload.get("compatibility", ""))
			if compat == "incompatible" or compat == "update_required":
				_update_required = true
				needs_client_update.emit(4004, compat)
			content_version.emit(payload)
		"quest.accept.result":
			_ack_pending(request_id)
			quest_accept_result.emit(payload, request_id)
		"quest.progress.delta":
			quest_progress_delta.emit(payload)
		"quest.reward.granted":
			_ack_pending(request_id)
			quest_reward_granted.emit(payload)
		"dialogue.node.state":
			_ack_pending(request_id)
			dialogue_node_state.emit(payload)
		"inventory.delta":
			_ack_pending(request_id)
			inventory_delta.emit(payload)
		"world.snapshot":
			world_snapshot.emit(payload)
		"world.delta":
			world_delta.emit(payload)
		"world.event":
			world_event.emit(payload)
		"world.cast.result":
			_ack_pending(request_id)
			world_cast_result.emit(payload)
		"dungeon.snapshot":
			_ack_pending(request_id)
			dungeon_snapshot.emit(payload)
		"dungeon.reward":
			_ack_pending(request_id)
			dungeon_reward.emit(payload)
			var dch: Variant = payload.get("changes", [])
			if typeof(dch) == TYPE_ARRAY and (dch as Array).size() > 0:
				inventory_delta.emit({"inventory_revision": payload.get("inventory_revision", 0), "changes": dch})
		"combat.loot.result":
			_ack_pending(request_id)
			loot_result.emit(payload)
			# One message carries the bag change too (a second same-revision delta would look like a gap).
			var changes: Variant = payload.get("changes", [])
			if typeof(changes) == TYPE_ARRAY and (changes as Array).size() > 0:
				inventory_delta.emit({"inventory_revision": payload.get("inventory_revision", 0), "changes": changes})
		"command.error":
			_handle_command_error(payload, request_id)
		"dungeon.state.delta", "event.snapshot", "event.contribution":
			pass  # accepted / logged via signals later if needed
		_:
			pass

func _handle_command_error(payload: Dictionary, envelope_rid: String) -> void:
	var rid := str(payload.get("request_id", envelope_rid))
	var code := str(payload.get("code", ""))
	if code == "RATE_LIMITED":
		var ms := int(payload.get("retry_after_ms", 0))
		if _pending.has(rid):
			_pending[rid]["retry_after_until"] = _clock + (float(ms) / 1000.0)
	else:
		# Other errors still correlate; keep pending for reconnect idempotency unless acked by design.
		pass
	command_error.emit(payload)

func _ack_pending(request_id: String) -> void:
	if request_id != "" and _pending.has(request_id):
		_pending.erase(request_id)

func _resend_pending() -> void:
	var ids: Array = _pending.keys()
	ids.sort()
	for rid in ids:
		var entry: Dictionary = _pending[rid]
		_send_c2s(str(entry.get("type")), entry.get("payload"), str(rid))

func _check_auth_timeout() -> void:
	if session_state != SessionState.AUTHING:
		return
	if _auth_deadline >= 0.0 and _clock >= _auth_deadline:
		_log("warn", "auth timeout")
		if transport:
			transport.close(4001, "auth timeout")
		_auth_deadline = -1.0

func _handle_transport_closed() -> void:
	if transport == null:
		return
	if transport.get_state() != TransportBase.State.CLOSED:
		return
	if not _transport_live:
		return
	_transport_live = false
	if session_state == SessionState.DISCONNECTED:
		return
	var code: int = transport.get_close_code()
	var reason: String = transport.get_close_reason()
	_log("info", "closed code=%d reason=%s" % [code, reason])
	_auth_deadline = -1.0
	match code:
		4005:
			session_replaced.emit()
			_halt_reconnect = true
			_set_state(SessionState.DISCONNECTED)
			return
		4002, 4004:
			_update_required = true
			needs_client_update.emit(code, reason)
			_halt_reconnect = true
			_set_state(SessionState.DISCONNECTED)
			return
		4001:
			# Auth failed — reconnect only if auto and not halted (new ticket).
			pass
		_:
			pass
	if auto_reconnect and not _halt_reconnect and not _update_required:
		_schedule_reconnect()
	else:
		_set_state(SessionState.DISCONNECTED)

func _schedule_reconnect() -> void:
	var idx := mini(_backoff_index, BACKOFF_SEC.size() - 1)
	var base: float = float(BACKOFF_SEC[idx])
	_backoff_index = mini(_backoff_index + 1, BACKOFF_SEC.size() - 1)
	# Deterministic jitter: ±20% using injected RNG.
	var jitter: float = 0.8 + 0.4 * rng.randf()
	var delay: float = base * jitter
	_reconnect_at = _clock + delay
	_set_state(SessionState.RECONNECTING)
	_log("info", "reconnect in %.3fs (base=%.1f jitter=%.3f)" % [delay, base, jitter])

func _check_reconnect_timer() -> void:
	if session_state != SessionState.RECONNECTING:
		return
	if _reconnect_at >= 0.0 and _clock >= _reconnect_at:
		_reconnect_at = -1.0
		_begin_connect(true)

func _set_state(s: int) -> void:
	if session_state == s:
		return
	session_state = s
	var name := "DISCONNECTED"
	match s:
		SessionState.CONNECTING:
			name = "CONNECTING"
		SessionState.AUTHING:
			name = "AUTHING"
		SessionState.READY:
			name = "READY"
		SessionState.RECONNECTING:
			name = "RECONNECTING"
		_:
			name = "DISCONNECTED"
	session_state_changed.emit(name)

func _log(level: String, message: String) -> void:
	protocol_log.emit(level, message)
