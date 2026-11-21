extends Control

@onready var name_label: Label = $SafeArea/Chrome/TopBar/Stats/Bars/Name
@onready var health: ProgressBar = $SafeArea/Chrome/TopBar/Stats/Bars/Health
@onready var mana: ProgressBar = $SafeArea/Chrome/TopBar/Stats/Bars/Mana
@onready var quest_text: Label = $SafeArea/Chrome/Quest/QuestText
@onready var quest_panel: PanelContainer = $SafeArea/Chrome/Quest
@onready var loot_log: Label = $SafeArea/Chrome/LootLog
@onready var auto_btn: Button = $SafeArea/Chrome/AutoCombat
@onready var interact_btn: Button = $SafeArea/Chrome/Interact
@onready var journal_btn: Button = $SafeArea/Chrome/JournalBtn
@onready var locale_btn: OptionButton = $SafeArea/Chrome/LocaleBtn
@onready var bag_btn: Button = $SafeArea/Chrome/BagBtn
@onready var credits_btn: Button = $SafeArea/Chrome/CreditsBtn
@onready var joystick: Control = $SafeArea/Chrome/Joystick
@onready var skills: Control = $SafeArea/Chrome/Skills

@export var player_name_placeholder := "…"

const LocaleSettings = preload("res://scripts/ui/locale_settings.gd")

var _player: Node = null
var _quest: Node = null
var _journal: Node = null
var _inventory: Node = null

func _ready() -> void:
	add_to_group("hud")
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	for i in range(skills.get_child_count()):
		var btn := skills.get_child(i)
		if btn is Button:
			var slot := i
			(btn as Button).pressed.connect(func(): _on_skill(slot))
	if auto_btn:
		auto_btn.pressed.connect(_on_auto)
	if quest_panel:
		quest_panel.mouse_filter = Control.MOUSE_FILTER_STOP
		quest_panel.gui_input.connect(_on_quest_tapped)
		quest_text.mouse_filter = Control.MOUSE_FILTER_IGNORE
	if interact_btn:
		interact_btn.pressed.connect(_on_interact)
	if journal_btn:
		journal_btn.pressed.connect(_on_journal)
	if bag_btn:
		bag_btn.pressed.connect(_on_bag)
	if credits_btn:
		credits_btn.pressed.connect(_on_credits)
	if locale_btn:
		_setup_locale_btn()
	if joystick and joystick.has_signal("direction_changed"):
		joystick.direction_changed.connect(_on_joystick)
	loot_log.text = ""
	_apply_chrome_labels()
	call_deferred("_bind_world")

func _notification(what: int) -> void:
	if what == NOTIFICATION_TRANSLATION_CHANGED:
		_apply_chrome_labels()

func _apply_chrome_labels() -> void:
	if auto_btn:
		auto_btn.text = tr("ui.auto_on") if _player != null and _player.auto_combat else tr("ui.auto")
	if interact_btn:
		interact_btn.text = tr("ui.talk")
	if journal_btn:
		journal_btn.text = tr("ui.journal")
	if bag_btn:
		bag_btn.text = tr("ui.inventory")
	if credits_btn:
		credits_btn.text = tr("ui.credits")

func _setup_locale_btn() -> void:
	var current := LocaleSettings.load_locale()
	TranslationServer.set_locale(current)
	locale_btn.set_block_signals(true)
	locale_btn.select(0 if current.begins_with("ru") else 1)
	locale_btn.set_block_signals(false)
	if not locale_btn.item_selected.is_connected(_on_locale_selected):
		locale_btn.item_selected.connect(_on_locale_selected)

func _on_locale_selected(index: int) -> void:
	var code := "ru" if index == 0 else "en"
	LocaleSettings.save_locale(code)
	TranslationServer.set_locale(code)
	_apply_chrome_labels()
	if _quest and _quest.has_method("get_tracker_text"):
		quest_text.text = _quest_text(_quest.get_tracker_text())

func _bind_world() -> void:
	_player = get_tree().get_first_node_in_group("player")
	_quest = get_tree().get_first_node_in_group("quest_manager")
	_journal = get_tree().get_first_node_in_group("quest_journal")
	if _player:
		if _player.has_signal("hp_changed"):
			_player.hp_changed.connect(_on_hp)
		if _player.has_signal("mp_changed"):
			_player.mp_changed.connect(_on_mp)
		if _player.has_signal("loot_gained"):
			_player.loot_gained.connect(_on_loot)
		if _player.has_signal("hint"):
			_player.hint.connect(_flash)
		_on_hp(_player.hp, _player.max_hp)
		_on_mp(_player.mp, _player.max_mp)
		name_label.text = player_name_placeholder
		var sess := get_tree().get_first_node_in_group("online_session")
		if sess != null and typeof(sess.get("character")) == TYPE_DICTIONARY and str((sess.character as Dictionary).get("name", "")) != "":
			name_label.text = str(sess.character.get("name"))
	if _quest and _quest.has_signal("quest_updated"):
		_quest.quest_updated.connect(_on_quest)
		quest_text.text = _quest_text(_quest.get_tracker_text())
	if _quest and _quest.has_signal("quest_completed"):
		_quest.quest_completed.connect(func(_id, reward): _flash(tr("ui.quest_complete_prefix") + str(reward)))

func _on_hp(hp: float, max_hp: float) -> void:
	health.max_value = max_hp
	health.value = hp
	var t := health.get_node_or_null("HpText") as Label
	if t:
		t.text = "%d / %d" % [roundi(hp), roundi(max_hp)]

func _on_mp(mp: float, max_mp: float) -> void:
	mana.max_value = max_mp
	mana.value = mp
	var t := mana.get_node_or_null("MpText") as Label
	if t:
		t.text = "%d / %d" % [roundi(mp), roundi(max_mp)]

func _quest_text(detail: String) -> String:
	return detail + "\n▶ " + tr("ui.quest_tap_hint")

func _on_quest(_id: String, _status: String, detail: String) -> void:
	quest_text.text = _quest_text(detail)

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
		auto_btn.text = tr("ui.auto_on") if on else tr("ui.auto")
		auto_btn.modulate = Color(1.0, 0.9, 0.5) if on else Color.WHITE

func _on_interact() -> void:
	if _player and _player.has_method("try_interact"):
		_player.try_interact()

func _on_quest_tapped(event: InputEvent) -> void:
	if event is InputEventMouseButton and event.pressed and event.button_index == MOUSE_BUTTON_LEFT:
		var world := get_tree().get_first_node_in_group("game_world")
		if world != null and world.has_method("navigate_to_quest"):
			world.navigate_to_quest()
	elif event is InputEventScreenTouch and event.pressed:
		var world := get_tree().get_first_node_in_group("game_world")
		if world != null and world.has_method("navigate_to_quest"):
			world.navigate_to_quest()

func _on_journal() -> void:
	if _journal == null:
		_journal = get_tree().get_first_node_in_group("quest_journal")
	if _journal and _journal.has_method("toggle"):
		_journal.toggle()

func _on_credits() -> void:
	var c := get_tree().get_first_node_in_group("credits_ui")
	if c != null and c.has_method("toggle"):
		c.toggle()

func _on_bag() -> void:
	if _inventory == null:
		_inventory = get_tree().get_first_node_in_group("inventory_ui")
	if _inventory and _inventory.has_method("toggle"):
		_inventory.toggle()
