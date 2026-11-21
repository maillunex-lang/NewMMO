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
	var feira := main.get_node_or_null("DawnValley/NPCSpawns/FeiraNiss")
	var ilra := main.get_node_or_null("DawnValley/NPCSpawns/IlraVenn")
	var quest := main.get_node_or_null("QuestManager")
	var hud := main.get_node_or_null("CanvasLayer/HUD")
	var slime := main.get_node_or_null("DawnValley/MonsterSpawns/Slime1")
	var failed := false
	for item in [
		["player", player],
		["feira", feira],
		["ilra", ilra],
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
	if not failed:
		var adjacent: Node2D = main.get_node("DawnValley/MonsterSpawns/Slime2")
		for monster in get_nodes_in_group("monsters"):
			monster.set_physics_process(false)
			monster.global_position = player.global_position + Vector2(300, 0)
		player.set_physics_process(false)
		slime.global_position = player.global_position - Vector2(24, 0)
		adjacent.global_position = player.global_position + Vector2(50, 0)
		var nearest_hp: float = slime.hp
		var adjacent_hp: float = adjacent.hp
		await player.use_skill(0)
		if slime.hp >= nearest_hp or adjacent.hp != adjacent_hp:
			push_error("Manual attack must only damage the closest enemy")
			failed = true
		slime.global_position = player.global_position + Vector2(300, 0)
		adjacent.global_position = player.global_position + Vector2(300, 0)
		player._attack_cd = 0.0
		var start_mp: float = player.mp
		await player.use_skill(1)
		if player.mp != start_mp or player._attack_cd != 0.0:
			push_error("Empty manual attack must not spend resource or start cooldown")
			failed = true
	if failed:
		quit(2)
	else:
		print("STAGE1_SMOKE_OK")
		quit(0)
