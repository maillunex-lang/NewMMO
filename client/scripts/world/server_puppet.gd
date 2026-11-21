extends Node2D
## T-038d: visual puppet of a server entity (enemy or another player). Position, facing, HP and death come
## from world.snapshot / world.delta; this node never decides anything. Smooths the 10 Hz updates.

var entity_id := 0
var kind := "enemy"
var target_pos := Vector2.ZERO
var moving := false
var dead := false
var hp := 0
var max_hp := 1
var display_name := ""
var ref := ""

@onready var animator: DirectionalAnimator = $DirectionalAnimator
@onready var hp_bar: ProgressBar = $HpBar
@onready var name_label: Label = $NameLabel

## Called right after instantiate(), before add_child(): children exist, _ready has not run yet.
func setup(view: Dictionary, sheet: Texture2D, frame: int, offset_y: float, bar_top: float) -> void:
	var anim := get_node("DirectionalAnimator")
	anim.sheet = sheet
	anim.frame_width = frame
	anim.frame_height = frame
	anim.idle_frames = 1
	anim.run_frames = 4
	anim.attack_frames = 3
	get_node("Sprite").offset = Vector2(0, offset_y)
	var bar := get_node("HpBar") as ProgressBar
	bar.offset_top = bar_top
	bar.offset_bottom = bar_top + 7.0
	var label := get_node("NameLabel") as Label
	label.offset_top = bar_top - 18.0
	label.offset_bottom = bar_top - 2.0
	entity_id = int(view.get("id", 0))
	kind = str(view.get("kind", "enemy"))
	display_name = str(view.get("name", ""))
	ref = str(view.get("ref", ""))
	global_position = Vector2(float(view.get("x", 0)), float(view.get("y", 0)))
	target_pos = global_position

func _ready() -> void:
	add_to_group("server_enemies" if kind == "enemy" else "server_players")
	name_label.text = display_name
	name_label.visible = kind == "player" or display_name != ""
	hp_bar.max_value = maxf(1.0, float(max_hp))
	hp_bar.value = float(hp)

func apply_view(v: Dictionary) -> void:
	target_pos = Vector2(float(v.get("x", target_pos.x)), float(v.get("y", target_pos.y)))
	moving = bool(v.get("moving", false))
	hp = int(v.get("hp", hp))
	max_hp = maxi(1, int(v.get("max_hp", max_hp)))
	if v.has("name") and str(v["name"]) != "":
		display_name = str(v["name"])
	var was_dead := dead
	dead = bool(v.get("dead", false))
	if is_node_ready():
		animator.set_direction_index(int(v.get("facing", 2)))
		hp_bar.max_value = float(max_hp)
		hp_bar.value = float(hp)
		name_label.text = display_name
		if was_dead and not dead:
			global_position = target_pos  # respawned: appear at the spawn, do not glide there
		visible = not dead

func is_alive() -> bool:
	return not dead and hp > 0

func _process(delta: float) -> void:
	z_index = int(global_position.y)
	if dead:
		return
	var d := target_pos - global_position
	if d.length() > 200.0:
		global_position = target_pos
	else:
		global_position = global_position.lerp(target_pos, 1.0 - exp(-12.0 * delta))
	if moving or d.length() > 3.0:
		animator.play_run()
	else:
		animator.play_idle()
