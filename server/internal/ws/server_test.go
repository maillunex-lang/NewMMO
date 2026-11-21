package ws

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/httpservice"
)

const requestID = "4a5a7114-59f0-4ab6-b34d-5938cdfbb424"

var fixtureBuild = strings.Repeat("a", 64)

type memoryStore struct {
	mu      sync.Mutex
	state   auth.WSState
	tickets map[string]int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{state: auth.WSState{
		Character: auth.Character{Name: "Fixture", ClassID: "warrior", Level: 2},
		XP:        101, Gold: 12, Revision: 3, BuildID: fixtureBuild, MinClientVersion: "0.1.0",
	}, tickets: make(map[string]int64)}
}

func (m *memoryStore) ticket(t *testing.T, id int64) string {
	t.Helper()
	var raw [32]byte
	_, _ = rand.Read(raw[:])
	ticket := base64.RawURLEncoding.EncodeToString(raw[:])
	m.mu.Lock()
	m.tickets[ticket] = id
	m.mu.Unlock()
	return ticket
}

func (m *memoryStore) ConsumeTicket(_ context.Context, ticket string) (string, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.tickets[ticket]
	if !ok {
		return "", 0, auth.ErrInvalidTicket
	}
	delete(m.tickets, ticket)
	return "owner", id, nil
}

func (m *memoryStore) LoadWSState(_ context.Context, _ string, id int64) (auth.WSState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.state
	state.ID = strconv.FormatInt(id, 10)
	return state, nil
}

type testLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *testLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *testLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func testServer(t *testing.T, cfg Config, store Store, wrap func(net.Listener) net.Listener) (*Server, string, *testLog) {
	t.Helper()
	log := &testLog{}
	logger := slog.New(slog.NewJSONHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	socket := New(t.Context(), cfg, store, logger)
	h := httptest.NewUnstartedServer(httpservice.NewHandler(t.Context(), logger, httpservice.Application{WS: socket}))
	h.Config.ReadTimeout, h.Config.WriteTimeout = 10*time.Second, 10*time.Second
	if wrap != nil {
		h.Listener = wrap(h.Listener)
	}
	h.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		defer h.Close()
		_ = socket.Shutdown(ctx)
	})
	return socket, "ws" + strings.TrimPrefix(h.URL, "http") + "/v1/ws", log
}

func dial(t *testing.T, url string, opts *websocket.DialOptions) *websocket.Conn {
	t.Helper()
	if opts == nil {
		opts = &websocket.DialOptions{}
	}
	opts.Subprotocols = []string{Subprotocol}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, url, opts)
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols || conn.Subprotocol() != Subprotocol || resp.Header.Get("Sec-WebSocket-Extensions") != "" {
		t.Fatal("unexpected upgrade negotiation")
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func send(t *testing.T, c *websocket.Conn, typ, id string, payload map[string]any) {
	t.Helper()
	fields := map[string]any{"protocol_version": 1, "type": typ, "payload": payload}
	if id != "" {
		fields["request_id"] = id
	}
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	sendRaw(t, c, websocket.MessageText, data)
}

func sendRaw(t *testing.T, c *websocket.Conn, mt websocket.MessageType, data []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, mt, data); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func login(t *testing.T, c *websocket.Conn, ticket, version, build string) {
	t.Helper()
	payload := map[string]any{"ticket": ticket, "client_version": version, "protocol_versions": []int{1}, "character_id": "999"}
	if build != "" {
		payload["content_build_id"] = build
	}
	send(t, c, "session.auth", requestID, payload)
}

type wireMessage struct {
	Version  int                        `json:"protocol_version"`
	Type     string                     `json:"type"`
	Session  string                     `json:"session_id"`
	Trace    string                     `json:"trace_id"`
	Request  string                     `json:"request_id"`
	Time     string                     `json:"server_time"`
	Revision int64                      `json:"revision"`
	Payload  map[string]json.RawMessage `json:"payload"`
}

func read(t *testing.T, c *websocket.Conn) (wireMessage, error) {
	t.Helper()
	var msg wireMessage
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	mt, data, err := c.Read(ctx)
	if err != nil {
		return msg, err
	}
	if mt != websocket.MessageText || json.Unmarshal(data, &msg) != nil || msg.Version != 1 || msg.Session == "" ||
		msg.Trace == "" || len(msg.Trace) > 64 || msg.Payload == nil {
		t.Fatal("invalid server envelope")
	}
	if _, err := time.Parse(timestampFormat, msg.Time); err != nil {
		t.Fatal("invalid server time")
	}
	return msg, nil
}

func expect(t *testing.T, c *websocket.Conn, typ string) wireMessage {
	t.Helper()
	msg, err := read(t, c)
	if err != nil || msg.Type != typ {
		t.Fatalf("want %s, got %q and %v", typ, msg.Type, err)
	}
	return msg
}

func expectClose(t *testing.T, c *websocket.Conn, code websocket.StatusCode) {
	t.Helper()
	_, err := read(t, c)
	if actual := websocket.CloseStatus(err); actual != code {
		t.Fatalf("close = %d, want %d (%v)", actual, code, err)
	}
}

func expectInitial(t *testing.T, c *websocket.Conn, char, compatibility string) wireMessage {
	t.Helper()
	ready := expect(t, c, "session.ready")
	version := expect(t, c, "content.version")
	snapshot := expect(t, c, "state.snapshot")
	if string(ready.Payload["character_id"]) != strconv.Quote(char) || string(ready.Payload["session_id"]) != strconv.Quote(ready.Session) ||
		string(version.Payload["compatibility"]) != strconv.Quote(compatibility) {
		t.Fatalf("wrong ready or compatibility payload: %v %v", ready.Payload, version.Payload)
	}
	for _, msg := range []wireMessage{ready, version, snapshot} {
		if msg.Session != ready.Session || msg.Request != requestID || msg.Revision != ready.Revision {
			t.Fatalf("incorrect session, correlation or revision for %s", msg.Type)
		}
	}
	return snapshot
}

func TestUpgradeSnapshotAndRevisionGap(t *testing.T) {
	store := newMemoryStore()
	_, url, log := testServer(t, DefaultConfig(), store, nil)
	c := dial(t, url, nil)
	ticket := store.ticket(t, 42)
	login(t, c, ticket, "0.1.0", fixtureBuild)
	initial := expectInitial(t, c, "42", "ok")
	scopes := string(initial.Payload["scopes"])
	if !strings.Contains(scopes, `"quests":{"active":[],"available":[],"completed":[]}`) ||
		!strings.Contains(scopes, `"character":{"character_id":"42"`) || initial.Revision != 3 {
		t.Fatalf("wrong initial snapshot: %s", scopes)
	}
	store.mu.Lock()
	store.state.Revision, store.state.Gold = 8, 17
	store.mu.Unlock()
	spoof := "untrusted-field-marker"
	sendRaw(t, c, websocket.MessageText, []byte(`{"protocol_version":1,"type":"state.snapshot.request","request_id":"`+requestID+
		`","revision":999,"server_time":"`+spoof+`","session_id":"`+spoof+`","trace_id":"`+spoof+
		`","extra":true,"payload":{"scopes":["character"],"character_id":"999"}}`))
	refresh := expect(t, c, "state.snapshot")
	scopes = string(refresh.Payload["scopes"])
	if refresh.Revision != 8 || string(refresh.Payload["revision"]) != "8" || refresh.Session != initial.Session ||
		refresh.Trace == spoof || refresh.Request != requestID || !strings.Contains(scopes, `"gold":17`) ||
		strings.Contains(scopes, `"quests"`) || !strings.Contains(scopes, `"character_id":"42"`) {
		t.Fatalf("wrong refreshed snapshot: %+v", refresh)
	}

	send(t, c, "future.request", requestID, map[string]any{"value": spoof})
	commandError := expect(t, c, "command.error")
	if commandError.Request != requestID || string(commandError.Payload["code"]) != `"UNKNOWN_TYPE"` {
		t.Fatal("unknown command did not return a correlated error")
	}
	send(t, c, "quest.accept.request", "", map[string]any{})
	assertCode(t, c, "BAD_REQUEST")
	send(t, c, "quest.accept.request", requestID, map[string]any{})
	assertCode(t, c, "UNKNOWN_TYPE")
	sendRaw(t, c, websocket.MessageText, []byte(`{"protocol_version":1,"type":"quest.accept.request","request_id":"`+spoof+`","payload":{}}`))
	assertCode(t, c, "BAD_REQUEST")
	if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get("http" + strings.TrimPrefix(url, "ws") + "?ticket=" + ticket)
	if err != nil {
		t.Fatal("query rejection request failed")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != http.StatusBadRequest || bytes.Contains(body, []byte(ticket)) {
		t.Fatal("query credential not safely rejected")
	}
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(log.String(), `"status":101`) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	logged := log.String()
	if strings.Count(logged, `"status":101`) != 1 || !strings.Contains(logged, "ignored client-owned") ||
		strings.Contains(logged, ticket) || strings.Contains(logged, spoof) {
		t.Fatal("unexpected HTTP 101/access-log count or client values in logs")
	}
}

func assertCode(t *testing.T, c *websocket.Conn, code string) wireMessage {
	t.Helper()
	msg := expect(t, c, "command.error")
	if string(msg.Payload["code"]) != strconv.Quote(code) ||
		string(msg.Payload["message_key"]) != strconv.Quote("error."+strings.ToLower(code)) {
		t.Fatalf("want error %s", code)
	}
	return msg
}

func TestTransportRejections(t *testing.T) {
	t.Run("subprotocol", func(t *testing.T) {
		_, url, _ := testServer(t, DefaultConfig(), newMemoryStore(), nil)
		for _, offers := range [][]string{nil, {"newmmo.v2"}, {"NEWMMO.V1"}} {
			c, res, err := websocket.Dial(t.Context(), url, &websocket.DialOptions{Subprotocols: offers})
			if c != nil {
				_ = c.CloseNow()
			}
			if err == nil || res == nil || res.StatusCode != http.StatusBadRequest {
				t.Fatalf("allowed invalid protocol offer %v", offers)
			}
		}
	})
	for _, tt := range []struct {
		name string
		mt   websocket.MessageType
		data string
		code websocket.StatusCode
	}{
		{"binary", websocket.MessageBinary, "{}", 1003},
		{"oversize", websocket.MessageText, strings.Repeat(" ", inputLimit+1), 1009},
		{"malformed", websocket.MessageText, "{", 1008},
		{"duplicate", websocket.MessageText, `{"payload":{"ticket":1,"tick\u0065t":2}}`, 1008},
		{"trailing", websocket.MessageText, "{} {}", 1008},
		{"deep", websocket.MessageText, `{"x":` + strings.Repeat("[", 8) + "0" + strings.Repeat("]", 8) + "}", 1008},
		{"bad UTF-8", websocket.MessageText, "{\"x\":\"\xff\"}", 1008},
		{"oversize integer", websocket.MessageText, `{"x":9007199254740992}`, 1008},
		{"unsupported version", websocket.MessageText, `{"protocol_version":2,"type":"session.auth","payload":{}}`, 4002},
		{"preauth command", websocket.MessageText, `{"protocol_version":1,"type":"state.snapshot.request","payload":{}}`, 4001},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, url, _ := testServer(t, DefaultConfig(), newMemoryStore(), nil)
			c := dial(t, url, nil)
			sendRaw(t, c, tt.mt, []byte(tt.data))
			expectClose(t, c, tt.code)
		})
	}
	t.Run("auth timeout", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AuthTimeout = 60 * time.Millisecond
		_, url, _ := testServer(t, cfg, newMemoryStore(), nil)
		expectClose(t, dial(t, url, nil), 4001)
	})
	t.Run("auth protocol list", func(t *testing.T) {
		store := newMemoryStore()
		_, url, _ := testServer(t, DefaultConfig(), store, nil)
		c := dial(t, url, nil)
		send(t, c, "session.auth", "", map[string]any{
			"ticket": store.ticket(t, 1), "client_version": "0.1.0", "protocol_versions": []int{2},
		})
		expectClose(t, c, 4002)
	})
}

func TestContentCompatibility(t *testing.T) {
	for _, tt := range []struct{ name, active, version, clientBuild, want string }{
		{"equal", fixtureBuild, "0.1.0", fixtureBuild, "ok"},
		{"absent", fixtureBuild, "0.1.1", "", "update_required"},
		{"different", fixtureBuild, "0.2.0", strings.Repeat("b", 64), "update_required"},
		{"below", fixtureBuild, "0.0.99", fixtureBuild, "incompatible"},
		{"prerelease", fixtureBuild, "0.1.0-rc.2", fixtureBuild, "incompatible"},
		{"unpublished", "", "0.1.0", fixtureBuild, "incompatible"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := newMemoryStore()
			st.state.BuildID = tt.active
			_, url, _ := testServer(t, DefaultConfig(), st, nil)
			c := dial(t, url, nil)
			login(t, c, st.ticket(t, 5), tt.version, tt.clientBuild)
			if tt.want != "incompatible" {
				expectInitial(t, c, "5", tt.want)
				return
			}
			version := expect(t, c, "content.version")
			if string(version.Payload["compatibility"]) != `"incompatible"` || string(version.Payload["content_build_id"]) != strconv.Quote(tt.active) {
				t.Fatal("wrong incompatible payload")
			}
			expectClose(t, c, 4004)
		})
	}
}

func TestHeartbeat(t *testing.T) {
	for _, pong := range []bool{true, false} {
		t.Run(strconv.FormatBool(pong), func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Heartbeat, cfg.PongTimeout = 30*time.Millisecond, 200*time.Millisecond
			st := newMemoryStore()
			_, url, _ := testServer(t, cfg, st, nil)
			pings := make(chan struct{}, 4)
			c := dial(t, url, &websocket.DialOptions{OnPingReceived: func(context.Context, []byte) bool {
				select {
				case pings <- struct{}{}:
				default:
				}
				return pong
			}})
			login(t, c, st.ticket(t, 2), "0.1.0", fixtureBuild)
			expectInitial(t, c, "2", "ok")
			if !pong {
				expectClose(t, c, 1001)
				if len(pings) == 0 {
					t.Fatal("server did not send a Ping")
				}
				return
			}

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			reply := make(chan error, 1)
			go func() {
				_, _, err := c.Read(ctx)
				reply <- err
			}()
			for range 2 {
				select {
				case <-pings:
				case <-ctx.Done():
					t.Fatal("heartbeat stopped despite Pong")
				}
			}
			send(t, c, "state.snapshot.request", requestID, map[string]any{})
			select {
			case err := <-reply:
				if err != nil {
					t.Fatalf("responsive peer could not read a snapshot: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("responsive peer could not request a snapshot")
			}
		})
	}
}

func TestNewestSessionAndShutdown(t *testing.T) {
	st := newMemoryStore()
	s, url, _ := testServer(t, DefaultConfig(), st, nil)
	old := dial(t, url, nil)
	login(t, old, st.ticket(t, 4), "0.1.0", fixtureBuild)
	oldID := expectInitial(t, old, "4", "ok").Session
	latest := dial(t, url, nil)
	login(t, latest, st.ticket(t, 4), "0.1.0", fixtureBuild)
	if expectInitial(t, latest, "4", "ok").Session == oldID {
		t.Fatal("replacement reused old session ID")
	}
	expectClose(t, old, 4005)
	pending := dial(t, url, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	drained := make(chan error, 1)
	go func() { drained <- s.Shutdown(ctx) }()
	expectClose(t, latest, 1001)
	expectClose(t, pending, 1001)
	if err := <-drained; err != nil {
		t.Fatal(err)
	}
	c, res, err := websocket.Dial(t.Context(), url, &websocket.DialOptions{Subprotocols: []string{Subprotocol}})
	if c != nil {
		_ = c.CloseNow()
	}
	if err == nil || res == nil || res.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("shutdown allowed a new connection")
	}
}

func TestRateLimits(t *testing.T) {
	t.Run("handshake", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Handshakes = 1
		_, url, _ := testServer(t, cfg, newMemoryStore(), nil)
		_ = dial(t, url, nil)
		expect429(t, url)
	})
	t.Run("pending", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Pending = 1
		_, url, _ := testServer(t, cfg, newMemoryStore(), nil)
		_ = dial(t, url, nil)
		expect429(t, url)
	})
	for _, typ := range []string{"future.request", "quest.accept.request"} {
		t.Run(typ, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Violations = 2
			if typ == "future.request" {
				cfg.Rate, cfg.Burst = 1, 1
			} else {
				cfg.MutationRate = 1
			}
			st := newMemoryStore()
			_, url, _ := testServer(t, cfg, st, nil)
			c := dial(t, url, nil)
			login(t, c, st.ticket(t, 2), "0.1.0", fixtureBuild)
			expectInitial(t, c, "2", "ok")
			send(t, c, typ, requestID, map[string]any{})
			assertCode(t, c, "UNKNOWN_TYPE")
			for range 2 {
				send(t, c, typ, requestID, map[string]any{})
				msg := assertCode(t, c, "RATE_LIMITED")
				var delay int
				if msg.Request != requestID || json.Unmarshal(msg.Payload["retry_after_ms"], &delay) != nil || delay <= 0 {
					t.Fatal("missing rate-limit correlation or retry time")
				}
			}
			expectClose(t, c, 4003)
		})
	}
}

func expect429(t *testing.T, url string) {
	t.Helper()
	c, res, err := websocket.Dial(t.Context(), url, &websocket.DialOptions{Subprotocols: []string{Subprotocol}})
	if c != nil {
		_ = c.CloseNow()
	}
	if err == nil || res == nil || res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") == "" {
		t.Fatal("admission limit did not reject with 429 and retry delay")
	}
}

type writeGate struct {
	first   atomic.Bool
	armed   atomic.Bool
	blocked chan struct{}
	release chan struct{}
}

type gateListener struct {
	net.Listener
	gate *writeGate
}

func (l gateListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil && l.gate.first.CompareAndSwap(false, true) {
		return gateConn{Conn: c, gate: l.gate}, nil
	}
	return c, err
}

type gateConn struct {
	net.Conn
	gate *writeGate
}

func (c gateConn) Write(data []byte) (int, error) {
	if c.gate.armed.CompareAndSwap(true, false) {
		close(c.gate.blocked)
		<-c.gate.release
	}
	return c.Conn.Write(data)
}

func TestSlowConsumerDoesNotBlockOtherSessions(t *testing.T) {
	gate := &writeGate{blocked: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(gate.release) })
	cfg := DefaultConfig()
	cfg.Queue = 3
	st := newMemoryStore()
	s, url, _ := testServer(t, cfg, st, func(l net.Listener) net.Listener { return gateListener{l, gate} })
	slow := dial(t, url, nil)
	login(t, slow, st.ticket(t, 1), "0.1.0", fixtureBuild)
	expectInitial(t, slow, "1", "ok")
	fast := dial(t, url, nil)
	login(t, fast, st.ticket(t, 2), "0.1.0", fixtureBuild)
	expectInitial(t, fast, "2", "ok")

	s.mu.Lock()
	slowPeer := s.characters[1]
	s.mu.Unlock()
	gate.armed.Store(true)
	send(t, slow, "state.snapshot.request", requestID, map[string]any{})
	waitSignal(t, gate.blocked, "slow write was not blocked")
	for range cfg.Queue + 1 {
		send(t, slow, "state.snapshot.request", requestID, map[string]any{})
	}
	waitSignal(t, slowPeer.closing, "full queue did not close slow peer")
	send(t, fast, "state.snapshot.request", requestID, map[string]any{})
	expect(t, fast, "state.snapshot")
	release.Do(func() { close(gate.release) })
	for range 2 {
		_, err := read(t, slow)
		if err != nil {
			if websocket.CloseStatus(err) != 1013 {
				t.Fatalf("expected slow-consumer close 1013: %v", err)
			}
			return
		}
	}
	t.Fatal("slow connection did not close")
}

func waitSignal(t *testing.T, ch <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal(failure)
	}
}

func TestUpgradeClearsHTTPDeadlines(t *testing.T) {
	st := newMemoryStore()
	logger := slog.New(slog.DiscardHandler)
	s := New(t.Context(), DefaultConfig(), st, logger)
	h := httptest.NewUnstartedServer(httpservice.NewHandler(t.Context(), logger, httpservice.Application{WS: s}))
	h.Config.ReadTimeout, h.Config.WriteTimeout = 150*time.Millisecond, 150*time.Millisecond
	h.Start()
	defer h.Close()
	c := dial(t, "ws"+strings.TrimPrefix(h.URL, "http")+"/v1/ws", nil)
	login(t, c, st.ticket(t, 1), "0.1.0", fixtureBuild)
	expectInitial(t, c, "1", "ok")
	time.Sleep(200 * time.Millisecond)
	send(t, c, "state.snapshot.request", requestID, map[string]any{})
	expect(t, c, "state.snapshot")
	_ = c.Close(websocket.StatusNormalClosure, "")
}

func TestShutdownDrainDeadline(t *testing.T) {
	s, url, _ := testServer(t, DefaultConfig(), newMemoryStore(), nil)
	c := dial(t, url, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()
	// The client does not read the close frame or answer the close handshake.
	if err := s.Shutdown(ctx); err != context.DeadlineExceeded {
		t.Fatalf("shutdown should force close at the drain deadline: %v", err)
	}
	_ = c.CloseNow()
	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("connection remained after forced drain: %v", err)
	}
}

func TestConfig(t *testing.T) {
	cfg, err := LoadConfig(func(string) string { return "" })
	if err != nil || cfg != DefaultConfig() || cfg.Handshakes != 10 || cfg.Pending != 3 || cfg.Rate != 20 ||
		cfg.Burst != 40 || cfg.MutationRate != 5 || cfg.Queue != 256 || cfg.AuthTimeout != 5*time.Second ||
		cfg.Heartbeat != 25*time.Second || cfg.PongTimeout != 15*time.Second || cfg.WriteTimeout != 10*time.Second {
		t.Fatal("unexpected WebSocket defaults")
	}
	values := map[string]string{"NEWMMO_WS_QUEUE_SIZE": "8", "NEWMMO_WS_PONG_TIMEOUT": "40ms"}
	cfg, err = LoadConfig(func(k string) string { return values[k] })
	if err != nil || cfg.Queue != 8 || cfg.PongTimeout != 40*time.Millisecond {
		t.Fatal("valid overrides were not applied")
	}
	values["NEWMMO_WS_QUEUE_SIZE"] = "untrusted-config-marker"
	_, err = LoadConfig(func(k string) string { return values[k] })
	if err == nil || strings.Contains(err.Error(), values["NEWMMO_WS_QUEUE_SIZE"]) {
		t.Fatal("bad config should be rejected without logging input")
	}
}
