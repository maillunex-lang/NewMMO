extends Control
## Login / register / character select+create. Layout lives in the .tscn.
## Emits session_ticket_ready; when enter_online is true, starts OnlineSession (T-023).

const AuthClient = preload("res://scripts/net/auth_client.gd")
const HttpClientTransport = preload("res://scripts/net/http_client_transport.gd")
const OnlineSession = preload("res://scripts/net/online_session.gd")

signal session_ticket_ready(ticket: String, ws_url: String, character: Dictionary)
signal online_session_started(session: Node)

@onready var panel_login: Control = $SafeArea/Chrome/LoginPanel
@onready var panel_register: Control = $SafeArea/Chrome/RegisterPanel
@onready var panel_chars: Control = $SafeArea/Chrome/CharacterListPanel
@onready var panel_create: Control = $SafeArea/Chrome/CharacterCreatePanel

@onready var login_user: LineEdit = $SafeArea/Chrome/LoginPanel/VBox/LoginEdit
@onready var login_pass: LineEdit = $SafeArea/Chrome/LoginPanel/VBox/PasswordEdit
@onready var login_remember: CheckBox = $SafeArea/Chrome/LoginPanel/VBox/RememberLogin
@onready var login_btn: Button = $SafeArea/Chrome/LoginPanel/VBox/Buttons/LoginBtn
@onready var goto_register_btn: Button = $SafeArea/Chrome/LoginPanel/VBox/Buttons/GotoRegisterBtn
@onready var login_status: Label = $SafeArea/Chrome/LoginPanel/VBox/Status

@onready var reg_user: LineEdit = $SafeArea/Chrome/RegisterPanel/VBox/LoginEdit
@onready var reg_pass: LineEdit = $SafeArea/Chrome/RegisterPanel/VBox/PasswordEdit
@onready var reg_btn: Button = $SafeArea/Chrome/RegisterPanel/VBox/Buttons/RegisterBtn
@onready var goto_login_btn: Button = $SafeArea/Chrome/RegisterPanel/VBox/Buttons/GotoLoginBtn
@onready var reg_status: Label = $SafeArea/Chrome/RegisterPanel/VBox/Status

@onready var char_list: ItemList = $SafeArea/Chrome/CharacterListPanel/VBox/List
@onready var char_enter_btn: Button = $SafeArea/Chrome/CharacterListPanel/VBox/Buttons/EnterBtn
@onready var char_create_btn: Button = $SafeArea/Chrome/CharacterListPanel/VBox/Buttons/CreateBtn
@onready var char_logout_btn: Button = $SafeArea/Chrome/CharacterListPanel/VBox/Buttons/LogoutBtn
@onready var char_status: Label = $SafeArea/Chrome/CharacterListPanel/VBox/Status

@onready var create_name: LineEdit = $SafeArea/Chrome/CharacterCreatePanel/VBox/NameEdit
@onready var create_class: OptionButton = $SafeArea/Chrome/CharacterCreatePanel/VBox/ClassOption
@onready var create_submit: Button = $SafeArea/Chrome/CharacterCreatePanel/VBox/Buttons/CreateBtn
@onready var create_back: Button = $SafeArea/Chrome/CharacterCreatePanel/VBox/Buttons/BackBtn
@onready var create_status: Label = $SafeArea/Chrome/CharacterCreatePanel/VBox/Status

@onready var title_login: Label = $SafeArea/Chrome/LoginPanel/VBox/Title
@onready var title_register: Label = $SafeArea/Chrome/RegisterPanel/VBox/Title
@onready var title_chars: Label = $SafeArea/Chrome/CharacterListPanel/VBox/Title
@onready var title_create: Label = $SafeArea/Chrome/CharacterCreatePanel/VBox/Title

var auth: RefCounted = null
## Online by default (Android has no environment variables): a ticket starts OnlineSession → main scene.
## NEWMMO_ONLINE=0 keeps the old ticket-only behaviour for dev/smokes.
var enter_online := true
var _characters: Array = []
var _class_ids: PackedStringArray = PackedStringArray()
var _busy := false
var _online_session: Node = null


func _ready() -> void:
	if OS.get_environment("NEWMMO_ONLINE") == "0":
		enter_online = false
	_ensure_auth()
	_wire_buttons()
	_load_classes()
	_apply_labels()
	_show_panel(panel_login)
	var remembered: String = str(auth.load_remembered_login())
	if remembered != "":
		login_user.text = remembered
		login_remember.button_pressed = true
	login_pass.secret = true
	reg_pass.secret = true


func configure_auth(p_auth: RefCounted) -> void:
	auth = p_auth


func _ensure_auth() -> void:
	if auth != null:
		return
	# Default: real HTTPClient transport (NEWMMO_API_URL / D-004 default).
	# Smoke / screenshots inject FakeHttp via configure_auth before add_child.
	var http: RefCounted = HttpClientTransport.new()
	auth = AuthClient.new()
	auth.configure(http)


func _wire_buttons() -> void:
	login_btn.pressed.connect(_on_login)
	goto_register_btn.pressed.connect(func(): _show_panel(panel_register))
	reg_btn.pressed.connect(_on_register)
	goto_login_btn.pressed.connect(func(): _show_panel(panel_login))
	char_enter_btn.pressed.connect(_on_enter_world)
	char_create_btn.pressed.connect(func(): _show_panel(panel_create))
	char_logout_btn.pressed.connect(_on_logout)
	create_submit.pressed.connect(_on_create_character)
	create_back.pressed.connect(func(): _show_panel(panel_chars))


func _notification(what: int) -> void:
	if what == NOTIFICATION_TRANSLATION_CHANGED and is_node_ready():
		_apply_labels()


func _apply_labels() -> void:
	if title_login == null:
		return
	title_login.text = tr("ui.auth.login_title")
	title_register.text = tr("ui.auth.register_title")
	title_chars.text = tr("ui.auth.characters_title")
	title_create.text = tr("ui.auth.create_title")
	login_user.placeholder_text = tr("ui.auth.login_placeholder")
	login_pass.placeholder_text = tr("ui.auth.password_placeholder")
	reg_user.placeholder_text = tr("ui.auth.login_placeholder")
	reg_pass.placeholder_text = tr("ui.auth.password_placeholder")
	login_remember.text = tr("ui.auth.remember_login")
	login_btn.text = tr("ui.auth.login")
	goto_register_btn.text = tr("ui.auth.goto_register")
	reg_btn.text = tr("ui.auth.register")
	goto_login_btn.text = tr("ui.auth.goto_login")
	char_enter_btn.text = tr("ui.auth.enter")
	char_create_btn.text = tr("ui.auth.create_character")
	char_logout_btn.text = tr("ui.auth.logout")
	create_name.placeholder_text = tr("ui.auth.name_placeholder")
	create_submit.text = tr("ui.auth.create")
	create_back.text = tr("ui.auth.back")


func _load_classes() -> void:
	create_class.clear()
	_class_ids = PackedStringArray()
	var path: String = "res://config/classes.json"
	if not FileAccess.file_exists(path):
		create_status.text = tr("ui.auth.error_classes")
		return
	var f := FileAccess.open(path, FileAccess.READ)
	var parsed = JSON.parse_string(f.get_as_text())
	f.close()
	if typeof(parsed) != TYPE_DICTIONARY:
		return
	var arr: Array = parsed.get("classes", [])
	for item in arr:
		if typeof(item) != TYPE_DICTIONARY:
			continue
		var id: String = str(item.get("id", ""))
		var nm: String = str(item.get("name", id))
		if id == "":
			continue
		create_class.add_item(nm)
		_class_ids.append(id)


func _show_panel(panel: Control) -> void:
	panel_login.visible = panel == panel_login
	panel_register.visible = panel == panel_register
	panel_chars.visible = panel == panel_chars
	panel_create.visible = panel == panel_create
	_set_busy(false)


func _set_busy(v: bool) -> void:
	_busy = v
	login_btn.disabled = v
	reg_btn.disabled = v
	char_enter_btn.disabled = v
	char_create_btn.disabled = v
	create_submit.disabled = v


func _set_status(label: Label, text: String) -> void:
	label.text = text


func _error_text(res: Dictionary) -> String:
	var key: String = str(res.get("message_key", "error.internal"))
	var translated: String = tr(key)
	if translated == key:
		return tr("ui.auth.error_generic") + " (%s)" % str(res.get("error_code", ""))
	return translated


func _on_login() -> void:
	if _busy:
		return
	_set_busy(true)
	_set_status(login_status, tr("ui.auth.wait"))
	var user: String = login_user.text.strip_edges()
	var passw: String = login_pass.text
	var res: Dictionary = auth.login(user, passw)
	# Clear password field after attempt (never write it).
	login_pass.text = ""
	if res.get("ok") != true:
		_set_status(login_status, _error_text(res))
		_set_busy(false)
		return
	if login_remember.button_pressed:
		auth.save_remembered_login(user)
	else:
		auth.clear_remembered_login()
	_set_status(login_status, "")
	_refresh_characters()


func _on_register() -> void:
	if _busy:
		return
	_set_busy(true)
	_set_status(reg_status, tr("ui.auth.wait"))
	var user: String = reg_user.text.strip_edges()
	var passw: String = reg_pass.text
	var res: Dictionary = auth.register(user, passw)
	reg_pass.text = ""
	if res.get("ok") != true:
		_set_status(reg_status, _error_text(res))
		_set_busy(false)
		return
	# Auto-login after register using a second call would need the password again —
	# ask user to sign in (password already cleared).
	_set_status(reg_status, tr("ui.auth.register_ok"))
	reg_user.text = user
	login_user.text = user
	_show_panel(panel_login)
	_set_status(login_status, tr("ui.auth.register_ok"))


func _refresh_characters() -> void:
	_set_busy(true)
	_set_status(char_status, tr("ui.auth.wait"))
	var res: Dictionary = auth.list_characters()
	if res.get("ok") != true:
		_set_status(char_status, _error_text(res))
		_set_busy(false)
		_show_panel(panel_chars)
		return
	_characters = res.get("data", {}).get("characters", [])
	char_list.clear()
	for c in _characters:
		if typeof(c) != TYPE_DICTIONARY:
			continue
		var line: String = "%s · %s · Lv.%d" % [
			str(c.get("name", "?")),
			_class_name(str(c.get("class_id", ""))),
			int(c.get("level", 1)),
		]
		char_list.add_item(line)
	_set_status(char_status, "")
	_show_panel(panel_chars)
	_set_busy(false)


func _class_name(class_id: String) -> String:
	for i in range(_class_ids.size()):
		if _class_ids[i] == class_id:
			return create_class.get_item_text(i)
	return class_id


func _on_create_character() -> void:
	if _busy:
		return
	if create_class.selected < 0 or create_class.selected >= _class_ids.size():
		_set_status(create_status, tr("ui.auth.error_class"))
		return
	_set_busy(true)
	_set_status(create_status, tr("ui.auth.wait"))
	var nm: String = create_name.text.strip_edges()
	var cid: String = _class_ids[create_class.selected]
	var res: Dictionary = auth.create_character(nm, cid)
	if res.get("ok") != true:
		_set_status(create_status, _error_text(res))
		_set_busy(false)
		return
	create_name.text = ""
	_set_status(create_status, "")
	_refresh_characters()


func _on_enter_world() -> void:
	if _busy:
		return
	var idx := char_list.get_selected_items()
	if idx.is_empty():
		_set_status(char_status, tr("ui.auth.select_character"))
		return
	var i: int = int(idx[0])
	if i < 0 or i >= _characters.size():
		return
	var character: Dictionary = _characters[i]
	_set_busy(true)
	_set_status(char_status, tr("ui.auth.wait"))
	var res: Dictionary = auth.request_ticket(str(character.get("character_id", "")))
	if res.get("ok") != true:
		_set_status(char_status, _error_text(res))
		_set_busy(false)
		return
	var data: Dictionary = res.get("data", {})
	var ticket: String = str(data.get("ticket", ""))
	var ws_url: String = str(data.get("ws_url", ""))
	# Do not print ticket.
	session_ticket_ready.emit(ticket, ws_url, character)
	if enter_online:
		_start_online(ticket, ws_url, character)
	_set_status(char_status, tr("ui.auth.ticket_ready"))
	_set_busy(false)


func _start_online(ticket: String, ws_url: String, character: Dictionary) -> void:
	if _online_session != null and is_instance_valid(_online_session):
		_online_session.queue_free()
	_online_session = OnlineSession.new()
	_online_session.name = "OnlineSession"
	_online_session.auth = auth
	add_child(_online_session)
	var err: Error = _online_session.configure_from_ticket(ticket, ws_url, character)
	if err != OK:
		_set_status(char_status, tr("ui.auth.error_generic"))
		return
	online_session_started.emit(_online_session)
	_online_session.ready_for_gameplay.connect(_on_online_ready, CONNECT_ONE_SHOT)


func _on_online_ready(_character: Dictionary) -> void:
	if _online_session == null:
		return
	# Keep session across scene change so GameWorld can attach quest_manager.
	var tree := get_tree()
	_online_session.reparent(tree.root)
	tree.change_scene_to_file("res://scenes/world/main.tscn")


func _on_logout() -> void:
	if _busy:
		return
	_set_busy(true)
	auth.logout()
	_characters.clear()
	char_list.clear()
	_show_panel(panel_login)
	_set_status(login_status, "")
	_set_busy(false)
