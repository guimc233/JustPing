package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
	"github.com/guimc233/JustPing/host/internal/ws"
)

type CreateAgentReq struct {
	Name string `json:"name" binding:"required"`
	Tags string `json:"tags"`
}

func adminListAgents(c *gin.Context) {
	var agents []model.Agent
	db.DB.Order("created_at desc").Find(&agents)
	c.JSON(http.StatusOK, agents)
}

func adminCreateAgent(c *gin.Context) {
	var req CreateAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenBytes := make([]byte, 24)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	agent := model.Agent{
		ID:        uuid.New().String(),
		Name:      strings.TrimSpace(req.Name),
		Token:     token,
		Tags:      strings.TrimSpace(req.Tags),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.DB.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	appURL := db.GetSetting("app_url")
	if appURL == "" {
		appURL = fmt.Sprintf("http://%s", c.Request.Host)
	}

	installCmd := fmt.Sprintf("curl -fsSL %s/install.sh | sudo bash -s -- --server %s --token %s", appURL, appURL, agent.Token)

	c.JSON(http.StatusCreated, gin.H{
		"agent":           agent,
		"token":           token, // Disclose token only once on creation
		"install_command": installCmd,
	})
}

func adminRotateAgentToken(c *gin.Context) {
	id := c.Param("id")
	var agent model.Agent
	if err := db.DB.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	tokenBytes := make([]byte, 24)
	_, _ = rand.Read(tokenBytes)
	newToken := hex.EncodeToString(tokenBytes)

	agent.Token = newToken
	agent.UpdatedAt = time.Now()
	if err := db.DB.Save(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate token"})
		return
	}

	// Kick active connection to enforce new token
	ws.DefaultHub.Disconnect(agent.ID)

	appURL := db.GetSetting("app_url")
	if appURL == "" {
		appURL = fmt.Sprintf("http://%s", c.Request.Host)
	}
	installCmd := fmt.Sprintf("curl -fsSL %s/install.sh | sudo bash -s -- --server %s --token %s", appURL, appURL, newToken)

	c.JSON(http.StatusOK, gin.H{
		"agent_id":        agent.ID,
		"token":           newToken,
		"install_command": installCmd,
	})
}

func adminUpdateAgent(c *gin.Context) {
	id := c.Param("id")
	var agent model.Agent
	if err := db.DB.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	var req struct {
		Name                 string  `json:"name"`
		Tags                 string  `json:"tags"`
		DisabledRouteTargets *string `json:"disabled_route_targets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		agent.Name = req.Name
	}
	agent.Tags = req.Tags
	if req.DisabledRouteTargets != nil {
		agent.DisabledRouteTargets = *req.DisabledRouteTargets
	}
	agent.UpdatedAt = time.Now()

	db.DB.Save(&agent)
	ws.DefaultHub.SyncSingleAgent(agent.ID)
	c.JSON(http.StatusOK, agent)
}

func adminDeleteAgent(c *gin.Context) {
	id := c.Param("id")
	ws.DefaultHub.Disconnect(id)
	db.DB.Delete(&model.Agent{}, "id = ?", id)
	db.DB.Delete(&model.PingMetric{}, "agent_id = ?", id)
	c.JSON(http.StatusOK, gin.H{"message": "Agent removed"})
}
