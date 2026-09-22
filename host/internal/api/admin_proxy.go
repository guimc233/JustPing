package api

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/auth"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
	"github.com/guimc233/JustPing/host/internal/proxy"
	"github.com/guimc233/JustPing/host/internal/ws"
)

type createProxySessionReq struct {
	AgentID string `json:"agent_id" binding:"required"`
}

type proxySessionView struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Username  string `json:"username"`
	ExpiresAt string `json:"expires_at"`
	TTLSec    int    `json:"ttl_sec"`
}

type issuedProxySessionView struct {
	proxySessionView
	Password string `json:"password"`
	ProxyURL string `json:"proxy_url"`
}

func adminCreateProxySession(c *gin.Context) {
	var req createProxySessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}
	var agent model.Agent
	if err := db.DB.First(&agent, "id = ?", req.AgentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "probe not found"})
		return
	}
	if !ws.DefaultHub.Online(req.AgentID) {
		c.JSON(http.StatusConflict, gin.H{"error": "probe is offline"})
		return
	}
	user := c.MustGet(auth.ContextUserKey).(*model.User)
	issued, err := proxy.Default.Sessions().Issue(agent.ID, agent.Name, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue proxy credential"})
		return
	}
	c.JSON(http.StatusCreated, issuedProxySessionView{
		proxySessionView: viewSession(issued.Session),
		Password:         issued.Password,
		ProxyURL:         proxyURL(c.Request.Host, issued.Username, issued.Password),
	})
}

func adminListProxySessions(c *gin.Context) {
	sessions := proxy.Default.Sessions().List()
	out := make([]proxySessionView, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, viewSession(sess))
	}
	c.JSON(http.StatusOK, out)
}

func adminRevokeProxySession(c *gin.Context) {
	if !proxy.Default.Revoke(c.Param("id")) {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy session not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

func viewSession(sess proxy.Session) proxySessionView {
	return proxySessionView{
		ID:        sess.ID,
		AgentID:   sess.AgentID,
		AgentName: sess.AgentName,
		Username:  sess.Username,
		ExpiresAt: sess.ExpiresAt.UTC().Format(timeRFC3339),
		TTLSec:    int(proxy.SessionTTL.Seconds()),
	}
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

func proxyURL(host, username, password string) string {
	if host == "" {
		host = "127.0.0.1:8080"
	}
	u := url.URL{
		Scheme: "http",
		User:   url.UserPassword(username, password),
		Host:   host,
	}
	return u.String()
}
