package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guimc233/JustPing/shared/protocol"
)

func (c *Client) connectionLoop(ctx context.Context) {
	backoff := 2 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		err := c.connectAndServe(ctx, &backoff)
		if err != nil {
			log.Printf("[Agent WS] Disconnected (%v). Retrying in %v...\n", err, backoff)
		}

		select {
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		}
	}
}

func (c *Client) connectAndServe(ctx context.Context, backoff *time.Duration) error {
	wsURL, err := c.buildWSURL()
	if err != nil {
		return fmt.Errorf("invalid server url: %w", err)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: false},
	}

	conn, resp, err := dialer.Dial(wsURL, http.Header{})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil {
			return fmt.Errorf("dial failed status %d: %w", resp.StatusCode, err)
		}
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.online = false
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.online = false
		_ = conn.Close()
		c.conn = nil
		c.mu.Unlock()
	}()

	hostname, _ := os.Hostname()
	regReq := protocol.RegisterRequest{
		Token:    c.cfg.Token,
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  c.cfg.Version,
	}

	if err := c.sendEnvelope(protocol.TypeRegisterRequest, regReq); err != nil {
		return err
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var regEnv protocol.Envelope
	if err := conn.ReadJSON(&regEnv); err != nil {
		return err
	}

	rawPayload, _ := json.Marshal(regEnv.Payload)
	var regResp protocol.RegisterResponse
	if err := json.Unmarshal(rawPayload, &regResp); err != nil || !regResp.Success {
		return fmt.Errorf("registration failed: %s", regResp.Message)
	}

	c.mu.Lock()
	c.agentID = regResp.AgentID
	c.online = true
	c.mu.Unlock()

	*backoff = 2 * time.Second // Reset backoff on successful registration
	log.Printf("[Agent WS] Registered with Host (ID: %s)\n", c.agentID)
	c.flushQueuedReports()

	heartbeatStop := make(chan struct{})
	defer close(heartbeatStop)
	go c.heartbeatLoop(heartbeatStop)

	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		var env protocol.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		if env.Type == protocol.TypeTargetSync {
			raw, _ := json.Marshal(env.Payload)
			var syncPayload protocol.TargetSyncPayload
			if err := json.Unmarshal(raw, &syncPayload); err == nil {
				c.pinger.UpdateTargets(syncPayload.Targets)
				if c.syncHook != nil {
					c.syncHook(syncPayload.Targets)
				}
			}
		}
	}
}

func (c *Client) heartbeatLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			c.mu.Lock()
			aid := c.agentID
			c.mu.Unlock()
			hb := protocol.HeartbeatPayload{AgentID: aid}
			if err := c.sendEnvelope(protocol.TypeHeartbeat, hb); err != nil {
				return
			}
		}
	}
}
