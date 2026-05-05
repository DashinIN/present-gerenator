package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/you/fungreet/internal/models"
)

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

type GoogleOAuthService struct {
	client       *http.Client
	clientID     string
	clientSecret string
	redirectURI  string
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type googleUserInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func NewGoogleOAuthService(clientID, clientSecret, redirectURI string) *GoogleOAuthService {
	return &GoogleOAuthService{
		client:       &http.Client{Timeout: 10 * time.Second},
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		redirectURI:  strings.TrimSpace(redirectURI),
	}
}

func (s *GoogleOAuthService) Enabled() bool {
	return s != nil && s.clientID != "" && s.clientSecret != "" && s.redirectURI != ""
}

func (s *GoogleOAuthService) AuthCodeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", s.clientID)
	q.Set("redirect_uri", s.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "select_account")
	return googleAuthURL + "?" + q.Encode()
}

func (s *GoogleOAuthService) ExchangeCode(ctx context.Context, code string) (*models.OAuthProfile, error) {
	form := url.Values{}
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", s.redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request token: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: status=%d", res.StatusCode)
	}

	var token googleTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access token")
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	res, err = s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request userinfo: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed: status=%d", res.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode userinfo response: %w", err)
	}
	if info.Sub == "" || info.Email == "" {
		return nil, fmt.Errorf("userinfo missing required fields")
	}

	displayName := strings.TrimSpace(info.Name)
	if displayName == "" {
		displayName = info.Email
	}

	return &models.OAuthProfile{
		Provider:    "google",
		ProviderID:  info.Sub,
		Email:       info.Email,
		DisplayName: displayName,
		AvatarURL:   info.Picture,
	}, nil
}
