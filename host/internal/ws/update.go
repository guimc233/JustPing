package ws

import (
	"errors"
	"time"

	"github.com/guimc233/JustPing/host/internal/feature"
	"github.com/guimc233/JustPing/host/internal/model"
	"github.com/guimc233/JustPing/shared/protocol"
)

// Errors returned when a probe cannot be asked to update natively.
var (
	// ErrAgentOffline means the probe has no live connection.
	ErrAgentOffline = errors.New("agent is offline")
	// ErrNativeUnsupported means the probe predates the update_check message, so
	// it has to be restarted instead (see RequestRestart).
	ErrNativeUnsupported = errors.New("agent predates the update_check message; restart it instead")
)

// legacyCrashTargetSync builds the deliberately out-of-range target sync that
// crashes a pre-update_check probe into restarting. See feature/legacy.go for
// why this works and which fields are used.
func legacyCrashTargetSync() protocol.TargetSyncPayload {
	return protocol.TargetSyncPayload{
		Targets: []protocol.TargetConfig{{
			ID:          "justping-force-update",
			Name:        "Force update (legacy)",
			Host:        "127.0.0.1",
			PacketCount: feature.LegacyCrashPacketCount,
			IntervalSec: feature.LegacyCrashIntervalSec,
		}},
	}
}

// liveProbe returns the connection and reported build of a connected probe.
func (h *Hub) liveProbe(agentID string) (*AgentConn, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ac, ok := h.agents[agentID]
	return ac, ok
}

// RequestUpdateCheck asks one probe to check for the latest release and install
// it immediately through the native update_check message.
func (h *Hub) RequestUpdateCheck(agentID string) error {
	ac, online := h.liveProbe(agentID)
	if !online {
		return ErrAgentOffline
	}
	if !feature.Supports(ac.Version, feature.UpdateCheck) {
		return ErrNativeUnsupported
	}
	return h.SendToAgent(agentID, protocol.TypeUpdateCheck, nil)
}

// UpdateBroadcast summarizes a broadcast update request.
type UpdateBroadcast struct {
	// Triggered counts probes the request was queued to.
	Triggered int `json:"triggered"`
	// Skipped counts probes this method does not apply to.
	Skipped int `json:"skipped"`
	// Unreachable counts probes whose send failed (already gone).
	Unreachable int `json:"unreachable"`
}

// BroadcastUpdateCheck asks every probe that supports the native message to
// update. Probes that only have the restart routes are skipped.
func (h *Hub) BroadcastUpdateCheck() UpdateBroadcast {
	h.mu.RLock()
	type probe struct {
		id      string
		version string
	}
	probes := make([]probe, 0, len(h.agents))
	for id, ac := range h.agents {
		probes = append(probes, probe{id: id, version: ac.Version})
	}
	h.mu.RUnlock()

	var result UpdateBroadcast
	for _, p := range probes {
		if !feature.Supports(p.version, feature.UpdateCheck) {
			result.Skipped++
			continue
		}
		if err := h.SendToAgent(p.id, protocol.TypeUpdateCheck, nil); err != nil {
			result.Unreachable++
			continue
		}
		result.Triggered++
	}
	return result
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
