extends "res://scripts/net/http_transport.gd"
## Scripted HTTP responses for headless auth tests (no network).

var scripts: Array = []  # [{method, path_suffix, status, body}]
var calls: Array = []  # recorded outbound calls (bodies included for test asserts only)

func enqueue(method: String, path_suffix: String, status: int, body) -> void:
	var body_str := ""
	if typeof(body) == TYPE_DICTIONARY or typeof(body) == TYPE_ARRAY:
		body_str = JSON.stringify(body)
	elif typeof(body) == TYPE_STRING:
		body_str = body
	scripts.append({
		"method": method.to_upper(),
		"path_suffix": path_suffix,
		"status": status,
		"body": body_str,
	})


func clear() -> void:
	scripts.clear()
	calls.clear()


func request(method: String, url: String, headers: PackedStringArray, body: String) -> Dictionary:
	var m := method.to_upper()
	calls.append({
		"method": m,
		"url": url,
		"headers": headers.duplicate(),
		"body": body,
	})
	if scripts.is_empty():
		return {
			"status": 0,
			"headers": PackedStringArray(),
			"body": "",
			"transport_error": "fake_http_empty",
		}
	var script: Dictionary = scripts.pop_front()
	var path := _path_of(url)
	if str(script.get("method", "")) != m:
		return {
			"status": 0,
			"headers": PackedStringArray(),
			"body": "",
			"transport_error": "fake_http_method_mismatch",
		}
	var want: String = str(script.get("path_suffix", ""))
	if not path.ends_with(want) and path != want:
		return {
			"status": 0,
			"headers": PackedStringArray(),
			"body": "",
			"transport_error": "fake_http_path_mismatch want=%s got=%s" % [want, path],
		}
	return {
		"status": int(script.get("status", 0)),
		"headers": PackedStringArray(),
		"body": str(script.get("body", "")),
		"transport_error": "",
	}


func _path_of(url: String) -> String:
	var u := url
	var scheme := u.find("://")
	if scheme >= 0:
		u = u.substr(scheme + 3)
	var slash := u.find("/")
	if slash < 0:
		return "/"
	return u.substr(slash)
