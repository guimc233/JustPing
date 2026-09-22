package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/feature"
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

	statuses := ws.DefaultHub.UpdateStatuses()
	for i := range agents {
		agents[i].UnsupportedFeatures = feature.Unsupported(agents[i].Version)
		agents[i].ForceRestartMode = string(feature.ForceRestartModeFor(agents[i].Version, agents[i].Arch))
		if status, ok := statuses[agents[i].ID]; ok {
			copied := status
			agents[i].UpdateStatus = &copied
		}
	}

	c.JSON(http.StatusOK, agents)
}

// adminListFeatures reports the tracked probe capabilities and the first probe
// release supporting each one, so the UI can explain why a control is disabled.
func adminListFeatures(c *gin.Context) {
	c.JSON(http.StatusOK, feature.All())
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

// adminAgentUpdateCheck asks a single probe to check for the latest release and
// install it immediately through the native update_check message. The probe
// reports the outcome asynchronously.
func adminAgentUpdateCheck(c *gin.Context) {
	id := c.Param("id")
	var agent model.Agent
	if err := db.DB.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	if err := ws.DefaultHub.RequestUpdateCheck(agent.ID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"triggered": 1})
}

// adminAllAgentsUpdateCheck asks every probe that supports the native message to
// check for updates.
func adminAllAgentsUpdateCheck(c *gin.Context) {
	result := ws.DefaultHub.BroadcastUpdateCheck()
	c.JSON(http.StatusOK, gin.H{
		"triggered":   result.Triggered,
		"skipped":     result.Skipped,
		"unreachable": result.Unreachable,
	})
}

// adminAgentCrashUpdate forces a probe that cannot be driven through
// update_check to restart so its start-up auto-updater picks up the latest
// release.
//
// Probes that support soft_exit are asked to exit cleanly first; only if that
// times out is the probe crashed. This handler blocks for up to
// ws.SoftExitEscalationTimeout so the response can report which route was used.
func adminAgentCrashUpdate(c *gin.Context) {
	id := c.Param("id")
	var agent model.Agent
	if err := db.DB.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	mode, err := ws.DefaultHub.RequestRestart(agent.ID)
	switch {
	case errors.Is(err, ws.ErrAgentOffline):
		c.JSON(http.StatusConflict, gin.H{"error": ws.ErrAgentOffline.Error()})
		return
	case errors.Is(err, ws.ErrRestartUnsupported):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "mode": string(mode)})
		return
	case errors.Is(err, ws.ErrSoftExitEscalated):
		c.JSON(http.StatusOK, gin.H{
			"triggered": 1,
			"mode":      string(mode),
			"escalated": true,
			"message":   err.Error(),
		})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"triggered": 1, "mode": string(mode)})
}

// adminAllAgentsCrashUpdate force-restarts every probe that cannot be driven
// through update_check, preferring soft_exit and escalating to a crash.
func adminAllAgentsCrashUpdate(c *gin.Context) {
	c.JSON(http.StatusOK, ws.DefaultHub.BroadcastRestart())
}
