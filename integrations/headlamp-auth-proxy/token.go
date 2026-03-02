package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenSigner mints and validates HMAC-signed JWTs that carry user identity
// between the front proxy and the K8s proxy within the same pod.
type TokenSigner struct {
	key []byte
}

// InternalClaims are the claims encoded in the internal HMAC token.
// The token is a valid JWT so Headlamp's /me endpoint can decode it.
type InternalClaims struct {
	jwt.RegisteredClaims
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
}

// Mint creates a new HMAC-signed JWT encoding the user identity.
func (s *TokenSigner) Mint(username string, groups []string) (string, error) {
	now := time.Now()
	claims := InternalClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
		Username: username,
		Email:    username,
		Groups:   groups,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.key)
}

// Validate parses and validates an internal HMAC token.
func (s *TokenSigner) Validate(tokenString string) (*InternalClaims, error) {
	var claims InternalClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.key, nil
	})
	if err != nil {
		return nil, err
	}
	return &claims, nil
}
