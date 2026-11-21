package ws

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

const (
	closeAuth     websocket.StatusCode = 4001
	closeProtocol websocket.StatusCode = 4002
	closeRate     websocket.StatusCode = 4003
	closeContent  websocket.StatusCode = 4004
	closeReplaced websocket.StatusCode = 4005
)

type Store interface {
	ConsumeTicket(context.Context, string) (string, int64, error)
	LoadWSState(context.Context, string, int64) (auth.WSState, error)
}

type Server struct {
	lifetime context.Context
	cfg      Config
	store    Store
	logger   *slog.Logger

	mu         sync.Mutex
	stopping   bool
	addresses  map[netip.Addr]*admission
	peers      map[*peer]struct{}
	characters map[int64]*peer
	nextSweep  time.Time
	handlers   sync.WaitGroup
}

func New(lifetime context.Context, cfg Config, store Store, logger *slog.Logger) *Server {
	return &Server{
		lifetime: lifetime, cfg: cfg, store: store, logger: logger,
		addresses: make(map[netip.Addr]*admission), peers: make(map[*peer]struct{}),
		characters: make(map[int64]*peer),
	}
}

type admission struct {
	attempts []time.Time
	pending  int
}

func (s *Server) admit(ip netip.Addr) (*admission, int, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping || s.lifetime.Err() != nil {
		return nil, http.StatusServiceUnavailable, 0
	}

	now := time.Now()
	cutoff := now.Add(-time.Minute)
	if !now.Before(s.nextSweep) {
		for address, entry := range s.addresses {
			if entry.pending == 0 && (len(entry.attempts) == 0 || !entry.attempts[len(entry.attempts)-1].After(cutoff)) {
				delete(s.addresses, address)
			}
		}
		s.nextSweep = now.Add(time.Minute)
	}
	entry := s.addresses[ip]
	if entry == nil {
		entry = &admission{}
		s.addresses[ip] = entry
	}
	expired := 0
	for expired < len(entry.attempts) && !entry.attempts[expired].After(cutoff) {
		expired++
	}
	if expired > 0 {
		entry.attempts = entry.attempts[:copy(entry.attempts, entry.attempts[expired:])]
	}
	if len(entry.attempts) >= s.cfg.Handshakes {
		return nil, http.StatusTooManyRequests, entry.attempts[0].Add(time.Minute).Sub(now)
	}
	entry.attempts = append(entry.attempts, now)
	if entry.pending >= s.cfg.Pending {
		return nil, http.StatusTooManyRequests, s.cfg.AuthTimeout
	}
	entry.pending++
	s.handlers.Add(1)
	return entry, 0, 0
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	addr, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil || r.URL.RawQuery != "" || r.URL.ForceQuery {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	ip := addr.Addr().Unmap().WithZone("")
	if !ip.IsLoopback() && r.TLS == nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	entry, status, retry := s.admit(ip)
	if status != 0 {
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", strconv.FormatInt(int64((retry-1)/time.Second)+1, 10))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
				"code": "RATE_LIMITED", "message_key": "error.rate_limited",
				"retry_after_ms": int64((retry-1)/time.Millisecond) + 1,
			}})
			s.logger.Debug("websocket admission limited", "code", "RATE_LIMITED")
			return
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	defer s.handlers.Done()
	pending := true
	defer func() {
		if pending {
			s.releasePending(entry)
		}
	}()

	if !offersProtocol(r.Header) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	// Coder's negotiation compares offered tokens case-insensitively. At
	// this point the exact protocol is known to have been offered.
	r = r.Clone(r.Context())
	r.Header.Set("Sec-WebSocket-Protocol", Subprotocol)
	upgrade := &upgradeWriter{ResponseWriter: w}
	conn, err := websocket.Accept(upgrade, r, &websocket.AcceptOptions{
		Subprotocols: []string{Subprotocol}, CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		if upgrade.raw != nil {
			_ = upgrade.raw.Close()
		}
		s.logger.Debug("websocket upgrade rejected")
		return
	}
	conn.SetReadLimit(inputLimit)
	p := newPeer(context.WithoutCancel(s.lifetime), conn, upgrade.raw, s.cfg, s.logger)
	attached := s.attach(p)
	defer func() {
		p.finish()
		s.detach(p)
	}()
	if !attached {
		p.close(websocket.StatusGoingAway)
		return
	}

	// Expiring a Coder Read context would close the transport without
	// sending the protocol close code. The timer drives Close instead.
	authCtx, cancelAuth := context.WithTimeout(p.work, s.cfg.AuthTimeout)
	defer cancelAuth()
	timer := time.AfterFunc(s.cfg.AuthTimeout, func() { p.close(closeAuth) })
	defer timer.Stop()

	request, m, state, owner, id, ok := s.authenticate(authCtx, p)
	if !ok || !timer.Stop() {
		return
	}
	cancelAuth()
	p.id = sessionID()

	compat := compatibility(state, request)
	versionPayload := map[string]string{"content_build_id": state.BuildID, "compatibility": compat}
	if compat == "incompatible" {
		if p.write(p.encode("content.version", m.requestID, state.Revision, versionPayload)) {
			p.close(closeContent)
		}
		return
	}

	if !s.claim(p, id) {
		p.close(websocket.StatusGoingAway)
		return
	}
	s.releasePending(entry)
	pending = false

	p.startWriter()
	at := timestamp()
	if !p.enqueue("session.ready", m.requestID, state.Revision, map[string]any{
		"session_id": p.id, "character_id": state.ID, "server_time": at,
		"heartbeat_ms": s.cfg.Heartbeat.Milliseconds(), "content_build_id": state.BuildID,
	}, 0) ||
		!p.enqueue("content.version", m.requestID, state.Revision, versionPayload, 0) ||
		!p.enqueue("state.snapshot", m.requestID, state.Revision, snapshot(state, nil), 0) {
		return
	}

	p.startHeartbeat()
	s.commands(p, owner, id, state.Revision)
}

func offersProtocol(headers http.Header) bool {
	for _, value := range headers.Values("Sec-WebSocket-Protocol") {
		for _, protocol := range strings.Split(value, ",") {
			if strings.TrimSpace(protocol) == Subprotocol {
				return true
			}
		}
	}
	return false
}

type upgradeWriter struct {
	http.ResponseWriter
	raw      net.Conn
	rejected bool
}

func (w *upgradeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	raw, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err != nil {
		return nil, nil, err
	}
	w.raw = raw
	if err := raw.SetDeadline(time.Time{}); err != nil {
		return nil, nil, err
	}
	return raw, rw, nil
}

func (w *upgradeWriter) WriteHeader(status int) {
	w.rejected = status >= 400
	w.ResponseWriter.WriteHeader(status)
}

func (w *upgradeWriter) Write(data []byte) (int, error) {
	if w.rejected {
		// Library handshake errors can contain untrusted header values.
		return len(data), nil
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) releasePending(entry *admission) {
	s.mu.Lock()
	entry.pending--
	s.mu.Unlock()
}

func (s *Server) attach(p *peer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[p] = struct{}{}
	return !s.stopping && s.lifetime.Err() == nil
}

func (s *Server) claim(p *peer, character int64) bool {
	s.mu.Lock()
	if s.stopping || s.lifetime.Err() != nil {
		s.mu.Unlock()
		return false
	}
	old := s.characters[character]
	p.character = character
	s.characters[character] = p
	s.mu.Unlock()
	if old != nil {
		old.close(closeReplaced)
	}
	return true
}

func (s *Server) detach(p *peer) {
	s.mu.Lock()
	delete(s.peers, p)
	if s.characters[p.character] == p {
		delete(s.characters, p.character)
	}
	s.mu.Unlock()
}

// Shutdown stops admissions, closes all peers and bounds the overall drain.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.stopping = true
	peers := make([]*peer, 0, len(s.peers))
	for p := range s.peers {
		peers = append(peers, p)
	}
	s.mu.Unlock()
	for _, p := range peers {
		p.close(websocket.StatusGoingAway)
	}
	done := make(chan struct{})
	go func() {
		s.handlers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		for p := range s.peers {
			_ = p.raw.Close()
		}
		s.mu.Unlock()
		return ctx.Err()
	}
}

func (s *Server) authenticate(ctx context.Context, p *peer) (request authMessage, m inbound, state auth.WSState, owner string, id int64, ok bool) {
	m, err := p.read()
	if err != nil || m.typ != "session.auth" {
		if err == nil || errors.Is(err, errEnvelope) {
			p.close(closeAuth)
		}
		return
	}
	request, err = readAuth(m.payload)
	if errors.Is(err, errVersion) {
		p.close(closeProtocol)
		return
	}
	if err != nil {
		p.close(closeAuth)
		return
	}
	owner, id, err = s.store.ConsumeTicket(ctx, request.ticket)
	if errors.Is(err, auth.ErrInvalidTicket) {
		p.close(closeAuth)
		return
	}
	if err != nil {
		if ctx.Err() == nil {
			p.close(websocket.StatusInternalError)
		} else {
			p.close(closeAuth)
		}
		return
	}
	state, err = s.store.LoadWSState(ctx, owner, id)
	if err != nil || !validState(state) {
		if ctx.Err() == nil {
			p.close(websocket.StatusInternalError)
		} else {
			p.close(closeAuth)
		}
		return
	}
	ok = true
	return
}
