extends Node
## Background asset downloader. The Android APK ships without the heavy 3D models/animations (export filter);
## this autoload fetches `manifest.json` from the assets host, downloads missing .pck packs in the background
## (HTTPS, sha256-verified, cached in user://packs, resumable by re-download) and mounts them with
## ProjectSettings.load_resource_pack. The game stays playable meanwhile (ModelLibrary falls back to placeholders)
## and View3D refreshes its models when `packs_ready` fires. Desktop/editor runs that already have res://assets/models
## skip it (set NEWMMO_FORCE_PACKS=1 to test). Host layout: tools/android/build_asset_packs.sh.

signal progress(done_bytes: int, total_bytes: int)
signal packs_ready()
signal failed(reason: String)

const DEFAULT_URL := "https://rikagro.ru/newmmo/assets/"
const CACHE_DIR := "user://packs/"
const MAX_RETRIES := 6

var base_url := DEFAULT_URL
var ready_state := false
var total_bytes := 0
var done_bytes := 0

var _http: HTTPRequest = null
var _manifest: Dictionary = {}
var _queue: Array = []
var _current: Dictionary = {}
var _retries := 0
var _label: Label = null
var _layer: CanvasLayer = null

func _ready() -> void:
	if ResourceLoader.exists("res://assets/models/tree_oak.glb") and OS.get_environment("NEWMMO_FORCE_PACKS") != "1":
		ready_state = true
		return
	if OS.get_environment("NEWMMO_ASSETS_URL") != "":
		base_url = OS.get_environment("NEWMMO_ASSETS_URL")
	if not base_url.ends_with("/"):
		base_url += "/"
	DirAccess.make_dir_recursive_absolute(CACHE_DIR)
	_http = HTTPRequest.new()
	_http.use_threads = true
	add_child(_http)
	_build_ui()
	_fetch_manifest()

func _build_ui() -> void:
	_layer = CanvasLayer.new()
	_layer.layer = 100
	add_child(_layer)
	_label = Label.new()
	_label.add_theme_font_size_override("font_size", 16)
	_label.add_theme_color_override("font_outline_color", Color(0, 0, 0, 0.9))
	_label.add_theme_constant_override("outline_size", 5)
	_label.set_anchors_and_offsets_preset(Control.PRESET_CENTER_BOTTOM)
	_label.offset_top = -34
	_label.offset_left = -200
	_label.offset_right = 200
	_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_label.visible = false
	_layer.add_child(_label)

func _fetch_manifest() -> void:
	_http.download_file = ""
	if not _http.request_completed.is_connected(_on_manifest):
		_http.request_completed.connect(_on_manifest, CONNECT_ONE_SHOT)
	if _http.request(base_url + "manifest.json") != OK:
		_retry("manifest request")

func _on_manifest(result: int, code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	if result != HTTPRequest.RESULT_SUCCESS or code != 200:
		_retry("manifest HTTP %d/%d" % [result, code])
		return
	var parsed: Variant = JSON.parse_string(body.get_string_from_utf8())
	if typeof(parsed) != TYPE_DICTIONARY or typeof((parsed as Dictionary).get("packs")) != TYPE_ARRAY:
		_retry("manifest format")
		return
	_manifest = parsed
	_retries = 0
	_queue.clear()
	total_bytes = 0
	done_bytes = 0
	var keep: Array = []
	for p in _manifest["packs"]:
		var file := str(p.get("file", ""))
		if file == "" or file.contains("/") or file.contains(".."):
			continue
		keep.append(file)
		var path := CACHE_DIR + file
		if FileAccess.file_exists(path) and FileAccess.get_sha256(path) == str(p.get("sha256", "")):
			_mount(path)
		else:
			_queue.append(p)
			total_bytes += int(p.get("size", 0))
	_cleanup_old(keep)
	if _queue.is_empty():
		_finish()
	else:
		_next_download()

func _cleanup_old(keep: Array) -> void:
	var d := DirAccess.open(CACHE_DIR)
	if d == null:
		return
	for f in d.get_files():
		if f.ends_with(".pck") and not keep.has(f):
			d.remove(f)
		elif f.ends_with(".part"):
			d.remove(f)

func _next_download() -> void:
	if _queue.is_empty():
		_finish()
		return
	_current = _queue[0]
	var tmp := CACHE_DIR + str(_current["file"]) + ".part"
	_http.download_file = tmp
	if not _http.request_completed.is_connected(_on_pack):
		_http.request_completed.connect(_on_pack, CONNECT_ONE_SHOT)
	if _http.request(base_url + str(_current["file"])) != OK:
		_retry("pack request")
		return
	set_process(true)

func _process(_delta: float) -> void:
	if _http == null or _current.is_empty():
		return
	var cur := _http.get_downloaded_bytes()
	var shown := done_bytes + cur
	progress.emit(shown, total_bytes)
	if _label != null and total_bytes > 0:
		_label.visible = true
		_label.text = tr("ui.assets_loading") % int(100.0 * float(shown) / float(total_bytes))

func _on_pack(result: int, code: int, _headers: PackedStringArray, _body: PackedByteArray) -> void:
	var file := str(_current.get("file", ""))
	var tmp := CACHE_DIR + file + ".part"
	if result != HTTPRequest.RESULT_SUCCESS or code != 200:
		DirAccess.remove_absolute(tmp)
		_retry("pack HTTP %d/%d" % [result, code])
		return
	if FileAccess.get_sha256(tmp) != str(_current.get("sha256", "")):
		DirAccess.remove_absolute(tmp)
		_retry("pack checksum")
		return
	var final := CACHE_DIR + file
	DirAccess.rename_absolute(tmp, final)
	done_bytes += int(_current.get("size", 0))
	_queue.pop_front()
	_current = {}
	_retries = 0
	_mount(final)
	_next_download()

func _mount(path: String) -> void:
	if not ProjectSettings.load_resource_pack(path, true):
		failed.emit("cannot mount " + path)

func _finish() -> void:
	set_process(false)
	ready_state = true
	if _label != null:
		_label.visible = false
	packs_ready.emit()

func _retry(reason: String) -> void:
	_retries += 1
	if _label != null:
		_label.visible = false
	if _retries > MAX_RETRIES:
		failed.emit(reason)
		return
	var wait := minf(60.0, 5.0 * pow(2.0, float(_retries - 1)))
	get_tree().create_timer(wait).timeout.connect(func():
		if _manifest.is_empty():
			_fetch_manifest()
		else:
			_next_download())
