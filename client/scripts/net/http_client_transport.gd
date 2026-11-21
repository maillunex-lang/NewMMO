extends "res://scripts/net/http_transport.gd"
## Blocking-ish HTTPClient transport for AuthClient (poll until done or timeout).
## Does not log request/response bodies (may contain secrets).

var timeout_sec: float = 15.0


func request(method: String, url: String, headers: PackedStringArray, body: String) -> Dictionary:
	var parsed := _parse_url(url)
	if parsed.is_empty():
		return _err("bad_url")
	var client := HTTPClient.new()
	var tls := TLSOptions.client() if bool(parsed.get("tls", false)) else null
	var err := client.connect_to_host(str(parsed["host"]), int(parsed["port"]), tls)
	if err != OK:
		return _err("connect_failed")
	var deadline := Time.get_ticks_msec() + int(timeout_sec * 1000.0)
	while client.get_status() == HTTPClient.STATUS_CONNECTING or client.get_status() == HTTPClient.STATUS_RESOLVING:
		client.poll()
		if Time.get_ticks_msec() > deadline:
			client.close()
			return _err("connect_timeout")
		OS.delay_msec(5)
	if client.get_status() != HTTPClient.STATUS_CONNECTED:
		client.close()
		return _err("not_connected")
	var path: String = str(parsed["path"])
	var m := _method_enum(method)
	err = client.request(m, path, headers, body)
	if err != OK:
		client.close()
		return _err("request_failed")
	while client.get_status() == HTTPClient.STATUS_REQUESTING:
		client.poll()
		if Time.get_ticks_msec() > deadline:
			client.close()
			return _err("request_timeout")
		OS.delay_msec(5)
	if client.get_status() != HTTPClient.STATUS_BODY and client.get_status() != HTTPClient.STATUS_CONNECTED:
		var st := client.get_status()
		client.close()
		return _err("bad_status_%d" % st)
	var status_code := client.get_response_code()
	var resp_headers := client.get_response_headers()
	var chunks := PackedByteArray()
	while client.get_status() == HTTPClient.STATUS_BODY:
		client.poll()
		var chunk := client.read_response_body_chunk()
		if chunk.size() > 0:
			chunks.append_array(chunk)
		if Time.get_ticks_msec() > deadline:
			client.close()
			return _err("body_timeout")
		OS.delay_msec(2)
	client.close()
	return {
		"status": status_code,
		"headers": resp_headers,
		"body": chunks.get_string_from_utf8(),
		"transport_error": "",
	}


func _err(code: String) -> Dictionary:
	return {
		"status": 0,
		"headers": PackedStringArray(),
		"body": "",
		"transport_error": code,
	}


func _method_enum(method: String) -> int:
	match method.to_upper():
		"GET":
			return HTTPClient.METHOD_GET
		"POST":
			return HTTPClient.METHOD_POST
		"PUT":
			return HTTPClient.METHOD_PUT
		"DELETE":
			return HTTPClient.METHOD_DELETE
		"PATCH":
			return HTTPClient.METHOD_PATCH
		_:
			return HTTPClient.METHOD_POST


func _parse_url(url: String) -> Dictionary:
	var u := url.strip_edges()
	var tls := false
	if u.begins_with("https://"):
		tls = true
		u = u.substr(8)
	elif u.begins_with("http://"):
		u = u.substr(7)
	else:
		return {}
	var slash := u.find("/")
	var hostport := u if slash < 0 else u.substr(0, slash)
	var path := "/" if slash < 0 else u.substr(slash)
	var host := hostport
	var port := 443 if tls else 80
	if hostport.contains(":"):
		var parts := hostport.split(":")
		host = parts[0]
		port = int(parts[1])
	return {"host": host, "port": port, "path": path, "tls": tls}
