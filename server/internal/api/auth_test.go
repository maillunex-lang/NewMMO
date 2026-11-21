package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidationBeforeDatabase(t *testing.T) {
	handler := New(nil, "")
	for _, tc := range []struct {
		path string
		body string
	}{
		{"/v1/accounts", `{"login":"ABC","password":"long enough password"}`},
		{"/v1/accounts", `{"login":"valid","password":"abc"}`},
		{"/v1/accounts", `{"login":"valid","password":"long enough password","email":"x"}`},
		{"/v1/auth/login", `{"login":"valid","login":"duplicate","password":"long enough password"}`},
		{"/v1/auth/login", `{"login":"valid","password":null}`},
		{"/v1/auth/login", `{"login":"valid","password":"long enough password"} {}`},
		{"/v1/auth/login", strings.Repeat(" ", 4097)},
		{"/v1/auth/login", "{\"login\":\"valid\",\"password\":\"abcdefghij\xff\"}"},
		{"/v1/auth/logout", `{"extra":"x"}`},
		{"/v1/characters", `{"name":"Bad Name","class_id":"CLS-01"}`},
		{"/v1/characters", `{"name":"Валидно","class_id":"CLS-06"}`},
		{"/v1/session/ticket", `{"character_id":1}`},
		{"/v1/session/ticket", `{"character_id":"9223372036854775808"}`},
		{"/v1/session/ticket", `{"character_id":"0"}`},
	} {
		request := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 400 || response.Body.String() != "{\"error\":{\"code\":\"BAD_REQUEST\",\"message_key\":\"error.bad_request\"}}\n" {
			t.Fatalf("%s invalid input returned %d %s", tc.path, response.Code, response.Body)
		}
	}
	request := httptest.NewRequest("POST", "/v1/accounts", strings.NewReader(`{"login":"valid","password":"long enough password"}`))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("non-JSON body returned %d", response.Code)
	}
}
