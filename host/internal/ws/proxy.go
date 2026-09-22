package ws

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
)

// Online reports whether the probe has a live websocket.
func (h *Hub) Online(agentID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.agents[agentID]
	return ok
}

// Send delivers one envelope to a probe, waiting briefly for queue space.
func (h *Hub) Send(agentID string, msgType protocol.MessageType, payload any) error {
	env := protocol.Envelope{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	h.mu.RLock()
	ac, ok := h.agents[agentID]
	h.mu.RUnlock()
	if !ok {
		return errors.New("agent offline")
	}
	return ac.SendWait(5*time.Second, b)
}
