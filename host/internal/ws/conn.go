package ws

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/ipgeo"
	"github.com/guimc233/JustPing/host/internal/model"
	"github.com/guimc233/JustPing/shared/protocol"
)

type AgentConn struct {
	AgentID  string
	RemoteIP string
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *Hub
	closeMu  sync.Mutex
	closed   bool
}

func (ac *AgentConn) Close() {
	ac.closeMu.Lock()
	defer ac.closeMu.Unlock()
	if !ac.closed {
		ac.closed = true
		close(ac.Send)
		_ = ac.Conn.Close()
	}
}

func (ac *AgentConn) SafeSend(msg []byte) error {
	ac.closeMu.Lock()
	defer ac.closeMu.Unlock()
	if ac.closed {
		return errors.New("connection closed")
	}
	select {
	case ac.Send <- msg:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

func (ac *AgentConn) QueueEnvelope(msgType protocol.MessageType, payload any) error {
	env := protocol.Envelope{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return ac.SafeSend(b)
}

func (ac *AgentConn) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		ac.Close()
	}()

	for {
		select {
		case msg, ok := <-ac.Send:
			_ = ac.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = ac.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := ac.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = ac.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := ac.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (ac *AgentConn) readPump() {
	defer func() {
		ac.Hub.Unregister(ac.AgentID, ac)
	}()

	ac.Conn.SetReadLimit(1024 * 512)
	_ = ac.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	ac.Conn.SetPongHandler(func(string) error {
		_ = ac.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, message, err := ac.Conn.ReadMessage()
		if err != nil {
			break
		}

		_ = ac.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		var env protocol.Envelope
		if err := json.Unmarshal(message, &env); err != nil {
			continue
		}

		ac.handleIncomingMessage(env)
	}
}

func (ac *AgentConn) handleIncomingMessage(env protocol.Envelope) {
	switch env.Type {
	case protocol.TypeHeartbeat:
		_ = db.DB.Model(&model.Agent{}).Where("id = ?", ac.AgentID).Updates(map[string]any{
			"is_online":    true,
			"last_seen_at": time.Now(),
		})
		_ = ac.QueueEnvelope(protocol.TypeHeartbeatAck, map[string]any{"status": "ok"})

	case protocol.TypePingReport:
		raw, _ := json.Marshal(env.Payload)
		var report protocol.PingReportPayload
		if err := json.Unmarshal(raw, &report); err != nil {
			return
		}

		var count int64
		if err := db.DB.Model(&model.Agent{}).Where("id = ?", ac.AgentID).Count(&count).Error; err != nil || count == 0 {
			ac.Close()
			return
		}

		ac.persistPingReport(report)

	case protocol.TypeTracerouteReport:
		raw, _ := json.Marshal(env.Payload)
		var report protocol.TracerouteReportPayload
		if err := json.Unmarshal(raw, &report); err != nil {
			return
		}
		go ac.persistTracerouteReport(report)
	}
}

func (ac *AgentConn) persistPingReport(report protocol.PingReportPayload) {
	if len(report.Results) == 0 {
		return
	}

	var validTargets []string
	db.DB.Model(&model.Target{}).Where("enabled = ?", true).Pluck("id", &validTargets)
	targetMap := make(map[string]bool, len(validTargets))
	for _, id := range validTargets {
		targetMap[id] = true
	}

	now := time.Now()
	metrics := make([]model.PingMetric, 0, len(report.Results))
	for _, r := range report.Results {
		if !targetMap[r.TargetID] {
			continue
		}
		ts := r.Timestamp
		if ts.IsZero() || ts.After(now.Add(5*time.Minute)) || ts.Before(now.Add(-48*time.Hour)) {
			ts = now
		}
		metrics = append(metrics, model.PingMetric{
			AgentID:     ac.AgentID,
			TargetID:    r.TargetID,
			Timestamp:   ts,
			PacketsSent: r.PacketsSent,
			PacketsRecv: r.PacketsRecv,
			LossPct:     r.LossPct,
			MinRTT:      r.MinRTT,
			MaxRTT:      r.MaxRTT,
			AvgRTT:      r.AvgRTT,
			Jitter:      r.Jitter,
			StdDev:      r.StdDev,
			ErrorMsg:    r.ErrorMsg,
		})
	}
	if len(metrics) > 0 {
		_ = db.DB.CreateInBatches(metrics, 100)
	}

	_ = db.DB.Model(&model.Agent{}).Where("id = ?", ac.AgentID).Updates(map[string]any{
		"is_online":    true,
		"last_seen_at": now,
	})
}

func (ac *AgentConn) persistTracerouteReport(report protocol.TracerouteReportPayload) {
	if len(report.Hops) == 0 && report.TargetHost == "" {
		return
	}

	now := time.Now()
	ts := report.Timestamp
	if ts.IsZero() || ts.After(now.Add(5*time.Minute)) || ts.Before(now.Add(-48*time.Hour)) {
		ts = now
	}

	// Enrich each hop with NextTrace GeoIP & ASN data concurrently with a context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	enrichedHops := make([]model.EnrichedHop, 0, len(report.Hops))
	for _, h := range report.Hops {
		eh := model.EnrichedHop{
			TTL:      h.TTL,
			IP:       h.IP,
			Hostname: h.Hostname,
			RTTs:     h.RTTs,
			AvgRTT:   h.AvgRTT,
			LossPct:  h.LossPct,
		}

		if h.IP != "" && h.IP != "*" {
			geo := ipgeo.Lookup(ctx, h.IP)
			eh.ASNumber = geo.ASNumber
			eh.ASOrg = geo.ASOrg
			eh.ISP = geo.ISP
			eh.Country = geo.Country
			eh.CountryCode = geo.CountryCode
			eh.City = geo.City
		}

		enrichedHops = append(enrichedHops, eh)
	}

	rec := model.TracerouteRecord{
		ID:         uuid.New().String(),
		AgentID:    ac.AgentID,
		TargetID:   report.TargetID,
		TargetHost: report.TargetHost,
		ResolvedIP: report.ResolvedIP,
		Timestamp:  ts,
		DurationMs: report.DurationMs,
		Reached:    report.Reached,
		HopCount:   len(enrichedHops),
		Hops:       enrichedHops,
	}

	_ = db.DB.Create(&rec)

	_ = db.DB.Model(&model.Agent{}).Where("id = ?", ac.AgentID).Updates(map[string]any{
		"is_online":    true,
		"last_seen_at": now,
	})
}
