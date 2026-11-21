package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

var (
	loginPattern = regexp.MustCompile(`^[a-z0-9_]{3,24}$`)
	namePattern  = regexp.MustCompile(`^[A-Za-zА-Яа-яЁё]{3,16}$`)
	classPattern = regexp.MustCompile(`^CLS-0[1-5]$`)
	idPattern    = regexp.MustCompile(`^[1-9][0-9]*$`)
)

type Handler struct {
	store *auth.Store
	wsURL string
}

func New(store *auth.Store, wsURL string) *Handler {
	return &Handler{store: store, wsURL: wsURL}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/accounts":
		h.credentials(w, r, true)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/login":
		h.credentials(w, r, false)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/logout":
		h.logout(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/characters":
		h.listCharacters(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/characters":
		h.createCharacter(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/session/ticket":
		h.issueTicket(w, r)
	default:
		writeError(w, auth.ErrNotFound)
	}
}

func (h *Handler) credentials(w http.ResponseWriter, r *http.Request, register bool) {
	fields, ok := readFields(w, r, "login", "password")
	password, validPassword := auth.NormalizePassword(fields["password"])
	login := fields["login"]
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if !ok || !validPassword || !loginPattern.MatchString(login) || err != nil {
		writeError(w, badRequest)
		return
	}
	ip := peer.Addr().Unmap().WithZone("")
	if register {
		accountID, err := h.store.Register(r.Context(), login, password, ip)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"account_id": accountID})
		return
	}
	session, err := h.store.Login(r.Context(), login, password, ip)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func bearer(r *http.Request) string {
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 {
		return ""
	}
	scheme, token, ok := strings.Cut(headers[0], " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || !auth.ValidSecret(token) {
		return ""
	}
	return token
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if _, ok := readFields(w, r); !ok {
		writeError(w, badRequest)
		return
	}
	if err := h.store.Logout(r.Context(), bearer(r)); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listCharacters(w http.ResponseWriter, r *http.Request) {
	// The GET contract has no request body.
	if r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		writeError(w, badRequest)
		return
	}
	accountID, err := h.store.Authenticate(r.Context(), bearer(r))
	if err != nil {
		writeError(w, err)
		return
	}
	characters, err := h.store.Characters(r.Context(), accountID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Characters []auth.Character `json:"characters"`
	}{Characters: characters})
}

func (h *Handler) createCharacter(w http.ResponseWriter, r *http.Request) {
	fields, ok := readFields(w, r, "name", "class_id")
	name, classID := fields["name"], fields["class_id"]
	if !ok || !namePattern.MatchString(name) || !classPattern.MatchString(classID) {
		writeError(w, badRequest)
		return
	}
	accountID, err := h.store.Authenticate(r.Context(), bearer(r))
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := h.store.CreateCharacter(r.Context(), accountID, name, classID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"character_id": id})
}

func (h *Handler) issueTicket(w http.ResponseWriter, r *http.Request) {
	fields, ok := readFields(w, r, "character_id")
	id := fields["character_id"]
	characterID, err := strconv.ParseInt(id, 10, 64)
	if !ok || !idPattern.MatchString(id) || err != nil {
		writeError(w, badRequest)
		return
	}
	accountID, err := h.store.Authenticate(r.Context(), bearer(r))
	if err != nil {
		writeError(w, err)
		return
	}
	ticket, err := h.store.IssueTicket(r.Context(), accountID, characterID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		auth.Ticket
		WSURL string `json:"ws_url"`
	}{Ticket: ticket, WSURL: h.wsURL})
}

func readFields(w http.ResponseWriter, r *http.Request, expected ...string) (map[string]string, bool) {
	contentType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" ||
		(params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
		return nil, false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<10))
	if err != nil || !utf8.Valid(raw) {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, false
	}
	fields := make(map[string]string, len(expected))
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok {
			return nil, false
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, false
		}
		var value *string
		if err := decoder.Decode(&value); err != nil || value == nil {
			return nil, false
		}
		fields[key] = *value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); err != io.EOF || len(fields) != len(expected) {
		return nil, false
	}
	for _, key := range expected {
		if _, found := fields[key]; !found {
			return nil, false
		}
	}
	return fields, true
}

var badRequest = &auth.Failure{Code: "BAD_REQUEST"}

func writeError(w http.ResponseWriter, err error) {
	var failure *auth.Failure
	if !errors.As(err, &failure) {
		failure = auth.ErrInternal
	}
	status := http.StatusInternalServerError
	switch failure.Code {
	case "BAD_REQUEST":
		status = http.StatusBadRequest
	case "UNAUTHENTICATED", "INVALID_CREDENTIALS":
		status = http.StatusUnauthorized
	case "FORBIDDEN":
		status = http.StatusForbidden
	case "NOT_FOUND":
		status = http.StatusNotFound
	case "LOGIN_TAKEN", "NAME_TAKEN", "LIMIT_REACHED":
		status = http.StatusConflict
	case "RATE_LIMITED":
		status = http.StatusTooManyRequests
		w.Header().Set("Retry-After", strconv.FormatInt((failure.RetryAfterMS+999)/1000, 10))
	}
	type detail struct {
		Code         string `json:"code"`
		MessageKey   string `json:"message_key"`
		RetryAfterMS int64  `json:"retry_after_ms,omitempty"`
	}
	writeJSON(w, status, struct {
		Error detail `json:"error"`
	}{Error: detail{failure.Code, "error." + strings.ToLower(failure.Code), failure.RetryAfterMS}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
