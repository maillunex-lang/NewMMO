extends "res://scripts/net/transport.gd"
## Godot WebSocketPeer transport. Subprotocol newmmo.v1; no compression; text only.
## On Godot 4.3 recreate the peer after CLOSED before reconnecting.

var _peer: WebSocketPeer = WebSocketPeer.new()
var _inbox: PackedStringArray = PackedStringArray()

func connect_to(url: String, protocols: PackedStringArray) -> Error:
	# Fresh peer avoids 4.3 reconnect stuck-CLOSED issues.
	_peer = WebSocketPeer.new()
	_peer.supported_protocols = protocols
	_inbox.clear()
	return _peer.connect_to_url(url)

func poll() -> void:
	if _peer == null:
		return
	_peer.poll()
	while _peer.get_ready_state() == WebSocketPeer.STATE_OPEN and _peer.get_available_packet_count() > 0:
		var pkt := _peer.get_packet()
		if _peer.was_string_packet():
			_inbox.append(pkt.get_string_from_utf8())
		else:
			# Binary frames are rejected by contract; close with 1003.
			_peer.close(1003, "binary not supported")

func send_text(text: String) -> Error:
	if _peer == null or _peer.get_ready_state() != WebSocketPeer.STATE_OPEN:
		return ERR_CONNECTION_ERROR
	return _peer.send_text(text)

func close(code: int = 1000, reason: String = "") -> void:
	if _peer:
		_peer.close(code, reason)

func get_state() -> int:
	if _peer == null:
		return State.CLOSED
	return _peer.get_ready_state()

func get_close_code() -> int:
	return _peer.get_close_code() if _peer else -1

func get_close_reason() -> String:
	return _peer.get_close_reason() if _peer else ""

func get_selected_protocol() -> String:
	return _peer.get_selected_protocol() if _peer else ""

func take_messages() -> PackedStringArray:
	var out := _inbox
	_inbox = PackedStringArray()
	return out
