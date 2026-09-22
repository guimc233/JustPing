package ws

import (
	"time"

	"github.com/guimc233/JustPing/host/internal/feature"
	"github.com/guimc233/JustPing/shared/protocol"
)

// SoftExitEscalationTimeout is how long the Host waits for a probe to act on a
// soft exit before falling back to crashing it. A probe that is hung, wedged, or
// simply too old to understand the message will not close its connection, and
// crashing remains the only remaining lever.
const SoftExitEscalationTimeout = 10 * time.Second

// softExitPollInterval is how often the wait above re-checks which probes left.
const softExitPollInterval = 200 * time.Millisecond

// restartProbe is a snapshot of one connected probe's restart-relevant state.
type restartProbe struct {
	id      string
	version string
	arch    string
	gone    <-chan struct{}
}

// ErrRestartUnsupported means the probe cannot be force-restarted: either it
// should be driven through update_check, or its build cannot be reached at all.
var ErrRestartUnsupported error = errRestartUnsupported{}

type errRestartUnsupported struct{}

func (errRestartUnsupported) Error() string {
	return "probe cannot be force-restarted; use the update check for native probes, or reinstall it"
}

// ErrSoftExitEscalated reports that the soft exit timed out and the probe was
// crashed instead.
var ErrSoftExitEscalated error = errSoftExitEscalated{}

type errSoftExitEscalated struct{}

func (errSoftExitEscalated) Error() string {
	return "probe did not exit in time; it was crashed instead"
}

// RequestRestart forces one probe to restart so its start-up auto-updater picks
// up the latest release.
//
// It escalates: probes that understand soft_exit are asked to exit cleanly
// first, and only if they are still connected after SoftExitEscalationTimeout
// are they crashed. Probes older than soft_exit are crashed straight away,
// because there is nothing to ask them.
//
// The returned ForceRestartMode is the route that was actually used. A non-nil
// error means nothing was delivered, except for ErrSoftExitEscalated, where the
// crash was delivered after the soft exit timed out.
func (h *Hub) RequestRestart(agentID string) (feature.ForceRestartMode, error) {
	ac, online := h.liveProbe(agentID)
	if !online {
		return "", ErrAgentOffline
	}

	mode := feature.ForceRestartModeFor(ac.Version, ac.Arch)

	switch mode {
	case feature.ForceRestartSoftExit:
		if err := h.SendToAgent(agentID, protocol.TypeSoftExit, protocol.SoftExitPayload{}); err != nil {
			return mode, err
		}

		select {
		case <-ac.Gone():
			return mode, nil
		case <-time.After(SoftExitEscalationTimeout):
		}

		// Still connected: the probe is not going to act on its own.
		return feature.ForceRestartCrash, h.sendCrash(agentID)

	case feature.ForceRestartCrash:
		return mode, h.sendCrash(agentID)

	default:
		// Too old to act on either request, or a 32-bit build that cannot
		// carry the crash payload.
		return mode, ErrRestartUnsupported
	}
}

// sendCrash delivers the deliberately malformed target sync that probes too old
// for soft_exit treat as a force-update signal.
func (h *Hub) sendCrash(agentID string) error {
	return h.SendToAgent(agentID, protocol.TypeTargetSync, legacyCrashTargetSync())
}

// BroadcastRestart summarizes a broadcast force-restart request.
type RestartBroadcast struct {
	// SoftExit counts probes that exited cleanly on request.
	SoftExit int `json:"soft_exit"`
	// Escalated counts probes that ignored the soft exit and were crashed.
	Escalated int `json:"escalated"`
	// Crash counts probes crashed immediately (too old for soft_exit).
	Crash int `json:"crash"`
	// Unsupported counts probes that cannot be restarted this way.
	Unsupported int `json:"unsupported"`
	// Unreachable counts probes whose send failed (already gone).
	Unreachable int `json:"unreachable"`
}

// BroadcastRestart force-restarts every probe that can be reached, preferring
// soft_exit and escalating to a crash.
func (h *Hub) BroadcastRestart() RestartBroadcast {
	h.mu.RLock()
	probes := make([]restartProbe, 0, len(h.agents))
	for id, ac := range h.agents {
		probes = append(probes, restartProbe{
			id:      id,
			version: ac.Version,
			arch:    ac.Arch,
			gone:    ac.Gone(),
		})
	}
	h.mu.RUnlock()

	var result RestartBroadcast
	var waiting []restartProbe

	for _, p := range probes {
		switch mode := feature.ForceRestartModeFor(p.version, p.arch); mode {
		case feature.ForceRestartSoftExit:
			if err := h.SendToAgent(p.id, protocol.TypeSoftExit, protocol.SoftExitPayload{}); err != nil {
				result.Unreachable++
				continue
			}
			waiting = append(waiting, p)
		case feature.ForceRestartCrash:
			if err := h.sendCrash(p.id); err != nil {
				result.Unreachable++
				continue
			}
			result.Crash++
		default:
			result.Unsupported++
		}
	}

	h.awaitSoftExits(waiting, &result)
	return result
}

// awaitSoftExits waits once for every soft-exited probe to disconnect, escalating
// the stragglers to a crash when the timeout expires.
func (h *Hub) awaitSoftExits(waiting []restartProbe, result *RestartBroadcast) {
	if len(waiting) == 0 {
		return
	}

	deadline := time.After(SoftExitEscalationTimeout)
	pending := waiting

	for len(pending) > 0 {
		remaining := make([]restartProbe, 0, len(pending))
		for _, p := range pending {
			select {
			case <-p.gone:
				result.SoftExit++
			default:
				remaining = append(remaining, p)
			}
		}
		pending = remaining
		if len(pending) == 0 {
			return
		}

		select {
		case <-time.After(softExitPollInterval):
		case <-deadline:
			for _, p := range pending {
				if err := h.sendCrash(p.id); err != nil {
					result.Unreachable++
					continue
				}
				result.Escalated++
			}
			return
		}
	}
}
