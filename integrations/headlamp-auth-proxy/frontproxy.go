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
		// No Teleport JWT — forward as-is (health checks, probes).
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
