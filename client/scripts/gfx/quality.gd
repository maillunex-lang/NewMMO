class_name GfxQuality
extends RefCounted
## Graphics presets Low → Ultra (Lunex Fantasy 3D). Pure data + an `apply` that configures a viewport, the sun and
## the environment. Persisted in user://settings.cfg [gfx] preset. Gameplay never reads this.

const PRESETS := ["low", "medium", "high", "ultra"]
const SETTINGS_PATH := "user://settings.cfg"

const DATA := {
	"low":    {"scale": 0.70, "msaa": Viewport.MSAA_DISABLED, "shadows": false, "shadow_size": 1024, "cascades": 1, "shadow_dist": 30.0, "glow": false, "particles": 0.35, "vegetation": 0.45, "rays": false, "fps": 60},
	"medium": {"scale": 0.85, "msaa": Viewport.MSAA_DISABLED, "shadows": true,  "shadow_size": 2048, "cascades": 1, "shadow_dist": 45.0, "glow": false, "particles": 0.6,  "vegetation": 0.7,  "rays": false, "fps": 60},
	"high":   {"scale": 1.00, "msaa": Viewport.MSAA_2X,       "shadows": true,  "shadow_size": 4096, "cascades": 2, "shadow_dist": 70.0, "glow": true,  "particles": 1.0,  "vegetation": 1.0,  "rays": true,  "fps": 60},
	"ultra":  {"scale": 1.00, "msaa": Viewport.MSAA_4X,       "shadows": true,  "shadow_size": 4096, "cascades": 4, "shadow_dist": 110.0, "glow": true, "particles": 1.5,  "vegetation": 1.4,  "rays": true,  "fps": 60},
}

static func load_preset() -> String:
	var cfg := ConfigFile.new()
	if cfg.load(SETTINGS_PATH) == OK:
		var p := str(cfg.get_value("gfx", "preset", "high"))
		if DATA.has(p):
			return p
	return "high"

static func save_preset(preset: String) -> void:
	if not DATA.has(preset):
		return
	var cfg := ConfigFile.new()
	cfg.load(SETTINGS_PATH)
	cfg.set_value("gfx", "preset", preset)
	cfg.save(SETTINGS_PATH)

static func settings(preset: String) -> Dictionary:
	return DATA.get(preset, DATA["high"])

## Apply a preset. `env` / `sun` may be null (tests).
static func apply(preset: String, viewport: Viewport, env: Environment, sun: DirectionalLight3D) -> void:
	var s := settings(preset)
	if viewport != null:
		viewport.scaling_3d_scale = float(s["scale"])
		viewport.msaa_3d = int(s["msaa"])
	if sun != null:
		sun.shadow_enabled = bool(s["shadows"])
		sun.directional_shadow_max_distance = float(s["shadow_dist"])
		sun.directional_shadow_mode = DirectionalLight3D.SHADOW_ORTHOGONAL if int(s["cascades"]) == 1 else (DirectionalLight3D.SHADOW_PARALLEL_2_SPLITS if int(s["cascades"]) == 2 else DirectionalLight3D.SHADOW_PARALLEL_4_SPLITS)
		sun.shadow_blur = 1.6 if preset in ["high", "ultra"] else 1.0
		RenderingServer.directional_shadow_atlas_set_size(int(s["shadow_size"]), true)
	if env != null:
		env.glow_enabled = bool(s["glow"])
