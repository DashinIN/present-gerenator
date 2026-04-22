package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenKind string

const (
	AccessToken  TokenKind = "access"
	RefreshToken TokenKind = "refresh"
)

var ErrInvalidToken = errors.New("invalid token")

type JWTService struct {
	secret []byte
}

func NewJWTService(secret string) *JWTService {
	return &JWTService{secret: []byte(secret)}
}

type Claims struct {
	jwt.RegisteredClaims
	UserID int64     `json:"uid"`
	Kind   TokenKind `json:"kind"`
}

func (s *JWTService) Issue(userID int64, kind TokenKind, ttl time.Duration) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
		Kind:   kind,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (s *JWTService) Verify(tokenStr string, kind TokenKind) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || claims.Kind != kind {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
