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
	"golang.org/x/oauth2/github"
)

type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type GitHubEmail struct {
	Email      string `json:"email"`
	Primary    bool   `json:"primary"`
	Verified   bool   `json:"verified"`
	Visibility string `json:"visibility"`
}

// GetOAuthConfig builds the OAuth2 config using DB settings or ENV fallbacks
func GetOAuthConfig() (*oauth2.Config, error) {
	clientID := db.GetSetting("github_client_id")
	if clientID == "" {
		clientID = os.Getenv("GITHUB_CLIENT_ID")
	}
	clientSecret := db.GetSetting("github_client_secret")
	if clientSecret == "" {
		clientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	}
	appURL := db.GetSetting("app_url")
	if appURL == "" {
		appURL = os.Getenv("APP_URL")
	}
	if appURL == "" {
		appURL = "http://localhost:8080"
	}
	appURL = strings.TrimRight(appURL, "/")

	if clientID == "" || clientSecret == "" {
		return nil, errors.New("github oauth is not configured yet")
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint:     github.Endpoint,
		RedirectURL:  fmt.Sprintf("%s/api/auth/github/callback", appURL),
	}, nil
}

// FetchGitHubUserInfo queries GitHub API with access token
func FetchGitHubUserInfo(ctx context.Context, token *oauth2.Token) (*GitHubUser, []string, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var ghUser GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, nil, err
	}

	respEmail, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch user emails: %w", err)
	}
	defer respEmail.Body.Close()
	if respEmail.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("github emails api returned status %d", respEmail.StatusCode)
	}

	var emails []GitHubEmail
	if err := json.NewDecoder(respEmail.Body).Decode(&emails); err != nil {
		return nil, nil, fmt.Errorf("failed to decode user emails: %w", err)
	}

	var verifiedEmails []string
	for _, e := range emails {
		if e.Verified {
			clean := strings.ToLower(strings.TrimSpace(e.Email))
			if clean != "" {
				verifiedEmails = append(verifiedEmails, clean)
				if e.Primary {
					ghUser.Email = clean
				}
			}
		}
	}

	if ghUser.Email == "" && len(verifiedEmails) > 0 {
		ghUser.Email = verifiedEmails[0]
	}

	return &ghUser, verifiedEmails, nil
}
