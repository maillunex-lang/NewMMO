extends SceneTree
## Headless: the background asset downloader against a local static server (NEWMMO_ASSETS_URL, NEWMMO_FORCE_PACKS=1).
func _init() -> void:
	call_deferred("_run")
func _run() -> void:
	var ap := root.get_node_or_null("AssetPacks")
	if ap == null:
		push_error("AssetPacks autoload missing")
		quit(1)
		return
	var ok := [false]
	var fail := [""]
	ap.packs_ready.connect(func(): ok[0] = true)
	ap.failed.connect(func(r): fail[0] = r)
	var t := 0.0
	while not ok[0] and fail[0] == "" and t < 40.0:
		await create_timer(0.25).timeout
		t += 0.25
	var files := DirAccess.get_files_at("user://packs/")
	var pcks := 0
	for f in files:
		if f.ends_with(".pck"):
			pcks += 1
	if not ok[0] or pcks < 1:
		push_error("download failed: ok=%s fail=%s files=%s" % [ok[0], fail[0], files])
		quit(1)
		return
	print("ASSETS_SMOKE_OK packs=%d bytes=%d" % [pcks, ap.done_bytes])
	quit(0)
