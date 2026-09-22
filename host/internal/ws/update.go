package ws

import (
	"errors"
	"time"

	"github.com/guimc233/JustPing/host/internal/model"
	"github.com/guimc233/JustPing/shared/protocol"
)

// RequestUpdateCheck asks a single connected probe to check for the latest
// release and install it immediately, bypassing the periodic auto-updater.
func (h *Hub) RequestUpdateCheck(agentID string) error {
	h.mu.RLock()
	_, online := h.agents[agentID]
	h.mu.RUnlock()

	if !online {
		return errors.New("agent is offline")
	}
	return h.SendToAgent(agentID, protocol.TypeUpdateCheck, nil)
}

// BroadcastUpdateCheck asks every connected probe to check for updates and
// returns the number of probes the request was queued to.
func (h *Hub) BroadcastUpdateCheck() int {
	h.mu.RLock()
	agentIDs := make([]string, 0, len(h.agents))
	for id := range h.agents {
		agentIDs = append(agentIDs, id)
	}
	h.mu.RUnlock()

	triggered := 0
	for _, id := range agentIDs {
		if err := h.SendToAgent(id, protocol.TypeUpdateCheck, nil); err == nil {
			triggered++
		}
	}
	return triggered
}

// RecordUpdateStatus stores the update check outcome reported by a probe.
func (h *Hub) RecordUpdateStatus(agentID string, res protocol.UpdateResultPayload) {
	if agentID == "" {
		return
	}

	h.updateMu.Lock()
	defer h.updateMu.Unlock()
	h.updates[agentID] = model.AgentUpdateStatus{
		CurrentVersion: res.CurrentVersion,
		LatestVersion:  res.LatestVersion,
		Updating:       res.Updating,
		Error:          res.Error,
		CheckedAt:      time.Now(),
	}
}

// UpdateStatuses returns a snapshot of the update checks reported so far.
func (h *Hub) UpdateStatuses() map[string]model.AgentUpdateStatus {
	h.updateMu.RLock()
	defer h.updateMu.RUnlock()

	out := make(map[string]model.AgentUpdateStatus, len(h.updates))
	for id, status := range h.updates {
		out[id] = status
	}
	return out
}
