/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/session"
)

// NewSessionRecordingAuthorizer creates a new session recording authorizer.
func NewSessionRecordingAuthorizer(authServer *Server, authorizer authz.Authorizer) *SessionRecordingAuthorizer {
	return &SessionRecordingAuthorizer{
		authServer: authServer,
		authorizer: authorizer,
	}
}

// SessionRecordingAuthorizer is a struct that implements the Authorizer interface
// for session recordings. It uses the provided authServer and authorizer to
// check if the user has permission to access a session recording.
type SessionRecordingAuthorizer struct {
	authServer *Server
	authorizer authz.Authorizer
}

// Authorize checks if the user has permission to access the session recording.
func (a *SessionRecordingAuthorizer) Authorize(ctx context.Context, sessionID string) error {
	userCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	serverWithRoles := &ServerWithRoles{
		authServer: a.authServer,
		alog:       a.authServer,
		context:    *userCtx,
	}

	return trace.Wrap(serverWithRoles.actionForKindSession(ctx, session.ID(sessionID)))
}
