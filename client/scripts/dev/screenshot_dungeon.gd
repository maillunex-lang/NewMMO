extends SceneTree
## Dev: renders one dungeon room (R02 by default) with its stand-in monsters, for visual review.

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	DisplayServer.window_set_size(Vector2i(1280, 720))
	var room := "R02"
	for a in OS.get_cmdline_user_args():
		if a.begins_with("--room="):
			room = a.get_slice("=", 1)
	var scene: Node2D = load("res://scenes/dungeon/dng_01_001.tscn").instantiate()
	root.add_child(scene)
	for r in scene.get_node("Rooms").get_children():
		r.visible = r.name == room
	var cam := Camera2D.new()
	cam.position = Vector2(380, 280)
	cam.zoom = Vector2(1.5, 1.5)
	root.add_child(cam)
	cam.make_current()
	for m in scene.get_node("Rooms/%s/Spawns" % room).get_children():
		var packed: PackedScene = load(str(m.get_meta("monster_scene")))
		var mon: Node2D = packed.instantiate()
		scene.get_node("Playground").add_child(mon)
		mon.position = m.position
	var hero: Node2D = load("res://scenes/characters/player.tscn").instantiate()
	scene.get_node("Playground").add_child(hero)
	hero.position = scene.get_node("Rooms/%s/Entry" % room).position + Vector2(80, -60)
	for i in range(30):
		await process_frame
	var out_dir := ProjectSettings.globalize_path("res://build")
	DirAccess.make_dir_recursive_absolute(out_dir)
	var err := root.get_texture().get_image().save_png(out_dir + "/dungeon_%s.png" % room)
	print("SCREENSHOT dungeon_%s err=%d" % [room, err])
	quit(0)
