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
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	delegationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/delegation/delegationv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestProfileService_CreateProfile(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		service, pack := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{types.VerbCreate}},
			AdminActionAuthState: authz.AdminActionAuthMFAVerified,
		})

		profile := newDelegationProfile("test-profile")

		created, err := service.CreateDelegationProfile(t.Context(),
			&delegationv1pb.CreateDelegationProfileRequest{
				DelegationProfile: profile,
			})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(profile.GetSpec(), created.GetSpec(), protocmp.Transform()))

		stored, err := pack.backend.GetDelegationProfile(t.Context(), profile.GetMetadata().GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(profile, stored, protocmp.Transform()))
	})

	t.Run("permission denied", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{}},
			AdminActionAuthState: authz.AdminActionAuthMFAVerified,
		})

		_, err := service.CreateDelegationProfile(t.Context(),
			&delegationv1pb.CreateDelegationProfileRequest{
				DelegationProfile: newDelegationProfile("test-profile"),
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("MFA required", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{types.VerbCreate}},
			AdminActionAuthState: authz.AdminActionAuthUnauthorized,
		})

		_, err := service.CreateDelegationProfile(t.Context(),
			&delegationv1pb.CreateDelegationProfileRequest{
				DelegationProfile: newDelegationProfile("test-profile"),
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}

func TestProfileService_GetProfile(t *testing.T) {
	t.Parallel()

	t.Run("success - with admin access", func(t *testing.T) {
		service, pack := profileServiceTestPack(t, &authz.Context{
			Checker: fakeChecker{allowedVerbs: []string{types.VerbRead}},
		})

		profile, err := pack.backend.CreateDelegationProfile(t.Context(), newDelegationProfile("test-profile"))
		require.NoError(t, err)

		got, err := service.GetDelegationProfile(t.Context(),
			&delegationv1pb.GetDelegationProfileRequest{
				Name: profile.GetMetadata().GetName(),
			})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(profile, got, protocmp.Transform()))
	})

	t.Run("success - with matching labels", func(t *testing.T) {
		service, pack := profileServiceTestPack(t, &authz.Context{
			Checker: fakeChecker{allowResourceAccess: true},
		})

		profile, err := pack.backend.CreateDelegationProfile(t.Context(), newDelegationProfile("test-profile"))
		require.NoError(t, err)

		got, err := service.GetDelegationProfile(t.Context(),
			&delegationv1pb.GetDelegationProfileRequest{
				Name: profile.GetMetadata().GetName(),
			})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(profile, got, protocmp.Transform()))
	})

	t.Run("not found - with admin access", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker: fakeChecker{allowedVerbs: []string{types.VerbRead}},
		})

		_, err := service.GetDelegationProfile(t.Context(),
			&delegationv1pb.GetDelegationProfileRequest{
				Name: "does-not-exist",
			})
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("not found - without admin access", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker: fakeChecker{allowedVerbs: []string{}},
		})

		_, err := service.GetDelegationProfile(t.Context(),
			&delegationv1pb.GetDelegationProfileRequest{
				Name: "does-not-exist",
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("permission denied", func(t *testing.T) {
		service, pack := profileServiceTestPack(t, &authz.Context{
			Checker: fakeChecker{allowedVerbs: []string{}},
		})

		profile, err := pack.backend.CreateDelegationProfile(t.Context(), newDelegationProfile("test-profile"))
		require.NoError(t, err)

		_, err = service.GetDelegationProfile(t.Context(),
			&delegationv1pb.GetDelegationProfileRequest{
				Name: profile.GetMetadata().GetName(),
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}

func TestProfileService_UpdateProfile(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		service, pack := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{types.VerbUpdate}},
			AdminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
		})

		profile, err := pack.backend.CreateDelegationProfile(t.Context(), newDelegationProfile("test-profile"))
		require.NoError(t, err)

		profile.Spec.AuthorizedUsers = append(
			profile.Spec.AuthorizedUsers,
			&delegationv1pb.DelegationUserSpec{
				Type: types.DelegationUserTypeBot,
				Matcher: &delegationv1pb.DelegationUserSpec_BotName{
					BotName: "new-bot",
				},
			},
		)

		revision := profile.GetMetadata().GetRevision()
		profile.Metadata.Revision = "something old"

		_, err = service.UpdateDelegationProfile(t.Context(),
			&delegationv1pb.UpdateDelegationProfileRequest{
				DelegationProfile: profile,
			})
		require.Error(t, err)
		require.True(t, trace.IsCompareFailed(err))

		profile.Metadata.Revision = revision

		updated, err := service.UpdateDelegationProfile(t.Context(),
			&delegationv1pb.UpdateDelegationProfileRequest{
				DelegationProfile: profile,
			})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(profile.GetSpec(), updated.GetSpec(), protocmp.Transform()))
	})

	t.Run("permission denied", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{}},
			AdminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
		})

		_, err := service.UpdateDelegationProfile(t.Context(),
			&delegationv1pb.UpdateDelegationProfileRequest{
				DelegationProfile: newDelegationProfile("test-profile"),
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("MFA required", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{types.VerbUpdate}},
			AdminActionAuthState: authz.AdminActionAuthUnauthorized,
		})

		_, err := service.UpdateDelegationProfile(t.Context(),
			&delegationv1pb.UpdateDelegationProfileRequest{
				DelegationProfile: newDelegationProfile("test-profile"),
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}

func TestProfileService_UpsertProfile(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{types.VerbCreate, types.VerbUpdate}},
			AdminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
		})

		profile := newDelegationProfile("test-profile")

		created, err := service.UpsertDelegationProfile(t.Context(),
			&delegationv1pb.UpsertDelegationProfileRequest{
				DelegationProfile: profile,
			})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(profile.GetSpec(), created.GetSpec(), protocmp.Transform()))

		createdRevision := created.GetMetadata().GetRevision()
		require.NotEmpty(t, createdRevision)

		profile.Spec.AuthorizedUsers = append(
			profile.Spec.AuthorizedUsers,
			&delegationv1pb.DelegationUserSpec{
				Type: types.DelegationUserTypeBot,
				Matcher: &delegationv1pb.DelegationUserSpec_BotName{
					BotName: "new-bot",
				},
			},
		)

		updated, err := service.UpsertDelegationProfile(t.Context(),
			&delegationv1pb.UpsertDelegationProfileRequest{
				DelegationProfile: profile,
			})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(profile.GetSpec(), updated.GetSpec(), protocmp.Transform()))
		require.NotEqual(t, createdRevision, updated.GetMetadata().GetRevision())
	})

	t.Run("permission denied", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{}},
			AdminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
		})

		_, err := service.UpsertDelegationProfile(t.Context(),
			&delegationv1pb.UpsertDelegationProfileRequest{
				DelegationProfile: newDelegationProfile("test-profile"),
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("MFA required", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{types.VerbCreate, types.VerbUpdate}},
			AdminActionAuthState: authz.AdminActionAuthUnauthorized,
		})

		_, err := service.UpsertDelegationProfile(t.Context(),
			&delegationv1pb.UpsertDelegationProfileRequest{
				DelegationProfile: newDelegationProfile("test-profile"),
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}

func TestProfileService_DeleteProfile(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		service, pack := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{types.VerbRead, types.VerbDelete}},
			AdminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
		})
		profile, err := pack.backend.CreateDelegationProfile(t.Context(), newDelegationProfile("test-profile"))
		require.NoError(t, err)

		_, err = service.DeleteDelegationProfile(t.Context(),
			&delegationv1pb.DeleteDelegationProfileRequest{
				Name: profile.GetMetadata().GetName(),
			})
		require.NoError(t, err)

		_, err = service.GetDelegationProfile(t.Context(),
			&delegationv1pb.GetDelegationProfileRequest{
				Name: profile.GetMetadata().GetName(),
			})
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("permission denied", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{}},
			AdminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
		})

		_, err := service.DeleteDelegationProfile(t.Context(),
			&delegationv1pb.DeleteDelegationProfileRequest{
				Name: "test-profile",
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("MFA required", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker:              fakeChecker{allowedVerbs: []string{types.VerbDelete}},
			AdminActionAuthState: authz.AdminActionAuthUnauthorized,
		})

		_, err := service.DeleteDelegationProfile(t.Context(),
			&delegationv1pb.DeleteDelegationProfileRequest{
				Name: "test-profile",
			})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}

func TestProfileService_ListProfiles(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		service, pack := profileServiceTestPack(t, &authz.Context{
			Checker: fakeChecker{allowedVerbs: []string{types.VerbList, types.VerbRead}},
		})

		for i := range 2 {
			_, err := pack.backend.CreateDelegationProfile(
				t.Context(),
				newDelegationProfile(fmt.Sprintf("profile-%d", i)),
			)
			require.NoError(t, err)
		}

		rsp, err := service.ListDelegationProfiles(t.Context(), &delegationv1pb.ListDelegationProfilesRequest{
			PageSize: 1,
		})
		require.NoError(t, err)
		require.Len(t, rsp.DelegationProfiles, 1)
		require.Equal(t, "profile-0", rsp.DelegationProfiles[0].GetMetadata().GetName())
		require.NotEmpty(t, rsp.NextPageToken)

		rsp, err = service.ListDelegationProfiles(t.Context(), &delegationv1pb.ListDelegationProfilesRequest{
			PageSize:  1,
			PageToken: rsp.NextPageToken,
		})
		require.NoError(t, err)
		require.Len(t, rsp.DelegationProfiles, 1)
		require.Equal(t, "profile-1", rsp.DelegationProfiles[0].GetMetadata().GetName())
		require.Empty(t, rsp.NextPageToken)
	})

	t.Run("permission denied", func(t *testing.T) {
		service, _ := profileServiceTestPack(t, &authz.Context{
			Checker: fakeChecker{allowedVerbs: []string{}},
		})

		_, err := service.ListDelegationProfiles(t.Context(), &delegationv1pb.ListDelegationProfilesRequest{
			PageSize: 1,
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}

type fakeChecker struct {
	allowResourceAccess bool
	allowedVerbs        []string

	services.AccessChecker
}

func (f fakeChecker) CheckAccess(r services.AccessCheckable, state services.AccessState, matchers ...services.RoleMatcher) error {
	if f.allowResourceAccess {
		return nil
	}
	return trace.AccessDenied("access to resource %T not allowed", r)
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindDelegationProfile {
		if slices.Contains(f.allowedVerbs, verb) {
			return nil
		}
	}
	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}

type profileTestPack struct {
	backend services.DelegationProfiles
}

func profileServiceTestPack(t *testing.T, authCtx *authz.Context) (*delegationv1.ProfileService, *profileTestPack) {
	t.Helper()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	upstream, err := local.NewDelegationProfileService(backend)
	require.NoError(t, err)

	service, err := delegationv1.NewProfileService(delegationv1.ProfileServiceConfig{
		Authorizer: authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
			return authCtx, nil
		}),
		Writer: upstream,
		Reader: upstream,
		Logger: logtest.NewLogger(),
	})
	require.NoError(t, err)

	return service, &profileTestPack{backend: upstream}
}

func newDelegationProfile(name string) *delegationv1pb.DelegationProfile {
	return &delegationv1pb.DelegationProfile{
		Kind:    types.KindDelegationProfile,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &delegationv1pb.DelegationProfileSpec{
			RequiredResources: []*delegationv1pb.DelegationResourceSpec{
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
			DefaultSessionLength: durationpb.New(1 * time.Hour),
			Consent: &delegationv1pb.DelegationConsentSpec{
				Title:       "Payroll Agent",
				Description: "Automates the payroll process",
				AllowedRedirectUrls: []string{
					"https://payroll.intranet.corp",
				},
			},
		},
	}
}
