package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

// AuthRequired ensures user is logged in
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := extractUser(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized", "details": err.Error()})
			c.Abort()
			return
		}
		c.Set(ContextUserKey, user)
		c.Next()
	}
}

// SuperadminRequired ensures user has the superadmin role
func SuperadminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := extractUser(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		if user.Role != "superadmin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Superadmin permission required"})
			c.Abort()
			return
		}
		c.Set(ContextUserKey, user)
		c.Next()
	}
}

// OptionalAuth attaches user if authenticated, otherwise proceeds without error
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := extractUser(c)
		if err == nil && user != nil {
			c.Set(ContextUserKey, user)
		}
		c.Next()
	}
}

func extractUser(c *gin.Context) (*model.User, error) {
	var tokenStr string
	if cookie, err := c.Cookie(CookieSessionName); err == nil && cookie != "" {
		tokenStr = cookie
	}
	if tokenStr == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if tokenStr == "" {
		return nil, errors.New("no auth token provided")
	}

	claims, err := ParseToken(tokenStr)
	if err != nil {
		return nil, err
	}

	var user model.User
	if err := db.DB.First(&user, claims.UserID).Error; err != nil {
		return nil, errors.New("user not found")
	}
	return &user, nil
}
