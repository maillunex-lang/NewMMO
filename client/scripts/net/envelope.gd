extends RefCounted
## Build C→S and validate S→C envelopes per D-003 §2–§3 / websocket-envelope.schema.json.

const PROTOCOL_VERSION := 1
const MAX_SAFE_INT := 9007199254740991  # 2^53 - 1
const MAX_DEPTH := 8
const MAX_C2S_BYTES := 16 * 1024
const TYPE_RE := "^[a-z]+(\\.[a-z_]+)+$"
const UUID_RE := "^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"
const TIME_RE := "^\\d{4}-\\d\\d-\\d\\dT\\d\\d:\\d\\d:\\d\\d(\\.\\d{1,3})?Z$"

const MUTATION_TYPES := {
	"quest.accept.request": true,
	"quest.turnin.request": true,
	"world.interact.request": true,
	"dialogue.open.request": true,
	"dialogue.choice.request": true,
	"inventory.move.request": true,
	"combat.loot.request": true,
	"dungeon.enter.request": true,
	"dungeon.interact.request": true,
	"dungeon.encounter.report": true,
	"dungeon.claim.request": true,
	"dungeon.leave.request": true,
}

static func is_mutation(type_name: String) -> bool:
	return MUTATION_TYPES.has(type_name)

static func uuid_v4(rng: RandomNumberGenerator) -> String:
	# RFC 4122 version 4 / variant 10xx, lowercase hex.
	var b: Array = []
	for i in range(16):
		b.append(rng.randi() & 0xff)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var hex := ""
	for i in range(16):
		hex += "%02x" % int(b[i])
	return "%s-%s-%s-%s-%s" % [hex.substr(0, 8), hex.substr(8, 4), hex.substr(12, 4), hex.substr(16, 4), hex.substr(20, 12)]

static func build_c2s(type_name: String, payload: Dictionary, request_id: String = "") -> Dictionary:
	var msg := {
		"protocol_version": PROTOCOL_VERSION,
		"type": type_name,
		"payload": payload if payload != null else {},
	}
	if request_id != "":
		msg["request_id"] = request_id
	elif is_mutation(type_name):
		push_error("mutation missing request_id: %s" % type_name)
	return msg

static func encode_c2s(msg: Dictionary) -> String:
	return JSON.stringify(msg)

static func parse_json_object(text: String) -> Dictionary:
	## Returns {ok:bool, value:Dictionary|null, error:String}
	if text == null or text.strip_edges() == "":
		return {"ok": false, "value": null, "error": "empty"}
	var json := JSON.new()
	var err := json.parse(text)
	if err != OK:
		return {"ok": false, "value": null, "error": "parse"}
	var parsed: Variant = json.data
	if typeof(parsed) != TYPE_DICTIONARY:
		return {"ok": false, "value": null, "error": "not_object"}
	var depth_err := _check_depth(parsed, 0)
	if depth_err != "":
		return {"ok": false, "value": null, "error": depth_err}
	return {"ok": true, "value": parsed, "error": ""}

static func validate_s2c(msg: Dictionary) -> Dictionary:
	## Returns {ok:bool, error:String, ignore_unknown:bool}
	if not msg.has("protocol_version") or not _is_int_eq(msg.get("protocol_version"), PROTOCOL_VERSION):
		return {"ok": false, "error": "protocol_version", "ignore_unknown": false}
	if not msg.has("type") or typeof(msg.get("type")) != TYPE_STRING:
		return {"ok": false, "error": "type", "ignore_unknown": false}
	var type_name: String = str(msg.get("type"))
	var tre := RegEx.new()
	tre.compile(TYPE_RE)
	if tre.search(type_name) == null:
		return {"ok": false, "error": "type_pattern", "ignore_unknown": false}
	for req in ["session_id", "server_time", "revision", "trace_id", "payload"]:
		if not msg.has(req):
			return {"ok": false, "error": "missing_%s" % req, "ignore_unknown": false}
	if typeof(msg.get("session_id")) != TYPE_STRING or str(msg.get("session_id")) == "":
		return {"ok": false, "error": "session_id", "ignore_unknown": false}
	if typeof(msg.get("server_time")) != TYPE_STRING:
		return {"ok": false, "error": "server_time", "ignore_unknown": false}
	var tre_time := RegEx.new()
	tre_time.compile(TIME_RE)
	if tre_time.search(str(msg.get("server_time"))) == null:
		return {"ok": false, "error": "server_time_format", "ignore_unknown": false}
	if not _is_safe_revision(msg.get("revision")):
		return {"ok": false, "error": "revision", "ignore_unknown": false}
	if typeof(msg.get("trace_id")) != TYPE_STRING:
		return {"ok": false, "error": "trace_id", "ignore_unknown": false}
	var tid: String = str(msg.get("trace_id"))
	if tid == "" or tid.length() > 64:
		return {"ok": false, "error": "trace_id_len", "ignore_unknown": false}
	if typeof(msg.get("payload")) != TYPE_DICTIONARY:
		return {"ok": false, "error": "payload", "ignore_unknown": false}
	if msg.has("request_id"):
		var rid: String = str(msg.get("request_id"))
		var ure := RegEx.new()
		ure.compile(UUID_RE)
		if ure.search(rid) == null:
			return {"ok": false, "error": "request_id", "ignore_unknown": false}
	return {"ok": true, "error": "", "ignore_unknown": false}

static func as_revision(v: Variant) -> int:
	if typeof(v) == TYPE_INT:
		return int(v)
	if typeof(v) == TYPE_FLOAT:
		return int(v)
	return -1

static func _is_int_eq(v: Variant, expected: int) -> bool:
	if typeof(v) == TYPE_INT:
		return int(v) == expected
	if typeof(v) == TYPE_FLOAT:
		return v == float(expected) and v == floor(v)
	return false

static func _is_safe_revision(v: Variant) -> bool:
	if typeof(v) == TYPE_INT:
		return int(v) >= 0 and int(v) <= MAX_SAFE_INT
	if typeof(v) == TYPE_FLOAT:
		if v != floor(v):
			return false
		return v >= 0.0 and v <= float(MAX_SAFE_INT)
	return false

static func _check_depth(v: Variant, depth: int) -> String:
	if depth > MAX_DEPTH:
		return "depth"
	match typeof(v):
		TYPE_DICTIONARY:
			for k in v.keys():
				var err := _check_depth(v[k], depth + 1)
				if err != "":
					return err
		TYPE_ARRAY:
			for item in v:
				var err2 := _check_depth(item, depth + 1)
				if err2 != "":
					return err2
	return ""
