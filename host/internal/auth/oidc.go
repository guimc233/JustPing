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
)

type oidcUserInfo struct {
	Sub               string `json:"sub"`
	ID                any    `json:"id"`
	Email             string `json:"email"`
	EmailVerified     *bool  `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Picture           string `json:"picture"`
	AvatarURL         string `json:"avatar_url"`
}

func GetGenericOAuthConfig() (*oauth2.Config, error) {
	clientID := db.GetSetting("oidc_client_id")
	if clientID == "" {
		clientID = os.Getenv("OIDC_CLIENT_ID")
	}
	clientSecret := db.GetSetting("oidc_client_secret")
	if clientSecret == "" {
		clientSecret = os.Getenv("OIDC_CLIENT_SECRET")
	}
	authURL := db.GetSetting("oidc_auth_url")
	if authURL == "" {
		authURL = os.Getenv("OIDC_AUTH_URL")
	}
	tokenURL := db.GetSetting("oidc_token_url")
	if tokenURL == "" {
		tokenURL = os.Getenv("OIDC_TOKEN_URL")
	}
	if clientID == "" || clientSecret == "" || authURL == "" || tokenURL == "" {
		return nil, errors.New("generic oidc is not fully configured")
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
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		RedirectURL: fmt.Sprintf("%s/api/auth/oidc/callback", appURL),
	}, nil
}

func FetchGenericOIDCUserInfo(ctx context.Context, token *oauth2.Token) (*AuthUser, error) {
	userInfoURL := db.GetSetting("oidc_userinfo_url")
	if userInfoURL == "" {
		userInfoURL = os.Getenv("OIDC_USERINFO_URL")
	}
	if userInfoURL == "" {
		return nil, errors.New("oidc userinfo url is not configured")
	}

	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get(userInfoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned status %d", resp.StatusCode)
	}

	var info oidcUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	email := strings.ToLower(info.Email)
	if email == "" {
		return nil, errors.New("no email found in oidc userinfo")
	}

	username := info.PreferredUsername
	if username == "" {
		username = info.Name
	}
	if username == "" {
		username = email
	}

	avatar := info.Picture
	if avatar == "" {
		avatar = info.AvatarURL
	}

	sub := info.Sub
	if sub == "" && info.ID != nil {
		sub = fmt.Sprintf("%v", info.ID)
	}

	providerName := db.GetSetting("oidc_name")
	if providerName == "" {
		providerName = "oidc"
	}

	return &AuthUser{
		Provider:       providerName,
		ProviderUserID: sub,
		Username:       username,
		Email:          email,
		AvatarURL:      avatar,
		VerifiedEmails: []string{email},
	}, nil
}
