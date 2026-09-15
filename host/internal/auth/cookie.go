package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/db"
)

const OAuthStateCookie = "justping_oauth_state"

// GenerateOAuthState creates a secure random nonce and sets an HttpOnly cookie
func GenerateOAuthState(c *gin.Context) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	state := hex.EncodeToString(b)

	isSecure := isHTTPSRequest(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(OAuthStateCookie, state, 600, "/api/auth", "", isSecure, true)
	return state
}

// ValidateOAuthState verifies that state parameter matches the signed/secure state cookie
func ValidateOAuthState(c *gin.Context, state string) bool {
	if state == "" {
		return false
	}
	cookie, err := c.Cookie(OAuthStateCookie)
	if err != nil || cookie == "" {
		return false
	}
	// Clear cookie immediately
	c.SetCookie(OAuthStateCookie, "", -1, "/api/auth", "", isHTTPSRequest(c), true)
	return cookie == state
}

// SetSessionCookie sets a secure, HttpOnly, SameSite=Lax session cookie
func SetSessionCookie(c *gin.Context, token string) {
	isSecure := isHTTPSRequest(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieSessionName, token, 7*86400, "/", "", isSecure, true)
}

// ClearSessionCookie removes the authentication session cookie
func ClearSessionCookie(c *gin.Context) {
	isSecure := isHTTPSRequest(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieSessionName, "", -1, "/", "", isSecure, true)
}

func isHTTPSRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		return true
	}
	appURL := db.GetSetting("app_url")
	if appURL == "" {
		appURL = os.Getenv("APP_URL")
	}
	return strings.HasPrefix(strings.ToLower(appURL), "https://")
}
