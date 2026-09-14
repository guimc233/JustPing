package api

import (
	"math"
	"time"

	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

type AgentQuality struct {
	AvgRTT  float64 `json:"avg_rtt_ms"`
	Jitter  float64 `json:"jitter_ms"`
	LossPct float64 `json:"loss_pct"`
	Score   int     `json:"quality_score"` // 0 - 100
	Grade   string  `json:"quality_grade"` // A+, A, B, C, F
	Status  string  `json:"network_status"`
}

func calculateAgentQuality(rtt, jitter, loss float64, isOnline bool) AgentQuality {
	if !isOnline {
		return AgentQuality{Score: 0, Grade: "Offline", Status: "offline"}
	}

	score := 100.0 - (loss * 3.0)
	if jitter > 5.0 {
		score -= (jitter - 5.0) * 0.5
	}
	if rtt > 100.0 {
		score -= (rtt - 100.0) * 0.1
	}
	if score < 0 {
		score = 0
	}
	finalScore := int(math.Round(score))

	grade := "A+"
	status := "excellent"
	if loss >= 20.0 || finalScore < 40 {
		grade = "F"
		status = "critical"
	} else if loss >= 5.0 || finalScore < 60 {
		grade = "C"
		status = "degraded"
	} else if jitter >= 15.0 || finalScore < 75 {
		grade = "B"
		status = "fair"
	} else if finalScore < 90 {
		grade = "A"
		status = "good"
	}

	return AgentQuality{
		AvgRTT:  math.Round(rtt*100) / 100,
		Jitter:  math.Round(jitter*100) / 100,
		LossPct: math.Round(loss*100) / 100,
		Score:   finalScore,
		Grade:   grade,
		Status:  status,
	}
}

// getRecentAgentMetrics fetches recent network statistics for an agent
func getRecentAgentMetrics(agentID string) (float64, float64, float64) {
	cutoff := time.Now().Add(-15 * time.Minute)
	type agg struct {
		AvgRTT  float64
		Jitter  float64
		LossPct float64
	}
	var a agg
	db.DB.Model(&model.PingMetric{}).
		Select("COALESCE(AVG(avg_rtt), 0) as avg_rtt, COALESCE(AVG(jitter), 0) as jitter, COALESCE(AVG(loss_pct), 0) as loss_pct").
		Where("agent_id = ? AND timestamp >= ?", agentID, cutoff).
		Scan(&a)
	return a.AvgRTT, a.Jitter, a.LossPct
}
