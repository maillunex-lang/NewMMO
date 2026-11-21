extends Node
## Stage-1 quest runtime for MQ-01-01…05.
## Offline: local stub (mq01_smoke). Online (T-023): server-authoritative via NetClient.

signal quest_updated(quest_id: String, status: String, detail: String)
signal quest_completed(quest_id: String, reward_text: String)
signal dialogue_flow_requested(npc_id: String, flow_ids: PackedStringArray, mode: String)
signal dialogue_server_node(npc_id: String, node: Dictionary)

const QUEST_PATH := "res://data/quests/mq_01_01_to_05.json"
const DIALOGUE_PATH := "res://data/dialogues/mq_01_01_to_05.json"
const INTERACTABLES_PATH := "res://config/interactables_reg_01.json"

var _quests: Array = []
var _dialogues: Dictionary = {}
var _flows: Dictionary = {}
var _flags: Dictionary = {"TUTORIAL-COMPLETE": true}
var _active_id := ""
var _status := "locked"  # locked|available|active|objectives_complete|completed
var _progress: Dictionary = {}  # objective_id -> count
var _xp := 0
var _rep_fac08 := 0
var _evidence: Array = []
var _online := false
var _net: RefCounted = null
var _content_build_id := ""
var _instances_by_target: Dictionary = {}  # target_id -> PackedStringArray of instance ids
var _used_instances: Dictionary = {}  # objective_id -> {instance_id: true}
var _dialogue_npc_id := ""
var _dialogue_node_id := ""
var _dialogue_id := ""

func _ready() -> void:
	add_to_group("quest_manager")
	_load_data()
	_load_interactables()
	_bootstrap_first_available()
	_emit()

## Attach NetClient; mutations go to the server. Dialogue open/choice when online (T-025).
func enable_online(net_client: RefCounted, content_build_id: String = "") -> void:
	_online = true
	_net = net_client
	_content_build_id = content_build_id
	if _net != null:
		if not _net.quest_accept_result.is_connected(_on_net_accept):
			_net.quest_accept_result.connect(_on_net_accept)
		if not _net.quest_progress_delta.is_connected(_on_net_progress):
			_net.quest_progress_delta.connect(_on_net_progress)
		if not _net.quest_reward_granted.is_connected(_on_net_reward):
			_net.quest_reward_granted.connect(_on_net_reward)
		if not _net.state_snapshot.is_connected(_on_net_snapshot):
			_net.state_snapshot.connect(_on_net_snapshot)
		if _net.has_signal("loot_result") and not _net.loot_result.is_connected(_on_net_loot):
			_net.loot_result.connect(_on_net_loot)
		if not _net.dialogue_node_state.is_connected(_on_net_dialogue_node):
			_net.dialogue_node_state.connect(_on_net_dialogue_node)
		if _content_build_id == "" and str(_net.content_build_id) != "":
			_content_build_id = str(_net.content_build_id)

func is_online() -> bool:
	return _online

## Online: ask the server to roll the drop for a canon enemy (T-034, server-authoritative).
## Returns true when the request was sent; the caller then skips the offline local roll.
func request_loot(canon_enemy_id: String) -> bool:
	if not _online or _net == null or canon_enemy_id == "":
		return false
	_net.request_loot(canon_enemy_id)
	return true

func _on_net_loot(payload: Dictionary) -> void:
	var player := get_tree().get_first_node_in_group("player")
	if player == null or not player.has_method("apply_server_loot"):
		return
	player.apply_server_loot(int(payload.get("gold", 0)), int(payload.get("gold_total", 0)), payload.get("items", []))

func _load_data() -> void:
	var qf := FileAccess.open(QUEST_PATH, FileAccess.READ)
	var df := FileAccess.open(DIALOGUE_PATH, FileAccess.READ)
	if qf == null or df == null:
		push_error("Missing quest/dialogue JSON")
		return
	var qdata: Dictionary = JSON.parse_string(qf.get_as_text())
	var ddata: Dictionary = JSON.parse_string(df.get_as_text())
	_quests = qdata.get("quests", [])
	_dialogues = ddata.get("lines", {})
	_flows = ddata.get("flows", {})

func _load_interactables() -> void:
	_instances_by_target.clear()
	var path := INTERACTABLES_PATH
	if not FileAccess.file_exists(path):
		path = "res://data/interactables_reg_01.json"
	if not FileAccess.file_exists(path):
		return
	var f := FileAccess.open(path, FileAccess.READ)
	if f == null:
		return
	var parsed = JSON.parse_string(f.get_as_text())
	f.close()
	if typeof(parsed) != TYPE_DICTIONARY:
		return
	for row in parsed.get("interactables", []):
		if typeof(row) != TYPE_DICTIONARY:
			continue
		var target := str(row.get("target_id", ""))
		var count := int(row.get("count", 1))
		var instances = row.get("instances", null)
		if count > 1 and typeof(instances) == TYPE_ARRAY:
			var packed := PackedStringArray()
			for inst in instances:
				packed.append(str(inst))
			_instances_by_target[target] = packed

func _next_instance_for(target_id: String) -> String:
	if not _instances_by_target.has(target_id):
		return ""
	var list: PackedStringArray = _instances_by_target[target_id]
	var used: Dictionary = _used_instances.get(target_id, {})
	for inst in list:
		if not used.has(inst):
			return inst
	return ""

func _mark_instance_used(target_id: String, instance_id: String) -> void:
	if instance_id == "":
		return
	if not _used_instances.has(target_id):
		_used_instances[target_id] = {}
	_used_instances[target_id][instance_id] = true

func get_line(dia_id: String) -> Dictionary:
	return _dialogues.get(dia_id, {"speaker": "SYSTEM", "text": dia_id})

func get_tracker_text() -> String:
	var q := _quest_by_id(_active_id)
	if q.is_empty():
		return tr("ui.no_active_quest")
	var lines: PackedStringArray = PackedStringArray()
	lines.append("%s · %s" % [q.get("id", ""), q.get("title", "")])
	match _status:
		"available":
			lines.append("Поговорить с %s" % str(q.get("contact", "")))
		"active", "objectives_complete":
			lines.append(str(q.get("journal", "")))
			for obj in q.get("objectives", []):
				var oid: String = str(obj.get("id", ""))
				var need: int = int(obj.get("count", 1))
				var have: int = int(_progress.get(oid, 0))
				var mark := "✓" if have >= need else "·"
				lines.append("%s %s (%d/%d)" % [mark, obj.get("text", oid), mini(have, need), need])
			if _status == "objectives_complete":
				lines.append("Сдать задание: %s" % str(q.get("contact", "")))
		"completed":
			lines.append("Завершено")
		_:
			lines.append(_status)
	return "\n".join(lines)

func get_active_quest_id() -> String:
	return _active_id

func get_status() -> String:
	return _status

## Read-only journal adapter for UI (EX-P06). Same method names can later wrap a server source.
## Does not mutate quest state.
func get_journal_active() -> Dictionary:
	var q := _quest_by_id(_active_id)
	if q.is_empty():
		return {}
	var objectives: Array = []
	for obj in q.get("objectives", []):
		var oid: String = str(obj.get("id", ""))
		var need: int = int(obj.get("count", 1))
		var have: int = int(_progress.get(oid, 0))
		objectives.append({
			"id": oid,
			"text": str(obj.get("text", oid)),
			"have": mini(have, need),
			"need": need,
		})
	var reward: Dictionary = q.get("reward", {})
	return {
		"id": str(q.get("id", "")),
		"title": str(q.get("title", "")),
		"journal": str(q.get("journal", "")),
		"status": _status,
		"contact": str(q.get("contact", "")),
		"objectives": objectives,
		"reward": {
			"xp": int(reward.get("xp", 0)),
			"gold": int(reward.get("gold", 0)),
			"reputation": int(reward.get("reputation", 0)),
			"evidence": str(reward.get("evidence", "")),
		},
	}

func get_journal_completed() -> Array:
	var out: Array = []
	var active_idx := _index_of(_active_id)
	for i in range(_quests.size()):
		var q: Dictionary = _quests[i]
		var qid := str(q.get("id", ""))
		var done := false
		if active_idx < 0:
			# Chain finished or empty active — treat every loaded MQ as completed when status says so.
			done = (_status == "completed")
		else:
			done = i < active_idx
		if done:
			out.append({"id": qid, "title": str(q.get("title", ""))})
	return out

func npc_display_name(npc_id: String) -> String:
	match npc_id:
		"NPC-802":
			return "Фейра Нисс"
		"NPC-102":
			return "Ильра Венн"
		_:
			return npc_id

func on_npc_talk(npc_id: String, player: Node) -> void:
	var q := _quest_by_id(_active_id)
	if q.is_empty():
		return
	var contact := str(q.get("contact", ""))
	if _online and _net != null and (npc_id == contact or _status == "active"):
		if _status == "active":
			_try_event("npc_interact", npc_id, player)
		_dialogue_npc_id = npc_id
		_dialogue_id = _active_id
		_net.request_dialogue_open(npc_id, _active_id)
		return
	# Objective npc_interact can target contact or other interactables routed as npc.
	if _status == "available" and npc_id == contact:
		_open_flow(npc_id, "start")
		return
	if _status == "objectives_complete" and npc_id == contact:
		_open_flow(npc_id, "turnin")
		return
	if _status == "active":
		# Completing npc_interact objectives
		_try_event("npc_interact", npc_id, player)
		if npc_id == contact:
			_open_flow(npc_id, "progress")
		return
	if _status == "completed" and npc_id == contact:
		_open_flow(npc_id, "progress")

func on_dialogue_action(action: String, npc_id: String, player: Node) -> void:
	if _online and _net != null and _dialogue_node_id != "":
		_net.request_dialogue_choice(
			_dialogue_node_id, action,
			_dialogue_id if _dialogue_id != "" else _active_id,
			npc_id if npc_id != "" else _dialogue_npc_id
		)
	match action:
		"ACCEPT":
			_accept(player)
		"TURN_IN":
			_complete(player)
		"ASK", "LATER", "CONTINUE":
			pass
		_:
			pass
	if npc_id != "":
		# After accept, first talk objective may complete
		if action == "ACCEPT":
			_try_event("npc_interact", npc_id, player)

func _on_net_dialogue_node(payload: Dictionary) -> void:
	_dialogue_node_id = str(payload.get("node_id", ""))
	_dialogue_id = str(payload.get("dialogue_id", _dialogue_id))
	var npc := _dialogue_npc_id
	if npc == "":
		var q := _quest_by_id(_active_id)
		npc = str(q.get("contact", ""))
	dialogue_server_node.emit(npc, payload)

func report_world_event(event: String, target_id: String, player: Node = null) -> void:
	report_world_event_ex(event, target_id, "", player)

func report_world_event_ex(event: String, target_id: String, instance_id: String, player: Node = null) -> void:
	if _online and _net != null and _status == "active":
		var iid := instance_id
		if iid == "":
			iid = _next_instance_for(target_id)
		_net.request_world_interact(event, target_id, iid)
		if iid != "":
			_mark_instance_used(target_id, iid)
		return
	_try_event(event, target_id, player)

func _open_flow(npc_id: String, mode: String) -> void:
	var flow: Dictionary = _flows.get(_active_id, {})
	var ids: Array = []
	var raw: Variant = flow.get(mode, [])
	if typeof(raw) == TYPE_DICTIONARY:
		# State-dependent variants, keyed by the number of fully completed objectives ("0", "1", "2"…).
		var key := str(_completed_objective_count())
		var variants: Dictionary = raw
		if variants.has(key):
			ids = variants[key]
		elif not variants.is_empty():
			ids = variants[variants.keys()[0]]
	else:
		ids = raw
	var packed := PackedStringArray()
	for id in ids:
		packed.append(str(id))
	if packed.is_empty():
		return
	dialogue_flow_requested.emit(npc_id, packed, mode)

func _completed_objective_count() -> int:
	var q := _quest_by_id(_active_id)
	var done := 0
	for obj in q.get("objectives", []):
		var oid: String = str(obj.get("id", ""))
		if int(_progress.get(oid, 0)) >= int(obj.get("count", 1)):
			done += 1
	return done

func _accept(_player: Node) -> void:
	if _status != "available":
		return
	if _online and _net != null:
		_net.request_quest_accept(_active_id, _content_build_id)
		return
	_status = "active"
	_progress.clear()
	_used_instances.clear()
	_flags["QF-%s-START" % _active_id] = true
	_emit()

func _complete(player: Node) -> void:
	if _status != "objectives_complete":
		return
	if _online and _net != null:
		_net.request_quest_turnin(_active_id)
		return
	_apply_local_reward(player)

func _apply_local_reward(player: Node) -> void:
	var q := _quest_by_id(_active_id)
	if q.is_empty():
		return
	var reward: Dictionary = q.get("reward", {})
	var xp: int = int(reward.get("xp", 0))
	var gold: int = int(reward.get("gold", 0))
	var rep: int = int(reward.get("reputation", 0))
	_xp += xp
	_rep_fac08 += rep
	if player and player.has_method("add_gold"):
		player.add_gold(gold)
	if reward.has("evidence"):
		_evidence.append(str(reward["evidence"]))
		if player and player.has_method("add_loot"):
			player.add_loot(str(reward["evidence"]), 1)
	for f in reward.get("flags", []):
		_flags[str(f)] = true
	_status = "completed"
	var reward_text := "+%d XP, +%d золота, +%d FAC-08" % [xp, gold, rep]
	if reward.has("evidence"):
		reward_text += ", %s" % str(reward["evidence"])
	quest_completed.emit(_active_id, reward_text)
	_emit()
	_advance_to_next()

func _on_net_accept(payload: Dictionary, _request_id: String) -> void:
	if str(payload.get("quest_id", "")) != _active_id:
		return
	if str(payload.get("state", "")) == "ACTIVE" or payload.get("ok") == true:
		_status = "active"
		_progress.clear()
		_used_instances.clear()
		_flags["QF-%s-START" % _active_id] = true
		_emit()

func _on_net_progress(payload: Dictionary) -> void:
	if str(payload.get("quest_id", "")) != _active_id:
		return
	var oid := str(payload.get("objective_id", ""))
	var have := int(payload.get("count", 0))
	if oid != "":
		_progress[oid] = have
	if payload.get("ready") == true:
		_status = "objectives_complete"
	_emit()

func _on_net_reward(payload: Dictionary) -> void:
	var qid := str(payload.get("quest_id", ""))
	if qid != "" and qid != _active_id:
		return
	_xp = int(payload.get("xp_total", _xp))
	var gold := int(payload.get("gold_total", 0))
	var rep_map: Dictionary = payload.get("reputation", {})
	if typeof(rep_map) == TYPE_DICTIONARY and rep_map.has("FAC-08"):
		_rep_fac08 = int(rep_map.get("FAC-08"))
	for item in payload.get("evidence", []):
		_evidence.append(str(item))
	_status = "completed"
	var reward_text := "+XP→%d, gold→%d" % [_xp, gold]
	quest_completed.emit(_active_id, reward_text)
	_emit()
	_advance_to_next()

func _on_net_snapshot(payload: Dictionary, _revision: int) -> void:
	var scopes: Dictionary = payload.get("scopes", {})
	if typeof(scopes) != TYPE_DICTIONARY:
		return
	var quests: Dictionary = scopes.get("quests", {})
	if typeof(quests) != TYPE_DICTIONARY:
		return
	var completed: Array = quests.get("completed", [])
	var active: Array = quests.get("active", [])
	var available: Array = quests.get("available", [])
	if not active.is_empty() and typeof(active[0]) == TYPE_DICTIONARY:
		var a: Dictionary = active[0]
		_active_id = str(a.get("quest_id", _active_id))
		var st := str(a.get("state", "ACTIVE"))
		_status = "objectives_complete" if st == "READY_TO_TURN_IN" else "active"
		_progress.clear()
		for obj in a.get("objectives", []):
			if typeof(obj) != TYPE_DICTIONARY:
				continue
			_progress[str(obj.get("objective_id", ""))] = int(obj.get("have", 0))
		_emit()
		return
	if not available.is_empty():
		_active_id = str(available[0])
		_status = "available"
		_progress.clear()
		_emit()
		return
	if not completed.is_empty() and _active_id != "" and completed.has(_active_id):
		_status = "completed"
		_emit()

func _advance_to_next() -> void:
	var idx := _index_of(_active_id)
	if idx < 0 or idx + 1 >= _quests.size():
		_active_id = ""
		_status = "completed"
		_emit()
		return
	var nxt: Dictionary = _quests[idx + 1]
	if _requirements_met(nxt):
		_active_id = str(nxt.get("id", ""))
		_status = "available"
		_progress.clear()
		_emit()

func _try_event(event: String, target_id: String, _player: Node) -> void:
	if _status != "active":
		return
	var q := _quest_by_id(_active_id)
	if q.is_empty():
		return
	for obj in q.get("objectives", []):
		if str(obj.get("event", "")) != event:
			continue
		if str(obj.get("target", "")) != target_id:
			continue
		var oid: String = str(obj.get("id", ""))
		var need: int = int(obj.get("count", 1))
		var have: int = int(_progress.get(oid, 0))
		if have >= need:
			continue
		_progress[oid] = have + 1
		_emit()
		_check_objectives_complete()
		return

func _check_objectives_complete() -> void:
	var q := _quest_by_id(_active_id)
	if q.is_empty():
		return
	for obj in q.get("objectives", []):
		var oid: String = str(obj.get("id", ""))
		var need: int = int(obj.get("count", 1))
		if int(_progress.get(oid, 0)) < need:
			return
	_status = "objectives_complete"
	_emit()

func _bootstrap_first_available() -> void:
	for q in _quests:
		if _requirements_met(q) and not _flags.get("QF-%s-DONE" % str(q.get("id", "")), false):
			_active_id = str(q.get("id", ""))
			_status = "available"
			return

func _requirements_met(q: Dictionary) -> bool:
	for req in q.get("requires", []):
		if not _flags.get(str(req), false):
			return false
	return true

func _quest_by_id(qid: String) -> Dictionary:
	for q in _quests:
		if str(q.get("id", "")) == qid:
			return q
	return {}

func _index_of(qid: String) -> int:
	for i in range(_quests.size()):
		if str(_quests[i].get("id", "")) == qid:
			return i
	return -1

func _emit() -> void:
	quest_updated.emit(_active_id, _status, get_tracker_text())

func debug_snapshot() -> Dictionary:
	return {
		"active": _active_id,
		"status": _status,
		"progress": _progress.duplicate(),
		"xp": _xp,
		"rep_fac08": _rep_fac08,
		"evidence": _evidence.duplicate(),
		"dialogue_node_id": _dialogue_node_id,
	}
