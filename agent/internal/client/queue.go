package client

import (
	"log"

	"github.com/guimc233/JustPing/shared/protocol"
)

// QueueReport enqueues or immediately sends ping results
func (c *Client) QueueReport(report protocol.PingReportPayload) {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()

	c.mu.Lock()
	isOnline := c.online && c.conn != nil
	c.mu.Unlock()

	if isOnline {
		if err := c.sendEnvelope(protocol.TypePingReport, report); err == nil {
			return
		}
	}

	if len(c.reportQueue) >= 500 {
		c.reportQueue = c.reportQueue[1:]
	}
	c.reportQueue = append(c.reportQueue, report)
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
		return nil
	}

	env := protocol.Envelope{
		Type:      msgType,
		Timestamp: 0,
		Payload:   payload,
	}
	return c.conn.WriteJSON(env)
}
