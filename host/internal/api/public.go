package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/auth"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

func getPublicSummary(c *gin.Context) {
	var totalAgents, onlineAgents, totalTargets int64
	db.DB.Model(&model.Agent{}).Count(&totalAgents)
	db.DB.Model(&model.Agent{}).Where("is_online = ?", true).Count(&onlineAgents)
	db.DB.Model(&model.Target{}).Where("enabled = ?", true).Count(&totalTargets)

	cutoff := time.Now().Add(-1 * time.Hour)
	type AggResult struct {
		AvgRTT  float64
		AvgLoss float64
	}
	var agg AggResult
	db.DB.Model(&model.PingMetric{}).
		Select("COALESCE(AVG(avg_rtt), 0) as avg_rtt, COALESCE(AVG(loss_pct), 0) as avg_loss").
		Where("timestamp >= ?", cutoff).
		Scan(&agg)

	c.JSON(http.StatusOK, gin.H{
		"total_agents":  totalAgents,
		"online_agents": onlineAgents,
		"total_targets": totalTargets,
		"avg_rtt_ms":    agg.AvgRTT,
		"avg_loss_pct":  agg.AvgLoss,
	})
}

func getPublicAgents(c *gin.Context) {
	_, isAdmin := c.Get(auth.ContextUserKey)
	var agents []model.Agent
	db.DB.Order("is_online desc, name asc").Find(&agents)

	type respItem struct {
		ID         string    `json:"id"`
		Name       string    `json:"name"`
		PublicIP   string    `json:"public_ip"`
		OS         string    `json:"os"`
		Arch       string    `json:"arch"`
		Version    string    `json:"version"`
		Tags       string    `json:"tags"`
		IsOnline   bool      `json:"is_online"`
		LastSeenAt time.Time `json:"last_seen_at"`
	}

	resp := make([]respItem, 0, len(agents))
	for _, a := range agents {
		ip := a.PublicIP
		if !isAdmin {
			ip = auth.MaskIP(ip)
		}
		resp = append(resp, respItem{
			ID:         a.ID,
			Name:       a.Name,
			PublicIP:   ip,
			OS:         a.OS,
			Arch:       a.Arch,
			Version:    a.Version,
			Tags:       a.Tags,
			IsOnline:   a.IsOnline,
			LastSeenAt: a.LastSeenAt,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func getPublicTargets(c *gin.Context) {
	_, isAdmin := c.Get(auth.ContextUserKey)
	var targets []model.Target
	db.DB.Where("enabled = ?", true).Order("name asc").Find(&targets)

	type respItem struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Host        string `json:"host"`
		PacketCount int    `json:"packet_count"`
		IntervalSec int    `json:"interval_sec"`
		Tags        string `json:"tags"`
	}

	resp := make([]respItem, 0, len(targets))
	for _, t := range targets {
		host := t.Host
		if !isAdmin {
			host = auth.MaskHost(host)
		}
		resp = append(resp, respItem{
			ID:          t.ID,
			Name:        t.Name,
			Host:        host,
			PacketCount: t.PacketCount,
			IntervalSec: t.IntervalSec,
			Tags:        t.Tags,
		})
	}
	c.JSON(http.StatusOK, resp)
}
