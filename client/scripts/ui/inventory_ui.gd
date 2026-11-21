extends Control
## Inventory panel (EX-P08 / T-030). Layout in inventory.tscn; binds server snapshot or offline player bag.

@onready var title_label: Label = $SafeArea/Chrome/Panel/Margin/VBox/Header/Title
@onready var close_btn: Button = $SafeArea/Chrome/Panel/Margin/VBox/Header/Close
@onready var materials_section: Label = $SafeArea/Chrome/Panel/Margin/VBox/MaterialsSection
@onready var materials_list: ItemList = $SafeArea/Chrome/Panel/Margin/VBox/MaterialsList
@onready var bag_section: Label = $SafeArea/Chrome/Panel/Margin/VBox/BagSection
@onready var bag_list: ItemList = $SafeArea/Chrome/Panel/Margin/VBox/BagList
@onready var quest_keys_section: Label = $SafeArea/Chrome/Panel/Margin/VBox/QuestKeysSection
@onready var quest_keys_list: ItemList = $SafeArea/Chrome/Panel/Margin/VBox/QuestKeysList
@onready var dim: ColorRect = $Dim
@onready var equip_title: Label = $SafeArea/Chrome/EquipPanel/Margin/VBox/Title
@onready var gear_score_label: Label = $SafeArea/Chrome/EquipPanel/Margin/VBox/GearScore
@onready var equipped_list: ItemList = $SafeArea/Chrome/EquipPanel/Margin/VBox/EquippedList
@onready var detail_label: Label = $SafeArea/Chrome/EquipPanel/Margin/VBox/Detail
@onready var action_btn: Button = $SafeArea/Chrome/EquipPanel/Margin/VBox/Action

const SLOT_ORDER := ["MH", "OH", "HD", "CH", "HA", "LG", "FT", "RG", "AM"]

var _net: RefCounted = null
var _bag: Array = []
var _materials: Array = []
var _quest_keys: Array = []
var _revision: int = 0
## T-039c: server-rolled gear. instance_id -> {template_id, rarity, item_level, required_level, stats}
var _items: Dictionary = {}
var _equipped: Dictionary = {}  # slot -> instance_id
var _gear_score: int = 0
var _bag_index: Array = []  # bag ItemList row -> stack dictionary
var _sel_instance := ""     # selected bag gear
var _sel_slot := ""         # selected worn slot

func _ready() -> void:
	visible = false
	mouse_filter = Control.MOUSE_FILTER_STOP
	add_to_group("inventory_ui")
	if close_btn:
		close_btn.pressed.connect(close)
	if dim:
		dim.gui_input.connect(_on_dim_input)
	if bag_list:
		bag_list.item_selected.connect(_on_bag_selected)
	if equipped_list:
		equipped_list.item_selected.connect(_on_worn_selected)
	if action_btn:
		action_btn.pressed.connect(_on_action)
	_apply_chrome_labels()

func _notification(what: int) -> void:
	if what == NOTIFICATION_TRANSLATION_CHANGED:
		_apply_chrome_labels()
		if visible:
			refresh()

func _apply_chrome_labels() -> void:
	if title_label:
		title_label.text = tr("ui.inventory_title")
	if close_btn:
		close_btn.text = tr("ui.close")
	if materials_section:
		materials_section.text = tr("ui.inventory_materials")
	if bag_section:
		bag_section.text = tr("ui.inventory_bag")
	if quest_keys_section:
		quest_keys_section.text = tr("ui.inventory_quest_keys")
	if equip_title:
		equip_title.text = tr("ui.equipment_title")

func bind_net(net_client: RefCounted) -> void:
	_net = net_client
	if _net == null:
		return
	if _net.has_signal("state_snapshot") and not _net.state_snapshot.is_connected(_on_snapshot):
		_net.state_snapshot.connect(_on_snapshot)
	if _net.has_signal("inventory_delta") and not _net.inventory_delta.is_connected(_on_delta):
		_net.inventory_delta.connect(_on_delta)
	if _net.has_signal("command_error") and not _net.command_error.is_connected(_on_command_error):
		_net.command_error.connect(_on_command_error)

func _on_snapshot(payload: Dictionary, _revision: int) -> void:
	var scopes: Dictionary = payload.get("scopes", {})
	if typeof(scopes) != TYPE_DICTIONARY:
		return
	var inv: Dictionary = scopes.get("inventory", {})
	if typeof(inv) != TYPE_DICTIONARY:
		return
	_apply_scope(inv)
	if visible:
		refresh()

func _on_delta(payload: Dictionary) -> void:
	_revision = int(payload.get("inventory_revision", _revision))
	for ch in payload.get("changes", []):
		if typeof(ch) != TYPE_DICTIONARY:
			continue
		_upsert_stack(ch)
	var items: Variant = payload.get("items", {})
	if typeof(items) == TYPE_DICTIONARY:
		for id in (items as Dictionary).keys():
			_items[str(id)] = items[id]
	for w in payload.get("equipped", []):
		if typeof(w) == TYPE_DICTIONARY:
			var slot := str(w.get("slot", ""))
			var inst := str(w.get("instance_id", ""))
			if inst == "":
				_equipped.erase(slot)
			else:
				_equipped[slot] = inst
	if payload.has("gear_score"):
		_gear_score = int(payload.get("gear_score", 0))
	_sel_instance = ""
	_sel_slot = ""
	if visible:
		refresh()

func _apply_scope(inv: Dictionary) -> void:
	_revision = int(inv.get("inventory_revision", 0))
	_bag = inv.get("bag", [])
	_materials = inv.get("materials", [])
	_quest_keys = inv.get("quest_keys", [])
	var items: Variant = inv.get("items", {})
	_items = (items as Dictionary).duplicate(true) if typeof(items) == TYPE_DICTIONARY else {}
	_equipped = {}
	for w in inv.get("equipped", []):
		if typeof(w) == TYPE_DICTIONARY and str(w.get("instance_id", "")) != "":
			_equipped[str(w.get("slot", ""))] = str(w.get("instance_id"))
	_gear_score = int(inv.get("gear_score", 0))
	_sel_instance = ""
	_sel_slot = ""
	if typeof(_bag) != TYPE_ARRAY:
		_bag = []
	if typeof(_materials) != TYPE_ARRAY:
		_materials = []
	if typeof(_quest_keys) != TYPE_ARRAY:
		_quest_keys = []

func _upsert_stack(ch: Dictionary) -> void:
	var container := str(ch.get("container", ""))
	var slot := int(ch.get("slot", -1))
	var qty := int(ch.get("quantity", 0))
	var item_id := str(ch.get("item_id", ""))
	var inst := str(ch.get("instance_id", ""))
	var target: Array = _bag if container == "bag" else _materials
	if container == "materials":
		target = _materials
	elif container == "bag":
		target = _bag
	else:
		return
	for i in range(target.size()):
		var row = target[i]
		if typeof(row) == TYPE_DICTIONARY and int(row.get("slot", -2)) == slot:
			if qty <= 0:
				target.remove_at(i)
			else:
				row["quantity"] = qty
				row["item_id"] = item_id
				if inst != "":
					row["instance_id"] = inst
				else:
					row.erase("instance_id")
				target[i] = row
			return
	if qty > 0:
		var fresh := {"container": container, "slot": slot, "item_id": item_id, "quantity": qty}
		if inst != "":
			fresh["instance_id"] = inst
		target.append(fresh)

func open() -> void:
	_pull_offline_if_needed()
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
	_fill_list(materials_list, _materials)
	_fill_bag()
	_fill_equipped()
	if quest_keys_list:
		quest_keys_list.clear()
		if _quest_keys.is_empty():
			quest_keys_list.add_item(tr("ui.inventory_empty"))
		else:
			for k in _quest_keys:
				quest_keys_list.add_item(str(k))

func _gear_name(inst: String) -> String:
	var d: Dictionary = _items.get(inst, {})
	if d.is_empty():
		return inst.left(8)
	var tpl := str(d.get("template_id", ""))
	var slot := tpl.get_slice("-", 3)
	return "%s · %s · %d" % [tr("ui.slot_" + slot), tr("ui.rarity_" + str(d.get("rarity", "COMMON"))), int(d.get("item_level", 0))]

func _rarity_color(inst: String) -> Color:
	match str((_items.get(inst, {}) as Dictionary).get("rarity", "COMMON")):
		"UNCOMMON": return Color(0.45, 0.9, 0.5)
		"RARE": return Color(0.45, 0.7, 1.0)
		"EPIC": return Color(0.75, 0.5, 1.0)
		"LEGENDARY": return Color(1.0, 0.8, 0.3)
		"MYTHIC": return Color(1.0, 0.55, 0.85)
	return Color(0.85, 0.85, 0.85)

func _fill_bag() -> void:
	if bag_list == null:
		return
	bag_list.clear()
	_bag_index.clear()
	var any := false
	for s in _bag:
		if typeof(s) != TYPE_DICTIONARY or int(s.get("quantity", 0)) <= 0:
			continue
		any = true
		var inst := str(s.get("instance_id", ""))
		if inst != "":
			var i := bag_list.add_item(_gear_name(inst))
			bag_list.set_item_custom_fg_color(i, _rarity_color(inst))
		else:
			bag_list.add_item("%s ×%d" % [str(s.get("item_id", "")), int(s.get("quantity", 0))])
		_bag_index.append(s)
	if not any:
		bag_list.add_item(tr("ui.inventory_empty"))

func _fill_equipped() -> void:
	if equipped_list == null:
		return
	equipped_list.clear()
	for slot in SLOT_ORDER:
		var inst := str(_equipped.get(slot, ""))
		var label := tr("ui.slot_" + slot)
		var i := equipped_list.add_item("%s: %s" % [label, _gear_name(inst) if inst != "" else "—"])
		if inst != "":
			equipped_list.set_item_custom_fg_color(i, _rarity_color(inst))
	if gear_score_label:
		gear_score_label.text = tr("ui.gear_score") % _gear_score
	_update_detail()

func _detail_text(inst: String) -> String:
	var d: Dictionary = _items.get(inst, {})
	if d.is_empty():
		return ""
	var lines: Array[String] = [tr("ui.item_level_req") % [int(d.get("item_level", 0)), int(d.get("required_level", 1))]]
	var st: Dictionary = d.get("stats", {})
	var keys := st.keys()
	keys.sort()
	var parts: Array[String] = []
	for k in keys:
		parts.append("%s +%d" % [str(k), int(st[k])])
	lines.append(" · ".join(parts))
	return "\n".join(lines)

func _update_detail() -> void:
	var inst := _sel_instance if _sel_instance != "" else str(_equipped.get(_sel_slot, ""))
	if detail_label:
		detail_label.text = _detail_text(inst) if inst != "" else tr("ui.item_pick_hint")
	if action_btn:
		action_btn.disabled = _net == null or inst == ""
		action_btn.text = tr("ui.equip") if _sel_instance != "" else tr("ui.unequip")

func _on_bag_selected(index: int) -> void:
	_sel_instance = ""
	_sel_slot = ""
	if index >= 0 and index < _bag_index.size():
		_sel_instance = str((_bag_index[index] as Dictionary).get("instance_id", ""))
	if equipped_list:
		equipped_list.deselect_all()
	_update_detail()

func _on_worn_selected(index: int) -> void:
	_sel_instance = ""
	_sel_slot = SLOT_ORDER[index] if index >= 0 and index < SLOT_ORDER.size() else ""
	if bag_list:
		bag_list.deselect_all()
	if not _equipped.has(_sel_slot):
		_sel_slot = ""
	_update_detail()

func _on_action() -> void:
	if _net == null:
		return
	if _sel_instance != "":
		_net.request_equip(_sel_instance)
	elif _sel_slot != "":
		_net.request_unequip(_sel_slot)

func _on_command_error(payload: Dictionary) -> void:
	var key := ""
	match str(payload.get("code", "")):
		"WRONG_CLASS": key = "ui.err_wrong_class"
		"LEVEL_TOO_LOW": key = "ui.err_level_low"
		"STATE_CONFLICT": key = "ui.err_bag_full"
	if key != "" and visible and detail_label:
		detail_label.text = tr(key)

func _fill_list(list: ItemList, stacks: Array) -> void:
	if list == null:
		return
	list.clear()
	if stacks.is_empty():
		list.add_item(tr("ui.inventory_empty"))
		return
	for s in stacks:
		if typeof(s) != TYPE_DICTIONARY:
			continue
		var qty := int(s.get("quantity", 0))
		if qty <= 0:
			continue
		list.add_item("%s ×%d" % [str(s.get("item_id", "")), qty])

func _pull_offline_if_needed() -> void:
	if _net != null:
		return
	var player := get_tree().get_first_node_in_group("player")
	if player == null:
		return
	var inv_var: Variant = player.get("inventory")
	if typeof(inv_var) != TYPE_DICTIONARY:
		return
	_bag.clear()
	_materials.clear()
	var slot := 0
	var inv: Dictionary = inv_var
	for item_id in inv.keys():
		var qty := int(inv[item_id])
		if qty <= 0:
			continue
		var row := {"container": "bag", "slot": slot, "item_id": str(item_id), "quantity": qty}
		if str(item_id).begins_with("MAT-"):
			_materials.append(row)
		else:
			_bag.append(row)
		slot += 1

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
