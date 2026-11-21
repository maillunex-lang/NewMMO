extends RefCounted
## REST auth/session client per docs/contracts/auth-session.md (D-004).
## Access token stays in memory only. Never logs password or token values.
## Unsafe methods (POST/PUT/PATCH/DELETE) are never auto-retried.

const HttpTransport = preload("res://scripts/net/http_transport.gd")

const DEFAULT_API_URL := "http://127.0.0.1:18086"
const PREFS_PATH := "user://auth_prefs.json"
const REQUEST_TIMEOUT_SEC := 15.0

var http: RefCounted = null
var base_url: String = ""
var access_token: String = ""
var account_id: String = ""
var timeout_sec: float = REQUEST_TIMEOUT_SEC

## Last non-secret diagnostic (status / error code only).
var last_status: int = 0
var last_error_code: String = ""
var last_message_key: String = ""
var last_retry_after_ms: int = 0


func configure(p_http: RefCounted, p_base_url: String = "") -> void:
	http = p_http
	if p_base_url != "":
		base_url = p_base_url.rstrip("/")
	else:
		base_url = _resolve_base_url()


func clear_session() -> void:
	access_token = ""
	account_id = ""


func register(login: String, password: String) -> Dictionary:
	return _json_request("POST", "/v1/accounts", {"login": login, "password": password}, false)


func login(login: String, password: String) -> Dictionary:
	var res := _json_request("POST", "/v1/auth/login", {"login": login, "password": password}, false)
	if res.get("ok") == true:
		var data: Dictionary = res.get("data", {})
		access_token = str(data.get("access_token", ""))
		account_id = str(data.get("account_id", ""))
	return res


func logout() -> Dictionary:
	var res := _json_request("POST", "/v1/auth/logout", {}, true)
	clear_session()
	return res


func list_characters() -> Dictionary:
	return _json_request("GET", "/v1/characters", {}, true)


func create_character(char_name: String, class_id: String) -> Dictionary:
	return _json_request("POST", "/v1/characters", {"name": char_name, "class_id": class_id}, true)


func request_ticket(character_id: String) -> Dictionary:
	return _json_request("POST", "/v1/session/ticket", {"character_id": character_id}, true)


func load_remembered_login() -> String:
	if not FileAccess.file_exists(PREFS_PATH):
		return ""
	var f := FileAccess.open(PREFS_PATH, FileAccess.READ)
	if f == null:
		return ""
	var raw := f.get_as_text()
	f.close()
	var parsed = JSON.parse_string(raw)
	if typeof(parsed) != TYPE_DICTIONARY:
		return ""
	var d: Dictionary = parsed
	# Refuse any accidental secret fields if present.
	if d.has("password") or d.has("access_token") or d.has("token") or d.has("ticket"):
		return ""
	return str(d.get("login", ""))


func save_remembered_login(login: String) -> void:
	var payload := {"login": login}
	var f := FileAccess.open(PREFS_PATH, FileAccess.WRITE)
	if f == null:
		return
	f.store_string(JSON.stringify(payload))
	f.close()


func clear_remembered_login() -> void:
	if FileAccess.file_exists(PREFS_PATH):
		DirAccess.remove_absolute(ProjectSettings.globalize_path(PREFS_PATH))


func prefs_path_global() -> String:
	return ProjectSettings.globalize_path(PREFS_PATH)


func _resolve_base_url() -> String:
	var env := OS.get_environment("NEWMMO_API_URL").strip_edges()
	if env != "":
		return env.rstrip("/")
	return DEFAULT_API_URL


func _json_request(method: String, path: String, body: Dictionary, with_auth: bool) -> Dictionary:
	last_status = 0
	last_error_code = ""
	last_message_key = ""
	last_retry_after_ms = 0
	if http == null:
		return _fail_local("NO_TRANSPORT", "error.internal")
	var url := base_url + path
	var headers := PackedStringArray(["Content-Type: application/json", "Accept: application/json"])
	if with_auth:
		if access_token == "":
			return _fail_local("UNAUTHENTICATED", "error.unauthenticated")
		headers.append("Authorization: Bearer %s" % access_token)
	var body_str := ""
	if method != "GET":
		body_str = JSON.stringify(body)
	# No auto-retry for unsafe methods (contract + task).
	var raw: Dictionary = http.request(method, url, headers, body_str)
	var terr := str(raw.get("transport_error", ""))
	if terr != "":
		last_error_code = "TRANSPORT"
		last_message_key = "error.transport"
		return {
			"ok": false,
			"status": 0,
			"data": {},
			"error_code": last_error_code,
			"message_key": last_message_key,
			"retry_after_ms": 0,
			"transport_error": terr,
		}
	var status := int(raw.get("status", 0))
	last_status = status
	var body_text := str(raw.get("body", ""))
	var parsed = {}
	if body_text.strip_edges() != "":
		var j = JSON.parse_string(body_text)
		if typeof(j) == TYPE_DICTIONARY:
			parsed = j
	if status >= 200 and status < 300:
		return {
			"ok": true,
			"status": status,
			"data": parsed,
			"error_code": "",
			"message_key": "",
			"retry_after_ms": 0,
		}
	var err: Dictionary = {}
	if typeof(parsed) == TYPE_DICTIONARY and parsed.has("error"):
		var e = parsed.get("error")
		if typeof(e) == TYPE_DICTIONARY:
			err = e
	last_error_code = str(err.get("code", _default_code_for_status(status)))
	last_message_key = str(err.get("message_key", "error.%s" % last_error_code.to_lower()))
	if err.has("retry_after_ms"):
		last_retry_after_ms = int(err.get("retry_after_ms"))
	elif parsed.has("retry_after_ms"):
		last_retry_after_ms = int(parsed.get("retry_after_ms"))
	return {
		"ok": false,
		"status": status,
		"data": parsed,
		"error_code": last_error_code,
		"message_key": last_message_key,
		"retry_after_ms": last_retry_after_ms,
	}


func _fail_local(code: String, message_key: String) -> Dictionary:
	last_error_code = code
	last_message_key = message_key
	return {
		"ok": false,
		"status": 0,
		"data": {},
		"error_code": code,
		"message_key": message_key,
		"retry_after_ms": 0,
	}


func _default_code_for_status(status: int) -> String:
	match status:
		400:
			return "BAD_REQUEST"
		401:
			return "UNAUTHENTICATED"
		403:
			return "FORBIDDEN"
		404:
			return "NOT_FOUND"
		409:
			return "CONFLICT"
		429:
			return "RATE_LIMITED"
		_:
			return "INTERNAL"
