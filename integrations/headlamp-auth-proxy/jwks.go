package main

import (
	"crypto"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

// JWKSProvider fetches and caches Teleport's public keys for JWT validation.
type JWKSProvider struct {
	jwksURL string

	mu   sync.RWMutex
	keys map[string]crypto.PublicKey
}

// NewJWKSProvider creates a provider that fetches keys from the Teleport
// proxy's JWKS endpoint.
func NewJWKSProvider(teleportProxy string) (*JWKSProvider, error) {
	p := &JWKSProvider{
		jwksURL: fmt.Sprintf("https://%s/.well-known/jwks.json", teleportProxy),
		keys:    make(map[string]crypto.PublicKey),
	}

	// Initial fetch so we fail fast if the endpoint is unreachable.
	if err := p.refresh(); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch: %w", err)
	}

	go p.refreshLoop()
	return p, nil
}

func (p *JWKSProvider) refresh() error {
	resp, err := http.Get(p.jwksURL)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS: %w", err)
	}

	keys := make(map[string]crypto.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		pub, ok := k.Key.(crypto.PublicKey)
		if !ok {
			continue
		}
		keys[k.KeyID] = pub
	}

	p.mu.Lock()
	p.keys = keys
	p.mu.Unlock()

	slog.Info("refreshed JWKS", "keys", len(keys))
	return nil
}

func (p *JWKSProvider) refreshLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if err := p.refresh(); err != nil {
			slog.Error("JWKS refresh failed", "error", err)
		}
	}
}

// TeleportClaims are the claims extracted from the Teleport app access JWT.
type TeleportClaims struct {
	jwt.RegisteredClaims
	Username string            `json:"username"`
	Roles    []string          `json:"roles"`
	Traits   map[string][]string `json:"traits"`
}

// Validate validates the JWT and returns the parsed claims.
func (p *JWKSProvider) Validate(tokenString string) (*TeleportClaims, error) {
	var claims TeleportClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)

		p.mu.RLock()
		defer p.mu.RUnlock()

		if kid != "" {
			key, ok := p.keys[kid]
			if !ok {
				return nil, fmt.Errorf("unknown key ID %q", kid)
			}
			return key, nil
		}

		// No kid — try all keys (some Teleport versions omit kid).
		for _, key := range p.keys {
			return key, nil
		}
		return nil, fmt.Errorf("no keys available")
	})
	if err != nil {
		return nil, fmt.Errorf("validating JWT: %w", err)
	}
	return &claims, nil
}
