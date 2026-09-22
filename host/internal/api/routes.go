package api

import (
	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/auth"
	"github.com/guimc233/JustPing/host/internal/ws"
)

func RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")

	// Agent WebSocket endpoint
	api.GET("/agent/ws", ws.DefaultHub.HandleWebSocket)

	// Setup Wizard endpoints
	api.GET("/setup/status", getSetupStatus)
	api.POST("/setup/config", saveSetupConfig)

	// Auth endpoints
	api.GET("/auth/github/login", githubLogin)
	api.GET("/auth/github/callback", githubCallback)
	api.GET("/auth/google/login", googleLogin)
	api.GET("/auth/google/callback", googleCallback)
	api.GET("/auth/oidc/login", oidcLogin)
	api.GET("/auth/oidc/callback", oidcCallback)
	api.GET("/auth/me", auth.OptionalAuth(), getCurrentUser)
	api.POST("/auth/logout", logout)

	// Public / Observer endpoints (IP masked for unauthenticated)
	public := api.Group("/public", auth.OptionalAuth())
	{
		public.GET("/summary", getPublicSummary)
		public.GET("/agents", getPublicAgents)
		public.GET("/targets", getPublicTargets)
		public.GET("/matrix", getPublicMatrix)
		public.GET("/metrics", getPublicMetrics)
		public.GET("/traceroute", getPublicTraceroute)
		public.GET("/traceroutes", getPublicTraceroutes)
		public.GET("/traceroute/:id", getPublicTracerouteByID)
	}

	// Admin endpoints (Protected)
	admin := api.Group("/admin", auth.AuthRequired())
	{
		// Targets
		admin.GET("/targets", adminListTargets)
		admin.POST("/targets", adminCreateTarget)
		admin.PUT("/targets/:id", adminUpdateTarget)
		admin.DELETE("/targets/:id", adminDeleteTarget)

		// Agents
		admin.GET("/agents", adminListAgents)
		admin.POST("/agents", adminCreateAgent)
		admin.POST("/agents/update-check", adminAllAgentsUpdateCheck)
		admin.POST("/agents/:id/rotate-token", adminRotateAgentToken)
		admin.POST("/agents/:id/update-check", adminAgentUpdateCheck)
		admin.PUT("/agents/:id", adminUpdateAgent)
		admin.DELETE("/agents/:id", adminDeleteAgent)

		// Capability registry (feature -> first supporting probe version)
		admin.GET("/features", adminListFeatures)

		admin.POST("/proxy/sessions", adminCreateProxySession)
		admin.GET("/proxy/sessions", adminListProxySessions)
		admin.DELETE("/proxy/sessions/:id", adminRevokeProxySession)

		// Email Whitelist
		admin.GET("/whitelist", adminListWhitelist)
		admin.POST("/whitelist", adminCreateWhitelist)
		admin.DELETE("/whitelist/:id", adminDeleteWhitelist)

		// Settings (Superadmin only)
		super := admin.Group("/settings", auth.SuperadminRequired())
		{
			super.GET("", adminGetSettings)
			super.POST("", adminSaveSettings)
		}
	}
}
