package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/auth"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

func handleOAuthLoginSuccess(c *gin.Context, u *auth.AuthUser, state string) {
	// For non-GitHub OAuth, if email is present, use Gravatar as avatar source
	if u.Provider != "github" && u.Email != "" {
		u.AvatarURL = auth.GetGravatarURL(u.Email)
	}

	isInit := db.GetSetting("is_initialized") == "true"

	if !isInit || state == "setup" {
		user := setupSuperadminUser(u)
		tokenStr, _ := auth.GenerateToken(&user)
		c.SetCookie(auth.CookieSessionName, tokenStr, 7*86400, "/", "", false, true)
		c.Redirect(http.StatusTemporaryRedirect, "/admin")
		return
	}

	// Normal login: verify email whitelist
	var matchedEmail string
	for _, email := range u.VerifiedEmails {
		var wl model.EmailWhitelist
		if err := db.DB.Where("email = ?", strings.ToLower(email)).First(&wl).Error; err == nil {
			matchedEmail = email
			break
		}
	}

	if matchedEmail == "" {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/login?error=email_not_whitelisted&email=%s", u.Email))
		return
	}

	user := upsertAdminUser(u, matchedEmail)
	tokenStr, _ := auth.GenerateToken(&user)
	c.SetCookie(auth.CookieSessionName, tokenStr, 7*86400, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, "/admin")
}

func setupSuperadminUser(u *auth.AuthUser) model.User {
	var user model.User
	res := db.DB.Where("provider = ? AND provider_id = ?", u.Provider, u.ProviderUserID).First(&user)
	if res.Error != nil {
		user = model.User{
			Provider:    u.Provider,
			ProviderID:  u.ProviderUserID,
			Username:    u.Username,
			Email:       u.Email,
			AvatarURL:   u.AvatarURL,
			Role:        "superadmin",
			CreatedAt:   time.Now(),
			LastLoginAt: time.Now(),
		}
		db.DB.Create(&user)
	} else {
		user.Role = "superadmin"
		user.LastLoginAt = time.Now()
		user.Username = u.Username
		user.AvatarURL = u.AvatarURL
		db.DB.Save(&user)
	}

	var wl model.EmailWhitelist
	if err := db.DB.Where("email = ?", strings.ToLower(u.Email)).First(&wl).Error; err != nil {
		db.DB.Create(&model.EmailWhitelist{
			Email:     strings.ToLower(u.Email),
			Remark:    "Initial Superadmin",
			CreatedBy: "system",
			CreatedAt: time.Now(),
		})
	}

	_ = db.SetSetting("is_initialized", "true")
	return user
}

func upsertAdminUser(u *auth.AuthUser, matchedEmail string) model.User {
	var user model.User
	if err := db.DB.Where("provider = ? AND provider_id = ?", u.Provider, u.ProviderUserID).First(&user).Error; err != nil {
		user = model.User{
			Provider:    u.Provider,
			ProviderID:  u.ProviderUserID,
			Username:    u.Username,
			Email:       matchedEmail,
			AvatarURL:   u.AvatarURL,
			Role:        "admin",
			CreatedAt:   time.Now(),
			LastLoginAt: time.Now(),
		}
		db.DB.Create(&user)
	} else {
		user.LastLoginAt = time.Now()
		user.Username = u.Username
		user.AvatarURL = u.AvatarURL
		db.DB.Save(&user)
	}
	return user
}
