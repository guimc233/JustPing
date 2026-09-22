package ws

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
	"github.com/guimc233/JustPing/host/internal/proxy"
	"github.com/guimc233/JustPing/shared/protocol"
)

type Hub struct {
	mu      sync.RWMutex
	agents  map[string]*AgentConn
	closeCh chan struct{}
}

var DefaultHub = NewHub()

func NewHub() *Hub {
	return &Hub{
		agents:  make(map[string]*AgentConn),
		closeCh: make(chan struct{}),
	}
}

// Register adds an agent connection, closing any stale predecessor
func (h *Hub) Register(agentID string, ac *AgentConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, exists := h.agents[agentID]; exists {
		old.Close()
	}
	h.agents[agentID] = ac
	if ac.RemoteIP != "" {
		log.Printf("[WS Hub] Agent %s connected from %s. Online probes: %d\n", agentID, ac.RemoteIP, len(h.agents))
	} else {
		log.Printf("[WS Hub] Agent %s connected. Online probes: %d\n", agentID, len(h.agents))
	}
}

// Unregister removes an agent connection only if it is the current registered instance
func (h *Hub) Unregister(agentID string, ac *AgentConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	current, exists := h.agents[agentID]
	if !exists || current != ac {
		return
	}

	delete(h.agents, agentID)
	ac.Close()
	if ac.RemoteIP != "" {
		log.Printf("[WS Hub] Agent %s (%s) disconnected. Online probes: %d\n", agentID, ac.RemoteIP, len(h.agents))
	} else {
		log.Printf("[WS Hub] Agent %s disconnected. Online probes: %d\n", agentID, len(h.agents))
	}

	_ = db.DB.Model(&model.Agent{}).Where("id = ?", agentID).Updates(map[string]any{
		"is_online":    false,
		"last_seen_at": time.Now(),
	})
	proxy.Default.DropAgent(agentID)
}

// Disconnect forcefully closes and removes the active connection for an agent (token rotation or deletion)
func (h *Hub) Disconnect(agentID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ac, exists := h.agents[agentID]; exists {
		delete(h.agents, agentID)
		ac.Close()
		log.Printf("[WS Hub] Force disconnected agent %s\n", agentID)
	}
	_ = db.DB.Model(&model.Agent{}).Where("id = ?", agentID).Updates(map[string]any{
		"is_online":    false,
		"last_seen_at": time.Now(),
	})
	proxy.Default.DropAgent(agentID)
}

// SendToAgent sends a typed envelope safely without panicking on closed channel
func (h *Hub) SendToAgent(agentID string, msgType protocol.MessageType, payload any) error {
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
	ac, exists := h.agents[agentID]
	h.mu.RUnlock()

	if !exists {
		return nil
	}

	return ac.SafeSend(b)
}

// BroadcastTargetSync pushes target configs to all active agents
func (h *Hub) BroadcastTargetSync() {
	var targets []model.Target
	if err := db.DB.Where("enabled = ?", true).Find(&targets).Error; err != nil {
		return
	}

	h.mu.RLock()
	agentIDs := make([]string, 0, len(h.agents))
	for id := range h.agents {
		agentIDs = append(agentIDs, id)
	}
	h.mu.RUnlock()

	for _, id := range agentIDs {
		configs := buildTargetConfigs(id, targets)
		_ = h.SendToAgent(id, protocol.TypeTargetSync, protocol.TargetSyncPayload{Targets: configs})
	}
}

// SyncSingleAgent sends the current target list to a specific agent
func (h *Hub) SyncSingleAgent(agentID string) {
	var targets []model.Target
	if err := db.DB.Where("enabled = ?", true).Find(&targets).Error; err != nil {
		return
	}

	configs := buildTargetConfigs(agentID, targets)
	_ = h.SendToAgent(agentID, protocol.TypeTargetSync, protocol.TargetSyncPayload{Targets: configs})
}

func buildTargetConfigs(agentID string, targets []model.Target) []protocol.TargetConfig {
	disabledMap := make(map[string]bool)
	if agentID != "" {
		var ag model.Agent
		if err := db.DB.Select("disabled_route_targets").First(&ag, "id = ?", agentID).Error; err == nil && ag.DisabledRouteTargets != "" {
			for _, id := range strings.Split(ag.DisabledRouteTargets, ",") {
				id = strings.TrimSpace(id)
				if id != "" {
					disabledMap[id] = true
				}
			}
		}
	}

	configs := make([]protocol.TargetConfig, 0, len(targets))
	for _, t := range targets {
		configs = append(configs, protocol.TargetConfig{
			ID:           t.ID,
			Name:         t.Name,
			Host:         t.Host,
			PacketCount:  t.PacketCount,
			IntervalSec:  t.IntervalSec,
			DisableRoute: t.DisableRoute || disabledMap[t.ID],
		})
	}
	return configs
}
