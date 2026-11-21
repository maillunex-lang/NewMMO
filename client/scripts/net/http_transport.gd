extends RefCounted
## Abstract HTTP transport for AuthClient. Implementations must not log bodies.

## Result dictionary keys:
##   status: int (HTTP status, 0 on transport failure)
##   headers: PackedStringArray
##   body: String (raw UTF-8)
##   transport_error: String (empty if OK)

func request(_method: String, _url: String, _headers: PackedStringArray, _body: String) -> Dictionary:
	return {
		"status": 0,
		"headers": PackedStringArray(),
		"body": "",
		"transport_error": "not_implemented",
	}
