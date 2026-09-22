package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/guimc233/JustPing/agent/internal/pinger"
	"github.com/guimc233/JustPing/shared/protocol"
)

type Config struct {
	ServerURL string
	Token     string
	Version   string
}

type Client struct {
	cfg     Config
	pinger  *pinger.Pinger
	conn    *websocket.Conn
	mu      sync.Mutex
	online  bool
	agentID string

	reportQueue []protocol.PingReportPayload
	queueMu     sync.Mutex
	stopCh      chan struct{}
	syncHook    func([]protocol.TargetConfig)
	updateHook  func()
	proxy       proxyManager
}

func NewClient(cfg Config, p *pinger.Pinger, syncHook func([]protocol.TargetConfig)) *Client {
	c := &Client{
		cfg:         cfg,
		pinger:      p,
		syncHook:    syncHook,
		reportQueue: make([]protocol.PingReportPayload, 0, 500),
		stopCh:      make(chan struct{}),
	}
	c.proxy.init(c.sendEnvelope)
	return c
}

// SetUpdateHook registers the callback invoked when the Host asks this probe to
// check for updates. It must be called before Start to avoid racing the read loop.
func (c *Client) SetUpdateHook(hook func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateHook = hook
}

func (c *Client) updateHandler() func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.updateHook
}

// SendUpdateResult reports the outcome of a Host-triggered update check.
// It is best-effort: the connection may already be gone.
func (c *Client) SendUpdateResult(res protocol.UpdateResultPayload) {
	c.mu.Lock()
	res.AgentID = c.agentID
	c.mu.Unlock()

	_ = c.sendEnvelope(protocol.TypeUpdateResult, res)
}

func (c *Client) Start(ctx context.Context) {
	go c.connectionLoop(ctx)
}

func (c *Client) Stop() {
	close(c.stopCh)
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.mu.Unlock()
}

func (c *Client) buildWSURL() (string, error) {
	raw := strings.TrimRight(c.cfg.ServerURL, "/")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw // Secure HTTPS/WSS by default
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	scheme := "wss"
	if u.Scheme == "http" || u.Scheme == "ws" {
		scheme = "ws"
	}

	wsPath := "/api/agent/ws"
	return fmt.Sprintf("%s://%s%s", scheme, u.Host, wsPath), nil
}
