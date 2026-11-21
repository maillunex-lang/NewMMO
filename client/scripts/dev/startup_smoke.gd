extends SceneTree
## Headless: the app starts on the login screen and is online-by-default (Android has no env vars); API = VPS;
## hero/NPC 3D models and the INTERNET permission are in the Android export. Marker STARTUP_SMOKE_OK.
const AuthClient = preload("res://scripts/net/auth_client.gd")
var _failed := false
func _init() -> void:
	call_deferred("_run")
func _assert(c: bool, l: String) -> void:
	if not c:
		push_error("ASSERT FAIL: " + l)
		_failed = true
func _run() -> void:
	_assert(str(ProjectSettings.get_setting("application/run/main_scene")) == "res://scenes/ui/auth_flow.tscn", "main scene is the login flow")
	var ui: Control = load("res://scenes/ui/auth_flow.tscn").instantiate()
	root.add_child(ui)
	await process_frame
	_assert(ui.enter_online == true, "online by default without NEWMMO_ONLINE")
	_assert(ui.get_node("SafeArea/Chrome/LoginPanel").visible, "login panel shown first")
	_assert(ui.get_node_or_null("SafeArea/Chrome/RegisterPanel") != null or ui.get("panel_register") != null, "registration reachable")
	var a := AuthClient.new()
	a.configure(null)
	_assert(a.base_url == "https://rikagro.ru/newmmo" or OS.get_environment("NEWMMO_API_URL") != "", "default API is the VPS: %s" % a.base_url)
	# Android export contract
	var cfg := FileAccess.get_file_as_string("res://export_presets.cfg")
	_assert(cfg.contains("permissions/internet=true"), "INTERNET permission")
	var excl := ""
	for line in cfg.split("\n"):
		if line.begins_with("exclude_filter="):
			excl = line
	_assert(excl != "" and not excl.contains("hero_") and not excl.contains("npc_") and not excl.contains("anims") and not excl.contains("models/*"), "hero/NPC models and animations are not excluded from the APK")
	for f in ["hero_cls-01", "hero_cls-02", "hero_cls-03", "hero_cls-04", "hero_cls-05", "npc_elder_mira", "npc_npc-102", "npc_npc-802"]:
		_assert(ResourceLoader.exists("res://assets/models/%s.glb" % f), "model present: " + f)
	for a_name in ["idle", "walk", "run", "attack", "cast", "hit", "dead"]:
		_assert(ResourceLoader.exists("res://assets/anims/humanoid/%s.res" % a_name), "animation present: " + a_name)
	if _failed:
		quit(1)
		return
	print("STARTUP_SMOKE_OK")
	quit(0)
