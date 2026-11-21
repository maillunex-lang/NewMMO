extends SceneTree

## Captures Stage-1 reference frames: world+HUD, dialogue, quest journal.
## Used for engine upgrade visual compare (T-015).

func _init() -> void:
	call_deferred("_run")


func _run() -> void:
	DisplayServer.window_set_size(Vector2i(1280, 720))
	change_scene_to_file("res://scenes/world/main.tscn")
	for i in range(45):
		await process_frame

	var out_dir := ProjectSettings.globalize_path("res://build")
	DirAccess.make_dir_recursive_absolute(out_dir)

	_save("stage1_dawn_valley.png")

	var world := get_root().get_child(get_root().get_child_count() - 1)
	var dialogue: Control = world.get_node_or_null("CanvasLayer/Dialogue") as Control
	var journal: Control = world.get_node_or_null("CanvasLayer/QuestJournal") as Control
	var qm: Node = world.get_node_or_null("QuestManager")

	if dialogue and dialogue.has_method("open_flow"):
		var tex: Texture2D = null
		if qm and qm.has_method("get_line"):
			pass
		dialogue.open_flow("NPC-802", "Старейшина Мира", PackedStringArray(["DIA.MQ-01-01.START.01"]), tex, "offer")
		for i in range(20):
			await process_frame
		_save("stage1_dialogue.png")
		if dialogue.has_method("close"):
			dialogue.close()
		elif dialogue.has_method("_on_close"):
			dialogue.visible = false
		else:
			dialogue.visible = false
		for i in range(10):
			await process_frame

	if journal and journal.has_method("open"):
		journal.open()
		for i in range(20):
			await process_frame
		_save("stage1_journal.png")

	print("SCREENSHOTS_OK dir=", out_dir)
	quit(0)


func _save(name: String) -> void:
	var img := get_root().get_viewport().get_texture().get_image()
	var out := ProjectSettings.globalize_path("res://build").path_join(name)
	var err := img.save_png(out)
	img.save_png(OS.get_user_data_dir().path_join(name))
	print("SCREENSHOT ", out, " err=", err)
