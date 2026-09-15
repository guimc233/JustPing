package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/auth"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

type MatrixCell struct {
	AgentID   string    `json:"agent_id"`
	TargetID  string    `json:"target_id"`
	Timestamp time.Time `json:"timestamp"`
	AvgRTT    float64   `json:"avg_rtt_ms"`
	MinRTT    float64   `json:"min_rtt_ms"`
	MaxRTT    float64   `json:"max_rtt_ms"`
	Jitter    float64   `json:"jitter_ms"`
	LossPct   float64   `json:"loss_pct"`
	Status    string    `json:"status"`
}

type PublicMetricPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	AvgRTT      float64   `json:"avg_rtt_ms"`
	MinRTT      float64   `json:"min_rtt_ms"`
	MaxRTT      float64   `json:"max_rtt_ms"`
	Jitter      float64   `json:"jitter_ms"`
	LossPct     float64   `json:"loss_pct"`
	PacketsSent int       `json:"packets_sent"`
	PacketsRecv int       `json:"packets_recv"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
}

func getPublicMatrix(c *gin.Context) {
	cutoff := time.Now().Add(-10 * time.Minute)
	var metrics []model.PingMetric

	subQuery := db.DB.Model(&model.PingMetric{}).
		Select("DISTINCT ON (agent_id, target_id) *").
		Where("timestamp >= ?", cutoff).
		Order("agent_id, target_id, timestamp DESC")

	if err := subQuery.Find(&metrics).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cells := make([]MatrixCell, 0, len(metrics))
	for _, m := range metrics {
		status := "healthy"
		if m.LossPct >= 50.0 || m.AvgRTT >= 300.0 {
			status = "critical"
		} else if m.LossPct > 5.0 || m.Jitter > 30.0 || m.AvgRTT > 150.0 {
			status = "warning"
		}
		cells = append(cells, MatrixCell{
			AgentID:   m.AgentID,
			TargetID:  m.TargetID,
			Timestamp: m.Timestamp,
			AvgRTT:    m.AvgRTT,
			MinRTT:    m.MinRTT,
			MaxRTT:    m.MaxRTT,
			Jitter:    m.Jitter,
			LossPct:   m.LossPct,
			Status:    status,
		})
	}
	c.JSON(http.StatusOK, cells)
}

func getPublicMetrics(c *gin.Context) {
	_, isAdmin := c.Get(auth.ContextUserKey)
	targetID := c.Query("target_id")
	agentID := c.Query("agent_id")
	timeRange := c.DefaultQuery("range", "1h")

	duration := 1 * time.Hour
	switch timeRange {
	case "6h":
		duration = 6 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	}

	cutoff := time.Now().Add(-duration)
	query := db.DB.Model(&model.PingMetric{}).Where("timestamp >= ?", cutoff)
	if targetID != "" {
		query = query.Where("target_id = ?", targetID)
	}
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}

	var metrics []model.PingMetric
	if err := query.Order("timestamp asc").Limit(1000).Find(&metrics).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	points := make([]PublicMetricPoint, 0, len(metrics))
	for _, m := range metrics {
		errMsg := ""
		if isAdmin {
			errMsg = m.ErrorMsg
		} else if m.ErrorMsg != "" {
			errMsg = "Ping failed" // Strip internal hostnames/IP addresses for anonymous users
		}

		points = append(points, PublicMetricPoint{
			Timestamp:   m.Timestamp,
			AvgRTT:      m.AvgRTT,
			MinRTT:      m.MinRTT,
			MaxRTT:      m.MaxRTT,
			Jitter:      m.Jitter,
			LossPct:     m.LossPct,
			PacketsSent: m.PacketsSent,
			PacketsRecv: m.PacketsRecv,
			ErrorMsg:    errMsg,
		})
	}

	c.JSON(http.StatusOK, points)
}
