extends Control

@onready var name_label: Label = $TopBar/Stats/Bars/Name
@onready var health: ProgressBar = $TopBar/Stats/Bars/Health
@onready var mana: ProgressBar = $TopBar/Stats/Bars/Mana
@onready var quest_text: Label = $Quest/QuestText
@onready var loot_log: Label = $LootLog
@onready var auto_btn: Button = $AutoCombat
@onready var interact_btn: Button = $Interact
@onready var joystick: Control = $Joystick
@onready var skills: HBoxContainer = $Skills

var _player: Node = null
var _quest: Node = null

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	for i in range(skills.get_child_count()):
		var btn := skills.get_child(i)
		if btn is Button:
			var slot := i
			(btn as Button).pressed.connect(func(): _on_skill(slot))
	if auto_btn:
		auto_btn.pressed.connect(_on_auto)
	if interact_btn:
		interact_btn.pressed.connect(_on_interact)
	if joystick and joystick.has_signal("direction_changed"):
		joystick.direction_changed.connect(_on_joystick)
	loot_log.text = ""
	call_deferred("_bind_world")

func _bind_world() -> void:
	_player = get_tree().get_first_node_in_group("player")
	_quest = get_tree().get_first_node_in_group("quest_manager")
	if _player:
		if _player.has_signal("hp_changed"):
			_player.hp_changed.connect(_on_hp)
		if _player.has_signal("mp_changed"):
			_player.mp_changed.connect(_on_mp)
		if _player.has_signal("loot_gained"):
			_player.loot_gained.connect(_on_loot)
		_on_hp(_player.hp, _player.max_hp)
		_on_mp(_player.mp, _player.max_mp)
		name_label.text = "Guardian · Lv 1"
	if _quest and _quest.has_signal("quest_updated"):
		_quest.quest_updated.connect(_on_quest)
		quest_text.text = _quest.get_tracker_text()
	if _quest and _quest.has_signal("quest_completed"):
		_quest.quest_completed.connect(func(_id, reward): _flash("Quest complete: " + reward))

func _on_hp(hp: float, max_hp: float) -> void:
	health.max_value = max_hp
	health.value = hp

func _on_mp(mp: float, max_mp: float) -> void:
	mana.max_value = max_mp
	mana.value = mp

func _on_quest(_id: String, _status: String, detail: String) -> void:
	quest_text.text = detail

func _on_loot(item_id: String, amount: int) -> void:
	_flash("+%d %s" % [amount, item_id])

func _flash(text: String) -> void:
	loot_log.text = text
	var tw := create_tween()
	loot_log.modulate.a = 1.0
	tw.tween_interval(2.0)
	tw.tween_property(loot_log, "modulate:a", 0.0, 0.6)

func _on_joystick(dir: Vector2) -> void:
	if _player and _player.has_method("set_joystick"):
		_player.set_joystick(dir)

func _on_skill(slot: int) -> void:
	if _player and _player.has_method("use_skill"):
		_player.use_skill(slot)

func _on_auto() -> void:
	if _player and _player.has_method("toggle_auto_combat"):
		var on: bool = _player.toggle_auto_combat()
		auto_btn.text = "AUTO ON" if on else "AUTO"
		auto_btn.modulate = Color(1.0, 0.9, 0.5) if on else Color.WHITE

func _on_interact() -> void:
	if _player and _player.has_method("try_interact"):
		_player.try_interact()
