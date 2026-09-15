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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func handleOAuthLoginSuccess(c *gin.Context, u *auth.AuthUser) {
	if u.Provider != "github" && u.Email != "" {
		u.AvatarURL = auth.GetGravatarURL(u.Email)
	}

	var user model.User
	var wasBootstrap bool

	// Transactionally check if platform needs bootstrap
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var setting model.SystemSetting
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("key = ?", "is_initialized").
			First(&setting).Error

		isInit := err == nil && setting.Value == "true"
		if !isInit {
			user, err = bootstrapSuperadminTx(tx, u)
			if err != nil {
				return err
			}
			wasBootstrap = true
			return nil
		}
		return nil
	})

	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/login?error=auth_failed&details=%s", err.Error()))
		return
	}

	if wasBootstrap {
		tokenStr, err := auth.GenerateToken(&user)
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/login?error=token_generation_failed")
			return
		}
		auth.SetSessionCookie(c, tokenStr)
		c.Redirect(http.StatusTemporaryRedirect, "/admin")
		return
	}

	// Normal login: verify against email whitelist
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

	user = upsertAdminUser(u, matchedEmail)
	tokenStr, err := auth.GenerateToken(&user)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/login?error=token_generation_failed")
		return
	}
	auth.SetSessionCookie(c, tokenStr)
	c.Redirect(http.StatusTemporaryRedirect, "/admin")
}

func bootstrapSuperadminTx(tx *gorm.DB, u *auth.AuthUser) (model.User, error) {
	var user model.User
	res := tx.Where("provider = ? AND provider_id = ?", u.Provider, u.ProviderUserID).First(&user)
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
		if err := tx.Create(&user).Error; err != nil {
			return user, err
		}
	} else {
		user.Role = "superadmin"
		user.LastLoginAt = time.Now()
		user.Username = u.Username
		user.AvatarURL = u.AvatarURL
		if err := tx.Save(&user).Error; err != nil {
			return user, err
		}
	}

	var wl model.EmailWhitelist
	if err := tx.Where("email = ?", strings.ToLower(u.Email)).First(&wl).Error; err != nil {
		if err := tx.Create(&model.EmailWhitelist{
			Email:     strings.ToLower(u.Email),
			Remark:    "Initial Superadmin",
			CreatedBy: "system",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}).Error; err != nil {
			return user, err
		}
	}

	s := model.SystemSetting{Key: "is_initialized", Value: "true", UpdatedAt: time.Now()}
	if err := tx.Save(&s).Error; err != nil {
		return user, err
	}
	return user, nil
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
