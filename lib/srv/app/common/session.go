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

package common

import (
	"context"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

// SessionContext contains common context parameters for an App session.
type SessionContext struct {
	// Identity is the requested identity.
	Identity *tlsca.Identity
	// App is the requested identity.
	App types.Application
	// ChunkID is the session chunk's uuid.
	ChunkID string
	// Audit is used to emit audit events for the session.
	Audit Audit
}

// WithSessionContext adds session context to provided request.
func WithSessionContext(r *http.Request, sessionCtx *SessionContext) *http.Request {
	return r.WithContext(context.WithValue(
		r.Context(),
		contextSessionKey,
		sessionCtx,
	))
}

// GetSessionContext retrieves the session context from a request.
func GetSessionContext(r *http.Request) (*SessionContext, error) {
	sessionCtxValue := r.Context().Value(contextSessionKey)
	sessionCtx, ok := sessionCtxValue.(*SessionContext)
	if !ok {
		return nil, trace.BadParameter("failed to get session context")
	}
	return sessionCtx, nil
}

const (
	// contextSessionKey is the context key for the session context.
	contextSessionKey contextKey = "app-session-context"
)
