@tool
extends EditorPlugin

const PALETTE := {
	"Vegetation/Tree": "res://scenes/props/tree.tscn",
	"Decorations/Rock": "res://scenes/props/rock.tscn",
	"Decorations/Crystal": "res://scenes/props/crystal.tscn",
	"Decorations/Lantern": "res://scenes/props/lantern.tscn",
	"Buildings/Cottage": "res://scenes/props/cottage.tscn",
	"Buildings/Fence": "res://scenes/props/fence.tscn",
}

var panel: VBoxContainer
var prop_choice: OptionButton
var filter_edit: LineEdit
var status: Label
var _keys: PackedStringArray = PackedStringArray()

func _enter_tree() -> void:
	panel = VBoxContainer.new()
	panel.name = "Lunex World Studio"
	var heading := Label.new()
	heading.text = "Lunex World Studio"
	panel.add_child(heading)
	filter_edit = LineEdit.new()
	filter_edit.placeholder_text = "Search props…"
	filter_edit.text_changed.connect(_rebuild_list)
	panel.add_child(filter_edit)
	prop_choice = OptionButton.new()
	panel.add_child(prop_choice)
	_rebuild_list("")
	var add_button := Button.new()
	add_button.text = "Add to selected layer"
	add_button.pressed.connect(_add_object)
	panel.add_child(add_button)
	status = Label.new()
	status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	status.text = "Open a location .tscn, select Ground/Vegetation/… layer."
	panel.add_child(status)
	add_control_to_dock(DOCK_SLOT_RIGHT_UL, panel)

func _exit_tree() -> void:
	remove_control_from_docks(panel)
	panel.queue_free()

func _rebuild_list(filter: String) -> void:
	prop_choice.clear()
	_keys = PackedStringArray()
	var q := filter.strip_edges().to_lower()
	for key in PALETTE.keys():
		if q == "" or str(key).to_lower().find(q) >= 0:
			prop_choice.add_item(str(key))
			_keys.append(str(key))

func _add_object() -> void:
	var root := get_editor_interface().get_edited_scene_root()
	var selected := get_editor_interface().get_selection().get_selected_nodes()
	if root == null or selected.is_empty() or not selected[0] is Node2D:
		status.text = "Open a location and select a Node2D layer."
		return
	var target: Node2D = selected[0]
	if target != root and not root.is_ancestor_of(target):
		status.text = "Select a node inside the edited scene."
		return
	if _keys.is_empty() or prop_choice.selected < 0:
		status.text = "No prop selected."
		return
	var key := _keys[prop_choice.selected]
	var packed: PackedScene = load(PALETTE[key])
	if packed == null:
		status.text = "Failed to load " + key
		return
	var prop := packed.instantiate()
	var undo := get_undo_redo()
	undo.create_action("Add " + key)
	undo.add_do_method(self, "_do_add", target, prop, root)
	undo.add_undo_method(self, "_do_remove", target, prop)
	undo.add_do_reference(prop)
	undo.commit_action()
	status.text = key + " added. Move with the 2D editor."

func _do_add(target: Node, prop: Node, root: Node) -> void:
	target.add_child(prop)
	prop.owner = root
	if prop is Node2D:
		(prop as Node2D).position = Vector2(40, 40)

func _do_remove(target: Node, prop: Node) -> void:
	if prop.get_parent() == target:
		target.remove_child(prop)
