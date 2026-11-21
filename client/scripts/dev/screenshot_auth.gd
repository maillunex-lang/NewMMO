extends SceneTree
## Captures auth UI panels for T-022 evidence (FakeHttp-backed, no network).

const AuthClient = preload("res://scripts/net/auth_client.gd")
const FakeHttp = preload("res://scripts/net/fake_http.gd")


func _init() -> void:
	call_deferred("_run")


func _run() -> void:
	DisplayServer.window_set_size(Vector2i(1280, 720))
	var packed: PackedScene = load("res://scenes/ui/auth_flow.tscn")
	var ui: Control = packed.instantiate()
	var fh := FakeHttp.new()
	var auth := AuthClient.new()
	auth.configure(fh, "http://127.0.0.1:18086")
	ui.configure_auth(auth)
	root.add_child(ui)
	for i in range(20):
		await process_frame

	var out_dir := ProjectSettings.globalize_path("res://build")
	DirAccess.make_dir_recursive_absolute(out_dir)
	_save(out_dir, "auth_login.png")

	ui._show_panel(ui.panel_register)
	for i in range(10):
		await process_frame
	_save(out_dir, "auth_register.png")

	auth.access_token = "mem"
	fh.enqueue("GET", "/v1/characters", 200, {
		"characters": [
			{"character_id": "1", "name": "Мира", "class_id": "CLS-01", "level": 1},
			{"character_id": "2", "name": "Рок", "class_id": "CLS-02", "level": 3},
		],
	})
	ui._refresh_characters()
	for i in range(15):
		await process_frame
	_save(out_dir, "auth_characters.png")

	ui._show_panel(ui.panel_create)
	for i in range(10):
		await process_frame
	_save(out_dir, "auth_create.png")

	print("AUTH_SCREENSHOTS_OK dir=", out_dir)
	quit(0)


func _save(out_dir: String, name: String) -> void:
	var img := root.get_viewport().get_texture().get_image()
	var path := out_dir.path_join(name)
	img.save_png(path)
	print("SCREENSHOT ", path)
