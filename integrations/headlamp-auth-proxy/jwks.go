/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
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
	expires time.Time
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

// keys returns the cached JWKS key set, refreshing it from the Teleport
// proxy if older than 5 minutes.
func (v *JWKSVerifier) keys() jose.JSONWebKeySet {
	v.mu.RLock()
	if time.Now().Before(v.expires) {
		ks := v.keySet
		v.mu.RUnlock()
		return ks
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check after acquiring write lock.
	if time.Now().Before(v.expires) {
		return v.keySet
	}

	if err := v.refresh(); err != nil {
		slog.Error("JWKS refresh failed, using cached keys", "error", err)
	}
	return v.keySet
}

// Verify parses the raw JWT string, verifies its signature against the cached
// JWKS keys, validates expiry, and returns the Teleport claims.
func (v *JWKSVerifier) Verify(rawToken string) (*TeleportClaims, error) {
	tok, err := jwt.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256, jose.ES256})
	if err != nil {
		return nil, fmt.Errorf("parsing JWT: %w", err)
	}

	keys := v.keys()

	var claims TeleportClaims
	var registered jwt.Claims

	if err := tok.Claims(keys, &registered, &claims); err != nil {
		return nil, fmt.Errorf("verifying JWT signature: %w", err)
	}

	// Allow 1 minute of clock skew between the Teleport proxy and this pod.
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return fmt.Errorf("reading JWKS response: %w", err)
	}

	var keySet jose.JSONWebKeySet
	if err := json.Unmarshal(body, &keySet); err != nil {
		return fmt.Errorf("parsing JWKS: %w", err)
	}

	v.keySet = keySet
	v.expires = time.Now().Add(5 * time.Minute)
	slog.Info("JWKS refreshed", "keys", len(keySet.Keys))
	return nil
}
