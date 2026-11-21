extends SceneTree
## Headless AuthClient + FakeHttp tests (D-004). Marker AUTH_SMOKE_OK only after all checks.
## Optional: AUTH_SMOKE_FAIL=1 forces a non-zero exit to prove the smoke harness is strict.

const AuthClient = preload("res://scripts/net/auth_client.gd")
const FakeHttp = preload("res://scripts/net/fake_http.gd")

var _failed := false
var _ticket_seen := ""
var _ws_seen := ""
var _char_seen: Dictionary = {}


func _init() -> void:
	call_deferred("_run")


func _fail(msg: String) -> void:
	# Never include secrets in fail messages.
	push_error("AUTH_SMOKE_FAIL: " + msg)
	_failed = true


func _assert(cond: bool, msg: String) -> void:
	if not cond:
		_fail(msg)


func _run() -> void:
	if OS.get_environment("AUTH_SMOKE_FAIL") == "1":
		_fail("forced failure via AUTH_SMOKE_FAIL=1")
		quit(1)
		return

	_test_register_ok_and_login_taken()
	_test_login_ok_and_invalid_credentials()
	_test_rate_limited()
	_test_logout_and_characters()
	_test_create_limit_reached()
	await _test_ticket_and_signal()
	_test_no_retry_queue_for_post()
	_test_secrets_not_on_disk_or_logs()
	_test_base_url_from_env()

	if _failed:
		quit(1)
		return
	print("AUTH_SMOKE_OK")
	quit(0)


func _make(http: RefCounted = null) -> Array:
	var fh: RefCounted = http if http != null else FakeHttp.new()
	var client: RefCounted = AuthClient.new()
	client.configure(fh, "http://127.0.0.1:18086")
	return [client, fh]


func _test_register_ok_and_login_taken() -> void:
	var pair: Array = _make()
	var client: RefCounted = pair[0]
	var fh: RefCounted = pair[1]
	fh.enqueue("POST", "/v1/accounts", 201, {"account_id": "42"})
	var ok: Dictionary = client.register("alice_01", "password-long")
	_assert(ok.get("ok") == true, "register 201 should ok")
	_assert(int(ok.get("status")) == 201, "register status 201")

	fh.enqueue("POST", "/v1/accounts", 409, {"error": {"code": "LOGIN_TAKEN", "message_key": "error.login_taken"}})
	var taken: Dictionary = client.register("alice_01", "password-long")
	_assert(taken.get("ok") == false, "LOGIN_TAKEN not ok")
	_assert(str(taken.get("error_code")) == "LOGIN_TAKEN", "LOGIN_TAKEN code")
	print("CASE register_ok_login_taken OK")


func _test_login_ok_and_invalid_credentials() -> void:
	var pair: Array = _make()
	var client: RefCounted = pair[0]
	var fh: RefCounted = pair[1]
	fh.enqueue("POST", "/v1/auth/login", 200, {
		"access_token": "tok_secret_value_do_not_log",
		"expires_at": "2099-01-01T00:00:00Z",
		"account_id": "7",
	})
	var ok: Dictionary = client.login("alice_01", "password-long")
	_assert(ok.get("ok") == true, "login 200 should ok")
	_assert(client.access_token == "tok_secret_value_do_not_log", "token in memory")
	_assert(client.account_id == "7", "account_id set")

	fh.enqueue("POST", "/v1/auth/login", 401, {"error": {"code": "INVALID_CREDENTIALS", "message_key": "error.invalid_credentials"}})
	var bad: Dictionary = client.login("alice_01", "wrong-password-xx")
	_assert(bad.get("ok") == false, "INVALID_CREDENTIALS not ok")
	_assert(str(bad.get("error_code")) == "INVALID_CREDENTIALS", "INVALID_CREDENTIALS code")
	print("CASE login_ok_invalid_credentials OK")


func _test_rate_limited() -> void:
	var pair: Array = _make()
	var client: RefCounted = pair[0]
	var fh: RefCounted = pair[1]
	fh.enqueue("POST", "/v1/auth/login", 429, {
		"error": {"code": "RATE_LIMITED", "message_key": "error.rate_limited"},
		"retry_after_ms": 15000,
	})
	var res: Dictionary = client.login("bob", "password-long")
	_assert(res.get("ok") == false, "RATE_LIMITED not ok")
	_assert(str(res.get("error_code")) == "RATE_LIMITED", "RATE_LIMITED code")
	_assert(int(res.get("retry_after_ms")) == 15000, "retry_after_ms")
	print("CASE rate_limited OK")


func _test_logout_and_characters() -> void:
	var pair: Array = _make()
	var client: RefCounted = pair[0]
	var fh: RefCounted = pair[1]
	client.access_token = "mem_token"
	fh.enqueue("POST", "/v1/auth/logout", 204, "")
	var out: Dictionary = client.logout()
	_assert(out.get("ok") == true, "logout 204 ok")
	_assert(client.access_token == "", "token cleared after logout")

	client.access_token = "mem_token"
	fh.enqueue("GET", "/v1/characters", 200, {
		"characters": [
			{"character_id": "100", "name": "Mira", "class_id": "CLS-01", "level": 1},
		],
	})
	var lst: Dictionary = client.list_characters()
	_assert(lst.get("ok") == true, "list characters ok")
	var chars: Array = lst.get("data", {}).get("characters", [])
	_assert(chars.size() == 1, "one character")
	print("CASE logout_and_characters OK")


func _test_create_limit_reached() -> void:
	var pair: Array = _make()
	var client: RefCounted = pair[0]
	var fh: RefCounted = pair[1]
	client.access_token = "mem_token"
	fh.enqueue("POST", "/v1/characters", 409, {"error": {"code": "LIMIT_REACHED", "message_key": "error.limit_reached"}})
	var res: Dictionary = client.create_character("HeroName", "CLS-02")
	_assert(res.get("ok") == false, "LIMIT_REACHED not ok")
	_assert(str(res.get("error_code")) == "LIMIT_REACHED", "LIMIT_REACHED code")
	print("CASE create_limit_reached OK")


func _test_ticket_and_signal() -> void:
	var pair: Array = _make()
	var client: RefCounted = pair[0]
	var fh: RefCounted = pair[1]
	client.access_token = "mem_token"
	fh.enqueue("POST", "/v1/session/ticket", 200, {
		"ticket": "ticket_secret_value",
		"expires_in_ms": 30000,
		"ws_url": "ws://127.0.0.1:18086/v1/ws",
	})
	var res: Dictionary = client.request_ticket("100")
	_assert(res.get("ok") == true, "ticket ok")
	_assert(str(res.get("data", {}).get("ticket", "")) == "ticket_secret_value", "ticket in data")

	# UI signal path: instantiate auth flow with injected FakeHttp/AuthClient.
	var packed: PackedScene = load("res://scenes/ui/auth_flow.tscn")
	_assert(packed != null, "auth_flow.tscn loads")
	var ui: Control = packed.instantiate()
	var fh2 := FakeHttp.new()
	var auth2 := AuthClient.new()
	auth2.configure(fh2, "http://127.0.0.1:18086")
	auth2.access_token = "mem_token"
	ui.configure_auth(auth2)
	root.add_child(ui)
	for i in range(5):
		await process_frame
	ui.session_ticket_ready.connect(_on_ticket)
	fh2.enqueue("GET", "/v1/characters", 200, {
		"characters": [{"character_id": "100", "name": "Mira", "class_id": "CLS-01", "level": 1}],
	})
	ui._refresh_characters()
	for i in range(3):
		await process_frame
	_assert(ui.char_list.item_count == 1, "list populated")
	ui.char_list.select(0)
	fh2.enqueue("POST", "/v1/session/ticket", 200, {
		"ticket": "ui_ticket_secret",
		"expires_in_ms": 30000,
		"ws_url": "ws://127.0.0.1:18086/v1/ws",
	})
	ui._on_enter_world()
	for i in range(3):
		await process_frame
	_assert(_ticket_seen == "ui_ticket_secret", "session_ticket_ready ticket")
	_assert(_ws_seen.begins_with("ws://"), "session_ticket_ready ws_url")
	_assert(str(_char_seen.get("character_id", "")) == "100", "character payload")
	ui.queue_free()
	print("CASE ticket_and_signal OK")


func _on_ticket(ticket: String, ws_url: String, character: Dictionary) -> void:
	_ticket_seen = ticket
	_ws_seen = ws_url
	_char_seen = character


func _test_no_retry_queue_for_post() -> void:
	# AuthClient issues exactly one http.request per call (no internal retry).
	var pair: Array = _make()
	var client: RefCounted = pair[0]
	var fh: RefCounted = pair[1]
	fh.enqueue("POST", "/v1/auth/login", 500, {"error": {"code": "INTERNAL", "message_key": "error.internal"}})
	client.login("x", "password-long")
	_assert(fh.calls.size() == 1, "unsafe POST must not auto-retry")
	# Second call needs its own enqueue — proves no silent replay of the first.
	fh.enqueue("POST", "/v1/auth/login", 200, {"access_token": "t", "expires_at": "x", "account_id": "1"})
	client.login("x", "password-long")
	_assert(fh.calls.size() == 2, "second login is a new request")
	print("CASE no_retry_unsafe_post OK")


func _test_secrets_not_on_disk_or_logs() -> void:
	var pair: Array = _make()
	var client: RefCounted = pair[0]
	var fh: RefCounted = pair[1]
	client.clear_remembered_login()
	fh.enqueue("POST", "/v1/auth/login", 200, {
		"access_token": "DISK_TOKEN_SHOULD_NOT_APPEAR",
		"expires_at": "2099-01-01T00:00:00Z",
		"account_id": "9",
	})
	client.login("remember_me", "SuperSecretPass99")
	client.save_remembered_login("remember_me")
	var path: String = client.prefs_path_global()
	_assert(FileAccess.file_exists("user://auth_prefs.json"), "prefs exist")
	var f := FileAccess.open("user://auth_prefs.json", FileAccess.READ)
	var raw := f.get_as_text()
	f.close()
	_assert(raw.find("SuperSecretPass99") < 0, "password must not be on disk")
	_assert(raw.find("DISK_TOKEN_SHOULD_NOT_APPEAR") < 0, "token must not be on disk")
	_assert(raw.find("access_token") < 0, "access_token key must not be on disk")
	_assert(raw.find("remember_me") >= 0, "login may be remembered")
	# Scan user:// text prefs only (skip binary screenshots etc.).
	var dir := DirAccess.open("user://")
	_assert(dir != null, "user:// open")
	dir.list_dir_begin()
	var fname := dir.get_next()
	while fname != "":
		if not dir.current_is_dir() and fname.ends_with(".json"):
			var fp := FileAccess.open("user://".path_join(fname), FileAccess.READ)
			if fp:
				var txt := fp.get_as_text()
				fp.close()
				_assert(txt.find("SuperSecretPass99") < 0, "password leaked in user:// " + fname)
				_assert(txt.find("DISK_TOKEN_SHOULD_NOT_APPEAR") < 0, "token leaked in user:// " + fname)
		fname = dir.get_next()
	dir.list_dir_end()
	client.clear_remembered_login()
	print("CASE secrets_not_on_disk OK path=", path.get_file())


func _test_base_url_from_env() -> void:
	# Default when env empty.
	var fh := FakeHttp.new()
	var client := AuthClient.new()
	OS.set_environment("NEWMMO_API_URL", "")
	client.configure(fh)
	_assert(client.base_url == "http://127.0.0.1:18086", "default API URL")
	OS.set_environment("NEWMMO_API_URL", "http://example.test:9")
	client.configure(fh)
	_assert(client.base_url == "http://example.test:9", "env API URL")
	OS.set_environment("NEWMMO_API_URL", "")
	print("CASE base_url_env OK")
