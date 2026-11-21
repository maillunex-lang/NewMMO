extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	var err := change_scene_to_file("res://scenes/world/main.tscn")
	if err != OK:
		push_error("Failed to load main scene: %s" % err)
		quit(1)
		return
	await process_frame
	await process_frame
	await process_frame
	var main := root.get_child(0)
	if main == null:
		push_error("No main root child")
		quit(1)
		return
	var player := main.get_node_or_null("DawnValley/Player")
	var mira := main.get_node_or_null("DawnValley/NPCSpawns/ElderMira")
	var quest := main.get_node_or_null("QuestManager")
	var hud := main.get_node_or_null("CanvasLayer/HUD")
	var slime := main.get_node_or_null("DawnValley/MonsterSpawns/Slime1")
	var failed := false
	for item in [
		["player", player],
		["mira", mira],
		["quest", quest],
		["hud", hud],
		["slime", slime],
	]:
		if item[1] == null:
			push_error("Missing node: %s" % item[0])
			failed = true
	if player and not player.is_in_group("player"):
		push_error("Player not in group")
		failed = true
	if quest and quest.has_method("get_tracker_text"):
		print("QUEST: ", quest.get_tracker_text())
	if player and player.has_method("move_to_world"):
		player.move_to_world(player.global_position + Vector2(40, 0))
	for _i in range(30):
		await process_frame
	if failed:
		quit(2)
	else:
		print("STAGE1_SMOKE_OK")
		quit(0)
