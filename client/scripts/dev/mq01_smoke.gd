extends SceneTree
## Headless progression of MQ-01-01…05 via quest_manager APIs.

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	var err := change_scene_to_file("res://scenes/world/main.tscn")
	if err != OK:
		push_error("load failed")
		quit(1)
		return
	for i in range(40):
		await process_frame
	var main := root.get_child(0)
	var qm: Node = main.get_node("QuestManager")
	var player: Node = main.get_node("DawnValley/Player")
	if qm == null or player == null:
		push_error("missing nodes")
		quit(2)
		return
	# Drive five quests
	var sequence := [
		["NPC-802", ["POI-BOARD-01", "POI-ACCEPT-01"]],
		["NPC-802", ["POI-WITNESS-A", "POI-WITNESS-B", "POI-EVIDENCE-TABLE"]],
		["NPC-102", ["POI-MARKER-A", "POI-MARKER-B", "POI-MEASURE-CRYSTAL"]],
		["NPC-102", ["POI-RESIDENT-GROUP", "POI-RESIDENT-GROUP", "POI-STABILIZE", "POI-STABILIZE", "POI-SHELTER"]],
		["NPC-802", ["POI-CLUE", "POI-CLUE", "POI-CLUE", "POI-FORGERY-REJECT", "NPC-102"]],
	]
	for step in sequence:
		var contact: String = step[0]
		var targets: Array = step[1]
		if str(qm.get_status()) != "available":
			push_error("expected available, got %s for %s" % [qm.get_status(), qm.get_active_quest_id()])
			quit(3)
			return
		qm.on_dialogue_action("ACCEPT", contact, player)
		await process_frame
		for t in targets:
			var ev := "npc_interact" if str(t).begins_with("NPC-") or str(t).begins_with("POI-WITNESS") else "world_interact"
			if str(t) == "POI-SHELTER":
				ev = "escort_checkpoint"
			qm.report_world_event(ev, str(t), player)
			await process_frame
		if str(qm.get_status()) != "objectives_complete":
			push_error("objectives incomplete %s status=%s snap=%s" % [qm.get_active_quest_id(), qm.get_status(), str(qm.debug_snapshot())])
			quit(4)
			return
		qm.on_dialogue_action("TURN_IN", contact, player)
		await process_frame
		print("DONE ", step)
	var snap: Dictionary = qm.debug_snapshot()
	print("MQ01_SMOKE_OK ", snap)
	if int(snap.get("xp", 0)) < 500:
		push_error("xp too low")
		quit(5)
		return
	quit(0)
