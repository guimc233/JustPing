package proxy

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guimc233/JustPing/shared/wsutil"
)

// TunnelPath is the WebSocket entrypoint that carries the same tunnels as the
// HTTP CONNECT handler.
//
// CONNECT is the natural transport, but a reverse proxy in front of the Host
// generally refuses the method before it ever reaches us: nginx answers 405 Not
// Allowed unless it was built with the third-party proxy_connect module. Reverse
// proxies do forward WebSocket upgrades, so the same tunnels are also served over
// this path, on the same port and the same TLS listener, with no extra port and
// no reverse-proxy configuration.
const TunnelPath = "/api/proxy/tunnel"

// tunnelHandshakeTimeout bounds the time a client has to name its target.
const tunnelHandshakeTimeout = 10 * time.Second

var tunnelUpgrader = websocket.Upgrader{
	ReadBufferSize:  16 * 1024,
	WriteBufferSize: 16 * 1024,
	// Clients are non-browser tools authenticating with the proxy credential, so
	// there is no browser origin to check.
	CheckOrigin: func(*http.Request) bool { return true },
}

// tunnelRequest is the first message a tunnel client sends: the target to dial.
type tunnelRequest struct {
	Target string `json:"target"`
}

// tunnelReply reports whether the probe accepted the target.
type tunnelReply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// IsTunnelRequest reports whether r is an upgrade request for the tunnel path.
func IsTunnelRequest(r *http.Request) bool {
	return r.URL != nil && r.URL.Path == TunnelPath &&
		websocket.IsWebSocketUpgrade(r)
}

// HandleTunnel serves the process-wide proxy over the WebSocket tunnel endpoint.
func HandleTunnel(w http.ResponseWriter, r *http.Request) {
	Default.HandleTunnel(w, r)
}

// HandleTunnel authenticates the credential, asks the probe to dial the requested
// target, and then relays bytes until either side closes or the credential
// expires.
func (s *Service) HandleTunnel(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := tunnelCredential(r)
	if !ok {
		rejectProxyAuth(w)
		return
	}
	sess, err := s.sessions.Authenticate(user, pass)
	if err != nil {
		rejectProxyAuth(w)
		return
	}

	conn, err := tunnelUpgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already wrote a response.
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(tunnelHandshakeTimeout))
	var req tunnelRequest
	if err := conn.ReadJSON(&req); err != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	host, port, err := ParseConnectTarget(req.Target)
	if err != nil {
		_ = conn.WriteJSON(tunnelReply{Error: "invalid target"})
		return
	}
	if !online(s, sess.AgentID) {
		_ = conn.WriteJSON(tunnelReply{Error: "probe offline"})
		return
	}

	t, err := s.openTunnel(sess, host, port)
	if err != nil {
		_ = conn.WriteJSON(tunnelReply{Error: err.Error()})
		return
	}
	if err := conn.WriteJSON(tunnelReply{OK: true}); err != nil {
		s.finish(t.id, "handshake failed", true)
		return
	}

	// The tunnel is established. From here the endpoint is a byte pipe; the
	// client's WebSocket pings keep it from looking idle to the reverse proxy.
	stream := wsutil.NewConn(conn)
	s.pump(stream, stream, t, sess.ExpiresAt)
}

// tunnelCredential reads the proxy credential from either header so the endpoint
// works with clients that set Authorization rather than Proxy-Authorization.
func tunnelCredential(r *http.Request) (string, string, bool) {
	if user, pass, ok := ParseBasicAuth(r.Header.Get("Proxy-Authorization")); ok {
		return user, pass, true
	}
	return ParseBasicAuth(r.Header.Get("Authorization"))
}
