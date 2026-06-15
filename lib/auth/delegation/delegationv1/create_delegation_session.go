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

package delegationv1

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// sessionMaxTTL is the maximum length of delegation session you can create
// using the CreateDelegationSession RPC. A week is chosen to allow for long-
// running AI agents, etc.
const sessionMaxTTL = 7 * 24 * time.Hour

// CreateDelegationSession creates a delegation session.
func (s *SessionService) CreateDelegationSession(
	ctx context.Context,
	req *delegationv1.CreateDelegationSessionRequest,
) (*delegationv1.DelegationSession, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This is a security-sensitive action, so require MFA.
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(boxofrad): Eventually, we'll need to support this to allow for agents
	// that sub-delegate to other agents, but the authorization logic around that
	// needs more thought, so we'll just disable it for now.
	if authCtx.Identity.GetIdentity().DelegationSessionID != "" {
		return nil, trace.AccessDenied("cannot create a delegation session from within a delegation session")
	}
	if authCtx.Identity.GetIdentity().DisallowReissue {
		return nil, trace.AccessDenied("cannot create a delegation session because certificate reissuance is prohibited")
	}

	if req.GetTtl() == nil {
		return nil, trace.BadParameter("ttl: is required")
	}
	if err := req.GetTtl().CheckValid(); err != nil {
		return nil, trace.BadParameter("ttl: %v", err)
	}

	ttl := req.GetTtl().AsDuration()
	if ttl <= 0 {
		return nil, trace.BadParameter("ttl: is required")
	}
	if ttl > sessionMaxTTL {
		return nil, trace.BadParameter("ttl: cannot be more than %d hours", sessionMaxTTL/time.Hour)
	}

	session := &delegationv1.DelegationSession{
		Kind:    types.KindDelegationSession,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    uuid.NewString(),
			Expires: timestamppb.New(time.Now().Add(ttl)),
		},
		Spec: req.GetSpec(),
	}
	spec := session.GetSpec()
	resources := spec.GetResources()

	// Read user login state from the backend to get current roles, traits, and
	// any enriched identity from external providers (e.g. GitHub). Falls back
	// to the plain user if no login state exists.
	user, err := services.GetUserOrLoginState(ctx, s.userGetter, authCtx.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if user.GetName() != spec.GetUser() {
		return nil, trace.AccessDenied("cannot create a delegation session for a different user")
	}

	if !wildcardPermissions(resources) {
		// Perform a best-effort check to see if the delegating user has access
		// to the required resources, so we can surface problems early.
		if err := s.bestEffortCheckResourceAccess(ctx, user, resources); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// The session expiry, resource list, allowed users, etc. is checked by
	// the backend service which calls ValidateDelegationSession.
	session, err = s.sessionWriter.CreateDelegationSession(ctx, session)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}
