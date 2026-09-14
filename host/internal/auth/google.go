package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/guimc233/JustPing/host/internal/db"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type googleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func GetGoogleOAuthConfig() (*oauth2.Config, error) {
	clientID := db.GetSetting("google_client_id")
	if clientID == "" {
		clientID = os.Getenv("GOOGLE_CLIENT_ID")
	}
	clientSecret := db.GetSetting("google_client_secret")
	if clientSecret == "" {
		clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	}
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("google oauth is not configured")
	}

	appURL := db.GetSetting("app_url")
	if appURL == "" {
		appURL = os.Getenv("APP_URL")
	}
	if appURL == "" {
		appURL = "http://localhost:8080"
	}
	appURL = strings.TrimRight(appURL, "/")

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint:     google.Endpoint,
		RedirectURL:  fmt.Sprintf("%s/api/auth/google/callback", appURL),
	}, nil
}

func FetchGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*AuthUser, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo returned status %d", resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	if !info.VerifiedEmail {
		return nil, errors.New("google email is not verified")
	}

	email := strings.ToLower(info.Email)
	return &AuthUser{
		Provider:       "google",
		ProviderUserID: info.ID,
		Username:       info.Name,
		Email:          email,
		AvatarURL:      info.Picture,
		VerifiedEmails: []string{email},
	}, nil
}
