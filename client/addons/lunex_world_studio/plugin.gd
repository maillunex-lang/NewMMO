@tool
extends EditorPlugin

const PALETTE := {
    "Tree": "res://scenes/props/tree.tscn",
    "Rock": "res://scenes/props/rock.tscn",
    "Crystal": "res://scenes/props/crystal.tscn",
}
var panel: VBoxContainer
var prop_choice: OptionButton
var status: Label

func _enter_tree() -> void:
    panel = VBoxContainer.new()
    panel.name = "Lunex World Studio"
    var heading := Label.new()
    heading.text = "Object library"
    panel.add_child(heading)
    prop_choice = OptionButton.new()
    for key in PALETTE:
        prop_choice.add_item(key)
    panel.add_child(prop_choice)
    var add_button := Button.new()
    add_button.text = "Add object to selected layer"
    add_button.pressed.connect(_add_object)
    panel.add_child(add_button)
    status = Label.new()
    status.text = "Select a layer in a location scene."
    panel.add_child(status)
    add_control_to_dock(DOCK_SLOT_RIGHT_UL, panel)

func _exit_tree() -> void:
    remove_control_from_docks(panel)
    panel.queue_free()

func _add_object() -> void:
    var root := get_editor_interface().get_edited_scene_root()
    var parent := get_editor_interface().get_selection().get_selected_nodes()
    if root == null or parent.is_empty() or not parent[0] is Node2D:
        status.text = "Open a location and select a Node2D layer."
        return
    var target: Node2D = parent[0]
    if target != root and not root.is_ancestor_of(target):
        status.text = "Select a node inside the edited scene."
        return
    var key := prop_choice.get_item_text(prop_choice.selected)
    var packed: PackedScene = load(PALETTE[key])
    var prop := packed.instantiate()
    var undo := get_undo_redo()
    undo.create_action("Add " + key)
    undo.add_do_method(target, "add_child", prop)
    undo.add_do_method(prop, "set_owner", root)
    undo.add_undo_method(target, "remove_child", prop)
    undo.add_do_reference(prop)
    undo.commit_action()
    status.text = key + " added. Move it with the 2D editor."
