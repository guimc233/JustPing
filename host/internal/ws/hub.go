package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
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

// Register adds an agent connection
func (h *Hub) Register(agentID string, ac *AgentConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, exists := h.agents[agentID]; exists {
		close(old.Send)
		_ = old.Conn.Close()
	}
	h.agents[agentID] = ac
	log.Printf("[WS Hub] Agent %s connected. Total online: %d\n", agentID, len(h.agents))
}

// Unregister removes an agent connection
func (h *Hub) Unregister(agentID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ac, exists := h.agents[agentID]; exists {
		delete(h.agents, agentID)
		close(ac.Send)
		_ = ac.Conn.Close()
		log.Printf("[WS Hub] Agent %s disconnected. Total online: %d\n", agentID, len(h.agents))
	}
	_ = db.DB.Model(&model.Agent{}).Where("id = ?", agentID).Updates(map[string]any{
		"is_online":    false,
		"last_seen_at": time.Now(),
	})
}

// SendToAgent sends a typed envelope message to a specific agent
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

	select {
	case ac.Send <- b:
	default:
		log.Printf("[WS Hub] Send buffer full for agent %s, dropping message\n", agentID)
	}
	return nil
}

// BroadcastTargetSync re-evaluates and pushes target configs to all active agents
func (h *Hub) BroadcastTargetSync() {
	var targets []model.Target
	if err := db.DB.Where("enabled = ?", true).Find(&targets).Error; err != nil {
		log.Printf("[WS Hub] Failed to query active targets: %v\n", err)
		return
	}

	syncConfigs := make([]protocol.TargetConfig, 0, len(targets))
	for _, t := range targets {
		syncConfigs = append(syncConfigs, protocol.TargetConfig{
			ID:          t.ID,
			Name:        t.Name,
			Host:        t.Host,
			PacketCount: t.PacketCount,
			IntervalSec: t.IntervalSec,
		})
	}

	payload := protocol.TargetSyncPayload{Targets: syncConfigs}

	h.mu.RLock()
	agentIDs := make([]string, 0, len(h.agents))
	for id := range h.agents {
		agentIDs = append(agentIDs, id)
	}
	h.mu.RUnlock()

	for _, id := range agentIDs {
		_ = h.SendToAgent(id, protocol.TypeTargetSync, payload)
	}
}

// SyncSingleAgent sends the current target list to a specific agent
func (h *Hub) SyncSingleAgent(agentID string) {
	var targets []model.Target
	if err := db.DB.Where("enabled = ?", true).Find(&targets).Error; err != nil {
		return
	}

	syncConfigs := make([]protocol.TargetConfig, 0, len(targets))
	for _, t := range targets {
		syncConfigs = append(syncConfigs, protocol.TargetConfig{
			ID:          t.ID,
			Name:        t.Name,
			Host:        t.Host,
			PacketCount: t.PacketCount,
			IntervalSec: t.IntervalSec,
		})
	}

	_ = h.SendToAgent(agentID, protocol.TypeTargetSync, protocol.TargetSyncPayload{Targets: syncConfigs})
}
