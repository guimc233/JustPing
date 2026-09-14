package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/auth"
	"github.com/guimc233/JustPing/host/internal/model"
	"golang.org/x/oauth2"
)

func githubLogin(c *gin.Context) {
	cfg, err := auth.GetOAuthConfig()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/setup?error=oauth_not_configured")
		return
	}
	state := c.DefaultQuery("state", "login")
	c.Redirect(http.StatusTemporaryRedirect, cfg.AuthCodeURL(state, oauth2.AccessTypeOnline))
}

func githubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=missing_code")
		return
	}

	cfg, err := auth.GetOAuthConfig()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/setup?error=oauth_not_configured")
		return
	}

	token, err := cfg.Exchange(c.Request.Context(), code)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/login?error=token_exchange_failed&details=%s", err.Error()))
		return
	}

	ghUser, verifiedEmails, err := auth.FetchGitHubUserInfo(c.Request.Context(), token)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/login?error=fetch_user_failed&details=%s", err.Error()))
		return
	}

	primaryEmail := ghUser.Email
	if primaryEmail == "" && len(verifiedEmails) > 0 {
		primaryEmail = verifiedEmails[0]
	}
	if primaryEmail == "" {
		primaryEmail = fmt.Sprintf("%s@users.noreply.github.com", ghUser.Login)
	}

	authUser := &auth.AuthUser{
		Provider:       "github",
		ProviderUserID: fmt.Sprintf("%d", ghUser.ID),
		Username:       ghUser.Login,
		Email:          primaryEmail,
		AvatarURL:      ghUser.AvatarURL,
		VerifiedEmails: verifiedEmails,
	}

	handleOAuthLoginSuccess(c, authUser, state)
}

func googleLogin(c *gin.Context) {
	cfg, err := auth.GetGoogleOAuthConfig()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/setup?error=google_oauth_not_configured")
		return
	}
	state := c.DefaultQuery("state", "login")
	c.Redirect(http.StatusTemporaryRedirect, cfg.AuthCodeURL(state, oauth2.AccessTypeOnline))
}

func googleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	cfg, err := auth.GetGoogleOAuthConfig()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/setup?error=google_oauth_not_configured")
		return
	}
	token, err := cfg.Exchange(c.Request.Context(), code)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=google_exchange_failed")
		return
	}
	authUser, err := auth.FetchGoogleUserInfo(c.Request.Context(), token)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=google_userinfo_failed")
		return
	}
	handleOAuthLoginSuccess(c, authUser, state)
}

func oidcLogin(c *gin.Context) {
	cfg, err := auth.GetGenericOAuthConfig()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/setup?error=oidc_not_configured")
		return
	}
	state := c.DefaultQuery("state", "login")
	c.Redirect(http.StatusTemporaryRedirect, cfg.AuthCodeURL(state, oauth2.AccessTypeOnline))
}

func oidcCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	cfg, err := auth.GetGenericOAuthConfig()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/setup?error=oidc_not_configured")
		return
	}
	token, err := cfg.Exchange(c.Request.Context(), code)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=oidc_exchange_failed")
		return
	}
	authUser, err := auth.FetchGenericOIDCUserInfo(c.Request.Context(), token)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=oidc_userinfo_failed")
		return
	}
	handleOAuthLoginSuccess(c, authUser, state)
}

func getCurrentUser(c *gin.Context) {
	val, exists := c.Get(auth.ContextUserKey)
	if !exists {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	user := val.(*model.User)
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user": gin.H{
			"id":         user.ID,
			"provider":   user.Provider,
			"username":   user.Username,
			"email":      user.Email,
			"avatar_url": user.AvatarURL,
			"role":       user.Role,
		},
	})
}

func logout(c *gin.Context) {
	c.SetCookie(auth.CookieSessionName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
