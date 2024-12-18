/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package alpnproxy

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// AuthorizationCheckerMiddleware is a middleware that checks `Authorization` header of incoming requests.
// If the header is missing, the request is passed through.
// If it is present, the middleware checks it is a bearer token with value matching the secret.
type AuthorizationCheckerMiddleware struct {
	DefaultLocalProxyHTTPMiddleware

	// Log is the Logger.
	Log *slog.Logger
	// Secret is the expected value of a bearer token.
	Secret string
}

var _ LocalProxyHTTPMiddleware = (*AuthorizationCheckerMiddleware)(nil)

// CheckAndSetDefaults checks configuration validity and sets defaults.
func (m *AuthorizationCheckerMiddleware) CheckAndSetDefaults() error {
	if m.Log == nil {
		m.Log = slog.With(teleport.ComponentKey, "authz")
	}

	if m.Secret == "" {
		return trace.BadParameter("missing Secret")
	}
	return nil
}

// HandleRequest checks Authorization header, which must be either missing or set to the secret value of a bearer token.
func (m *AuthorizationCheckerMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	auth := req.Header.Get("Authorization")
	if auth == "" {
		m.Log.DebugContext(req.Context(), "No Authorization header present, ignoring request")
		return false
	}

	expectedAuth := "Bearer " + m.Secret

	if subtle.ConstantTimeCompare([]byte(auth), []byte(expectedAuth)) != 1 {
		trace.WriteError(rw, trace.AccessDenied("Invalid Authorization header"))
		return true
	}

	return false
}
