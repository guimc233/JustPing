package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
)

const (
	proxyChunkSize    = 16 * 1024
	proxyWriteQueue   = 32
	proxyWriteTimeout = 10 * time.Second
)

type proxyTunnel struct {
	mu     sync.Mutex
	conn   net.Conn
	cancel context.CancelFunc
	writes chan []byte
	closed bool
}

func (t *proxyTunnel) close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	close(t.writes)
	cancel := t.cancel
	conn := t.conn
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func (t *proxyTunnel) setConn(conn net.Conn) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return false
	}
	t.conn = conn
	return true
}

func (t *proxyTunnel) enqueue(data []byte) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return false
	}
	select {
	case t.writes <- data:
		return true
	default:
		return false
	}
}

type proxyManager struct {
	send func(protocol.MessageType, any) error
	dial func(context.Context, string) (net.Conn, error)
	mu   sync.Mutex
	tabs map[string]*proxyTunnel
}

func (m *proxyManager) init(send func(protocol.MessageType, any) error) {
	m.send = send
	m.dial = func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
	m.tabs = make(map[string]*proxyTunnel)
}

func (c *Client) handleProxyMessage(env protocol.Envelope) bool {
	switch env.Type {
	case protocol.TypeProxyOpen:
		var req protocol.ProxyOpenPayload
		if decodeProxy(env.Payload, &req) != nil || req.TunnelID == "" {
			return true
		}
		c.proxy.open(req)
		return true
	case protocol.TypeProxyData:
		var msg protocol.ProxyDataPayload
		if decodeProxy(env.Payload, &msg) != nil || msg.TunnelID == "" || msg.Data == "" {
			return true
		}
		raw, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil || len(raw) == 0 || len(raw) > proxyChunkSize*4 {
			c.proxy.drop(msg.TunnelID, true)
			return true
		}
		c.proxy.write(msg.TunnelID, raw)
		return true
	case protocol.TypeProxyClose:
		var msg protocol.ProxyClosePayload
		if decodeProxy(env.Payload, &msg) != nil || msg.TunnelID == "" {
			return true
		}
		c.proxy.drop(msg.TunnelID, false)
		return true
	default:
		return false
	}
}

func (m *proxyManager) open(req protocol.ProxyOpenPayload) {
	if req.Port < 1 || req.Port > 65535 || req.Host == "" {
		_ = m.send(protocol.TypeProxyOpenResult, protocol.ProxyOpenResultPayload{
			TunnelID: req.TunnelID,
			OK:       false,
			Error:    "invalid target",
		})
		return
	}

	m.mu.Lock()
	if m.tabs == nil {
		m.tabs = make(map[string]*proxyTunnel)
	}
	if _, exists := m.tabs[req.TunnelID]; exists {
		m.mu.Unlock()
		_ = m.send(protocol.TypeProxyOpenResult, protocol.ProxyOpenResultPayload{
			TunnelID: req.TunnelID,
			OK:       false,
			Error:    "tunnel already open",
		})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	tab := &proxyTunnel{
		cancel: cancel,
		writes: make(chan []byte, proxyWriteQueue),
	}
	m.tabs[req.TunnelID] = tab
	m.mu.Unlock()

	go func() {
		addr := net.JoinHostPort(req.Host, itoa(req.Port))
		conn, err := m.dial(ctx, addr)
		if err != nil {
			m.dropIf(req.TunnelID, tab, false)
			_ = m.send(protocol.TypeProxyOpenResult, protocol.ProxyOpenResultPayload{
				TunnelID: req.TunnelID,
				OK:       false,
				Error:    trimErr(err),
			})
			return
		}
		if !tab.setConn(conn) {
			_ = conn.Close()
			return
		}
		go m.writeLoop(req.TunnelID, tab, conn)
		if err := m.send(protocol.TypeProxyOpenResult, protocol.ProxyOpenResultPayload{
			TunnelID: req.TunnelID,
			OK:       true,
		}); err != nil {
			m.dropIf(req.TunnelID, tab, false)
			return
		}
		m.readLoop(req.TunnelID, tab, conn)
	}()
}

func (m *proxyManager) readLoop(id string, tab *proxyTunnel, conn net.Conn) {
	buf := make([]byte, proxyChunkSize)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			payload := protocol.ProxyDataPayload{
				TunnelID: id,
				Data:     base64.StdEncoding.EncodeToString(buf[:n]),
			}
			if sendErr := m.send(protocol.TypeProxyData, payload); sendErr != nil {
				m.dropIf(id, tab, false)
				return
			}
		}
		if err != nil {
			m.dropIf(id, tab, true)
			return
		}
	}
}

func (m *proxyManager) writeLoop(id string, tab *proxyTunnel, conn net.Conn) {
	for data := range tab.writes {
		_ = conn.SetWriteDeadline(time.Now().Add(proxyWriteTimeout))
		if _, err := conn.Write(data); err != nil {
			m.dropIf(id, tab, true)
			return
		}
	}
}

func (m *proxyManager) write(id string, data []byte) {
	m.mu.Lock()
	tab := m.tabs[id]
	m.mu.Unlock()
	if tab == nil {
		return
	}
	if !tab.enqueue(data) {
		m.dropIf(id, tab, true)
	}
}

func (m *proxyManager) drop(id string, notify bool) {
	m.dropIf(id, nil, notify)
}

func (m *proxyManager) dropIf(id string, expect *proxyTunnel, notify bool) {
	m.mu.Lock()
	tab := m.tabs[id]
	if tab == nil || (expect != nil && tab != expect) {
		m.mu.Unlock()
		return
	}
	delete(m.tabs, id)
	m.mu.Unlock()
	tab.close()
	if notify {
		_ = m.send(protocol.TypeProxyClose, protocol.ProxyClosePayload{TunnelID: id})
	}
}

func (m *proxyManager) dropAll() {
	m.mu.Lock()
	tabs := m.tabs
	m.tabs = make(map[string]*proxyTunnel)
	m.mu.Unlock()
	for _, tab := range tabs {
		tab.close()
	}
}

func decodeProxy(payload any, dest any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func trimErr(err error) string {
	msg := err.Error()
	if len(msg) > 180 {
		return msg[:180]
	}
	return msg
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [6]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
