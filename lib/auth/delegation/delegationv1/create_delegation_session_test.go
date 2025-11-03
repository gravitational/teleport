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

	t.Run("success with profile", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			service, pack := sessionServiceTestPack(t)

			// Create user with role that allows the use of any profile and
			// any application (to match the profile's required_resources).
			pack.authenticate(t,
				"bob",
				authz.AdminActionAuthMFAVerified,
				types.RoleSpecV6{
					Allow: types.RoleConditions{
						DelegationProfileLabels: types.Labels{
							types.Wildcard: {types.Wildcard},
						},
						AppLabels: types.Labels{
							types.Wildcard: {types.Wildcard},
						},
					},
				},
			)

			// Create delegation profile.
			profile, err := pack.profiles.CreateDelegationProfile(
				t.Context(),
				newDelegationProfile("my-profile"),
			)
			require.NoError(t, err)

			// Call endpoint to create session.
			session, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
				From: &delegationv1pb.CreateDelegationSessionRequest_Profile{
					Profile: &delegationv1pb.DelegationProfileReference{
						Name:     profile.GetMetadata().GetName(),
						Revision: profile.GetMetadata().GetRevision(),
					},
				},
				Ttl: durationpb.New(5 * time.Minute),
			})
			require.NoError(t, err)

			// Check user name is captured.
			assert.Equal(t,
				pack.user.GetName(),
				session.GetSpec().GetUser(),
			)

			// Check resources and authorized users copied from profile.
			assert.Empty(t,
				cmp.Diff(
					profile.GetSpec().GetRequiredResources(),
					session.GetSpec().GetResources(),
					protocmp.Transform(),
				),
			)
			assert.Empty(t,
				cmp.Diff(
					profile.GetSpec().GetAuthorizedUsers(),
					session.GetSpec().GetAuthorizedUsers(),
					protocmp.Transform(),
				),
			)

			// Check TTL is applied.
			assert.Equal(t,
				5*time.Minute,
				session.GetMetadata().GetExpires().AsTime().Sub(time.Now()),
			)
		})
	})

	t.Run("success with manual parameters", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			service, pack := sessionServiceTestPack(t)

			// Create user with role that allows the use of any application.
			pack.authenticate(t,
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

			// Call endpoint to create session.
			session, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
				From: &delegationv1pb.CreateDelegationSessionRequest_Parameters{
					Parameters: &delegationv1pb.DelegationSessionParameters{
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
					},
				},
				Ttl: durationpb.New(5 * time.Minute),
			})
			require.NoError(t, err)

			// Check user name is captured.
			assert.Equal(t,
				pack.user.GetName(),
				session.GetSpec().GetUser(),
			)

			// Check resources and authorized users copied from profile.
			assert.Empty(t,
				cmp.Diff(
					[]*delegationv1pb.DelegationResourceSpec{
						{
							Kind: types.KindApp,
							Name: "hr-system",
						},
					},
					session.GetSpec().GetResources(),
					protocmp.Transform(),
				),
			)
			assert.Empty(t,
				cmp.Diff(
					[]*delegationv1pb.DelegationUserSpec{
						{
							Type: types.DelegationUserTypeBot,
							Matcher: &delegationv1pb.DelegationUserSpec_BotName{
								BotName: "payroll-agent",
							},
						},
					},
					session.GetSpec().GetAuthorizedUsers(),
					protocmp.Transform(),
				),
			)

			// Check TTL is applied.
			assert.Equal(t,
				5*time.Minute,
				session.GetMetadata().GetExpires().AsTime().Sub(time.Now()),
			)
		})
	})

	t.Run("not allowed to use profile", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		// Create user with role that allows the use of any application but not
		// the delegation profile.
		pack.authenticate(t,
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

		// Create delegation profile.
		profile, err := pack.profiles.CreateDelegationProfile(
			t.Context(),
			newDelegationProfile("my-profile"),
		)
		require.NoError(t, err)

		// Call endpoint to create session.
		_, err = service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			From: &delegationv1pb.CreateDelegationSessionRequest_Profile{
				Profile: &delegationv1pb.DelegationProfileReference{
					Name:     profile.GetMetadata().GetName(),
					Revision: profile.GetMetadata().GetRevision(),
				},
			},
			Ttl: durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("not allowed to use resources", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		// Create user with role that allows the use of any delegation profile
		// but not the required resources.
		pack.authenticate(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					DelegationProfileLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		// Create delegation profile.
		profile, err := pack.profiles.CreateDelegationProfile(
			t.Context(),
			newDelegationProfile("my-profile"),
		)
		require.NoError(t, err)

		// Call endpoint to create session.
		_, err = service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			From: &delegationv1pb.CreateDelegationSessionRequest_Profile{
				Profile: &delegationv1pb.DelegationProfileReference{
					Name:     profile.GetMetadata().GetName(),
					Revision: profile.GetMetadata().GetRevision(),
				},
			},
			Ttl: durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "You do not have permission to delegate access to all of the required resources")
	})

	t.Run("resource does not exist", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		// Create user with role that allows the use of any profile and
		// any application (to match the profile's required_resources).
		pack.authenticate(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					DelegationProfileLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		profile := newDelegationProfile("my-profile")
		profile.Spec.RequiredResources = []*delegationv1pb.DelegationResourceSpec{
			{
				Kind: types.KindApp,
				Name: "unknown-app",
			},
		}

		// Create delegation profile.
		profile, err := pack.profiles.CreateDelegationProfile(t.Context(), profile)
		require.NoError(t, err)

		// Call endpoint to create session.
		_, err = service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			From: &delegationv1pb.CreateDelegationSessionRequest_Profile{
				Profile: &delegationv1pb.DelegationProfileReference{
					Name:     profile.GetMetadata().GetName(),
					Revision: profile.GetMetadata().GetRevision(),
				},
			},
			Ttl: durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "You do not have permission to delegate access to all of the required resources")
	})

	t.Run("profile revision changed", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		// Create user with role that allows the use of any profile and
		// any application (to match the profile's required_resources).
		pack.authenticate(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					DelegationProfileLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		// Create delegation profile.
		profile, err := pack.profiles.CreateDelegationProfile(
			t.Context(),
			newDelegationProfile("my-profile"),
		)
		require.NoError(t, err)

		// Call endpoint to create session.
		_, err = service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			From: &delegationv1pb.CreateDelegationSessionRequest_Profile{
				Profile: &delegationv1pb.DelegationProfileReference{
					Name:     profile.GetMetadata().GetName(),
					Revision: "not-the-same-revision",
				},
			},
			Ttl: durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.True(t, trace.IsCompareFailed(err))
	})

	t.Run("requires MFA", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		// Create user with role that allows the use of any profile and
		// any application (to match the profile's required_resources).
		pack.authenticate(t,
			"bob",
			authz.AdminActionAuthUnauthorized,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					DelegationProfileLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		// Call endpoint to create session.
		_, err := service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			From: &delegationv1pb.CreateDelegationSessionRequest_Parameters{
				Parameters: &delegationv1pb.DelegationSessionParameters{
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
				},
			},
			Ttl: durationpb.New(5 * time.Minute),
		})
		require.Error(t, err)
		require.ErrorIs(t, err, &mfa.ErrAdminActionMFARequired)
	})

	t.Run("no TTL or default session length", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)

		// Create user with role that allows the use of any profile and
		// any application (to match the profile's required_resources).
		pack.authenticate(t,
			"bob",
			authz.AdminActionAuthMFAVerified,
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					DelegationProfileLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
					AppLabels: types.Labels{
						types.Wildcard: {types.Wildcard},
					},
				},
			},
		)

		profile := newDelegationProfile("my-profile")
		profile.Spec.DefaultSessionLength = nil

		// Create delegation profile.
		profile, err := pack.profiles.CreateDelegationProfile(t.Context(), profile)
		require.NoError(t, err)

		// Call endpoint to create session.
		_, err = service.CreateDelegationSession(t.Context(), &delegationv1pb.CreateDelegationSessionRequest{
			From: &delegationv1pb.CreateDelegationSessionRequest_Profile{
				Profile: &delegationv1pb.DelegationProfileReference{
					Name:     profile.GetMetadata().GetName(),
					Revision: profile.GetMetadata().GetRevision(),
				},
			},
		})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "ttl is require")
	})
}
