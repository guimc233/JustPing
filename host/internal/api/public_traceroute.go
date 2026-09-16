package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

// getPublicTraceroute finds a traceroute record closest to the given timestamp, or by ID.
func getPublicTraceroute(c *gin.Context) {
	id := c.Query("id")
	if id != "" {
		var rec model.TracerouteRecord
		if err := db.DB.Where("id = ?", id).First(&rec).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Traceroute record not found"})
			return
		}
		c.JSON(http.StatusOK, rec)
		return
	}

	agentID := c.Query("agent_id")
	targetID := c.Query("target_id")
	timeStr := c.Query("time")

	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	query := db.DB.Model(&model.TracerouteRecord{}).Where("agent_id = ?", agentID)
	if targetID != "" {
		query = query.Where("target_id = ?", targetID)
	}

	if timeStr != "" {
		var targetTime time.Time
		// Check if millisecond timestamp
		if ms, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
			targetTime = time.UnixMilli(ms)
		} else if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			targetTime = t
		}

		if !targetTime.IsZero() {
			// Find closest record within +/- 15 minutes window
			windowStart := targetTime.Add(-15 * time.Minute)
			windowEnd := targetTime.Add(15 * time.Minute)

			var records []model.TracerouteRecord
			_ = query.Where("timestamp >= ? AND timestamp <= ?", windowStart, windowEnd).
				Order("timestamp desc").
				Limit(5).
				Find(&records).Error

			if len(records) > 0 {
				// Pick the record with minimum time difference
				bestIdx := 0
				minDiff := diffDuration(records[0].Timestamp, targetTime)
				for i := 1; i < len(records); i++ {
					diff := diffDuration(records[i].Timestamp, targetTime)
					if diff < minDiff {
						minDiff = diff
						bestIdx = i
					}
				}
				c.JSON(http.StatusOK, records[bestIdx])
				return
			}
		}
	}

	// Fallback to the latest traceroute record
	var latest model.TracerouteRecord
	if err := query.Order("timestamp desc").First(&latest).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No traceroute telemetry available"})
		return
	}

	c.JSON(http.StatusOK, latest)
}

// getPublicTracerouteByID returns a single traceroute record by its primary key
func getPublicTracerouteByID(c *gin.Context) {
	id := c.Param("id")
	var rec model.TracerouteRecord
	if err := db.DB.Where("id = ?", id).First(&rec).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Traceroute record not found"})
		return
	}
	c.JSON(http.StatusOK, rec)
}

// getPublicTraceroutes lists recent traceroute history headers
func getPublicTraceroutes(c *gin.Context) {
	agentID := c.Query("agent_id")
	targetID := c.Query("target_id")

	query := db.DB.Model(&model.TracerouteRecord{})
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if targetID != "" {
		query = query.Where("target_id = ?", targetID)
	}

	type TraceSummary struct {
		ID         string    `json:"id"`
		AgentID    string    `json:"agent_id"`
		TargetID   string    `json:"target_id"`
		TargetHost string    `json:"target_host"`
		ResolvedIP string    `json:"resolved_ip"`
		Timestamp  time.Time `json:"timestamp"`
		DurationMs int64     `json:"duration_ms"`
		Reached    bool      `json:"reached"`
		HopCount   int       `json:"hop_count"`
	}

	var summaries []TraceSummary
	if err := query.Order("timestamp desc").Limit(50).Find(&summaries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summaries)
}

func diffDuration(a, b time.Time) time.Duration {
	d := a.Sub(b)
	if d < 0 {
		return -d
	}
	return d
}
