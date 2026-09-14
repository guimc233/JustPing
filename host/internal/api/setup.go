package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/db"
)

type SetupConfigRequest struct {
	GitHubClientID     string `json:"github_client_id"`
	GitHubClientSecret string `json:"github_client_secret"`
	GoogleClientID     string `json:"google_client_id,omitempty"`
	GoogleClientSecret string `json:"google_client_secret,omitempty"`
	OIDCClientID       string `json:"oidc_client_id,omitempty"`
	OIDCClientSecret   string `json:"oidc_client_secret,omitempty"`
	OIDCAuthURL        string `json:"oidc_auth_url,omitempty"`
	OIDCTokenURL       string `json:"oidc_token_url,omitempty"`
	OIDCUserInfoURL    string `json:"oidc_userinfo_url,omitempty"`
	OIDCName           string `json:"oidc_name,omitempty"`
	AppURL             string `json:"app_url"`
}

func getSetupStatus(c *gin.Context) {
	isInit := db.GetSetting("is_initialized") == "true"
	clientID := db.GetSetting("github_client_id")
	if clientID == "" {
		clientID = os.Getenv("GITHUB_CLIENT_ID")
	}
	googleID := db.GetSetting("google_client_id")
	if googleID == "" {
		googleID = os.Getenv("GOOGLE_CLIENT_ID")
	}
	oidcID := db.GetSetting("oidc_client_id")
	if oidcID == "" {
		oidcID = os.Getenv("OIDC_CLIENT_ID")
	}

	appURL := db.GetSetting("app_url")
	if appURL == "" {
		appURL = os.Getenv("APP_URL")
	}
	if appURL == "" {
		appURL = fmt.Sprintf("http://%s", c.Request.Host)
	}
	appURL = strings.TrimRight(appURL, "/")

	c.JSON(http.StatusOK, gin.H{
		"initialized":       isInit,
		"has_oauth_config":  clientID != "" || googleID != "" || oidcID != "",
		"github_configured": clientID != "",
		"google_configured": googleID != "",
		"oidc_configured":   oidcID != "",
		"app_url":           appURL,
		"oauth_callback_url": fmt.Sprintf("%s/api/auth/github/callback", appURL),
	})
}

func saveSetupConfig(c *gin.Context) {
	if db.GetSetting("is_initialized") == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Platform is already initialized"})
		return
	}

	var req SetupConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}

	appURL := strings.TrimRight(req.AppURL, "/")
	if appURL == "" {
		appURL = fmt.Sprintf("http://%s", c.Request.Host)
	}
	_ = db.SetSetting("app_url", appURL)

	if req.GitHubClientID != "" && req.GitHubClientSecret != "" {
		_ = db.SetSetting("github_client_id", strings.TrimSpace(req.GitHubClientID))
		_ = db.SetSetting("github_client_secret", strings.TrimSpace(req.GitHubClientSecret))
	}
	if req.GoogleClientID != "" && req.GoogleClientSecret != "" {
		_ = db.SetSetting("google_client_id", strings.TrimSpace(req.GoogleClientID))
		_ = db.SetSetting("google_client_secret", strings.TrimSpace(req.GoogleClientSecret))
	}
	if req.OIDCClientID != "" && req.OIDCClientSecret != "" {
		_ = db.SetSetting("oidc_client_id", strings.TrimSpace(req.OIDCClientID))
		_ = db.SetSetting("oidc_client_secret", strings.TrimSpace(req.OIDCClientSecret))
		_ = db.SetSetting("oidc_auth_url", strings.TrimSpace(req.OIDCAuthURL))
		_ = db.SetSetting("oidc_token_url", strings.TrimSpace(req.OIDCTokenURL))
		_ = db.SetSetting("oidc_userinfo_url", strings.TrimSpace(req.OIDCUserInfoURL))
		_ = db.SetSetting("oidc_name", strings.TrimSpace(req.OIDCName))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "OAuth credentials saved. Please complete the initial superadmin login.",
		"redirect_url": "/api/auth/github/login?state=setup",
	})
}
