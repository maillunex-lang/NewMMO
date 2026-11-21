extends SceneTree
## Dev-only end-to-end against a RUNNING server (not CI): real HTTP + real WebSocket, no mocks.
## clean start → register → login → create character → ticket → OnlineSession → main scene → world joined.
## env: NEWMMO_API_URL (e.g. http://127.0.0.1:8099). Marker AUTH_E2E_OK.
const AuthClient = preload("res://scripts/net/auth_client.gd")
const HttpClientTransport = preload("res://scripts/net/http_client_transport.gd")
const OnlineSession = preload("res://scripts/net/online_session.gd")
var _failed := false
func _init() -> void:
	call_deferred("_run")
func _assert(c: bool, l: String) -> void:
	if not c:
		push_error("ASSERT FAIL: " + l)
		_failed = true
func _run() -> void:
	var auth := AuthClient.new()
	auth.configure(HttpClientTransport.new())
	var login := "e2e%d" % (Time.get_ticks_usec() % 1000000000)
	var r: Dictionary = auth.register(login, "e2e-password")
	_assert(r.get("ok") == true, "register: %s" % str(r))
	r = auth.login(login, "e2e-password")
	_assert(r.get("ok") == true, "login: %s" % str(r))
	r = auth.create_character("Teston", "CLS-03")
	_assert(r.get("ok") == true, "create character: %s" % str(r))
	r = auth.list_characters()
	_assert(r.get("ok") == true and (r.get("data", {}).get("characters", []) as Array).size() >= 1, "list characters")
	var ch: Dictionary = (r["data"]["characters"] as Array)[0]
	r = auth.request_ticket(str(ch.get("character_id")))
	_assert(r.get("ok") == true, "ticket: %s" % str(r))
	var sess := OnlineSession.new()
	sess.name = "OnlineSession"
	sess.auth = auth
	root.add_child(sess)
	var ready := [false]
	sess.ready_for_gameplay.connect(func(_c): ready[0] = true)
	var err: Error = sess.configure_from_ticket(str(r["data"]["ticket"]), str(r["data"]["ws_url"]), ch)
	_assert(err == OK, "ws connect")
	var t := 0.0
	while not ready[0] and t < 10.0:
		await create_timer(0.1).timeout
		t += 0.1
	_assert(ready[0], "session ready (WebSocket authenticated)")
	change_scene_to_file("res://scenes/world/main.tscn")
	for i in 90:
		await process_frame
	var wc := get_first_node_in_group("world_controller")
	var tw := 0.0
	while wc != null and not wc.is_active() and tw < 8.0:
		await create_timer(0.1).timeout
		tw += 0.1
	_assert(wc != null and wc.is_active(), "joined the online world")
	_assert(get_first_node_in_group("view3d") != null, "3D view running")
	if _failed:
		quit(1)
		return
	print("AUTH_E2E_OK")
	quit(0)
