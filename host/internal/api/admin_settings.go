package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

var allowedSettingsKeys = map[string]bool{
	"app_url":              true,
	"retention_days":       true,
	"github_client_id":     true,
	"github_client_secret": true,
	"google_client_id":     true,
	"google_client_secret": true,
	"oidc_client_id":       true,
	"oidc_client_secret":   true,
	"oidc_auth_url":        true,
	"oidc_token_url":       true,
	"oidc_userinfo_url":    true,
	"oidc_name":            true,
}

func adminGetSettings(c *gin.Context) {
	var settings []model.SystemSetting
	db.DB.Find(&settings)

	res := make(map[string]string)
	for _, s := range settings {
		if !allowedSettingsKeys[s.Key] {
			continue // Never expose internal keys like is_initialized or jwt_secret
		}
		if strings.Contains(s.Key, "secret") && len(s.Value) > 4 {
			res[s.Key] = s.Value[:2] + "********" + s.Value[len(s.Value)-2:]
		} else {
			res[s.Key] = s.Value
		}
	}
	c.JSON(http.StatusOK, res)
}

func adminSaveSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for k, v := range req {
		if !allowedSettingsKeys[k] {
			continue // Prevent tampering with is_initialized, jwt_secret, etc.
		}
		if strings.Contains(k, "secret") && strings.Contains(v, "********") {
			continue
		}
		_ = db.SetSetting(k, strings.TrimSpace(v))
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}
