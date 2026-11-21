extends SceneTree
## Headless CLI:  godot --headless --path client -s res://addons/lunex_gen/cli.gd -- <verify|generate|dry-run|export> <scene.tscn> [seed=N] [out=path]
## verify: exit 1 if any anchor is unreachable or a quest target has no node. generate: rewrites the scene's Generated nodes.

func _init() -> void:
	var args := OS.get_cmdline_user_args()
	if args.size() < 2:
		push_error("usage: <verify|generate|dry-run|export> <scene.tscn> [seed=N] [out=path]")
		quit(2)
		return
	var cmd := args[0]
	var path := args[1]
	var opts := {}
	for a in args.slice(2):
		var kv := a.split("=", true, 1)
		if kv.size() == 2:
			opts[kv[0]] = kv[1]
	var packed: PackedScene = load(path)
	if packed == null:
		push_error("cannot load " + path)
		quit(2)
		return
	var root: Node = packed.instantiate()
	get_root().add_child(root)
	var code := 0
	var params := LunexGen.params_from_scene(root)
	if opts.has("seed"):
		params["seed"] = int(opts["seed"])
	match cmd:
		"verify":
			var v := LunexGen.verify(root)
			var missing := LunexGen.missing_quest_targets(root)
			print("MAPGEN_VERIFY ", JSON.stringify({"ok": v["ok"], "anchors": v["anchors"], "unreachable": v["unreachable"], "missing_quest_targets": missing}))
			code = 0 if (bool(v["ok"]) and missing.is_empty()) else 1
		"generate", "dry-run":
			var rep := LunexGen.generate(root, params, cmd == "generate")
			print("MAPGEN_REPORT ", JSON.stringify(rep))
			if not bool(rep["ok"]):
				code = 1
			elif cmd == "generate":
				var out := PackedScene.new()
				if out.pack(root) != OK or ResourceSaver.save(out, opts.get("out", path)) != OK:
					push_error("save failed")
					code = 2
		"export":
			var f := FileAccess.open(opts.get("out", "res://data/maps/obstacles.json"), FileAccess.WRITE)
			f.store_string(JSON.stringify(LunexGen.export_obstacles(root), "", false))
			f.close()
		_:
			code = 2
	root.queue_free()
	quit(code)
