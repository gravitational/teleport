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

package delegationv1_test

import (
	"testing"

	delegationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/delegation/delegationv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestSessionService_TerminateSession(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"sally",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{},
		)

		session := pack.createSession(t, &delegationv1pb.DelegationSessionSpec{
			User: "sally",
			Resources: []*delegationv1pb.DelegationResourceSpec{
				{
					Kind: types.KindApp,
					Name: "hr-system",
				},
			},
			AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
				{
					Type: types.DelegationUserTypeBot,
					Matcher: &delegationv1pb.DelegationUserSpec_BotName{
						BotName: "payroll-agent",
					},
				},
			},
		})

		_, err := service.TerminateDelegationSession(t.Context(), &delegationv1pb.TerminateDelegationSessionRequest{
			DelegationSessionId: session.GetMetadata().GetName(),
		})
		require.NoError(t, err)

		locks, err := pack.access.GetLocks(
			t.Context(),
			true, /* in-force only */
			types.LockTarget{DelegationSessionID: session.GetMetadata().GetName()},
		)
		require.NoError(t, err)
		require.Len(t, locks, 1)
		require.Equal(t, "Delegation session was terminated by the user.", locks[0].Message())
	})

	t.Run("session does not exist", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"sally",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{},
		)

		_, err := service.TerminateDelegationSession(t.Context(), &delegationv1pb.TerminateDelegationSessionRequest{
			DelegationSessionId: "bogus",
		})
		require.ErrorIs(t, err, delegationv1.ErrDelegationSessionNotFound)
	})

	t.Run("session belongs to somebody else", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		session := pack.createSession(t, &delegationv1pb.DelegationSessionSpec{
			User: "bob",
			Resources: []*delegationv1pb.DelegationResourceSpec{
				{
					Kind: types.KindApp,
					Name: "hr-system",
				},
			},
			AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
				{
					Type: types.DelegationUserTypeBot,
					Matcher: &delegationv1pb.DelegationUserSpec_BotName{
						BotName: "payroll-agent",
					},
				},
			},
		})

		pack.authenticateUser(t,
			"sally",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{},
		)

		_, err := service.TerminateDelegationSession(t.Context(), &delegationv1pb.TerminateDelegationSessionRequest{
			DelegationSessionId: session.GetMetadata().GetName(),
		})
		require.ErrorIs(t, err, delegationv1.ErrDelegationSessionNotFound)
	})

	t.Run("mfa required", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"sally",
			authz.AdminActionAuthUnauthorized,
			types.RoleSpecV6{},
		)

		session := pack.createSession(t, &delegationv1pb.DelegationSessionSpec{
			User: "sally",
			Resources: []*delegationv1pb.DelegationResourceSpec{
				{
					Kind: types.KindApp,
					Name: "hr-system",
				},
			},
			AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
				{
					Type: types.DelegationUserTypeBot,
					Matcher: &delegationv1pb.DelegationUserSpec_BotName{
						BotName: "payroll-agent",
					},
				},
			},
		})

		_, err := service.TerminateDelegationSession(t.Context(), &delegationv1pb.TerminateDelegationSessionRequest{
			DelegationSessionId: session.GetMetadata().GetName(),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}
