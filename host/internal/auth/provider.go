package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type AuthUser struct {
	Provider       string
	ProviderUserID string
	Username       string
	Email          string
	AvatarURL      string
	VerifiedEmails []string
}

// GetGravatarURL generates a Gravatar avatar URL using SHA-256 of the email
func GetGravatarURL(email string) string {
	cleanEmail := strings.TrimSpace(strings.ToLower(email))
	if cleanEmail == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(cleanEmail))
	return fmt.Sprintf("https://www.gravatar.com/avatar/%s?d=identicon", hex.EncodeToString(hash[:]))
}
