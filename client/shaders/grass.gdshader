shader_type spatial;
render_mode cull_disabled, skip_vertex_transform;
// Volumetric grass. Clump instances sit on a static grid that wraps around `focus` (no CPU updates while moving).
// `press_map` (R = how flattened 0..1, GB = push direction) is rebuilt by Grass.gd from recent footsteps: stepped
// grass is held down for ~0.7 s, then slowly rises back. Wind sways the tips. Blades vanish on roads.
uniform vec3 focus;
uniform float area = 26.0;
uniform sampler2D press_map : filter_linear, hint_default_black;
uniform vec2 press_origin;
uniform float press_size = 26.0;
uniform sampler2D road_mask : hint_default_black, filter_linear;
uniform vec2 map_size = vec2(64.0, 44.0);
uniform float t_now = 0.0;

float hash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }

void vertex() {
	vec3 base = MODEL_MATRIX[3].xyz;
	vec2 d = base.xz - focus.xz;
	vec2 wrapped = d - area * floor((d + area * 0.5) / area);
	vec2 o = focus.xz + wrapped;
	float h = hash(base.xz);
	float h2 = hash(base.xz + 17.0);
	float tip = clamp(VERTEX.y, 0.0, 1.0);
	// road + edge-of-area fade (blades shrink to nothing)
	float road = texture(road_mask, o / map_size).r;
	float fade = (1.0 - smoothstep(0.25, 0.5, road)) * (1.0 - smoothstep(area * 0.38, area * 0.5, length(wrapped)));
	float blade_h = (0.28 + 0.38 * h) * fade;
	// footsteps
	vec3 pm = texture(press_map, (o - press_origin) / press_size).rgb;
	float press = pm.r;
	vec2 push = (pm.gb - 0.5) * 2.0;
	blade_h *= 1.0 - 0.62 * press;
	// wind
	float wind = sin(t_now * 1.6 + o.x * 0.45 + o.y * 0.35 + h * 6.0) * 0.09 + sin(t_now * 2.9 + o.y * 0.9) * 0.04;
	vec2 lean = vec2(wind, wind * 0.6) + push * (0.9 * press);
	vec3 local = vec3(VERTEX.x, VERTEX.y * blade_h, VERTEX.z);
	// rotate the clump randomly
	float a = h2 * 6.2831;
	local.xz = vec2(local.x * cos(a) - local.z * sin(a), local.x * sin(a) + local.z * cos(a));
	vec3 world = vec3(o.x, 0.0, o.y) + local + vec3(lean.x, 0.0, lean.y) * tip * tip * (blade_h + 0.2);
	world.y -= (lean.x * lean.x + lean.y * lean.y) * 0.25 * tip * tip;
	VERTEX = (VIEW_MATRIX * vec4(world, 1.0)).xyz;
	NORMAL = (VIEW_MATRIX * vec4(0.0, 1.0, 0.0, 0.0)).xyz;
	// colour: dark base -> bright tip, per-blade hue drift (lime / teal / golden)
	vec3 base_c = vec3(0.08, 0.30, 0.06);
	vec3 tip_c = mix(vec3(0.50, 0.85, 0.20), vec3(0.28, 0.78, 0.38), h2);
	tip_c = mix(tip_c, vec3(0.85, 0.82, 0.28), step(0.88, h) * 0.8);
	COLOR = vec4(mix(base_c, tip_c, tip) * (1.0 - 0.25 * press), 1.0);
}

void fragment() {
	ALBEDO = COLOR.rgb;
	ROUGHNESS = 0.85;
	BACKLIGHT = COLOR.rgb * 0.25;
}
