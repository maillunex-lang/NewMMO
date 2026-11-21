extends RefCounted
## Abstract WebSocket-like transport. Implementations: WebSocketTransport, FakeTransport.
## Text JSON frames only; callers own poll() cadence.

enum State {
	CONNECTING = 0,
	OPEN = 1,
	CLOSING = 2,
	CLOSED = 3,
}

func connect_to(url: String, protocols: PackedStringArray) -> Error:
	push_error("Transport.connect_to not implemented")
	return ERR_UNAVAILABLE

func poll() -> void:
	pass

func send_text(text: String) -> Error:
	push_error("Transport.send_text not implemented")
	return ERR_UNAVAILABLE

func close(code: int = 1000, reason: String = "") -> void:
	pass

func get_state() -> int:
	return State.CLOSED

func get_close_code() -> int:
	return -1

func get_close_reason() -> String:
	return ""

func get_selected_protocol() -> String:
	return ""

## Drain inbound text messages (empty if none).
func take_messages() -> PackedStringArray:
	return PackedStringArray()
