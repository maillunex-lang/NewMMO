package ws

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

const maxJSONInteger = 1<<53 - 1
const timestampFormat = "2006-01-02T15:04:05.000Z"

var (
	eventName    = regexp.MustCompile(`^[a-z]+(\.[a-z_]+)+$`)
	uuidV4       = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
	buildID      = regexp.MustCompile(`^[a-f0-9]{64}$`)
	semver       = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	errMalformed = errors.New("malformed websocket JSON")
	errEnvelope  = errors.New("invalid websocket envelope")
	errVersion   = errors.New("unsupported websocket protocol")
)

type inbound struct {
	typ, requestID string
	payload        map[string]json.RawMessage
	ignored        bool
}

func decodeMessage(raw []byte) (inbound, error) {
	var m inbound
	if !utf8.Valid(raw) {
		return m, errMalformed
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	root, err := decoder.Token()
	if err != nil || root != json.Delim('{') || !validValue(decoder, root, 1) {
		return m, errMalformed
	}
	if _, err := decoder.Token(); err != io.EOF {
		return m, errMalformed
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return m, errMalformed
	}
	if id, present := fields["request_id"]; present {
		if json.Unmarshal(id, &m.requestID) != nil || !uuidV4.MatchString(m.requestID) {
			m.requestID = ""
			return m, errEnvelope
		}
	}
	var version int
	if json.Unmarshal(fields["protocol_version"], &version) != nil || version < 1 {
		return m, errEnvelope
	}
	if version != 1 {
		return m, errVersion
	}
	if json.Unmarshal(fields["type"], &m.typ) != nil || len(m.typ) > 64 || !eventName.MatchString(m.typ) ||
		json.Unmarshal(fields["payload"], &m.payload) != nil || m.payload == nil {
		return m, errEnvelope
	}
	for _, field := range []string{"session_id", "server_time", "revision"} {
		if _, ok := fields[field]; ok {
			m.ignored = true
		}
	}
	return m, nil
}

func validValue(decoder *json.Decoder, token json.Token, depth int) bool {
	if number, ok := token.(json.Number); ok {
		n, err := number.Float64()
		return err == nil && !math.IsInf(n, 0) && (n != math.Trunc(n) || math.Abs(n) <= maxJSONInteger)
	}
	delimiter, nested := token.(json.Delim)
	if !nested {
		return true
	}
	if depth > 8 || (delimiter != '{' && delimiter != '[') {
		return false
	}
	var keys map[string]struct{}
	if delimiter == '{' {
		keys = make(map[string]struct{})
	}
	for decoder.More() {
		if keys != nil {
			k, err := decoder.Token()
			key, ok := k.(string)
			if err != nil || !ok {
				return false
			}
			if _, duplicate := keys[key]; duplicate {
				return false
			}
			keys[key] = struct{}{}
		}
		value, err := decoder.Token()
		if err != nil || !validValue(decoder, value, depth+1) {
			return false
		}
	}
	_, err := decoder.Token()
	return err == nil
}

type authMessage struct {
	ticket, version, build string
}

func readAuth(payload map[string]json.RawMessage) (authMessage, error) {
	var request authMessage
	if json.Unmarshal(payload["ticket"], &request.ticket) != nil || !auth.ValidSecret(request.ticket) ||
		json.Unmarshal(payload["client_version"], &request.version) != nil {
		return authMessage{}, errEnvelope
	}
	if _, valid := parseVersion(request.version); !valid {
		return authMessage{}, errEnvelope
	}
	if raw, present := payload["content_build_id"]; present {
		if json.Unmarshal(raw, &request.build) != nil || !buildID.MatchString(request.build) {
			return authMessage{}, errEnvelope
		}
	}
	var protocols []int
	if json.Unmarshal(payload["protocol_versions"], &protocols) != nil || len(protocols) == 0 {
		return authMessage{}, errEnvelope
	}
	supported := false
	for _, version := range protocols {
		if version < 1 {
			return authMessage{}, errEnvelope
		}
		supported = supported || version == 1
	}
	if !supported {
		return authMessage{}, errVersion
	}
	return request, nil
}

type version struct {
	core [3]uint64
	pre  []string
}

func parseVersion(raw string) (version, bool) {
	var v version
	if len(raw) > 64 {
		return v, false
	}
	parts := semver.FindStringSubmatch(raw)
	if parts == nil {
		return v, false
	}
	for i := range v.core {
		n, err := strconv.ParseUint(parts[i+1], 10, 64)
		if err != nil {
			return v, false
		}
		v.core[i] = n
	}
	if parts[4] != "" {
		v.pre = strings.Split(parts[4], ".")
		for _, part := range v.pre {
			if len(part) > 1 && part[0] == '0' && digits(part) {
				return v, false
			}
		}
	}
	return v, true
}

func digits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (v version) less(other version) bool {
	for i, n := range v.core {
		if n != other.core[i] {
			return n < other.core[i]
		}
	}
	if len(v.pre) == 0 || len(other.pre) == 0 {
		return len(v.pre) != 0
	}
	for i, a := range v.pre {
		if i == len(other.pre) {
			return false
		}
		b := other.pre[i]
		if a == b {
			continue
		}
		aNum, bNum := digits(a), digits(b)
		if aNum != bNum {
			return aNum
		}
		if aNum && len(a) != len(b) {
			return len(a) < len(b)
		}
		return a < b
	}
	return len(v.pre) < len(other.pre)
}

func compatibility(state auth.WSState, request authMessage) string {
	client, clientValid := parseVersion(request.version)
	minimum, minValid := parseVersion(state.MinClientVersion)
	if !buildID.MatchString(state.BuildID) || !clientValid || !minValid || client.less(minimum) {
		return "incompatible"
	}
	if request.build != state.BuildID {
		return "update_required"
	}
	return "ok"
}

type outbound struct {
	Version    int    `json:"protocol_version"`
	Type       string `json:"type"`
	SessionID  string `json:"session_id"`
	RequestID  string `json:"request_id,omitempty"`
	ServerTime string `json:"server_time"`
	Revision   int64  `json:"revision"`
	TraceID    string `json:"trace_id"`
	Payload    any    `json:"payload"`
}

func timestamp() string { return time.Now().UTC().Format(timestampFormat) }

func sessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func isMutation(typ string) bool {
	switch typ {
	case "quest.accept.request", "quest.turnin.request", "world.interact.request",
		"dialogue.open.request", "dialogue.choice.request",
		"inventory.move.request", "dungeon.enter.request":
		return true
	}
	return false
}
