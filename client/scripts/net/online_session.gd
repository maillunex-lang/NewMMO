extends Node
## Bridges AuthClient ticket → NetClient WebSocket session (T-023).
## Poll every frame. Offline by default until configure_from_ticket / bootstrap.

const AuthClient = preload("res://scripts/net/auth_client.gd")
const NetClient = preload("res://scripts/net/net_client.gd")
const WebSocketTransport = preload("res://scripts/net/websocket_transport.gd")
const HttpClientTransport = preload("res://scripts/net/http_client_transport.gd")

signal ready_for_gameplay(character: Dictionary)
signal session_failed(code: String)
signal connection_state(state: String)

var auth: RefCounted = null
var net: RefCounted = null
var character: Dictionary = {}
var _ticket: String = ""
var _ws_url: String = ""
var _active := false


func _ready() -> void:
	add_to_group("online_session")
	set_process(false)


func _process(delta: float) -> void:
	if net != null:
		net.poll(delta)


func configure_injected(p_auth: RefCounted, p_net: RefCounted) -> void:
	auth = p_auth
	net = p_net
	_wire_net_signals()


func configure_from_ticket(ticket: String, ws_url: String, p_character: Dictionary, build_id: String = "") -> Error:
	_ticket = ticket
	_ws_url = ws_url
	character = p_character.duplicate(true)
	if auth == null:
		auth = AuthClient.new()
		auth.configure(HttpClientTransport.new())
	if net == null:
		net = NetClient.new()
	var transport := WebSocketTransport.new()
	var ticket_fn := func() -> String: return _ticket
	net.configure(transport, ticket_fn, _ws_url)
	if build_id != "":
		net.content_build_id = build_id
	_wire_net_signals()
	_active = true
	set_process(true)
	return net.connect_session()


func is_online() -> bool:
	return _active and net != null and net.session_state == NetClient.SessionState.READY


func get_net() -> RefCounted:
	return net


func refresh_ticket_and_reconnect() -> Error:
	if auth == null or character.is_empty():
		return ERR_UNCONFIGURED
	var cid := str(character.get("character_id", ""))
	var res: Dictionary = auth.request_ticket(cid)
	if res.get("ok") != true:
		session_failed.emit(str(res.get("error_code", "TICKET")))
		return ERR_UNAUTHORIZED
	var data: Dictionary = res.get("data", {})
	_ticket = str(data.get("ticket", ""))
	_ws_url = str(data.get("ws_url", _ws_url))
	if net == null:
		return ERR_UNCONFIGURED
	net.url = _ws_url
	net.auto_reconnect = true
	return net.connect_session()


func _wire_net_signals() -> void:
	if net == null:
		return
	if not net.session_state_changed.is_connected(_on_state):
		net.session_state_changed.connect(_on_state)
	if not net.session_ready.is_connected(_on_ready):
		net.session_ready.connect(_on_ready)
	if not net.command_error.is_connected(_on_error):
		net.command_error.connect(_on_error)


func _on_state(state: String) -> void:
	connection_state.emit(state)


func _on_ready(_payload: Dictionary) -> void:
	ready_for_gameplay.emit(character)


func _on_error(payload: Dictionary) -> void:
	var code := str(payload.get("code", ""))
	if code == "INVALID_TICKET" or code == "UNAUTHENTICATED":
		session_failed.emit(code)
