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
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

const (
	// secureSessionRegistrationHeader is the header that triggers DBSC registration in the browser.
	secureSessionRegistrationHeader = "Secure-Session-Registration"
	// secureSessionResponseHeader is the header containing the browser's signed JWT.
	secureSessionResponseHeader = "Secure-Session-Response"
	// dbscCookieMaxAge is the short-lived cookie max age when DBSC is active.
	dbscCookieMaxAge = 5 * time.Minute
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

	responseJWT := r.Header.Get(secureSessionResponseHeader)
	if responseJWT == "" {
		return trace.BadParameter("missing %s header", secureSessionResponseHeader)
	}

	if err := h.c.AuthClient.SetAppSessionDBSCPublicKey(ctx, sessionID, []byte(responseJWT)); err != nil {
		return trace.Wrap(err)
	}

	// Re-issue the cookie with a short TTL now that DBSC is active.
	setAppSessionCookie(w, cookie.Value, dbscCookieMaxAge)

	config := dbscSessionConfig{
		SessionIdentifier: sessionID,
		RefreshURL:        dbscRefreshPath,
		Scope: dbscScope{
			IncludeSite: true,
		},
		Credentials: []dbscCredential{
			{
				Type:       "cookie",
				Name:       CookieName,
				Attributes: "Path=/; HttpOnly; Secure; SameSite=None",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(config); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// handleDBSCRefresh handles DBSC session refresh requests.
func (h *Handler) handleDBSCRefresh(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	return trace.NotImplemented("DBSC refresh not yet implemented")
}

// setDBSCRegistrationHeader sets the Secure-Session-Registration header to trigger
// DBSC registration in the browser.
func (h *Handler) setDBSCRegistrationHeader(ctx context.Context, w http.ResponseWriter, sessionID string) error {
	challenge, err := h.signDBSCChallenge(ctx, sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	headerValue := fmt.Sprintf(`(ES256); path="%s"; challenge="%s"`, dbscRegistrationPath, challenge)
	w.Header().Set(secureSessionRegistrationHeader, headerValue)

	return nil
}

// signDBSCChallenge creates a signed challenge.
func (h *Handler) signDBSCChallenge(ctx context.Context, sessionID string) (string, error) {
	challenge, err := h.c.AuthClient.SignDBSCChallenge(ctx, sessionID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return challenge, nil
}
