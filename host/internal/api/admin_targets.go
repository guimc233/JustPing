package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
	"github.com/guimc233/JustPing/host/internal/ws"
)

type CreateTargetReq struct {
	Name        string `json:"name" binding:"required"`
	Host        string `json:"host" binding:"required"`
	PacketCount int    `json:"packet_count"`
	IntervalSec int    `json:"interval_sec"`
	Tags        string `json:"tags"`
	Enabled     *bool  `json:"enabled"`
}

func adminListTargets(c *gin.Context) {
	var targets []model.Target
	db.DB.Order("created_at desc").Find(&targets)
	c.JSON(http.StatusOK, targets)
}

func adminCreateTarget(c *gin.Context) {
	var req CreateTargetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pktCount := 15
	if req.PacketCount > 0 && req.PacketCount <= 100 {
		pktCount = req.PacketCount
	}
	interval := 60
	if req.IntervalSec >= 10 && req.IntervalSec <= 3600 {
		interval = req.IntervalSec
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	target := model.Target{
		ID:          uuid.New().String(),
		Name:        strings.TrimSpace(req.Name),
		Host:        strings.TrimSpace(req.Host),
		PacketCount: pktCount,
		IntervalSec: interval,
		Tags:        strings.TrimSpace(req.Tags),
		Enabled:     enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.DB.Create(&target).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ws.DefaultHub.BroadcastTargetSync()
	c.JSON(http.StatusCreated, target)
}

func adminUpdateTarget(c *gin.Context) {
	id := c.Param("id")
	var target model.Target
	if err := db.DB.First(&target, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target not found"})
		return
	}

	var req CreateTargetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		target.Name = strings.TrimSpace(req.Name)
	}
	if req.Host != "" {
		target.Host = strings.TrimSpace(req.Host)
	}
	if req.PacketCount > 0 {
		target.PacketCount = req.PacketCount
	}
	if req.IntervalSec > 0 {
		target.IntervalSec = req.IntervalSec
	}
	target.Tags = strings.TrimSpace(req.Tags)
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
	}
	target.UpdatedAt = time.Now()

	if err := db.DB.Save(&target).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ws.DefaultHub.BroadcastTargetSync()
	c.JSON(http.StatusOK, target)
}

func adminDeleteTarget(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&model.Target{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ws.DefaultHub.BroadcastTargetSync()
	c.JSON(http.StatusOK, gin.H{"message": "Target deleted"})
}
