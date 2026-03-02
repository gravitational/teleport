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
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// FrontProxy sits between Teleport and Headlamp. It verifies the Teleport
// JWT, mints an internal HMAC token, and injects it as a Headlamp session
// cookie and X-Auth-Token header before forwarding the request.
type FrontProxy struct {
	proxy    *httputil.ReverseProxy
	verifier *JWKSVerifier
	signer   *TokenSigner
}

// NewFrontProxy creates the HTTP proxy that faces Teleport.
func NewFrontProxy(verifier *JWKSVerifier, signer *TokenSigner) *FrontProxy {
	target, _ := url.Parse("http://" + headlampAddr)
	return &FrontProxy{
		verifier: verifier,
		signer:   signer,
		proxy:    httputil.NewSingleHostReverseProxy(target),
	}
}

func (fp *FrontProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jwtToken := r.Header.Get("teleport-jwt-assertion")
	if jwtToken == "" {
		// No Teleport JWT — forward as-is. This path is only hit by
		// Kubernetes health checks and readiness probes. Headlamp itself
		// does not expose sensitive data without a valid session cookie.
		fp.proxy.ServeHTTP(w, r)
		return
	}

	claims, err := fp.verifier.Verify(jwtToken)
	if err != nil {
		slog.Error("JWT verification failed", "error", err)
		http.Error(w, "invalid JWT", http.StatusUnauthorized)
		return
	}

	internalToken, err := fp.signer.Mint(claims.Username, claims.Roles)
	if err != nil {
		slog.Error("minting internal token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("authenticated request", "user", claims.Username, "groups", claims.Roles, "path", r.URL.Path)

	// Inject the cookie so Headlamp's /me endpoint shows user info.
	r.AddCookie(&http.Cookie{
		Name:     cookieName,
		Value:    internalToken,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	// Pass the token via a custom header that survives Headlamp's
	// internal reverse proxy (httputil.ReverseProxy preserves
	// non-hop-by-hop headers). The k8s-proxy reads this for impersonation.
	r.Header.Set("X-Auth-Token", internalToken)

	// Remove the Teleport JWT header — Headlamp doesn't need it.
	r.Header.Del("teleport-jwt-assertion")

	fp.proxy.ServeHTTP(w, r)
}
