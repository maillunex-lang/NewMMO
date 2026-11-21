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

var _net: RefCounted = null
var _bag: Array = []
var _materials: Array = []
var _quest_keys: Array = []
var _revision: int = 0

func _ready() -> void:
	visible = false
	mouse_filter = Control.MOUSE_FILTER_STOP
	add_to_group("inventory_ui")
	if close_btn:
		close_btn.pressed.connect(close)
	if dim:
		dim.gui_input.connect(_on_dim_input)
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

func bind_net(net_client: RefCounted) -> void:
	_net = net_client
	if _net == null:
		return
	if _net.has_signal("state_snapshot") and not _net.state_snapshot.is_connected(_on_snapshot):
		_net.state_snapshot.connect(_on_snapshot)
	if _net.has_signal("inventory_delta") and not _net.inventory_delta.is_connected(_on_delta):
		_net.inventory_delta.connect(_on_delta)

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
	if visible:
		refresh()

func _apply_scope(inv: Dictionary) -> void:
	_revision = int(inv.get("inventory_revision", 0))
	_bag = inv.get("bag", [])
	_materials = inv.get("materials", [])
	_quest_keys = inv.get("quest_keys", [])
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
				target[i] = row
			return
	if qty > 0:
		target.append({"container": container, "slot": slot, "item_id": item_id, "quantity": qty})

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
	_fill_list(bag_list, _bag)
	if quest_keys_list:
		quest_keys_list.clear()
		if _quest_keys.is_empty():
			quest_keys_list.add_item(tr("ui.inventory_empty"))
		else:
			for k in _quest_keys:
				quest_keys_list.add_item(str(k))

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
