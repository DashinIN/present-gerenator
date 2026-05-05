package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type OAuthStateService struct {
	secret []byte
}

type oauthStatePayload struct {
	ReturnTo string `json:"return_to"`
	Expires  int64  `json:"exp"`
}

func NewOAuthStateService(secret string) *OAuthStateService {
	return &OAuthStateService{secret: []byte(secret)}
}

func (s *OAuthStateService) Encode(returnTo string) (string, error) {
	payload := oauthStatePayload{
		ReturnTo: returnTo,
		Expires:  time.Now().Add(10 * time.Minute).Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal state payload: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	return body + "." + s.sign(body), nil
}

func (s *OAuthStateService) Decode(state string) (string, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid state format")
	}

	body, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sig), []byte(s.sign(body))) {
		return "", fmt.Errorf("invalid state signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", fmt.Errorf("decode state payload: %w", err)
	}

	var payload oauthStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("unmarshal state payload: %w", err)
	}
	if payload.Expires < time.Now().Unix() {
		return "", fmt.Errorf("state expired")
	}
	if strings.TrimSpace(payload.ReturnTo) == "" {
		return "/", nil
	}
	return payload.ReturnTo, nil
}

func (s *OAuthStateService) sign(body string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
