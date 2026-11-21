extends Node
## Stage 1 local quest state for the Dawn Valley prologue.

signal quest_updated(quest_id: String, status: String, detail: String)
signal quest_completed(quest_id: String, reward_text: String)

const QUEST_PROLOGUE := "prologue_slimes"

var _active := {
	"id": QUEST_PROLOGUE,
	"title": "Prologue · Clear the Path",
	"status": "ready",  # ready | active | turn_in | done
	"need": 3,
	"have": 0,
	"target": "slime",
}

func _ready() -> void:
	add_to_group("quest_manager")
	_emit()

func get_tracker_text() -> String:
	match _active["status"]:
		"ready":
			return "%s\nTalk to Elder Mira" % _active["title"]
		"active":
			return "%s\nSlimes %d/%d" % [_active["title"], _active["have"], _active["need"]]
		"turn_in":
			return "%s\nReturn to Elder Mira" % _active["title"]
		"done":
			return "Prologue complete · Explore freely"
		_:
			return _active["title"]

func on_dialogue_finished(npc_id: String, player: Node) -> void:
	if npc_id != "elder_mira":
		return
	if _active["status"] == "ready":
		_active["status"] = "active"
		_emit()
	elif _active["status"] == "turn_in":
		_active["status"] = "done"
		if player and player.has_method("add_gold"):
			player.add_gold(25)
		if player and player.has_method("add_loot"):
			player.add_loot("dawn_charm", 1)
		if player and player.has_method("heal"):
			player.heal(999.0)
		quest_completed.emit(QUEST_PROLOGUE, "+25 gold, Dawn Charm, full heal")
		_emit()

func on_monster_killed(monster_id: String) -> void:
	if _active["status"] != "active":
		return
	if monster_id != _active["target"]:
		return
	_active["have"] = mini(int(_active["need"]), int(_active["have"]) + 1)
	if int(_active["have"]) >= int(_active["need"]):
		_active["status"] = "turn_in"
	_emit()

func _emit() -> void:
	quest_updated.emit(str(_active["id"]), str(_active["status"]), get_tracker_text())
