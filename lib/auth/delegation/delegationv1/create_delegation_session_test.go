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

package delegationv1_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	delegationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

func TestSessionService_CreateSession(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			service, pack := sessionServiceTestPack(t)

			pack.authenticateUser(t,
				"bob",
				authz.AdminActionAuthMFAVerified,
				types.RoleSpecV6{
					Allow: types.RoleConditions{
						AppLabels: types.Labels{
							types.Wildcard: {types.Wildcard},
						},
					},
				},
			)

			expectedExpires := time.Now().Add(5 * time.Minute)
			expectedSpec := newDelegationSessionSpec("bob")

			session, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
				Spec: expectedSpec,
				Ttl:  durationpb.New(5 * time.Minute),
			})
			require.NoError(t, err)

			assert.Equal(t, types.KindDelegationSession, session.GetKind())
			assert.Equal(t, types.V1, session.GetVersion())
			assert.Equal(t, pack.user.GetName(), session.GetSpec().GetUser())
			assert.NotEmpty(t, session.GetMetadata().GetName())
			assert.Empty(t, cmp.Diff(
				expectedSpec.GetResources(),
				session.GetSpec().GetResources(),
				protocmp.Transform(),
			))
			assert.Empty(t, cmp.Diff(
				expectedSpec.GetAuthorizedUsers(),
				session.GetSpec().GetAuthorizedUsers(),
				protocmp.Transform(),
			))
			assert.True(t, expectedExpires.Equal(session.GetMetadata().GetExpires().AsTime()))
		})
	})

	t.Run("success with wildcard resources", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{},
		)

		session, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec(
				"bob",
				&delegationv1pb.DelegationResourceSpec{
					Kind: types.Wildcard,
					Name: types.Wildcard,
				},
			),
			Ttl: durationpb.New(5 * time.Minute),
		})
		require.NoError(t, err)
		assert.Equal(t, types.Wildcard, session.GetSpec().GetResources()[0].GetKind())
		assert.Equal(t, types.Wildcard, session.GetSpec().GetResources()[0].GetName())
	})

	t.Run("not allowed to use resources", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{},
		)

		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec("bob"),
			Ttl:  durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "user does not have permission to delegate access to all of the required resources")
	})

	t.Run("resource does not exist", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec(
				"bob",
				&delegationv1pb.DelegationResourceSpec{
					Kind: types.KindApp,
					Name: "unknown-app",
				},
			),
			Ttl: durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "missing resources: [app/unknown-app]")
	})

	t.Run("cannot create a session for a different user", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec("alice"),
			Ttl:  durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "cannot create a delegation session for a different user")
	})

	t.Run("requires MFA", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"bob",
			authz.AdminActionAuthUnauthorized,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec("bob"),
			Ttl:  durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.ErrorIs(t, err, &mfa.ErrAdminActionMFARequired)
	})

	t.Run("cannot create a session from within a delegation session", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUserInDelegationSession(
			t,
			"bob",
			"source-session",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec("bob"),
			Ttl:  durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "cannot create a delegation session from within a delegation session")
	})

	t.Run("cannot create a session with a non-reissuable certificate", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUserWithDisallowReissue(
			t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec("bob"),
			Ttl:  durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "cannot create a delegation session because certificate reissuance is prohibited")
	})

	t.Run("missing ttl", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec("bob"),
		})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "ttl: is required")
	})

	t.Run("ttl too large", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		pack.authenticateUser(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			Spec: newDelegationSessionSpec("bob"),
			Ttl:  durationpb.New(14 * 24 * time.Hour),
		})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "ttl: cannot be more than 168 hours")
	})
}

func newDelegationSessionSpec(
	user string,
	resources ...*delegationv1pb.DelegationResourceSpec,
) *delegationv1pb.DelegationSessionSpec {
	if len(resources) == 0 {
		resources = []*delegationv1pb.DelegationResourceSpec{
			{
				Kind: types.KindApp,
				Name: "hr-system",
			},
		}
	}

	return &delegationv1pb.DelegationSessionSpec{
		User:      user,
		Resources: resources,
		AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
			{
				Kind: types.KindBot,
				Matcher: &delegationv1pb.DelegationUserSpec_BotName{
					BotName: "payroll-agent",
				},
			},
		},
	}
}
