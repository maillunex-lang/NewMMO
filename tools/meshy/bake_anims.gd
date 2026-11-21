extends SceneTree
## Bakes Meshy animation GLBs into small Animation resources: res://assets/anims/humanoid/<name>.res
## usage: godot --headless --path client --script res://../tools/meshy/bake_anims.gd -- <dir with idle.glb walk.glb ...>
## (run from tools/meshy/bake_anims.sh, which copies this script into client/scripts/dev first)
const LOOPS := ["idle", "walk", "run"]
func _init() -> void:
	call_deferred("_run")
func _run() -> void:
	var dir := OS.get_cmdline_user_args()[0]
	for f in DirAccess.get_files_at(dir):
		if not f.ends_with(".glb"):
			continue
		var name := f.get_basename()
		var doc := GLTFDocument.new()
		var st := GLTFState.new()
		if doc.append_from_file(dir + "/" + f, st) != OK:
			push_error("cannot read " + f)
			continue
		var n := doc.generate_scene(st)
		root.add_child(n)
		var ap := n.find_children("*", "AnimationPlayer", true, false)
		if ap.is_empty():
			continue
		var player := ap[0] as AnimationPlayer
		var an: Animation = player.get_animation(player.get_animation_list()[0]).duplicate()
		an.loop_mode = Animation.LOOP_LINEAR if name in LOOPS else Animation.LOOP_NONE
		var out := "res://assets/anims/humanoid/%s.res" % name
		ResourceSaver.save(an, out, ResourceSaver.FLAG_COMPRESS)
		print("BAKED ", out, " len=", snappedf(an.length, 0.01), " tracks=", an.get_track_count())
		n.queue_free()
	quit(0)
