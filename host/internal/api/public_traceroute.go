package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/ipgeo"
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
		fixMissingRoutePath(&rec)
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
				fixMissingRoutePath(&records[bestIdx])
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

	fixMissingRoutePath(&latest)
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
	fixMissingRoutePath(&rec)
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

	var records []model.TracerouteRecord
	if err := query.Order("timestamp desc").Limit(50).Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type TraceSummary struct {
		ID         string         `json:"id"`
		AgentID    string         `json:"agent_id"`
		TargetID   string         `json:"target_id"`
		TargetHost string         `json:"target_host"`
		ResolvedIP string         `json:"resolved_ip"`
		Timestamp  time.Time      `json:"timestamp"`
		DurationMs int64          `json:"duration_ms"`
		Reached    bool           `json:"reached"`
		HopCount   int            `json:"hop_count"`
		RoutePath  string         `json:"route_path"`
		ASPath     string         `json:"as_path"`
		ASNodes    []model.ASNode `json:"as_nodes"`
	}

	summaries := make([]TraceSummary, 0, len(records))
	for i := range records {
		fixMissingRoutePath(&records[i])
		summaries = append(summaries, TraceSummary{
			ID:         records[i].ID,
			AgentID:    records[i].AgentID,
			TargetID:   records[i].TargetID,
			TargetHost: records[i].TargetHost,
			ResolvedIP: records[i].ResolvedIP,
			Timestamp:  records[i].Timestamp,
			DurationMs: records[i].DurationMs,
			Reached:    records[i].Reached,
			HopCount:   records[i].HopCount,
			RoutePath:  records[i].RoutePath,
			ASPath:     records[i].ASPath,
			ASNodes:    records[i].ASNodes,
		})
	}

	c.JSON(http.StatusOK, summaries)
}

// fixMissingRoutePath automatically backfills and corrects route_path and as_path for historical records.
func fixMissingRoutePath(rec *model.TracerouteRecord) {
	if rec == nil || len(rec.Hops) == 0 {
		return
	}

	needsUpdate := false
	hopMetas := make([]ipgeo.HopMeta, len(rec.Hops))
	for i, h := range rec.Hops {
		hopMetas[i] = ipgeo.HopMeta{
			IP:       h.IP,
			ASNumber: h.ASNumber,
			ASOrg:    h.ASOrg,
			ISP:      h.ISP,
		}
	}

	if rec.RoutePath == "" || rec.RoutePath == "Unknown" {
		rec.RoutePath = ipgeo.ClassifyRoute(hopMetas, rec.TargetHost, "")
		needsUpdate = true
	}

	if rec.ASPath == "" || len(rec.ASNodes) == 0 {
		rec.ASPath, rec.ASNodes = ipgeo.GenerateASPath(hopMetas)
		needsUpdate = true
	}

	if needsUpdate && rec.ID != "" {
		go func(id, path, asPath string, nodes []model.ASNode) {
			_ = db.DB.Model(&model.TracerouteRecord{}).Where("id = ?", id).Updates(map[string]any{
				"route_path": path,
				"as_path":    asPath,
				"as_nodes":   nodes,
			})
		}(rec.ID, rec.RoutePath, rec.ASPath, rec.ASNodes)
	}
}

// AutoBackfillHistoricalTraceroutes runs in background at host startup to fix legacy traceroute records.
func AutoBackfillHistoricalTraceroutes() {
	go func() {
		time.Sleep(3 * time.Second)
		var records []model.TracerouteRecord
		err := db.DB.Where("route_path = '' OR route_path IS NULL OR as_path = '' OR as_path IS NULL").
			Limit(500).
			Find(&records).Error
		if err != nil || len(records) == 0 {
			return
		}

		log.Printf("[DB] Auto-backfilling route_path and as_path for %d historical traceroute records...\n", len(records))
		for i := range records {
			fixMissingRoutePath(&records[i])
		}
	}()
}

func diffDuration(a, b time.Time) time.Duration {
	d := a.Sub(b)
	if d < 0 {
		return -d
	}
	return d
}
