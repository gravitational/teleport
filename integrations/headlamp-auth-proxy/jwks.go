package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// TeleportClaims are the claims extracted from the Teleport app access JWT.
// The JWT is injected by the Teleport proxy — we trust it without signature
// verification since the auth-proxy is only reachable through Teleport.
type TeleportClaims struct {
	jwt.RegisteredClaims
	Username string              `json:"username"`
	Roles    []string            `json:"roles"`
	Traits   map[string][]string `json:"traits"`
}

// ParseTeleportJWT decodes the JWT payload without signature verification.
// Teleport app access guarantees that the header is authentic.
func ParseTeleportJWT(tokenString string) (*TeleportClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT payload: %w", err)
	}

	var claims TeleportClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parsing JWT claims: %w", err)
	}

	if claims.Username == "" {
		return nil, fmt.Errorf("JWT missing username claim")
	}

	return &claims, nil
}
