package client

import (
	"errors"
	"log"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
)

// QueueReport enqueues or sends ping results
func (c *Client) QueueReport(report protocol.PingReportPayload) {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()

	c.mu.Lock()
	conn := c.conn
	isOnline := c.online && conn != nil
	agentID := c.agentID
	c.mu.Unlock()

	if isOnline {
		report.AgentID = agentID
		if err := c.sendEnvelope(protocol.TypePingReport, report); err == nil {
			return
		}
	}

	if len(c.reportQueue) >= 500 {
		c.reportQueue = c.reportQueue[1:]
	}
	c.reportQueue = append(c.reportQueue, report)
}

// SendTracerouteReport sends a traceroute report immediately to host if online
func (c *Client) SendTracerouteReport(report protocol.TracerouteReportPayload) {
	c.mu.Lock()
	conn := c.conn
	isOnline := c.online && conn != nil
	agentID := c.agentID
	c.mu.Unlock()

	if isOnline {
		report.AgentID = agentID
		_ = c.sendEnvelope(protocol.TypeTracerouteReport, report)
	}
}

func (c *Client) flushQueuedReports() {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()

	if len(c.reportQueue) == 0 {
		return
	}

	log.Printf("[Agent] Flushing %d cached offline reports to host...\n", len(c.reportQueue))
	remaining := make([]protocol.PingReportPayload, 0, len(c.reportQueue))
	for _, report := range c.reportQueue {
		report.AgentID = c.agentID
		if err := c.sendEnvelope(protocol.TypePingReport, report); err != nil {
			remaining = append(remaining, report)
		}
	}
	c.reportQueue = remaining
}

func (c *Client) sendEnvelope(msgType protocol.MessageType, payload any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return errors.New("websocket connection is nil")
	}

	env := protocol.Envelope{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(env)
}
