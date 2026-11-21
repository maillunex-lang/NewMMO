class_name DirectionalAnimator
extends Node
## Builds SpriteFrames from an atlas sheet: rows = 8 dirs, columns = idle|run|attack.

@export var sprite_path: NodePath
@export var sheet: Texture2D
@export var frame_width := 64
@export var frame_height := 64
@export var idle_frames := 4
@export var run_frames := 6
@export var attack_frames := 4
@export var idle_fps := 6.0
@export var run_fps := 10.0
@export var attack_fps := 12.0

var _sprite: AnimatedSprite2D
var _dir := 2  # south
var _playing_attack := false

func _ready() -> void:
	_sprite = get_node(sprite_path) as AnimatedSprite2D
	if sheet == null or _sprite == null:
		push_warning("DirectionalAnimator missing sheet or sprite")
		return
	_sprite.sprite_frames = _build_frames()
	play_idle()

func _build_frames() -> SpriteFrames:
	var frames := SpriteFrames.new()
	for di in range(8):
		_add_clip(frames, "idle_%d" % di, di, 0, idle_frames, idle_fps, true)
		_add_clip(frames, "run_%d" % di, di, idle_frames, run_frames, run_fps, true)
		_add_clip(frames, "attack_%d" % di, di, idle_frames + run_frames, attack_frames, attack_fps, false)
	return frames

func _add_clip(frames: SpriteFrames, anim: String, row: int, start_col: int, count: int, fps: float, loop: bool) -> void:
	if frames.has_animation(anim):
		frames.remove_animation(anim)
	frames.add_animation(anim)
	frames.set_animation_speed(anim, fps)
	frames.set_animation_loop(anim, loop)
	for i in range(count):
		var atlas := AtlasTexture.new()
		atlas.atlas = sheet
		atlas.region = Rect2((start_col + i) * frame_width, row * frame_height, frame_width, frame_height)
		frames.add_frame(anim, atlas)

func set_direction_from_vector(v: Vector2) -> void:
	if v.length() < 0.1:
		return
	var ang := fposmod(atan2(v.y, v.x), TAU)
	_dir = int(round(ang / (TAU / 8.0))) % 8

func set_direction_index(i: int) -> void:
	_dir = clampi(i, 0, 7)

func play_idle() -> void:
	if _playing_attack or _sprite == null:
		return
	_sprite.play("idle_%d" % _dir)

func play_run() -> void:
	if _playing_attack or _sprite == null:
		return
	_sprite.play("run_%d" % _dir)

func play_attack() -> void:
	if _sprite == null:
		return
	_playing_attack = true
	var anim := "attack_%d" % _dir
	_sprite.play(anim)
	await _sprite.animation_finished
	_playing_attack = false

func is_attacking() -> bool:
	return _playing_attack
