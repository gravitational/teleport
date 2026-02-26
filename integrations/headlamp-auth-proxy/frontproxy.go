package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// FrontProxy sits between Teleport and Headlamp. It validates the Teleport
// JWT, mints an internal HMAC token, and injects it as a Headlamp session
// cookie before forwarding the request.
type FrontProxy struct {
	proxy       *httputil.ReverseProxy
	jwks        *JWKSProvider
	signer      *TokenSigner
	groupsClaim string
	cookieName  string
}

// NewFrontProxy creates the HTTP proxy that faces Teleport.
func NewFrontProxy(headlampAddr string, jwks *JWKSProvider, signer *TokenSigner, groupsClaim, cookieName string) (*FrontProxy, error) {
	target, err := url.Parse(fmt.Sprintf("http://%s", headlampAddr))
	if err != nil {
		return nil, fmt.Errorf("parsing headlamp address: %w", err)
	}

	fp := &FrontProxy{
		jwks:        jwks,
		signer:      signer,
		groupsClaim: groupsClaim,
		cookieName:  cookieName,
	}

	fp.proxy = httputil.NewSingleHostReverseProxy(target)
	return fp, nil
}

func (fp *FrontProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jwtToken := r.Header.Get("teleport-jwt-assertion")
	if jwtToken == "" {
		http.Error(w, "missing teleport-jwt-assertion header", http.StatusUnauthorized)
		return
	}

	claims, err := fp.jwks.Validate(jwtToken)
	if err != nil {
		slog.Error("JWT validation failed", "error", err)
		http.Error(w, "invalid JWT", http.StatusUnauthorized)
		return
	}

	groups := fp.extractGroups(claims)

	internalToken, err := fp.signer.Mint(claims.Username, groups)
	if err != nil {
		slog.Error("minting internal token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Inject the cookie so Headlamp treats the user as authenticated.
	r.AddCookie(&http.Cookie{
		Name:  fp.cookieName,
		Value: internalToken,
	})

	// Remove the Teleport JWT header — Headlamp doesn't need it.
	r.Header.Del("teleport-jwt-assertion")

	fp.proxy.ServeHTTP(w, r)
}

func (fp *FrontProxy) extractGroups(claims *TeleportClaims) []string {
	if fp.groupsClaim == "roles" {
		return claims.Roles
	}

	// Support traits.<key> syntax (e.g. "traits.groups").
	if strings.HasPrefix(fp.groupsClaim, "traits.") {
		key := strings.TrimPrefix(fp.groupsClaim, "traits.")
		if vals, ok := claims.Traits[key]; ok {
			return vals
		}
	}

	return nil
}
