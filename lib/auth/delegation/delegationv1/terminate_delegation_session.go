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

package delegationv1

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// ErrDelegationSessionNotFound is returned when the user attempted to terminate
// a delegation session that does not exist (or does not belong to them).
var ErrDelegationSessionNotFound = &trace.AccessDeniedError{
	Message: "You have no active delegation session with the given identifier",
}

// TerminateDelegationSession terminates an ongoing delegation session.
func (s *SessionService) TerminateDelegationSession(ctx context.Context, req *delegationv1.TerminateDelegationSessionRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := s.sessionReader.GetDelegationSession(ctx, req.GetDelegationSessionId())
	switch {
	case trace.IsNotFound(err):
		return nil, trace.Wrap(ErrDelegationSessionNotFound)
	case err != nil:
		return nil, trace.Wrap(err)
	}

	// This endpoint may only be used by the session owner. Administrators can
	// manually create locks targetting the session.
	if sess.GetSpec().GetUser() != authCtx.User.GetName() {
		return nil, trace.Wrap(ErrDelegationSessionNotFound)
	}

	lock, err := types.NewLock(req.GetDelegationSessionId(), types.LockSpecV2{
		Message: "Delegation session was terminated by the user.",
		Target: types.LockTarget{
			DelegationSessionID: req.GetDelegationSessionId(),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.lockWriter.UpsertLock(ctx, lock); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
