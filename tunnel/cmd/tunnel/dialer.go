package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guimc233/JustPing/shared/wsutil"
)

// tunnelPath mirrors host/internal/proxy.TunnelPath.
const tunnelPath = "/api/proxy/tunnel"

// handshakeTimeout bounds the WebSocket dial and the target-open round trip.
const handshakeTimeout = 15 * time.Second

// Keepalive for established tunnels. A reverse proxy in front of the Host closes
// a WebSocket it considers idle (nginx's proxy_read_timeout defaults to 60s), so
// the client pings on an interval well under that. The rolling read deadline then
// doubles as dead-peer detection: the Host pongs automatically, and if pongs stop
// arriving the relay ends instead of hanging.
const (
	keepAliveInterval = 25 * time.Second
	keepAliveTimeout  = 90 * time.Second
)

// ErrUnauthorized means the proxy credential was rejected, usually because it
// expired (the Host issues them with a 10-minute TTL).
var ErrUnauthorized = errors.New("credential rejected or expired; issue a new one in the web UI")

// The first frame the Host expects, and the reply it sends back.
type tunnelRequest struct {
	Target string `json:"target"`
}

type tunnelReply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Tunnel is one open WebSocket tunnel to a target address.
type Tunnel struct {
	ID      int64
	Target  string
	Started time.Time
	Tx      atomic.Int64
	Rx      atomic.Int64

	// conn closes the tunnel; kept here so shutdown can release every tunnel.
	conn io.Closer

	done      chan struct{}
	closeOnce sync.Once

	mu     sync.Mutex
	closed bool
	err    error
}

// close marks the tunnel closed and releases anything waiting on it.
func (t *Tunnel) close() {
	t.closeOnce.Do(func() { close(t.done) })
}

// Done reports whether the tunnel has closed and why.
func (t *Tunnel) Done() (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed, t.err
}

// Bytes reports bytes sent to and received from the target.
func (t *Tunnel) Bytes() (int64, int64) { return t.Tx.Load(), t.Rx.Load() }

// Dialer opens tunnels through the Host and one probe.
type Dialer struct {
	wsURL    string
	auth     string
	insecure bool

	mu      sync.Mutex
	tunnels map[int64]*Tunnel
	nextID  int64

	clientOnce sync.Once
	client     *http.Client
}

// NewDialer builds a dialer for serverURL (the JustPing Host URL) using the
// username and password shown when the proxy credential was issued.
func NewDialer(serverURL, username, password string, insecure bool) (*Dialer, error) {
	wsURL, err := buildTunnelURL(serverURL)
	if err != nil {
		return nil, err
	}
	return &Dialer{
		wsURL:    wsURL,
		auth:     "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password)),
		insecure: insecure,
		tunnels:  make(map[int64]*Tunnel),
	}, nil
}

// URL reports the tunnel endpoint the dialer connects to.
func (d *Dialer) URL() string { return d.wsURL }

// buildTunnelURL maps a Host URL onto the wss:// tunnel endpoint.
func buildTunnelURL(serverURL string) (string, error) {
	raw := strings.TrimSpace(serverURL)
	if raw == "" {
		return "", errors.New("server URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid server URL: %w", err)
	}
	if u.Host == "" {
		return "", errors.New("server URL has no host")
	}

	scheme := "wss"
	switch u.Scheme {
	case "http", "ws":
		scheme = "ws"
	case "https", "wss":
		scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported server URL scheme %q", u.Scheme)
	}
	return fmt.Sprintf("%s://%s%s", scheme, u.Host, tunnelPath), nil
}

// Dial opens a tunnel to addr (host:port) and returns it as a net.Conn.
func (d *Dialer) Dial(addr string) (net.Conn, *Tunnel, error) {
	host, port, err := splitTarget(addr)
	if err != nil {
		return nil, nil, err
	}
	target := net.JoinHostPort(host, port)
	return d.dialTarget(target)
}

func (d *Dialer) dialTarget(target string) (net.Conn, *Tunnel, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: handshakeTimeout,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: d.insecure},
	}

	header := http.Header{}
	header.Set("Authorization", d.auth)

	conn, resp, err := dialer.Dial(d.wsURL, header)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusProxyAuthRequired {
			return nil, nil, ErrUnauthorized
		}
		if resp != nil {
			return nil, nil, fmt.Errorf("tunnel handshake failed (HTTP %d): %w", resp.StatusCode, err)
		}
		return nil, nil, fmt.Errorf("cannot reach %s: %w", d.wsURL, err)
	}

	t := &Tunnel{
		ID:      atomic.AddInt64(&d.nextID, 1),
		Target:  target,
		Started: time.Now(),
		done:    make(chan struct{}),
	}

	tracked := &countedConn{Conn: wsutil.NewConn(conn), t: t, onClose: func(err error) {
		t.mu.Lock()
		t.closed = true
		if t.err == nil {
			t.err = err
		}
		t.mu.Unlock()
		t.close()
		d.forget(t.ID)
	}}

	// Ask the probe to dial the target before handing the stream over.
	if err := writeJSON(conn, tunnelRequest{Target: target}); err != nil {
		_ = tracked.Close()
		return nil, nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	var reply tunnelReply
	if err := conn.ReadJSON(&reply); err != nil {
		_ = tracked.Close()
		return nil, nil, fmt.Errorf("no reply from %s: %w", target, err)
	}
	_ = conn.SetReadDeadline(time.Time{})

	if !reply.OK {
		_ = tracked.Close()
		msg := reply.Error
		if msg == "" {
			msg = "probe refused the target"
		}
		return nil, nil, fmt.Errorf("%s: %s", target, msg)
	}

	// The handshake is done: switch from a fixed deadline to the rolling
	// keepalive deadline that doubles as dead-peer detection.
	_ = conn.SetReadDeadline(time.Now().Add(keepAliveTimeout))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(keepAliveTimeout))
		return nil
	})
	go keepAlive(conn, t)

	t.conn = tracked
	d.mu.Lock()
	d.tunnels[t.ID] = t
	d.mu.Unlock()

	return tracked, t, nil
}

// keepAlive pings the Host so intermediaries do not treat the tunnel as idle.
// WriteControl may run concurrently with the relay's writes; plain writes may not.
func keepAlive(ws *websocket.Conn, t *Tunnel) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				return
			}
		}
	}
}

func (d *Dialer) forget(id int64) {
	d.mu.Lock()
	delete(d.tunnels, id)
	d.mu.Unlock()
}

// Tunnels returns a snapshot of the open tunnels, newest last.
func (d *Dialer) Tunnels() []*Tunnel {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]*Tunnel, 0, len(d.tunnels))
	for _, t := range d.tunnels {
		out = append(out, t)
	}
	// Stable order by id.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].ID > out[j].ID; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// CloseAll closes every tunnel the dialer opened.
func (d *Dialer) CloseAll() {
	for _, t := range d.Tunnels() {
		t.mu.Lock()
		t.closed = true
		conn := t.conn
		t.mu.Unlock()
		t.close()
		if conn != nil {
			_ = conn.Close()
		}
	}
}

// countedConn counts bytes in both directions and reports the close reason.
type countedConn struct {
	net.Conn
	t       *Tunnel
	onClose func(error)
	once    sync.Once
}

func (c *countedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.t.Rx.Add(int64(n))
	}
	return n, err
}

func (c *countedConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if n > 0 {
		c.t.Tx.Add(int64(n))
	}
	return n, err
}

func (c *countedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() {
		if c.onClose != nil {
			c.onClose(closeReason(err))
		}
	})
	return err
}

func closeReason(err error) error {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func writeJSON(conn *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(handshakeTimeout))
	return conn.WriteMessage(websocket.TextMessage, b)
}

// splitTarget accepts "host:port" or a bare host, defaulting to 443.
func splitTarget(addr string) (string, string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", "", errors.New("empty target")
	}
	if !strings.Contains(addr, ":") {
		return addr, "443", nil
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("invalid target %q: %w", addr, err)
	}
	if host == "" || port == "" {
		return "", "", fmt.Errorf("invalid target %q", addr)
	}
	return host, port, nil
}
