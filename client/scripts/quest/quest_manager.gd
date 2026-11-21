extends Node
## Local Stage-1 quest runtime for MQ-01-01…05.
## Mirrors canon event names; NOT authoritative (Stage 2 will move this to Go/Postgres).

signal quest_updated(quest_id: String, status: String, detail: String)
signal quest_completed(quest_id: String, reward_text: String)
signal dialogue_flow_requested(npc_id: String, flow_ids: PackedStringArray, mode: String)

const QUEST_PATH := "res://data/quests/mq_01_01_to_05.json"
const DIALOGUE_PATH := "res://data/dialogues/mq_01_01_to_05.json"

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

func _ready() -> void:
	add_to_group("quest_manager")
	_load_data()
	_bootstrap_first_available()
	_emit()

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

func get_line(dia_id: String) -> Dictionary:
	return _dialogues.get(dia_id, {"speaker": "SYSTEM", "text": dia_id})

func get_tracker_text() -> String:
	var q := _quest_by_id(_active_id)
	if q.is_empty():
		return "Нет активного основного задания"
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
	match action:
		"ACCEPT":
			_accept(player)
		"TURN_IN":
			_complete(player)
		"ASK":
			pass
		_:
			pass
	if npc_id != "":
		# After accept, first talk objective may complete
		if action == "ACCEPT":
			_try_event("npc_interact", npc_id, player)

func report_world_event(event: String, target_id: String, player: Node = null) -> void:
	_try_event(event, target_id, player)

func _open_flow(npc_id: String, mode: String) -> void:
	var flow: Dictionary = _flows.get(_active_id, {})
	var ids: Array = flow.get(mode, [])
	var packed := PackedStringArray()
	for id in ids:
		packed.append(str(id))
	if packed.is_empty():
		return
	dialogue_flow_requested.emit(npc_id, packed, mode)

func _accept(_player: Node) -> void:
	if _status != "available":
		return
	_status = "active"
	_progress.clear()
	_flags["QF-%s-START" % _active_id] = true
	_emit()

func _complete(player: Node) -> void:
	if _status != "objectives_complete":
		return
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
	}
