extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	DisplayServer.window_set_size(Vector2i(1280, 720))
	change_scene_to_file("res://scenes/world/main.tscn")
	for i in range(45):
		await process_frame
	var img := get_root().get_viewport().get_texture().get_image()
	var path := "res://build/stage1_dawn_valley.png"
	# Write next to project for CI artifact pickup
	var err := img.save_png("user://stage1_dawn_valley.png")
	var abs_path := OS.get_user_data_dir().path_join("stage1_dawn_valley.png")
	var out := ProjectSettings.globalize_path("res://build/stage1_dawn_valley.png")
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path("res://build"))
	img.save_png(out)
	print("SCREENSHOT ", out, " err=", err, " userdata=", abs_path)
	quit(0)
