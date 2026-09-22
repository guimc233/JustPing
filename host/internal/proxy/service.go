package proxy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
)

const (
	maxTunnelsPerSession = 64
	proxyChunkSize       = 16 * 1024
	openTimeout          = 15 * time.Second
	inboundQueue         = 32
)

// inboundWait is how long one probe chunk may wait for a slow client before the tunnel closes.
var inboundWait = 5 * time.Second

// Bridge is the host path to a connected probe.
type Bridge interface {
	Online(agentID string) bool
	Send(agentID string, msgType protocol.MessageType, payload any) error
}

type tunnel struct {
	id        string
	agentID   string
	sessionID string
	opened    chan protocol.ProxyOpenResultPayload
	inbound   chan []byte
	done      chan struct{}
	once      sync.Once
}

func newTunnel(agentID, sessionID string) (*tunnel, error) {
	id, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	return &tunnel{
		id:        id,
		agentID:   agentID,
		sessionID: sessionID,
		opened:    make(chan protocol.ProxyOpenResultPayload, 1),
		inbound:   make(chan []byte, inboundQueue),
		done:      make(chan struct{}),
	}, nil
}

func (t *tunnel) close() {
	t.once.Do(func() { close(t.done) })
}

// Service authenticates CONNECT requests and relays them through one probe.
type Service struct {
	sessions *Store
	mu       sync.Mutex
	bridge   Bridge
	tunnels  map[string]*tunnel
}

func NewService() *Service {
	return &Service{
		sessions: NewStore(),
		tunnels:  make(map[string]*tunnel),
	}
}

// Default is the process-wide proxy used by the admin API and the CONNECT handler.
var Default = NewService()

// Sessions exposes credential issue, list, and revoke.
func (s *Service) Sessions() *Store { return s.sessions }

// SetBridge attaches the live probe hub. Safe to call once before serving.
func (s *Service) SetBridge(b Bridge) {
	s.mu.Lock()
	s.bridge = b
	s.mu.Unlock()
}

// Revoke invalidates a credential and drops tunnels opened with it.
func (s *Service) Revoke(id string) bool {
	if !s.sessions.Revoke(id) {
		return false
	}
	s.dropSession(id)
	return true
}

// HandleAgent delivers probe replies for proxy tunnels.
func (s *Service) HandleAgent(agentID string, env protocol.Envelope) {
	switch env.Type {
	case protocol.TypeProxyOpenResult:
		var res protocol.ProxyOpenResultPayload
		if decodePayload(env.Payload, &res) != nil || res.TunnelID == "" {
			return
		}
		t := s.tunnel(res.TunnelID)
		if t == nil || t.agentID != agentID {
			return
		}
		select {
		case t.opened <- res:
		default:
		}
	case protocol.TypeProxyData:
		var msg protocol.ProxyDataPayload
		if decodePayload(env.Payload, &msg) != nil || msg.TunnelID == "" || msg.Data == "" {
			return
		}
		t := s.tunnel(msg.TunnelID)
		if t == nil || t.agentID != agentID {
			return
		}
		raw, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil || len(raw) == 0 || len(raw) > proxyChunkSize*4 {
			s.finish(t.id, "bad payload", true)
			return
		}
		timer := time.NewTimer(inboundWait)
		select {
		case <-t.done:
			timer.Stop()
		case t.inbound <- raw:
			timer.Stop()
		case <-timer.C:
			s.finish(t.id, "slow consumer", true)
		}
	case protocol.TypeProxyClose:
		var msg protocol.ProxyClosePayload
		if decodePayload(env.Payload, &msg) != nil || msg.TunnelID == "" {
			return
		}
		t := s.tunnel(msg.TunnelID)
		if t == nil || t.agentID != agentID {
			return
		}
		s.finish(t.id, "agent closed", false)
	}
}

// DropAgent closes every tunnel for a probe that just disconnected.
func (s *Service) DropAgent(agentID string) {
	s.mu.Lock()
	var ids []string
	for id, t := range s.tunnels {
		if t.agentID == agentID {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		s.finish(id, "agent disconnected", false)
	}
}

func (s *Service) dropSession(sessionID string) {
	s.mu.Lock()
	var ids []string
	for id, t := range s.tunnels {
		if t.sessionID == sessionID {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		s.finish(id, "revoked", true)
	}
}

func (s *Service) tunnel(id string) *tunnel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tunnels[id]
}

func (s *Service) addTunnel(t *tunnel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, cur := range s.tunnels {
		if cur.sessionID == t.sessionID {
			n++
		}
	}
	if n >= maxTunnelsPerSession {
		return errors.New("too many tunnels")
	}
	s.tunnels[t.id] = t
	return nil
}

func (s *Service) finish(id, reason string, notify bool) {
	s.mu.Lock()
	t := s.tunnels[id]
	if t != nil {
		delete(s.tunnels, id)
	}
	bridge := s.bridge
	s.mu.Unlock()
	if t == nil {
		return
	}
	t.close()
	if notify && bridge != nil {
		_ = bridge.Send(t.agentID, protocol.TypeProxyClose, protocol.ProxyClosePayload{
			TunnelID: t.id,
			Reason:   reason,
		})
	}
}

func (s *Service) bridgeSend(agentID string, msgType protocol.MessageType, payload any) error {
	s.mu.Lock()
	bridge := s.bridge
	s.mu.Unlock()
	if bridge == nil || !bridge.Online(agentID) {
		return errors.New("agent offline")
	}
	return bridge.Send(agentID, msgType, payload)
}

// openTunnel asks the probe to dial and waits until the dial succeeds or fails.
func (s *Service) openTunnel(sess *Session, host string, port int) (*tunnel, error) {
	t, err := newTunnel(sess.AgentID, sess.ID)
	if err != nil {
		return nil, err
	}
	if err := s.addTunnel(t); err != nil {
		return nil, err
	}
	if err := s.bridgeSend(sess.AgentID, protocol.TypeProxyOpen, protocol.ProxyOpenPayload{
		TunnelID: t.id,
		Host:     host,
		Port:     port,
	}); err != nil {
		s.finish(t.id, "agent offline", false)
		return nil, err
	}
	timer := time.NewTimer(openTimeout)
	defer timer.Stop()
	select {
	case res := <-t.opened:
		if !res.OK {
			s.finish(t.id, "dial failed", false)
			if res.Error == "" {
				res.Error = "exit dial failed"
			}
			return nil, errors.New(res.Error)
		}
		return t, nil
	case <-timer.C:
		s.finish(t.id, "timeout", true)
		return nil, errors.New("exit timed out")
	case <-t.done:
		return nil, errors.New("tunnel closed")
	}
}

func online(s *Service, agentID string) bool {
	s.mu.Lock()
	bridge := s.bridge
	s.mu.Unlock()
	return bridge != nil && bridge.Online(agentID)
}

func (s *Service) pump(client net.Conn, reader io.Reader, t *tunnel, deadline time.Time) {
	defer client.Close()
	ctxDone := time.After(time.Until(deadline))
	stop := make(chan struct{})
	// Do not close the client on t.done. Bytes already queued still have to be written.
	go func() {
		select {
		case <-ctxDone:
			_ = client.Close()
		case <-stop:
		}
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, proxyChunkSize)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				payload := protocol.ProxyDataPayload{
					TunnelID: t.id,
					Data:     base64.StdEncoding.EncodeToString(buf[:n]),
				}
				if sendErr := s.bridgeSend(t.agentID, protocol.TypeProxyData, payload); sendErr != nil {
					s.finish(t.id, "send failed", false)
					return
				}
			}
			if err != nil {
				s.finish(t.id, "client closed", true)
				return
			}
		}
	}()
	s.writeToClient(client, t)
	_ = client.Close()
	wg.Wait()
	close(stop)
}

func (s *Service) writeToClient(client net.Conn, t *tunnel) {
	writeChunk := func(chunk []byte) bool {
		_, err := client.Write(chunk)
		if err != nil {
			s.finish(t.id, "client write failed", true)
		}
		return err == nil
	}
	for {
		select {
		case chunk, ok := <-t.inbound:
			if !ok || !writeChunk(chunk) {
				return
			}
		case <-t.done:
			for {
				select {
				case chunk, ok := <-t.inbound:
					if !ok || !writeChunk(chunk) {
						return
					}
				default:
					return
				}
			}
		}
	}
}

func decodePayload(payload any, dest any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

// HandleConnect is the HTTP CONNECT entrypoint. w is written only before hijack.
func (s *Service) HandleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "connect required", http.StatusMethodNotAllowed)
		return
	}
	user, pass, ok := ParseBasicAuth(r.Header.Get("Proxy-Authorization"))
	if !ok {
		rejectProxyAuth(w)
		return
	}
	sess, err := s.sessions.Authenticate(user, pass)
	if err != nil {
		rejectProxyAuth(w)
		return
	}
	target := r.Host
	if r.URL != nil && r.URL.Host != "" {
		target = r.URL.Host
	}
	host, port, err := ParseConnectTarget(target)
	if err != nil {
		http.Error(w, "invalid connect target", http.StatusBadRequest)
		return
	}
	if !online(s, sess.AgentID) {
		http.Error(w, "probe offline", http.StatusBadGateway)
		return
	}
	t, err := s.openTunnel(sess, host, port)
	if err != nil {
		if err.Error() == "too many tunnels" {
			http.Error(w, "too many tunnels", http.StatusTooManyRequests)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		s.finish(t.id, "hijack unsupported", true)
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		s.finish(t.id, "hijack failed", true)
		return
	}
	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		s.finish(t.id, "establish failed", true)
		_ = conn.Close()
		return
	}
	var reader io.Reader = conn
	if buf != nil && buf.Reader != nil && buf.Reader.Buffered() > 0 {
		reader = io.MultiReader(buf.Reader, conn)
	}
	s.pump(conn, reader, t, sess.ExpiresAt)
}

func rejectProxyAuth(w http.ResponseWriter) {
	w.Header().Set("Proxy-Authenticate", `Basic realm="JustPing"`)
	http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
}

// HandleConnect serves CONNECT on the process-wide proxy.
func HandleConnect(w http.ResponseWriter, r *http.Request) {
	Default.HandleConnect(w, r)
}
