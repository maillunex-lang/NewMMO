extends "res://scripts/net/transport.gd"
## In-memory transport for headless protocol tests (no real socket).

var _state: int = State.CLOSED
var _close_code: int = -1
var _close_reason: String = ""
var _protocol: String = ""
var _inbox: PackedStringArray = PackedStringArray()
var outbound: Array = []  # Array of String (C→S)
var auto_open := true
var fail_connect := false
var last_url := ""
var last_protocols: PackedStringArray = PackedStringArray()

func connect_to(url: String, protocols: PackedStringArray) -> Error:
	last_url = url
	last_protocols = protocols.duplicate()
	outbound.clear()
	_inbox.clear()
	_close_code = -1
	_close_reason = ""
	if fail_connect:
		_state = State.CLOSED
		return ERR_CANT_CONNECT
	_state = State.CONNECTING
	_protocol = protocols[0] if protocols.size() > 0 else ""
	if auto_open:
		_state = State.OPEN
	return OK

func poll() -> void:
	# No-op; tests inject messages via inject_text / force_open.
	pass

func force_open() -> void:
	_state = State.OPEN

func inject_text(text: String) -> void:
	_inbox.append(text)

func send_text(text: String) -> Error:
	if _state != State.OPEN:
		return ERR_CONNECTION_ERROR
	outbound.append(text)
	return OK

func close(code: int = 1000, reason: String = "") -> void:
	_close_code = code
	_close_reason = reason
	_state = State.CLOSED

func remote_close(code: int, reason: String = "") -> void:
	_close_code = code
	_close_reason = reason
	_state = State.CLOSED

func get_state() -> int:
	return _state

func get_close_code() -> int:
	return _close_code

func get_close_reason() -> String:
	return _close_reason

func get_selected_protocol() -> String:
	return _protocol

func take_messages() -> PackedStringArray:
	var out := _inbox
	_inbox = PackedStringArray()
	return out

func pop_outbound() -> Array:
	var out := outbound.duplicate()
	outbound.clear()
	return out
