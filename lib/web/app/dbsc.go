// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
)

const (
	// secureSessionRegistrationHeader is the header that triggers DBSC registration in the browser.
	secureSessionRegistrationHeader = "Secure-Session-Registration"
	// secureSessionResponseHeader is the header containing the browser's signed JWT.
	secureSessionResponseHeader = "Secure-Session-Response"
	// secureSessionIDHeader is the header containing the session ID for refresh requests.
	secureSessionIDHeader = "Sec-Secure-Session-Id"
	// secureSessionChallengeHeader is the header containing a challenge for refresh.
	secureSessionChallengeHeader = "Secure-Session-Challenge"
	// dbscCookieMaxAge is the short-lived cookie max age when DBSC is active.
	dbscCookieMaxAge = 10 * time.Minute
	// dbscRegistrationPath is the DBSC registration endpoint.
	dbscRegistrationPath = "/x-teleport-dbsc"
	// dbscRefreshPath is the DBSC refresh endpoint.
	dbscRefreshPath = "/x-teleport-dbsc/refresh"
)

// dbscSessionConfig is the JSON response returned after successful DBSC registration.
type dbscSessionConfig struct {
	SessionIdentifier string           `json:"session_identifier"`
	RefreshURL        string           `json:"refresh_url"`
	Scope             dbscScope        `json:"scope"`
	Credentials       []dbscCredential `json:"credentials"`
}

// dbscScope defines which resources are covered by the DBSC session.
type dbscScope struct {
	IncludeSite bool `json:"include_site"`
}

// dbscCredential describes a cookie protected by DBSC.
type dbscCredential struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	Attributes string `json:"attributes"`
}

// handleDBSCRegistration handles DBSC registration from the browser.
// The browser POSTs a signed JWT proving possession of the private key.
func (h *Handler) handleDBSCRegistration(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	ctx := r.Context()

	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return trace.AccessDenied("missing session cookie")
	}
	sessionID := cookie.Value

	subjectCookie, err := r.Cookie(SubjectCookieName)
	if err != nil {
		return trace.AccessDenied("missing subject session cookie")
	}

	ws, err := h.getAppSessionFromAccessPoint(ctx, sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkSubjectToken(subjectCookie.Value, ws); err != nil {
		return trace.Wrap(err)
	}

	responseJWT, ok, err := getDBSCHeaderString(r, secureSessionResponseHeader)
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		return trace.BadParameter("missing %s header", secureSessionResponseHeader)
	}

	if err := h.c.AuthClient.SetAppSessionDBSCPublicKey(ctx, sessionID, []byte(responseJWT)); err != nil {
		return trace.Wrap(err)
	}

	setAppSessionCookies(w, ws, dbscCookieMaxAge)

	return h.writeDBSCSessionConfig(w, sessionID)
}

// handleDBSCRefresh handles DBSC session refresh requests.
// This is a two-step flow:
// 1. Browser POSTs with Secure-Session-Id but no response -> return 403 with challenge
// 2. Browser POSTs with Secure-Session-Response -> verify and re-issue cookies
func (h *Handler) handleDBSCRefresh(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	ctx := r.Context()

	sessionID, ok, err := getDBSCHeaderString(r, secureSessionIDHeader)
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		return trace.BadParameter("missing %s header", secureSessionIDHeader)
	}

	ws, err := h.getAppSessionFromAccessPoint(ctx, sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	storedPublicKey := ws.GetDBSCPublicKey()
	if len(storedPublicKey) == 0 {
		return trace.BadParameter("session does not have DBSC enabled")
	}

	responseJWT, ok, err := getDBSCHeaderString(r, secureSessionResponseHeader)
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		challenge, err := h.c.AuthClient.SignDBSCChallenge(ctx, sessionID)
		if err != nil {
			return trace.Wrap(err)
		}
		w.Header().Set(secureSessionChallengeHeader, fmt.Sprintf(`%s;id=%s`, strconv.Quote(challenge), strconv.Quote(sessionID)))
		w.WriteHeader(http.StatusForbidden)
		return nil
	}

	if err := h.verifyDBSCRefreshResponse(ctx, responseJWT, sessionID, dbscRequestURL(r), storedPublicKey); err != nil {
		return trace.Wrap(err)
	}

	setAppSessionCookies(w, ws, dbscCookieMaxAge)

	// This is optional, per spec.
	return h.writeDBSCSessionConfig(w, sessionID)
}

// writeDBSCSessionConfig writes the session configuration JSON response.
func (h *Handler) writeDBSCSessionConfig(w http.ResponseWriter, sessionID string) error {
	cookieAttributes := "Path=/; HttpOnly; Secure; SameSite=None"
	config := dbscSessionConfig{
		SessionIdentifier: sessionID,
		RefreshURL:        dbscRefreshPath,
		Scope: dbscScope{
			IncludeSite: false,
		},
		Credentials: []dbscCredential{
			{
				Type:       "cookie",
				Name:       CookieName,
				Attributes: cookieAttributes,
			},
			{
				Type:       "cookie",
				Name:       SubjectCookieName,
				Attributes: cookieAttributes,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(config); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// verifyDBSCRefreshResponse verifies the browser's DBSC refresh JWT using the stored public key.
func (h *Handler) verifyDBSCRefreshResponse(ctx context.Context, rawJWT string, sessionID string, audience string, storedPublicKey []byte) error {
	var jwk jose.JSONWebKey
	if err := json.Unmarshal(storedPublicKey, &jwk); err != nil {
		return trace.Wrap(err, "parsing stored DBSC public key")
	}

	claims, err := verifyDBSCProof(rawJWT, &jwk)
	if err != nil {
		return trace.Wrap(err, "verifying DBSC refresh")
	}
	if claims.ID == "" {
		return trace.BadParameter("missing jti claim (challenge) in DBSC refresh")
	}
	if len(claims.Audience) == 0 {
		return trace.BadParameter("missing aud claim in DBSC refresh")
	}
	if claims.Subject == "" {
		return trace.BadParameter("missing sub claim in DBSC refresh")
	}
	if claims.Subject != sessionID {
		return trace.BadParameter("invalid sub claim %q in DBSC refresh", claims.Subject)
	}
	validAudience := slices.Contains(claims.Audience, audience)
	if !validAudience {
		return trace.BadParameter("invalid aud claim %q in DBSC refresh", []string(claims.Audience))
	}

	if err := h.verifyDBSCChallenge(ctx, claims.ID, sessionID); err != nil {
		return trace.Wrap(err, "verifying DBSC challenge")
	}

	return nil
}

func dbscRequestURL(r *http.Request) string {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}

	return (&url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     r.URL.Path,
		RawPath:  r.URL.RawPath,
		RawQuery: r.URL.RawQuery,
	}).String()
}

// verifyDBSCChallenge verifies that the challenge was signed by this cluster.
func (h *Handler) verifyDBSCChallenge(ctx context.Context, challenge string, sessionID string) error {
	ca, err := h.c.AccessPoint.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: h.clusterName,
	}, false /* loadKeys */)
	if err != nil {
		return trace.Wrap(err, "getting JWT CA")
	}

	return jwt.VerifyDBSCChallengeWithCA(jwt.VerifyDBSCChallengeParams{
		Challenge:   challenge,
		SessionID:   sessionID,
		ClusterName: ca.GetClusterName(),
		Clock:       h.c.Clock,
		KeyPairs:    ca.GetTrustedJWTKeyPairs(),
	})
}

// setDBSCRegistrationHeader sets the Secure-Session-Registration header to trigger
// DBSC registration in the browser.
func (h *Handler) setDBSCRegistrationHeader(ctx context.Context, w http.ResponseWriter, sessionID string) error {
	challenge, err := h.c.AuthClient.SignDBSCChallenge(ctx, sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	headerValue := fmt.Sprintf(`(ES256 RS256); path="%s"; challenge="%s"`, dbscRegistrationPath, challenge)
	w.Header().Set(secureSessionRegistrationHeader, headerValue)

	return nil
}

func verifyDBSCProof(rawJWT string, key any) (*josejwt.Claims, error) {
	tok, err := josejwt.ParseSigned(rawJWT)
	if err != nil {
		return nil, trace.Wrap(err, "parsing DBSC proof JWT")
	}
	if err := jwt.ValidateDBSCProofHeader(tok); err != nil {
		return nil, trace.Wrap(err)
	}

	var claims josejwt.Claims
	if err := tok.Claims(key, &claims); err != nil {
		return nil, trace.Wrap(err, "verifying DBSC proof signature")
	}

	return &claims, nil
}

func getDBSCHeaderString(r *http.Request, header string) (string, bool, error) {
	raw := strings.TrimSpace(r.Header.Get(header))
	if raw == "" {
		return "", false, nil
	}

	if !strings.HasPrefix(raw, "\"") {
		return raw, true, nil
	}

	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", false, trace.BadParameter("invalid %s header", header)
	}
	return value, true, nil
}
