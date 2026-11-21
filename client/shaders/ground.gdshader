shader_type spatial;
// Lunex Fantasy 3D ground: non-repeating grass (3 noise scales: broad colour drift, patches, fine speckle + flowers)
// blended with a dirt road taken from the road mask (rendered from the location's Line2D paths).
uniform sampler2D road_mask : hint_default_black, filter_linear;
uniform sampler2D noise_tex : repeat_enable, filter_linear;
uniform vec2 map_size = vec2(64.0, 44.0);
varying vec3 wpos;

void vertex() {
	wpos = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
}

float n(vec2 p) { return texture(noise_tex, p).r; }

void fragment() {
	vec2 uv = wpos.xz / map_size;
	float road = texture(road_mask, uv).r;
	float n1 = n(wpos.xz * 0.030);
	float n2 = n(wpos.xz * 0.15 + vec2(7.3, 1.9));
	float n3 = n(wpos.xz * 0.85);
	float n4 = n(wpos.xz * 2.4 + vec2(3.1, 9.4));
	vec3 g_dark = vec3(0.14, 0.48, 0.11);
	vec3 g_lime = vec3(0.48, 0.80, 0.20);
	vec3 g_teal = vec3(0.20, 0.62, 0.34);
	vec3 g_dry  = vec3(0.70, 0.74, 0.26);
	vec3 grass = mix(g_dark, g_lime, smoothstep(0.25, 0.75, n1));
	grass = mix(grass, g_teal, smoothstep(0.55, 0.85, n2) * 0.55);
	grass = mix(grass, g_dry, smoothstep(0.60, 0.92, n1 * (0.6 + n2)) * 0.5);
	grass *= 0.86 + 0.28 * n3;
	// wildflower speckles, clustered where the patch noise is high
	float fl = step(0.90, n4) * smoothstep(0.35, 0.65, n2);
	vec3 flower = mix(vec3(1.0, 0.86, 0.25), vec3(1.0, 0.55, 0.75), step(0.5, n3));
	grass = mix(grass, flower, fl * 0.85);
	// dirt road: warm variation, pebbles, trodden darker centre, grassy ragged edge
	vec3 dirt = mix(vec3(0.60, 0.42, 0.25), vec3(0.80, 0.62, 0.38), n2);
	dirt *= 0.88 + 0.22 * n3;
	float peb = step(0.93, n4);
	dirt = mix(dirt, vec3(0.55, 0.5, 0.46), peb * 0.6);
	dirt *= 1.0 - 0.18 * smoothstep(0.7, 1.0, road);
	float edge = smoothstep(0.30 + 0.12 * n3, 0.62, road);
	ALBEDO = mix(grass, dirt, edge);
	ROUGHNESS = 1.0;
}
