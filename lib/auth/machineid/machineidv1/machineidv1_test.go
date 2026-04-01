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

package machineidv1_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func TestBotResourceName(t *testing.T) {
	require.Equal(
		t,
		"bot-name",
		machineidv1.BotResourceName("name"),
	)
	require.Equal(
		t,
		"bot-name-with-spaces",
		machineidv1.BotResourceName("name with spaces"),
	)
}

// TestCreateBot is an integration test that uses a real gRPC client/server.
func TestCreateBot(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	t.Setenv("TELEPORT_UNSTABLE_SCOPES_MWI", "yes")
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botCreator, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"bot-creator",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindBot},
				Verbs:     []string{types.VerbCreate},
			},
		})
	require.NoError(t, err)
	botCreatorWhere, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"bot-creator-where",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindBot},
				Verbs:     []string{types.VerbCreate},
				Where:     `has_prefix(resource.metadata.name, "foo")`,
			},
		})
	require.NoError(t, err)
	testRole, err := authtest.CreateRole(
		ctx, srv.Auth(), "test-role", types.RoleSpecV6{},
	)
	require.NoError(t, err)
	unprivilegedUser, err := authtest.CreateUser(
		ctx, srv.Auth(), "no-perms", testRole,
	)
	require.NoError(t, err)

	client, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "pre-existing",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)
	expiry := time.Now().Add(time.Hour)

	scopedSvc := client.ScopedAccessServiceClient()
	scopedRole, err := scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-bot-creator",
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/scopes/granted", "/scopes/ungranted"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Verbs:     []string{types.VerbCreate},
						Resources: []string{types.KindBot},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	scopedUser, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-user")
	require.NoError(t, err)

	// Create scoped role assignments linking users to scoped roles.
	_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			SubKind: scopedaccess.SubKindDynamic,
			Scope:   "/scopes",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: scopedUser.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: scopedRole.Role.Metadata.Name, Scope: "/scopes/granted"},
				},
			},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		identity authtest.TestIdentity
		req      *machineidv1pb.CreateBotRequest

		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.Bot
		wantUser    *types.UserV2
		wantRole    *types.RoleV6
	}{
		{
			name:     "success",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "success",
						Labels: map[string]string{
							"my-label":       "my-value",
							"my-other-label": "my-other-value",
						},
						Description: "Property of US Robotics and Mechanical Men.",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
						Traits: []*machineidv1pb.Trait{
							{
								Name:   constants.TraitLogins,
								Values: []string{"root"},
							},
							{
								Name:   constants.TraitKubeUsers,
								Values: []string{},
							},
						},
						// Note: Deliberately omitting MaxSessionTtl here to verify
						// the default value.
					},
				},
			},

			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "success",
					Labels: map[string]string{
						"my-label":       "my-value",
						"my-other-label": "my-other-value",
					},
					Description: "Property of US Robotics and Mechanical Men.",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
					Traits: []*machineidv1pb.Trait{
						{
							Name:   constants.TraitLogins,
							Values: []string{"root"},
						},
					},
					MaxSessionTtl: durationpb.New(libdefaults.DefaultBotMaxSessionTTL),
				},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-success",
					RoleName: "bot-success",
				},
			},
			wantUser: &types.UserV2{
				Kind:    types.KindUser,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "bot-success",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel:           "success",
						types.BotGenerationLabel: "0",
						"my-label":               "my-value",
						"my-other-label":         "my-other-value",
					},
					Description: "Property of US Robotics and Mechanical Men.",
				},
				Spec: types.UserSpecV2{
					CreatedBy: types.CreatedBy{
						User: types.UserRef{Name: botCreator.GetName()},
					},
					Roles: []string{"bot-success"},
					Traits: map[string][]string{
						constants.TraitLogins: {"root"},
					},
				},
				Status: types.UserStatusV2{
					PasswordState: types.PasswordState_PASSWORD_STATE_UNSET,
				},
			},
			wantRole: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:      "bot-success",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel: "success",
					},
					Description: "Automatically generated role for bot success",
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(libdefaults.DefaultBotMaxSessionTTL),
					},
					Allow: types.RoleConditions{
						Impersonate: &types.ImpersonateConditions{
							Roles: []string{testRole.GetName()},
						},
						Rules: []types.Rule{
							types.NewRule(
								types.KindCertAuthority,
								[]string{types.VerbReadNoSecrets},
							),
						},
					},
				},
			},
		},
		{
			name:     "success with expiry",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "success-with-expiry",
						Labels: map[string]string{
							"my-label":       "my-value",
							"my-other-label": "my-other-value",
						},
						Expires: timestamppb.New(expiry),
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
						Traits: []*machineidv1pb.Trait{
							{
								Name:   constants.TraitLogins,
								Values: []string{"root"},
							},
							{
								Name:   constants.TraitKubeUsers,
								Values: []string{},
							},
						},
						// Note: Deliberately omitting MaxSessionTtl here to
						// validate the default value.
					},
				},
			},

			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "success-with-expiry",
					Labels: map[string]string{
						"my-label":       "my-value",
						"my-other-label": "my-other-value",
					},
					Expires: timestamppb.New(expiry),
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
					Traits: []*machineidv1pb.Trait{
						{
							Name:   constants.TraitLogins,
							Values: []string{"root"},
						},
					},
					MaxSessionTtl: durationpb.New(libdefaults.DefaultBotMaxSessionTTL),
				},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-success-with-expiry",
					RoleName: "bot-success-with-expiry",
				},
			},
			wantUser: &types.UserV2{
				Kind:    types.KindUser,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "bot-success-with-expiry",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel:           "success-with-expiry",
						types.BotGenerationLabel: "0",
						"my-label":               "my-value",
						"my-other-label":         "my-other-value",
					},
					Expires: &expiry,
				},
				Spec: types.UserSpecV2{
					CreatedBy: types.CreatedBy{
						User: types.UserRef{Name: botCreator.GetName()},
					},
					Roles: []string{"bot-success-with-expiry"},
					Traits: map[string][]string{
						constants.TraitLogins: {"root"},
					},
				},
				Status: types.UserStatusV2{
					PasswordState: types.PasswordState_PASSWORD_STATE_UNSET,
				},
			},
			wantRole: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:      "bot-success-with-expiry",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel: "success-with-expiry",
					},
					Description: "Automatically generated role for bot success-with-expiry",
					Expires:     &expiry,
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(libdefaults.DefaultBotMaxSessionTTL),
					},
					Allow: types.RoleConditions{
						Impersonate: &types.ImpersonateConditions{
							Roles: []string{testRole.GetName()},
						},
						Rules: []types.Rule{
							types.NewRule(
								types.KindCertAuthority,
								[]string{types.VerbReadNoSecrets},
							),
						},
					},
				},
			},
		},
		{
			name:     "success with max ttl",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "success-with-max-ttl",
						Labels: map[string]string{
							"my-label":       "my-value",
							"my-other-label": "my-other-value",
						},
						Expires: timestamppb.New(expiry),
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
						Traits: []*machineidv1pb.Trait{
							{
								Name:   constants.TraitLogins,
								Values: []string{"root"},
							},
							{
								Name:   constants.TraitKubeUsers,
								Values: []string{},
							},
						},
						MaxSessionTtl: durationpb.New(libdefaults.MaxRenewableCertTTL),
					},
				},
			},
			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "success-with-max-ttl",
					Labels: map[string]string{
						"my-label":       "my-value",
						"my-other-label": "my-other-value",
					},
					Expires: timestamppb.New(expiry),
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
					Traits: []*machineidv1pb.Trait{
						{
							Name:   constants.TraitLogins,
							Values: []string{"root"},
						},
					},
					MaxSessionTtl: durationpb.New(libdefaults.MaxRenewableCertTTL),
				},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-success-with-max-ttl",
					RoleName: "bot-success-with-max-ttl",
				},
			},
			wantRole: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:      "bot-success-with-max-ttl",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel: "success-with-max-ttl",
					},
					Description: "Automatically generated role for bot success-with-max-ttl",
					Expires:     &expiry,
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(libdefaults.MaxRenewableCertTTL),
					},
					Allow: types.RoleConditions{
						Impersonate: &types.ImpersonateConditions{
							Roles: []string{testRole.GetName()},
						},
						Rules: []types.Rule{
							types.NewRule(
								types.KindCertAuthority,
								[]string{types.VerbReadNoSecrets},
							),
						},
					},
				},
			},
		},
		{
			name:     "success with where on name",
			identity: authtest.TestUser(botCreatorWhere.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name:   "foo-xyzzy",
						Labels: map[string]string{},
					},
					Spec: &machineidv1pb.BotSpec{
						Roles:  []string{testRole.GetName()},
						Traits: []*machineidv1pb.Trait{},
					},
				},
			},
			assertError: require.NoError,
		},
		{
			name:     "failure with where on name",
			identity: authtest.TestUser(botCreatorWhere.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name:   "bar-xyzzy",
						Labels: map[string]string{},
					},
					Spec: &machineidv1pb.BotSpec{
						Roles:  []string{testRole.GetName()},
						Traits: []*machineidv1pb.Trait{},
					},
				},
			},
			assertError: require.Error,
		},
		{
			name:     "bot already exists",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: preExistingBot,
			},

			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAlreadyExists(err), "error should be already exists")
			},
		},
		{
			name:     "no permissions",
			identity: authtest.TestUser(unprivilegedUser.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: preExistingBot,
			},

			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name:     "validation - nil bot",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: nil,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "validation - nil metadata",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:     types.KindBot,
					Version:  types.V1,
					Metadata: nil,
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "validation - no name",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:     types.KindBot,
					Version:  types.V1,
					Metadata: &headerv1.Metadata{},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "validation - nil spec",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "terminator",
					},
					Spec: nil,
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "spec: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "validation - empty role",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "empty-string-role",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles:  []string{"foo", "", "bar"},
						Traits: []*machineidv1pb.Trait{},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "spec.roles: must not contain empty strings")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "scoped identity creates scoped bot",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "scoped-bot-success",
					},
					Scope: "/scopes/granted",
					Spec:  &machineidv1pb.BotSpec{},
				},
			},
			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "scoped-bot-success",
				},
				Scope: "/scopes/granted",
				Spec:  &machineidv1pb.BotSpec{},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-scoped-bot-success",
				},
			},
		},
		{
			name:     "unscoped identity creates scoped bot",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "scoped-bot-from-unscoped",
					},
					Scope: "/scopes/granted",
					Spec:  &machineidv1pb.BotSpec{},
				},
			},
			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "scoped-bot-from-unscoped",
				},
				Scope: "/scopes/granted",
				Spec:  &machineidv1pb.BotSpec{},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-scoped-bot-from-unscoped",
				},
			},
		},
		{
			name:     "scoped identity wrong scope",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "scoped-bot-denied",
					},
					Scope: "/scopes/ungranted",
					Spec:  &machineidv1pb.BotSpec{},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			},
		},
		{
			name:     "scoped identity cannot create unscoped bot",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "unscoped-from-scoped",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(tt.identity)
			require.NoError(t, err)

			bot, err := client.BotServiceClient().CreateBot(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned bot matches
				require.Empty(t, cmp.Diff(tt.want, bot, protocmp.Transform()))
			}
			if tt.wantUser != nil {
				gotUser, err := srv.Auth().GetUser(ctx, tt.wantUser.GetName(), false)
				require.NoError(t, err)
				require.Empty(t,
					cmp.Diff(
						tt.wantUser,
						gotUser,
						cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
						cmpopts.IgnoreFields(types.CreatedBy{}, "Time"),
						cmpopts.IgnoreFields(types.UserStatusV2{}, "MfaWeakestDevice"),
					),
				)
			}
			if tt.wantRole != nil {
				require.NoError(t, tt.wantRole.CheckAndSetDefaults())

				gotUser, err := srv.Auth().GetRole(ctx, tt.wantRole.GetName())
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(
					tt.wantRole,
					gotUser,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")),
				)
			}
		})
	}
}

// TestUpdateBot is an integration test that uses a real gRPC client/server.
func TestUpdateBot(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botUpdaterUser, _, err := authtest.CreateUserAndRole(srv.Auth(), "bot-updater", []string{}, []types.Rule{
		{
			Resources: []string{types.KindBot},
			Verbs:     []string{types.VerbUpdate},
		},
	})
	require.NoError(t, err)
	beforeRole, err := authtest.CreateRole(ctx, srv.Auth(), "before-role", types.RoleSpecV6{})
	require.NoError(t, err)
	afterRole, err := authtest.CreateRole(ctx, srv.Auth(), "after-role", types.RoleSpecV6{})
	require.NoError(t, err)
	unprivilegedUser, err := authtest.CreateUser(ctx, srv.Auth(), "no-perms", beforeRole)
	require.NoError(t, err)

	// Create a pre-existing bot so we can check you can update an existing bot.
	client, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:        "pre-existing",
				Description: "before",
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: []string{beforeRole.GetName()},
				Traits: []*machineidv1pb.Trait{
					{
						Name:   constants.TraitLogins,
						Values: []string{"before"},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	// We find the user associated with the Bot and set the generation label. This allows us to ensure that the
	// generation label is preserved when UpsertBot is called.
	{
		preExistingBotUser, err := srv.Auth().GetUser(ctx, preExistingBot.Status.UserName, false)
		require.NoError(t, err)
		meta := preExistingBotUser.GetMetadata()
		meta.Labels[types.BotGenerationLabel] = "1337"
		preExistingBotUser.SetMetadata(meta)
		_, err = srv.Auth().UpsertUser(ctx, preExistingBotUser)
		require.NoError(t, err)
	}

	tests := []struct {
		name string
		user string
		req  *machineidv1pb.UpdateBotRequest

		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.Bot
		wantUser    *types.UserV2
		wantRole    *types.RoleV6
	}{
		{
			name: "success",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:        preExistingBot.Metadata.Name,
						Description: "after",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{afterRole.GetName()},
						Traits: []*machineidv1pb.Trait{
							{
								Name:   constants.TraitLogins,
								Values: []string{"after"},
							},
							{
								Name: constants.TraitKubeUsers,
								Values: []string{
									"after",
								},
							},
						},
						MaxSessionTtl: durationpb.New(libdefaults.MaxRenewableCertTTL),
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles", "spec.traits", "spec.max_session_ttl", "metadata.description"},
				},
			},

			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:        preExistingBot.Metadata.Name,
					Description: "after",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{afterRole.GetName()},
					Traits: []*machineidv1pb.Trait{
						{
							Name:   constants.TraitLogins,
							Values: []string{"after"},
						},
						{
							Name: constants.TraitKubeUsers,
							Values: []string{
								"after",
							},
						},
					},
					MaxSessionTtl: durationpb.New(libdefaults.MaxRenewableCertTTL),
				},
				Status: &machineidv1pb.BotStatus{
					UserName: preExistingBot.Status.UserName,
					RoleName: preExistingBot.Status.RoleName,
				},
			},
			wantUser: &types.UserV2{
				Kind:    types.KindUser,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:        preExistingBot.Status.UserName,
					Description: "after",
					Namespace:   defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel:           preExistingBot.Metadata.Name,
						types.BotGenerationLabel: "1337",
					},
				},
				Spec: types.UserSpecV2{
					Roles: []string{preExistingBot.Status.RoleName},
					Traits: map[string][]string{
						constants.TraitLogins:    {"after"},
						constants.TraitKubeUsers: {"after"},
					},
					CreatedBy: types.CreatedBy{
						// We don't expect this to change because an update does
						// not adjust the CreatedBy field.
						User: types.UserRef{Name: "Admin.localhost"},
					},
				},
				Status: types.UserStatusV2{
					PasswordState: types.PasswordState_PASSWORD_STATE_UNSET,
				},
			},
			wantRole: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:      preExistingBot.Status.RoleName,
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel: preExistingBot.Metadata.Name,
					},
					Description: "Automatically generated role for bot pre-existing",
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(libdefaults.MaxRenewableCertTTL),
					},
					Allow: types.RoleConditions{
						Impersonate: &types.ImpersonateConditions{
							Roles: []string{afterRole.GetName()},
						},
						Rules: []types.Rule{
							types.NewRule(types.KindCertAuthority, []string{types.VerbReadNoSecrets}),
						},
					},
				},
			},
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:        "valid-bot",
						Description: preExistingBot.Metadata.Description,
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "validation - nil bot",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: nil,
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles"},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "bot: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - nil bot spec",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:        "bernard-lowe",
						Description: "before",
					},
					Spec: nil,
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles"},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "bot.spec: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - nil metadata",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles"},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "bot.metadata: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no name",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:        "",
						Description: preExistingBot.Metadata.Description,
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles"},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "bot.metadata.name: must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no update mask",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:        "foo",
						Description: "before",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
				UpdateMask: nil,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "update_mask: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no update mask paths",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:        "foo",
						Description: preExistingBot.Metadata.Description,
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "update_mask.paths: must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - empty string role",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:        preExistingBot.Metadata.Name,
						Description: preExistingBot.Metadata.Description,
					},
					Spec: &machineidv1pb.BotSpec{
						Roles:  []string{"foo", "", "bar"},
						Traits: []*machineidv1pb.Trait{},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles"},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "spec.roles: must not contain empty strings")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			bot, err := client.BotServiceClient().UpdateBot(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned bot matches
				require.Empty(
					t,
					cmp.Diff(
						tt.want,
						bot,
						protocmp.Transform(),
						protocmp.SortRepeatedFields(
							&machineidv1pb.BotSpec{},
							"traits",
						),
					),
				)
			}
			if tt.wantUser != nil {
				gotUser, err := srv.Auth().GetUser(ctx, tt.wantUser.GetName(), false)
				require.NoError(t, err)
				require.Empty(t,
					cmp.Diff(
						tt.wantUser,
						gotUser,
						cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
						cmpopts.IgnoreFields(types.CreatedBy{}, "Time"),
						cmpopts.IgnoreFields(types.UserStatusV2{}, "MfaWeakestDevice"),
					),
				)
			}
			if tt.wantRole != nil {
				require.NoError(t, tt.wantRole.CheckAndSetDefaults())
				gotUser, err := srv.Auth().GetRole(ctx, tt.wantRole.GetName())
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(
					tt.wantRole,
					gotUser,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")),
				)
			}
		})
	}
}

// TestUpsertBot is an integration test that uses a real gRPC client/server.
func TestUpsertBot(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	t.Setenv("TELEPORT_UNSTABLE_SCOPES_MWI", "yes")
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botCreator, _, err := authtest.CreateUserAndRole(srv.Auth(), "bot-creator", []string{}, []types.Rule{
		{
			Resources: []string{types.KindBot},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate},
		},
	})
	require.NoError(t, err)
	botWhereCreator, _, err := authtest.CreateUserAndRole(srv.Auth(), "bot-where-creator", []string{}, []types.Rule{
		{
			Resources: []string{types.KindBot},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate},
			Where:     `has_prefix(resource.metadata.name, "foo")`,
		},
	})
	require.NoError(t, err)
	testRole, err := authtest.CreateRole(ctx, srv.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)
	unprivilegedUser, err := authtest.CreateUser(ctx, srv.Auth(), "no-perms", testRole)
	require.NoError(t, err)

	// Create a pre-existing bot so we can check you can upsert over an existing bot.
	client, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "pre-existing",
				Labels: map[string]string{
					"my-label":       "my-value",
					"my-other-label": "my-other-value",
				},
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: []string{testRole.GetName()},
			},
		},
	})
	require.NoError(t, err)
	expiry := time.Now().Add(time.Hour)

	// We find the user associated with the Bot and set the generation label. This allows us to ensure that the
	// generation label is preserved when UpsertBot is called.
	{
		preExistingBotUser, err := srv.Auth().GetUser(ctx, preExistingBot.Status.UserName, false)
		require.NoError(t, err)
		meta := preExistingBotUser.GetMetadata()
		meta.Labels[types.BotGenerationLabel] = "1337"
		preExistingBotUser.SetMetadata(meta)
		_, err = srv.Auth().UpsertUser(ctx, preExistingBotUser)
		require.NoError(t, err)
	}

	// Scoped identity setup.
	scopedSvc := client.ScopedAccessServiceClient()
	scopedRole, err := scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-bot-upserter",
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/scopes/granted", "/scopes/ungranted"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Verbs:     []string{types.VerbCreate, types.VerbUpdate},
						Resources: []string{types.KindBot},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	scopedUser, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-user")
	require.NoError(t, err)

	_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			SubKind: scopedaccess.SubKindDynamic,
			Scope:   "/scopes",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: scopedUser.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: scopedRole.Role.Metadata.Name, Scope: "/scopes/granted"},
				},
			},
		},
	})
	require.NoError(t, err)

	// Pre-existing scoped bot for scope-transition tests.
	_, err = client.BotServiceClient().UpsertBot(ctx, &machineidv1pb.UpsertBotRequest{
		Bot: &machineidv1pb.Bot{
			Metadata: &headerv1.Metadata{Name: "scope-change-test"},
			Scope:    "/scopes/granted",
			Spec:     &machineidv1pb.BotSpec{},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		identity authtest.TestIdentity
		req      *machineidv1pb.UpsertBotRequest

		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.Bot
		wantUser    *types.UserV2
		wantRole    *types.RoleV6
	}{
		{
			name:     "new",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
						Labels: map[string]string{
							"my-label":       "my-value",
							"my-other-label": "my-other-value",
						},
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
						Traits: []*machineidv1pb.Trait{
							{
								Name:   constants.TraitLogins,
								Values: []string{"root"},
							},
						},
					},
				},
			},

			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "new",
					Labels: map[string]string{
						"my-label":       "my-value",
						"my-other-label": "my-other-value",
					},
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
					Traits: []*machineidv1pb.Trait{
						{
							Name:   constants.TraitLogins,
							Values: []string{"root"},
						},
					},
					MaxSessionTtl: durationpb.New(libdefaults.DefaultBotMaxSessionTTL),
				},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-new",
					RoleName: "bot-new",
				},
			},
			wantUser: &types.UserV2{
				Kind:    types.KindUser,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "bot-new",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel:           "new",
						types.BotGenerationLabel: "0",
						"my-label":               "my-value",
						"my-other-label":         "my-other-value",
					},
				},
				Spec: types.UserSpecV2{
					Roles: []string{"bot-new"},
					Traits: map[string][]string{
						constants.TraitLogins: {"root"},
					},
					CreatedBy: types.CreatedBy{
						User: types.UserRef{Name: botCreator.GetName()},
					},
				},
			},
			wantRole: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:      "bot-new",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel: "new",
					},
					Description: "Automatically generated role for bot new",
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(libdefaults.DefaultBotMaxSessionTTL),
					},
					Allow: types.RoleConditions{
						Impersonate: &types.ImpersonateConditions{
							Roles: []string{testRole.GetName()},
						},
						Rules: []types.Rule{
							types.NewRule(types.KindCertAuthority, []string{types.VerbReadNoSecrets}),
						},
					},
				},
			},
		},
		{
			name:     "new with expiry",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new-with-expiry",
						Labels: map[string]string{
							"my-label":       "my-value",
							"my-other-label": "my-other-value",
						},
						Expires: timestamppb.New(expiry),
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
						Traits: []*machineidv1pb.Trait{
							{
								Name:   constants.TraitLogins,
								Values: []string{"root"},
							},
						},
					},
				},
			},

			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "new-with-expiry",
					Labels: map[string]string{
						"my-label":       "my-value",
						"my-other-label": "my-other-value",
					},
					Expires: timestamppb.New(expiry),
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
					Traits: []*machineidv1pb.Trait{
						{
							Name:   constants.TraitLogins,
							Values: []string{"root"},
						},
					},
					MaxSessionTtl: durationpb.New(libdefaults.DefaultBotMaxSessionTTL),
				},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-new-with-expiry",
					RoleName: "bot-new-with-expiry",
				},
			},
			wantUser: &types.UserV2{
				Kind:    types.KindUser,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "bot-new-with-expiry",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel:           "new-with-expiry",
						types.BotGenerationLabel: "0",
						"my-label":               "my-value",
						"my-other-label":         "my-other-value",
					},
					Expires: &expiry,
				},
				Spec: types.UserSpecV2{
					Roles: []string{"bot-new-with-expiry"},
					Traits: map[string][]string{
						constants.TraitLogins: {"root"},
					},
					CreatedBy: types.CreatedBy{
						User: types.UserRef{Name: botCreator.GetName()},
					},
				},
			},
			wantRole: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:      "bot-new-with-expiry",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel: "new-with-expiry",
					},
					Description: "Automatically generated role for bot new-with-expiry",
					Expires:     &expiry,
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(libdefaults.DefaultBotMaxSessionTTL),
					},
					Allow: types.RoleConditions{
						Impersonate: &types.ImpersonateConditions{
							Roles: []string{testRole.GetName()},
						},
						Rules: []types.Rule{
							types.NewRule(types.KindCertAuthority, []string{types.VerbReadNoSecrets}),
						},
					},
				},
			},
		},
		{
			name:     "already exists",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: preExistingBot,
			},

			assertError: require.NoError,
			want:        preExistingBot,
			wantUser: &types.UserV2{
				Kind:    types.KindUser,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "bot-pre-existing",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel:           "pre-existing",
						types.BotGenerationLabel: "1337",
						"my-label":               "my-value",
						"my-other-label":         "my-other-value",
					},
				},
				Spec: types.UserSpecV2{
					CreatedBy: types.CreatedBy{
						User: types.UserRef{Name: botCreator.GetName()},
					},
					Roles:  []string{"bot-pre-existing"},
					Traits: nil,
				},
			},
			wantRole: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:      "bot-pre-existing",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel: "pre-existing",
					},
					Description: "Automatically generated role for bot pre-existing",
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(libdefaults.DefaultBotMaxSessionTTL),
					},
					Allow: types.RoleConditions{
						Impersonate: &types.ImpersonateConditions{
							Roles: []string{testRole.GetName()},
						},
						Rules: []types.Rule{
							types.NewRule(
								types.KindCertAuthority,
								[]string{types.VerbReadNoSecrets},
							),
						},
					},
				},
			},
		},
		{
			name:     "already exists with max session ttl",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "pre-existing",
						Labels: map[string]string{
							"my-label":       "my-value",
							"my-other-label": "my-other-value",
						},
					},
					Spec: &machineidv1pb.BotSpec{
						Roles:         []string{testRole.GetName()},
						MaxSessionTtl: durationpb.New(libdefaults.MaxRenewableCertTTL),
					},
				},
			},

			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "pre-existing",
					Labels: map[string]string{
						"my-label":       "my-value",
						"my-other-label": "my-other-value",
					},
				},
				Spec: &machineidv1pb.BotSpec{
					Roles:         []string{testRole.GetName()},
					MaxSessionTtl: durationpb.New(libdefaults.MaxRenewableCertTTL),
				},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-pre-existing",
					RoleName: "bot-pre-existing",
				},
			},
			wantUser: &types.UserV2{
				Kind:    types.KindUser,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "bot-pre-existing",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel:           "pre-existing",
						types.BotGenerationLabel: "1337",
						"my-label":               "my-value",
						"my-other-label":         "my-other-value",
					},
				},
				Spec: types.UserSpecV2{
					CreatedBy: types.CreatedBy{
						User: types.UserRef{Name: botCreator.GetName()},
					},
					Roles:  []string{"bot-pre-existing"},
					Traits: nil,
				},
			},
			wantRole: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:      "bot-pre-existing",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						types.BotLabel: "pre-existing",
					},
					Description: "Automatically generated role for bot pre-existing",
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(libdefaults.MaxRenewableCertTTL),
					},
					Allow: types.RoleConditions{
						Impersonate: &types.ImpersonateConditions{
							Roles: []string{testRole.GetName()},
						},
						Rules: []types.Rule{
							types.NewRule(
								types.KindCertAuthority,
								[]string{types.VerbReadNoSecrets},
							),
						},
					},
				},
			},
		},
		{
			name:     "new with where",
			identity: authtest.TestUser(botWhereCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "foo-new",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: require.NoError,
		},
		{
			name:     "failed new with where",
			identity: authtest.TestUser(botWhereCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "not-foo-new",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name:     "no permissions",
			identity: authtest.TestUser(unprivilegedUser.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "not-foo-new",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},

			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name:     "validation - nil bot",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: nil,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "validation - nil metadata",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:     types.KindBot,
					Version:  types.V1,
					Metadata: nil,
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "validation - no name",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:     types.KindBot,
					Version:  types.V1,
					Metadata: &headerv1.Metadata{},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "validation - empty role",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Kind:    types.KindBot,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "empty-string-role",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles:  []string{"foo", "", "bar"},
						Traits: []*machineidv1pb.Trait{},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "spec.roles: must not contain empty strings")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "scoped identity upserts scoped bot",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "scoped-upsert-success",
					},
					Scope: "/scopes/granted",
					Spec:  &machineidv1pb.BotSpec{},
				},
			},
			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "scoped-upsert-success",
				},
				Scope: "/scopes/granted",
				Spec:  &machineidv1pb.BotSpec{},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-scoped-upsert-success",
				},
			},
		},
		{
			name:     "unscoped identity upserts scoped bot",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "scoped-upsert-from-unscoped",
					},
					Scope: "/scopes/granted",
					Spec:  &machineidv1pb.BotSpec{},
				},
			},
			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "scoped-upsert-from-unscoped",
				},
				Scope: "/scopes/granted",
				Spec:  &machineidv1pb.BotSpec{},
				Status: &machineidv1pb.BotStatus{
					UserName: "bot-scoped-upsert-from-unscoped",
				},
			},
		},
		{
			name:     "scoped identity wrong scope",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "scoped-upsert-denied",
					},
					Scope: "/scopes/ungranted",
					Spec:  &machineidv1pb.BotSpec{},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			},
		},
		{
			name:     "scoped identity cannot upsert unscoped bot",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "unscoped-from-scoped",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			},
		},
		{
			name:     "cannot change scope: scoped to unscoped",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{Name: "scope-change-test"},
					Spec:     &machineidv1pb.BotSpec{Roles: []string{testRole.GetName()}},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
			},
		},
		{
			name:     "cannot change scope: unscoped to scoped",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{Name: "pre-existing"},
					Scope:    "/scopes/granted",
					Spec:     &machineidv1pb.BotSpec{},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
			},
		},
		{
			name:     "cannot change scope: scoped to different scope",
			identity: authtest.TestUser(botCreator.GetName()),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{Name: "scope-change-test"},
					Scope:    "/scopes/ungranted",
					Spec:     &machineidv1pb.BotSpec{},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(tt.identity)
			require.NoError(t, err)

			bot, err := client.BotServiceClient().UpsertBot(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned bot matches
				require.Empty(t, cmp.Diff(tt.want, bot, protocmp.Transform()))
			}
			if tt.wantUser != nil {
				gotUser, err := srv.Auth().GetUser(ctx, tt.wantUser.GetName(), false)
				require.NoError(t, err)
				require.Empty(t,
					cmp.Diff(
						tt.wantUser,
						gotUser,
						cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
						cmpopts.IgnoreFields(types.CreatedBy{}, "Time"),
						cmpopts.IgnoreFields(types.UserStatusV2{}, "MfaWeakestDevice"),
					),
				)
			}
			if tt.wantRole != nil {
				require.NoError(t, tt.wantRole.CheckAndSetDefaults())
				gotUser, err := srv.Auth().GetRole(ctx, tt.wantRole.GetName())
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(
					tt.wantRole,
					gotUser,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")),
				)
			}
		})
	}
}

// TestGetBot is an integration test that uses a real gRPC client/server.
func TestGetBot(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	t.Setenv("TELEPORT_UNSTABLE_SCOPES_MWI", "yes")
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botGetterUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"bot-getter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindBot},
				Verbs:     []string{types.VerbRead},
			},
		})
	require.NoError(t, err)
	botGetterWhereUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"bot-getter-where",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindBot},
				Verbs:     []string{types.VerbRead},
				Where:     `has_prefix(resource.metadata.name, "foo")`,
			},
		})
	require.NoError(t, err)
	testRole, err := authtest.CreateRole(
		ctx, srv.Auth(), "test-role", types.RoleSpecV6{},
	)
	require.NoError(t, err)
	unprivilegedUser, err := authtest.CreateUser(
		ctx, srv.Auth(), "no-perms", testRole,
	)
	require.NoError(t, err)

	client, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "pre-existing",
					Labels: map[string]string{
						"my-label":       "my-value",
						"my-other-label": "my-other-value",
					},
					Description: "The maze wasn't meant for you",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)
	preExistingBot2, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "foo-pre-existing",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)

	// Scoped identity setup.
	scopedSvc := client.ScopedAccessServiceClient()
	scopedRole, err := scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-bot-getter",
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/scopes/granted", "/scopes/ungranted"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Verbs:     []string{types.VerbReadNoSecrets},
						Resources: []string{types.KindBot},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	scopedUser, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-user")
	require.NoError(t, err)

	_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			SubKind: scopedaccess.SubKindDynamic,
			Scope:   "/scopes",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: scopedUser.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: scopedRole.Role.Metadata.Name, Scope: "/scopes/granted"},
				},
			},
		},
	})
	require.NoError(t, err)

	scopedPreExisting, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-pre-existing",
			},
			Spec:  &machineidv1pb.BotSpec{},
			Scope: "/scopes/granted",
		},
	})
	require.NoError(t, err)
	_, err = client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-pre-existing-wrong-scope",
			},
			Spec:  &machineidv1pb.BotSpec{},
			Scope: "/scopes/ungranted",
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		identity    authtest.TestIdentity
		req         *machineidv1pb.GetBotRequest
		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.Bot
	}{
		{
			name:     "success",
			identity: authtest.TestUser(botGetterUser.GetName()),
			req: &machineidv1pb.GetBotRequest{
				BotName: preExistingBot.Metadata.Name,
			},

			assertError: require.NoError,
			want:        preExistingBot,
		},
		{
			name:     "success with where",
			identity: authtest.TestUser(botGetterWhereUser.GetName()),
			req: &machineidv1pb.GetBotRequest{
				BotName: preExistingBot2.Metadata.Name,
			},

			assertError: require.NoError,
			want:        preExistingBot2,
		},
		{
			name:     "no permissions with where",
			identity: authtest.TestUser(botGetterWhereUser.GetName()),
			req: &machineidv1pb.GetBotRequest{
				BotName: preExistingBot.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
		{
			name:     "no permissions",
			identity: authtest.TestUser(unprivilegedUser.GetName()),
			req: &machineidv1pb.GetBotRequest{
				BotName: preExistingBot.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name:     "validation - no bot name",
			identity: authtest.TestUser(botGetterUser.GetName()),
			req: &machineidv1pb.GetBotRequest{
				BotName: "",
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name:     "bot doesnt exist",
			identity: authtest.TestUser(botGetterUser.GetName()),
			req: &machineidv1pb.GetBotRequest{
				BotName: "non-existent",
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be bad parameter")
			},
		},
		{
			name:     "scoped identity gets scoped bot",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.GetBotRequest{
				BotName: scopedPreExisting.Metadata.Name,
			},
			assertError: require.NoError,
			want:        scopedPreExisting,
		},
		{
			name:     "unscoped identity gets scoped bot",
			identity: authtest.TestUser(botGetterUser.GetName()),
			req: &machineidv1pb.GetBotRequest{
				BotName: scopedPreExisting.Metadata.Name,
			},
			assertError: require.NoError,
			want:        scopedPreExisting,
		},
		{
			name:     "scoped identity wrong scope",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.GetBotRequest{
				BotName: "scoped-pre-existing-wrong-scope",
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				// GetBot returns NotFound rather than AccessDenied to avoid leaking existence.
				require.True(t, trace.IsNotFound(err), "expected not found, got: %v", err)
			},
		},
		{
			name:     "scoped identity cannot get unscoped bot",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.GetBotRequest{
				BotName: preExistingBot.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				// GetBot returns NotFound rather than AccessDenied to avoid leaking existence.
				require.True(t, trace.IsNotFound(err), "expected not found, got: %v", err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(tt.identity)
			require.NoError(t, err)

			bot, err := client.BotServiceClient().GetBot(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned bot matches
				require.Empty(t, cmp.Diff(tt.want, bot, protocmp.Transform()))
			}
		})
	}
}

// TestListBots is an integration test that uses a real gRPC client/server.
func TestListBots(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	t.Setenv("TELEPORT_UNSTABLE_SCOPES_MWI", "yes")
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botListerUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"bot-lister",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindBot},
				Verbs:     []string{types.VerbList},
			},
		})
	require.NoError(t, err)
	botListWhereUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"bot-lister-where",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindBot},
				Verbs:     []string{types.VerbList},
				Where:     `has_prefix(resource.metadata.name, "foo")`,
			},
		})
	require.NoError(t, err)
	testRole, err := authtest.CreateRole(
		ctx, srv.Auth(), "test-role", types.RoleSpecV6{},
	)
	require.NoError(t, err)
	unprivilegedUser, err := authtest.CreateUser(
		ctx, srv.Auth(), "no-perms", testRole,
	)
	require.NoError(t, err)

	client, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "pre-existing",
					Labels: map[string]string{
						"my-label":       "my-value",
						"my-other-label": "my-other-value",
					},
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)
	preExistingBot2, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "pre-existing-2",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)
	preExistingBot3, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "foo-pre-existing-2",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)

	// Scoped identity setup.
	scopedSvc := client.ScopedAccessServiceClient()
	scopedRole, err := scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-bot-lister",
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/scopes/granted", "/scopes/ungranted"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Verbs:     []string{types.VerbList},
						Resources: []string{types.KindBot},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	scopedUser, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-user")
	require.NoError(t, err)

	_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			Scope:   "/scopes",
			SubKind: scopedaccess.SubKindDynamic,
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: scopedUser.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: scopedRole.Role.Metadata.Name, Scope: "/scopes/granted"},
				},
			},
		},
	})
	require.NoError(t, err)

	// Create a 2nd scoped user with assignment at /scopes/ungranted (where no bots exist).
	scopedUser2, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-user-2")
	require.NoError(t, err)
	_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			SubKind: scopedaccess.SubKindDynamic,
			Scope:   "/scopes",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: scopedUser2.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: scopedRole.Role.Metadata.Name, Scope: "/scopes/ungranted"},
				},
			},
		},
	})
	require.NoError(t, err)

	scopedPreExisting, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-pre-existing",
			},
			Spec:  &machineidv1pb.BotSpec{},
			Scope: "/scopes/granted",
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		identity    authtest.TestIdentity
		req         *machineidv1pb.ListBotsRequest
		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.ListBotsResponse
	}{
		{
			name:        "success",
			identity:    authtest.TestUser(botListerUser.GetName()),
			req:         &machineidv1pb.ListBotsRequest{},
			assertError: require.NoError,
			want: &machineidv1pb.ListBotsResponse{
				Bots: []*machineidv1pb.Bot{
					preExistingBot,
					preExistingBot2,
					preExistingBot3,
					scopedPreExisting,
				},
			},
		},
		{
			name:        "success with where",
			identity:    authtest.TestUser(botListWhereUser.GetName()),
			req:         &machineidv1pb.ListBotsRequest{},
			assertError: require.NoError,
			want: &machineidv1pb.ListBotsResponse{
				Bots: []*machineidv1pb.Bot{
					preExistingBot3,
				},
			},
		},
		{
			name:     "no permissions",
			identity: authtest.TestUser(unprivilegedUser.GetName()),
			req:      &machineidv1pb.ListBotsRequest{},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name:        "scoped identity lists scoped bots",
			identity:    authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req:         &machineidv1pb.ListBotsRequest{},
			assertError: require.NoError,
			want: &machineidv1pb.ListBotsResponse{
				Bots: []*machineidv1pb.Bot{
					scopedPreExisting,
				},
			},
		},
		{
			// Scoped user at /scopes/ungranted where no bots exist: returns empty list.
			name:        "scoped identity at scope with no bots lists nothing",
			identity:    authtest.TestScopedUser(scopedUser2.GetName(), "/scopes/ungranted"),
			req:         &machineidv1pb.ListBotsRequest{},
			assertError: require.NoError,
			want: &machineidv1pb.ListBotsResponse{
				Bots: []*machineidv1pb.Bot{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(tt.identity)
			require.NoError(t, err)

			res, err := client.BotServiceClient().ListBots(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned data matches
				require.Empty(
					t, cmp.Diff(
						tt.want,
						res,
						protocmp.Transform(),
						protocmp.SortRepeatedFields(&machineidv1pb.ListBotsResponse{}, "bots"),
					),
				)
			}
		})
	}
}

// TestDeleteBot is an integration test that uses a real gRPC client/server.
func TestDeleteBot(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	t.Setenv("TELEPORT_UNSTABLE_SCOPES_MWI", "yes")
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botDeleterUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"bot-deleter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindBot},
				Verbs:     []string{types.VerbDelete},
			},
		})
	require.NoError(t, err)
	botWhereDeleterUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"bot-deleter-where",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindBot},
				Verbs:     []string{types.VerbDelete},
				Where:     `has_prefix(resource.metadata.name, "foo")`,
			},
		})
	require.NoError(t, err)
	testRole, err := authtest.CreateRole(
		ctx, srv.Auth(), "test-role", types.RoleSpecV6{},
	)
	require.NoError(t, err)
	unprivilegedUser, err := authtest.CreateUser(
		ctx, srv.Auth(), "no-perms", testRole,
	)
	require.NoError(t, err)

	// Create a user/role with a bot-like name but that isn't a bot to ensure we
	// don't delete it
	_, err = authtest.CreateUser(
		ctx, srv.Auth(), "bot-not-bot", testRole,
	)
	require.NoError(t, err)
	_, err = authtest.CreateRole(
		ctx, srv.Auth(), "bot-not-bot", types.RoleSpecV6{},
	)
	require.NoError(t, err)

	client, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "pre-existing",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)
	preExistingBot3, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "pre-existing-3",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)
	preExistingBot4, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "foo-pre-existing",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)
	preExistingBot5, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "not-foo-pre-existing",
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: []string{testRole.GetName()},
				},
			},
		},
	)
	require.NoError(t, err)

	// Scoped identity setup: create a scoped role, user, and assignment.
	scopedSvc := client.ScopedAccessServiceClient()
	scopedRole, err := scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-bot-deleter",
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/scopes/granted", "/scopes/ungranted"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Verbs:     []string{types.VerbDelete},
						Resources: []string{types.KindBot},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	scopedUser, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-user")
	require.NoError(t, err)

	// Create scoped role assignment linking user to scoped role.
	_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			SubKind: scopedaccess.SubKindDynamic,
			Scope:   "/scopes",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: scopedUser.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: scopedRole.Role.Metadata.Name, Scope: "/scopes/granted"},
				},
			},
		},
	})
	require.NoError(t, err)

	// Create scoped bots for delete tests.
	// Note: scoped bots cannot have roles set.
	scopedPreExisting, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "scoped-pre-existing",
				},
				Spec:  &machineidv1pb.BotSpec{},
				Scope: "/scopes/granted",
			},
		},
	)
	require.NoError(t, err)
	scopedPreExistingUnscoped, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "scoped-pre-existing-unscoped",
				},
				Spec:  &machineidv1pb.BotSpec{},
				Scope: "/scopes/granted",
			},
		},
	)
	require.NoError(t, err)
	scopedPreExistingWrongScope, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "scoped-pre-existing-wrong-scope",
				},
				Spec:  &machineidv1pb.BotSpec{},
				Scope: "/scopes/ungranted",
			},
		},
	)
	require.NoError(t, err)

	tests := []struct {
		name                  string
		identity              authtest.TestIdentity
		req                   *machineidv1pb.DeleteBotRequest
		assertError           require.ErrorAssertionFunc
		checkResourcesDeleted bool
		scoped                bool
	}{
		{
			name:     "success",
			identity: authtest.TestUser(botDeleterUser.GetName()),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: preExistingBot.Metadata.Name,
			},
			assertError:           require.NoError,
			checkResourcesDeleted: true,
		},
		{
			name:     "success with where",
			identity: authtest.TestUser(botWhereDeleterUser.GetName()),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: preExistingBot4.Metadata.Name,
			},
			assertError:           require.NoError,
			checkResourcesDeleted: true,
		},
		{
			name:     "no permissions with where",
			identity: authtest.TestUser(botWhereDeleterUser.GetName()),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: preExistingBot5.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name:     "no permissions",
			identity: authtest.TestUser(unprivilegedUser.GetName()),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: preExistingBot3.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name:     "non existent",
			identity: authtest.TestUser(botDeleterUser.GetName()),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: "does-not-exist",
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
		{
			name:     "non-bot role",
			identity: authtest.TestUser(botDeleterUser.GetName()),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: "not-bot",
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "missing bot label matching bot name")
			},
		},
		{
			name:     "validation - no bot name",
			identity: authtest.TestUser(botDeleterUser.GetName()),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: "",
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "bot_name: must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be access denied")
			},
		},
		{
			name:     "scoped identity deletes scoped bot",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: scopedPreExisting.Metadata.Name,
			},
			assertError:           require.NoError,
			checkResourcesDeleted: true,
			scoped:                true,
		},
		{
			name:     "unscoped identity deletes scoped bot",
			identity: authtest.TestUser(botDeleterUser.GetName()),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: scopedPreExistingUnscoped.Metadata.Name,
			},
			assertError:           require.NoError,
			checkResourcesDeleted: true,
			scoped:                true,
		},
		{
			name:     "scoped identity wrong scope",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: scopedPreExistingWrongScope.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			},
		},
		{
			name:     "scoped identity cannot delete unscoped bot",
			identity: authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: preExistingBot3.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(tt.identity)
			require.NoError(t, err)

			_, err = client.BotServiceClient().DeleteBot(ctx, tt.req)
			tt.assertError(t, err)
			if tt.checkResourcesDeleted {
				_, err := srv.Auth().GetUser(ctx, machineidv1.BotResourceName(tt.req.BotName), false)
				require.True(t, trace.IsNotFound(err), "bot user should be deleted")
				if !tt.scoped {
					_, err = srv.Auth().GetRole(ctx, machineidv1.BotResourceName(tt.req.BotName))
					require.True(t, trace.IsNotFound(err), "bot role should be deleted")
				}
			}
		})
	}
}

func TestStrongValidateBot(t *testing.T) {
	newBot := func(mutate func(bot *machineidv1pb.Bot)) *machineidv1pb.Bot {
		bot := &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test-bot",
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: []string{"test-role"},
			},
		}
		if mutate != nil {
			mutate(bot)
		}
		return bot
	}
	newScopedBot := func(mutate func(bot *machineidv1pb.Bot)) *machineidv1pb.Bot {
		bot := &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test-bot",
			},
			Scope: "/test/scope",
			Spec:  &machineidv1pb.BotSpec{},
		}
		if mutate != nil {
			mutate(bot)
		}
		return bot
	}

	isBadParam := func(t require.TestingT, err error, i ...any) {
		require.True(t, trace.IsBadParameter(err), "expected bad parameter error, got: %v", err)
	}

	tests := []struct {
		name        string
		bot         *machineidv1pb.Bot
		assertError require.ErrorAssertionFunc
	}{
		{
			name:        "nil bot",
			bot:         nil,
			assertError: isBadParam,
		},
		{
			name:        "nil metadata",
			bot:         newBot(func(b *machineidv1pb.Bot) { b.Metadata = nil }),
			assertError: isBadParam,
		},
		{
			name:        "empty name",
			bot:         newBot(func(b *machineidv1pb.Bot) { b.Metadata.Name = "" }),
			assertError: isBadParam,
		},
		{
			name:        "nil spec",
			bot:         newBot(func(b *machineidv1pb.Bot) { b.Spec = nil }),
			assertError: isBadParam,
		},
		{
			name: "roles contains empty string",
			bot: newBot(func(b *machineidv1pb.Bot) {
				b.Spec.Roles = []string{"valid-role", ""}
			}),
			assertError: isBadParam,
		},
		{
			name:        "valid unscoped bot",
			bot:         newBot(nil),
			assertError: require.NoError,
		},
		{
			name: "valid unscoped bot with no roles",
			bot: newBot(func(b *machineidv1pb.Bot) {
				b.Spec.Roles = nil
			}),
			assertError: require.NoError,
		},
		{
			name: "valid unscoped bot with traits and max_session_ttl",
			bot: newBot(func(b *machineidv1pb.Bot) {
				b.Spec.Traits = []*machineidv1pb.Trait{{Name: "foo", Values: []string{"bar"}}}
				b.Spec.MaxSessionTtl = durationpb.New(time.Hour)
			}),
			assertError: require.NoError,
		},
		{
			name:        "scoped bot with invalid scope",
			bot:         newScopedBot(func(b *machineidv1pb.Bot) { b.Scope = "no-leading-slash" }),
			assertError: isBadParam,
		},
		{
			name: "scoped bot with roles set",
			bot: newScopedBot(func(b *machineidv1pb.Bot) {
				b.Spec.Roles = []string{"some-role"}
			}),
			assertError: isBadParam,
		},
		{
			name: "scoped bot with traits set",
			bot: newScopedBot(func(b *machineidv1pb.Bot) {
				b.Spec.Traits = []*machineidv1pb.Trait{{Name: "foo", Values: []string{"bar"}}}
			}),
			assertError: isBadParam,
		},
		{
			name: "scoped bot with max_session_ttl set",
			bot: newScopedBot(func(b *machineidv1pb.Bot) {
				b.Spec.MaxSessionTtl = durationpb.New(time.Hour)
			}),
			assertError: isBadParam,
		},
		{
			name:        "valid scoped bot",
			bot:         newScopedBot(nil),
			assertError: require.NoError,
		},
		{
			name:        "valid scoped bot at root scope",
			bot:         newScopedBot(func(b *machineidv1pb.Bot) { b.Scope = "/" }),
			assertError: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := machineidv1.StrongValidateBot(tt.bot)
			tt.assertError(t, err)
		})
	}
}

// TestStrongValidateBotScopedFuzz fuzzes the spec fields of a scoped Bot. This
// is designed to ensure that a new field that is added for unscoped Bots is
// not accidentally valid for scoped Bots, which have a constrained set of
// fields.
//
// All new spec fields which are permitted for scoped bots must be added to
// the allowedScopedSpecFields map.
func TestStrongValidateBotScopedFuzz(t *testing.T) {
	allowedScopedSpecFields := map[protoreflect.Name]bool{
		// No fields are currently allowed on scoped bots.
	}

	specDesc := (&machineidv1pb.BotSpec{}).ProtoReflect().Descriptor()
	fields := specDesc.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		t.Run(string(fd.Name()), func(t *testing.T) {
			spec := &machineidv1pb.BotSpec{}
			protoSetNonZeroField(spec.ProtoReflect(), fd)

			bot := &machineidv1pb.Bot{
				Metadata: &headerv1.Metadata{Name: "test-bot"},
				Scope:    "/test/scope",
				Spec:     spec,
			}

			err := machineidv1.StrongValidateBot(bot)
			if allowedScopedSpecFields[fd.Name()] {
				require.NoError(t, err,
					"field %q is in allow-list but StrongValidateBot rejected it", fd.Name())
			} else {
				require.Error(t, err,
					"field %q is not in allow-list but StrongValidateBot accepted a scoped bot "+
						"with it set; either forbid it in StrongValidateBot or add it to allowedScopedSpecFields",
					fd.Name())
				require.True(t, trace.IsBadParameter(err),
					"field %q: expected bad parameter, got: %v", fd.Name(), err)
			}
		})
	}
}

// protoSetNonZeroField sets a non-zero value for fd in m.
// For list fields it appends one non-zero element.
// For message fields it recursively sets scalar sub-fields to non-zero values.
func protoSetNonZeroField(m protoreflect.Message, fd protoreflect.FieldDescriptor) {
	if fd.IsList() {
		list := m.Mutable(fd).List()
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			protoSetNonZeroMessageFields(elem.Message())
			list.Append(elem)
		} else {
			list.Append(protoNonZeroScalarValue(fd))
		}
		return
	}
	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		protoSetNonZeroMessageFields(m.Mutable(fd).Message())
		return
	}
	m.Set(fd, protoNonZeroScalarValue(fd))
}

// protoSetNonZeroMessageFields sets scalar fields inside a message to non-zero
// values (one level deep, to avoid infinite recursion on cyclic messages).
func protoSetNonZeroMessageFields(m protoreflect.Message) {
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
			continue // skip nested messages to avoid recursion
		}
		if fd.IsList() {
			m.Mutable(fd).List().Append(protoNonZeroScalarValue(fd))
		} else {
			m.Set(fd, protoNonZeroScalarValue(fd))
		}
	}
}

// protoNonZeroScalarValue returns a non-zero protoreflect.Value for a scalar field.
func protoNonZeroScalarValue(fd protoreflect.FieldDescriptor) protoreflect.Value {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(true)
	case protoreflect.EnumKind:
		return protoreflect.ValueOfEnum(1)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(1)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(1)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(1)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(1)
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(1)
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(1)
	case protoreflect.StringKind:
		return protoreflect.ValueOfString("placeholder")
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes([]byte("placeholder"))
	default:
		panic(fmt.Sprintf("unhandled proto field kind: %v", fd.Kind()))
	}
}

func newTestTLSServer(t testing.TB) (*authtest.TLSServer, *eventstest.MockRecorderEmitter) {
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)

	emitter := &eventstest.MockRecorderEmitter{}
	srv, err := as.NewTestTLSServer(func(config *authtest.TLSServerConfig) {
		config.APIConfig.Emitter = emitter
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv, emitter
}
