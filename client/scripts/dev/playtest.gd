extends SceneTree
## Dev-only live playtest bot (NOT part of CI): connects the REAL client to a running server with a ticket from
## the environment, plays with auto-combat and logs what a new player would see. Needs a display (xvfb) for shots.
## env: NEWMMO_PT_TICKET, NEWMMO_PT_WS, NEWMMO_PT_NAME, NEWMMO_PT_CLASS, NEWMMO_PT_SECONDS, NEWMMO_PT_OUT, NEWMMO_PT_AUTO (1|0)

const OnlineSession = preload("res://scripts/net/online_session.gd")

var _out := "/tmp"

func _init() -> void:
	call_deferred("_run")

func _shot(tag: String) -> void:
	var img := root.get_viewport().get_texture().get_image()
	img.save_png("%s/pt_%s.png" % [_out, tag])

func _run() -> void:
	_out = OS.get_environment("NEWMMO_PT_OUT") if OS.get_environment("NEWMMO_PT_OUT") != "" else "/tmp"
	var secs := int(OS.get_environment("NEWMMO_PT_SECONDS")) if OS.get_environment("NEWMMO_PT_SECONDS") != "" else 90
	var sess := OnlineSession.new()
	sess.name = "OnlineSession"
	root.add_child(sess)
	var ch := {"character_id": OS.get_environment("NEWMMO_PT_CHAR_ID"), "name": OS.get_environment("NEWMMO_PT_NAME"), "class_id": OS.get_environment("NEWMMO_PT_CLASS")}
	var err: Error = sess.configure_from_ticket(OS.get_environment("NEWMMO_PT_TICKET"), OS.get_environment("NEWMMO_PT_WS"), ch)
	print("PT connect err=", err)
	var waited := 0.0
	var ready := false
	sess.ready_for_gameplay.connect(func(_c): ready = true)
	while not ready and waited < 10.0:
		await process_frame
		waited += 0.016
	print("PT session ready=", ready)
	change_scene_to_file("res://scenes/world/main.tscn")
	for i in 90:
		await process_frame
	var wc := get_first_node_in_group("world_controller")
	var player := get_first_node_in_group("player")
	print("PT world active=", wc.is_active() if wc else "no wc")
	await create_timer(2.0).timeout
	_shot("00_start")
	var v3 := get_first_node_in_group("view3d")
	if v3 != null and OS.get_environment("NEWMMO_PT_DEBUG3D") == "1":
		for id in v3._views.keys():
			var e: Dictionary = v3._views[id]
			if e["kind"] in ["hero", "enemy"]:
				var vw: Node3D = e["view"]
				var ab := ModelLibrary._aabb_of(vw)
				print("PT3D ", e["kind"], " pos=", vw.position, " vis=", vw.visible, " scale=", vw.scale, " aabb=", ab, " ap=", e["ap"] != null, " anim=", e["anim"])
	if OS.get_environment("NEWMMO_PT_AUTO") != "0":
		player.toggle_auto_combat()
	if OS.get_environment("NEWMMO_PT_WALK") == "1":
		player.move_to_world(player.global_position + Vector2(260, 140))
	var errs := {}
	var casts := [0]
	sess.net.command_error.connect(func(p): errs[str(p.get("code"))] = int(errs.get(str(p.get("code")), 0)) + 1)
	sess.net.world_cast_result.connect(func(_p): casts[0] += 1)
	var deaths := 0
	var was_dead := false
	var t := 0
	var shot_death := false
	while t < secs:
		await create_timer(1.0).timeout
		t += 1
		var dead: bool = bool(player.get("online_dead"))
		if dead and not was_dead:
			deaths += 1
			if not shot_death:
				_shot("death")
				shot_death = true
		was_dead = dead
		var near = wc.nearest_enemy(2000.0, true)
		var dist: float = -1.0
		var ehp := ""
		if near != null:
			dist = (wc.get("_server_position") as Vector2).distance_to(near.target_pos)
			ehp = "%s hp=%s" % [str(near.get("ref")), str(near.get("hp"))]
		var ents: Dictionary = wc.get("_entities")
		var n_enemies := 0
		for k in ents.keys():
			if is_instance_valid(ents[k]) and str(ents[k].get("kind")) == "enemy":
				n_enemies += 1
		print("PT t=%d hp=%.0f/%.0f mp=%.0f dead=%s deaths=%d enemies=%d nearest=%.0f %s auto=%s casts=%d errs=%s" % [t, player.hp, player.max_hp, player.mp, str(dead), deaths, n_enemies, dist, ehp, str(player.auto_combat), casts[0], str(errs)])
		if t in [8, 30, 60] or (OS.get_environment("NEWMMO_PT_BURST") == "1" and t >= 3 and t <= 9):
			_shot("t%02d" % t)
	_shot("end")
	print("PT DONE deaths=%d" % deaths)
	quit(0)
