package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

const (
	CookieSessionName = "justping_token"
	ContextUserKey    = "current_user"
)

type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	GitHubID int64  `json:"github_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// GetJWTSecret gets the configured or seeded JWT signing secret
func GetJWTSecret() []byte {
	secret := db.GetSetting("jwt_secret")
	if secret != "" {
		return []byte(secret)
	}
	secret = os.Getenv("JWT_SECRET")
	if secret != "" {
		return []byte(secret)
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	secret = hex.EncodeToString(b)
	_ = db.SetSetting("jwt_secret", secret)
	return []byte(secret)
}

// GenerateToken creates a signed JWT for an authenticated user
func GenerateToken(user *model.User) (string, error) {
	claims := JWTClaims{
		UserID:   user.ID,
		GitHubID: user.GitHubID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}

// ParseToken parses and validates a JWT token string
func ParseToken(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return GetJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
