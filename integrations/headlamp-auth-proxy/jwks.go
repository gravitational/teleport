package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// TeleportClaims are the claims extracted from the Teleport app access JWT.
type TeleportClaims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

// JWKSVerifier verifies Teleport app access JWTs against JWKS public keys
// fetched from the Teleport proxy's /.well-known/jwks.json endpoint.
type JWKSVerifier struct {
	mu      sync.RWMutex
	keySet  jose.JSONWebKeySet
	jwksURL string
	client  http.Client
}

// NewJWKSVerifier creates a verifier and performs an initial JWKS fetch.
// It returns an error if the initial fetch fails (fail-fast at startup).
func NewJWKSVerifier(proxyAddr string) (*JWKSVerifier, error) {
	v := &JWKSVerifier{
		jwksURL: proxyAddr + "/.well-known/jwks.json",
		client:  http.Client{Timeout: 10 * time.Second},
	}

	if err := v.refresh(); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch: %w", err)
	}

	return v, nil
}

// RefreshLoop periodically refreshes the JWKS key set. It blocks until ctx is cancelled.
func (v *JWKSVerifier) RefreshLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := v.refresh(); err != nil {
				slog.Error("JWKS refresh failed", "error", err)
			}
		}
	}
}

// Verify parses the raw JWT string, verifies its signature against the cached
// JWKS keys, validates expiry, and returns the Teleport claims.
func (v *JWKSVerifier) Verify(rawToken string) (*TeleportClaims, error) {
	tok, err := jwt.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256, jose.ES256})
	if err != nil {
		return nil, fmt.Errorf("parsing JWT: %w", err)
	}

	v.mu.RLock()
	keys := v.keySet
	v.mu.RUnlock()

	var claims TeleportClaims
	var registered jwt.Claims

	if err := tok.Claims(keys, &registered, &claims); err != nil {
		return nil, fmt.Errorf("verifying JWT signature: %w", err)
	}

	if err := registered.ValidateWithLeeway(jwt.Expected{Time: time.Now()}, time.Minute); err != nil {
		return nil, fmt.Errorf("validating JWT claims: %w", err)
	}

	if claims.Username == "" {
		return nil, fmt.Errorf("JWT missing username claim")
	}

	return &claims, nil
}

func (v *JWKSVerifier) refresh() error {
	resp, err := v.client.Get(v.jwksURL)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading JWKS response: %w", err)
	}

	var keySet jose.JSONWebKeySet
	if err := json.Unmarshal(body, &keySet); err != nil {
		return fmt.Errorf("parsing JWKS: %w", err)
	}

	v.mu.Lock()
	v.keySet = keySet
	v.mu.Unlock()

	slog.Info("JWKS refreshed", "keys", len(keySet.Keys))
	return nil
}
