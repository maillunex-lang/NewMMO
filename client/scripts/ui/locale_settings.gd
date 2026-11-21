extends RefCounted
## Persist UI locale (EX-P06). Path: user://settings.cfg

const PATH := "user://settings.cfg"
const SECTION := "i18n"
const KEY := "locale"

static func load_locale() -> String:
	var cfg := ConfigFile.new()
	if cfg.load(PATH) != OK:
		var forced := str(ProjectSettings.get_setting("internationalization/locale/locale", ""))
		if forced != "":
			return forced
		return "ru"
	var v := str(cfg.get_value(SECTION, KEY, "ru"))
	if v.begins_with("en"):
		return "en"
	return "ru"

static func save_locale(code: String) -> void:
	var cfg := ConfigFile.new()
	cfg.load(PATH)  # ignore missing
	var normalized := "en" if str(code).begins_with("en") else "ru"
	cfg.set_value(SECTION, KEY, normalized)
	cfg.save(PATH)
