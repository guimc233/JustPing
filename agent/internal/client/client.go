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
}

func NewClient(cfg Config, p *pinger.Pinger) *Client {
	return &Client{
		cfg:         cfg,
		pinger:      p,
		reportQueue: make([]protocol.PingReportPayload, 0, 500),
		stopCh:      make(chan struct{}),
	}
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
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") &&
		!strings.HasPrefix(raw, "ws://") && !strings.HasPrefix(raw, "wss://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	scheme := "ws"
	if u.Scheme == "https" || u.Scheme == "wss" {
		scheme = "wss"
	}

	wsPath := "/api/agent/ws"
	return fmt.Sprintf("%s://%s%s", scheme, u.Host, wsPath), nil
}
