extends Control
## Quest journal binder. Layout lives in quest_journal.tscn; this script only binds data
## through QuestManager.get_journal_active / get_journal_completed (or a future server adapter).

@onready var title_label: Label = $SafeArea/Chrome/Panel/Margin/VBox/Header/Title
@onready var close_btn: Button = $SafeArea/Chrome/Panel/Margin/VBox/Header/Close
@onready var active_section: Label = $SafeArea/Chrome/Panel/Margin/VBox/ActiveSection
@onready var active_title: Label = $SafeArea/Chrome/Panel/Margin/VBox/ActiveTitle
@onready var active_status: Label = $SafeArea/Chrome/Panel/Margin/VBox/ActiveStatus
@onready var active_journal: Label = $SafeArea/Chrome/Panel/Margin/VBox/ActiveJournal
@onready var objectives_section: Label = $SafeArea/Chrome/Panel/Margin/VBox/ObjectivesSection
@onready var objectives_list: ItemList = $SafeArea/Chrome/Panel/Margin/VBox/ObjectivesList
@onready var reward_section: Label = $SafeArea/Chrome/Panel/Margin/VBox/RewardSection
@onready var reward_text: Label = $SafeArea/Chrome/Panel/Margin/VBox/RewardText
@onready var completed_section: Label = $SafeArea/Chrome/Panel/Margin/VBox/CompletedSection
@onready var completed_list: ItemList = $SafeArea/Chrome/Panel/Margin/VBox/CompletedList
@onready var dim: ColorRect = $Dim

var _source: Node = null

func _ready() -> void:
	visible = false
	mouse_filter = Control.MOUSE_FILTER_STOP
	add_to_group("quest_journal")
	if close_btn:
		close_btn.pressed.connect(close)
	if dim:
		dim.gui_input.connect(_on_dim_input)
	_apply_chrome_labels()
	call_deferred("_bind_source")

func _notification(what: int) -> void:
	if what == NOTIFICATION_TRANSLATION_CHANGED:
		_apply_chrome_labels()
		if visible:
			refresh()

func _apply_chrome_labels() -> void:
	if title_label:
		title_label.text = tr("ui.journal_title")
	if close_btn:
		close_btn.text = tr("ui.close")
	if active_section:
		active_section.text = tr("ui.active_section")
	if objectives_section:
		objectives_section.text = tr("ui.objectives_section")
	if reward_section:
		reward_section.text = tr("ui.reward_section")
	if completed_section:
		completed_section.text = tr("ui.completed_section")

func _bind_source() -> void:
	_source = get_tree().get_first_node_in_group("quest_manager")
	if _source and _source.has_signal("quest_updated") and not _source.quest_updated.is_connected(_on_source_updated):
		_source.quest_updated.connect(_on_source_updated)

func _on_source_updated(_quest_id: String, _status: String, _detail: String) -> void:
	if visible:
		refresh()

func open() -> void:
	_bind_source()
	visible = true
	refresh()

func close() -> void:
	visible = false

func toggle() -> void:
	if visible:
		close()
	else:
		open()

func refresh() -> void:
	if objectives_list:
		objectives_list.clear()
	if completed_list:
		completed_list.clear()
	if _source == null or not _source.has_method("get_journal_active"):
		active_title.text = tr("ui.no_active_quest")
		active_status.text = ""
		active_journal.text = ""
		reward_text.text = ""
		return
	var active: Dictionary = _source.get_journal_active()
	if active.is_empty():
		active_title.text = tr("ui.no_active_quest")
		active_status.text = ""
		active_journal.text = ""
		reward_text.text = ""
	else:
		active_title.text = str(active.get("title", ""))
		active_status.text = _status_label(str(active.get("status", "")))
		active_journal.text = _humanize(str(active.get("journal", "")))
		for obj in active.get("objectives", []):
			var line := tr("ui.objective_fmt") % [
				_humanize(str(obj.get("text", ""))),
				int(obj.get("have", 0)),
				int(obj.get("need", 1)),
			]
			objectives_list.add_item(line)
		reward_text.text = _format_reward(active.get("reward", {}))
	if _source.has_method("get_journal_completed"):
		for item in _source.get_journal_completed():
			completed_list.add_item(str(item.get("title", "")))

func _humanize(text: String) -> String:
	if _source != null and _source.has_method("humanize"):
		return _source.humanize(text)
	return text

func _status_label(status: String) -> String:
	match status:
		"available":
			return tr("ui.status_available")
		"active":
			return tr("ui.status_active")
		"objectives_complete":
			return tr("ui.status_objectives_complete")
		"completed":
			return tr("ui.status_completed")
		_:
			return status

func _format_reward(reward: Variant) -> String:
	if typeof(reward) != TYPE_DICTIONARY:
		return ""
	var r: Dictionary = reward
	var parts: PackedStringArray = PackedStringArray()
	var xp := int(r.get("xp", 0))
	var gold := int(r.get("gold", 0))
	var rep := int(r.get("reputation", 0))
	if xp > 0:
		parts.append("+%d %s" % [xp, tr("ui.reward_xp")])
	if gold > 0:
		parts.append("+%d %s" % [gold, tr("ui.reward_gold")])
	if rep > 0:
		parts.append("+%d %s" % [rep, tr("ui.reward_rep")])
	var evidence := str(r.get("evidence", ""))
	if evidence != "":
		parts.append(evidence)
	return ", ".join(parts)

func _on_dim_input(event: InputEvent) -> void:
	if event is InputEventMouseButton and event.pressed and event.button_index == MOUSE_BUTTON_LEFT:
		close()
	elif event is InputEventScreenTouch and event.pressed:
		close()

func _unhandled_input(event: InputEvent) -> void:
	if not visible:
		return
	if event is InputEventKey and event.pressed and not event.echo and event.keycode == KEY_ESCAPE:
		close()
		get_viewport().set_input_as_handled()
