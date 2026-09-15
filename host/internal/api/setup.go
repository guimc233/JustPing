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
	PrimaryProvider    string `json:"primary_provider"` // "github", "google", "oidc"
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
		"initialized":         isInit,
		"has_oauth_config":    clientID != "" || googleID != "" || oidcID != "",
		"github_configured":   clientID != "",
		"google_configured":   googleID != "",
		"oidc_configured":     oidcID != "",
		"app_url":             appURL,
		"github_callback_url": fmt.Sprintf("%s/api/auth/github/callback", appURL),
		"google_callback_url": fmt.Sprintf("%s/api/auth/google/callback", appURL),
		"oidc_callback_url":   fmt.Sprintf("%s/api/auth/oidc/callback", appURL),
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
	if err := db.SetSetting("app_url", appURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist app_url"})
		return
	}

	var saveErr error
	if req.GitHubClientID != "" && req.GitHubClientSecret != "" {
		saveErr = joinErrors(saveErr, db.SetSetting("github_client_id", strings.TrimSpace(req.GitHubClientID)))
		saveErr = joinErrors(saveErr, db.SetSetting("github_client_secret", strings.TrimSpace(req.GitHubClientSecret)))
	}
	if req.GoogleClientID != "" && req.GoogleClientSecret != "" {
		saveErr = joinErrors(saveErr, db.SetSetting("google_client_id", strings.TrimSpace(req.GoogleClientID)))
		saveErr = joinErrors(saveErr, db.SetSetting("google_client_secret", strings.TrimSpace(req.GoogleClientSecret)))
	}
	if req.OIDCClientID != "" && req.OIDCClientSecret != "" {
		saveErr = joinErrors(saveErr, db.SetSetting("oidc_client_id", strings.TrimSpace(req.OIDCClientID)))
		saveErr = joinErrors(saveErr, db.SetSetting("oidc_client_secret", strings.TrimSpace(req.OIDCClientSecret)))
		saveErr = joinErrors(saveErr, db.SetSetting("oidc_auth_url", strings.TrimSpace(req.OIDCAuthURL)))
		saveErr = joinErrors(saveErr, db.SetSetting("oidc_token_url", strings.TrimSpace(req.OIDCTokenURL)))
		saveErr = joinErrors(saveErr, db.SetSetting("oidc_userinfo_url", strings.TrimSpace(req.OIDCUserInfoURL)))
		saveErr = joinErrors(saveErr, db.SetSetting("oidc_name", strings.TrimSpace(req.OIDCName)))
	}

	if saveErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save OAuth settings: " + saveErr.Error()})
		return
	}

	redirectURL := "/api/auth/github/login"
	if req.PrimaryProvider == "google" {
		redirectURL = "/api/auth/google/login"
	} else if req.PrimaryProvider == "oidc" {
		redirectURL = "/api/auth/oidc/login"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "OAuth credentials saved. Complete initial login to finish setup.",
		"redirect_url": redirectURL,
	})
}

func joinErrors(e1, e2 error) error {
	if e1 == nil {
		return e2
	}
	if e2 == nil {
		return e1
	}
	return fmt.Errorf("%v; %w", e1, e2)
}
