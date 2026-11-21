extends Control
## HUD extras (layout lives in hud.tscn): target frame, death screen, off-screen quest pointer.
## Pure presentation: reads the player, the world controller and the quest target; decides nothing.

@onready var death_dim: ColorRect = $DeathDim
@onready var death_title: Label = $DeathDim/DeathTitle
@onready var death_sub: Label = $DeathDim/DeathSub
@onready var target_frame: PanelContainer = $TargetFrame
@onready var target_name: Label = $TargetFrame/VBox/TargetName
@onready var target_hp: ProgressBar = $TargetFrame/VBox/TargetHp
@onready var target_hp_text: Label = $TargetFrame/VBox/TargetHp/TargetHpText
@onready var pointer: Control = $Pointer
@onready var arrow: Label = $Pointer/Arrow
@onready var dist_label: Label = $Pointer/Dist

const UNITS_PER_METER := 32.0
const EDGE := 70.0
const TARGET_SHOW_RADIUS := 420.0
var _t := 0.0
var _dead_for := 0.0
var _target_node: Node2D = null

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_process(true)

func _process(delta: float) -> void:
	var player := get_tree().get_first_node_in_group("player") as Node2D
	if player == null:
		return
	_update_death(player, delta)
	_t -= delta
	if _t <= 0.0:
		_t = 0.15
		_update_target(player)
	_update_pointer(player)

func _update_death(player: Node, delta: float) -> void:
	var dead: bool = bool(player.get("online_dead"))
	death_dim.visible = dead
	if dead:
		_dead_for += delta
		death_title.text = tr("ui.death_title")
		death_sub.text = tr("ui.death_sub")
	else:
		_dead_for = 0.0

func _update_target(player: Node2D) -> void:
	var wc := get_tree().get_first_node_in_group("world_controller")
	var e = null
	if wc != null and wc.has_method("is_active") and wc.is_active():
		e = wc.nearest_enemy(TARGET_SHOW_RADIUS, false)
	target_frame.visible = e != null and not death_dim.visible
	if e == null:
		return
	target_name.text = str(e.get("display_name"))
	var mx := maxf(1.0, float(e.get("max_hp")))
	target_hp.max_value = mx
	target_hp.value = float(e.get("hp"))
	target_hp_text.text = "%d / %d" % [int(e.get("hp")), int(mx)]

func _update_pointer(player: Node2D) -> void:
	var gw := get_tree().get_first_node_in_group("game_world")
	if gw == null or not gw.has_method("quest_target_node") or death_dim.visible:
		pointer.visible = false
		return
	if _target_node == null or not is_instance_valid(_target_node) or Engine.get_process_frames() % 30 == 0:
		_target_node = gw.quest_target_node()
	if _target_node == null:
		pointer.visible = false
		return
	var vp := get_viewport_rect().size
	var screen: Vector2 = _target_node.get_global_transform_with_canvas().origin
	var dist_m: float = player.global_position.distance_to(_target_node.global_position) / UNITS_PER_METER
	var inside := screen.x > EDGE and screen.y > EDGE and screen.x < vp.x - EDGE and screen.y < vp.y - EDGE
	pointer.visible = true
	if inside:
		# On screen: a bobbing marker just above the target.
		pointer.position = screen + Vector2(0, -92 + sin(Time.get_ticks_msec() / 220.0) * 6.0)
		arrow.rotation = PI
	else:
		var centre := vp / 2.0
		var dir := (screen - centre).normalized()
		var half := vp / 2.0 - Vector2(EDGE, EDGE)
		var k := minf(half.x / maxf(absf(dir.x), 0.001), half.y / maxf(absf(dir.y), 0.001))
		pointer.position = centre + dir * k
		arrow.rotation = dir.angle() + PI / 2.0
	dist_label.text = "%d м" % int(round(dist_m))
