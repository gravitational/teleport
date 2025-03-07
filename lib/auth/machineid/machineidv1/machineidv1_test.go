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
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
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
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botCreator, _, err := auth.CreateUserAndRole(
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
	testRole, err := auth.CreateRole(
		ctx, srv.Auth(), "test-role", types.RoleSpecV6{},
	)
	require.NoError(t, err)
	unprivilegedUser, err := auth.CreateUser(
		ctx, srv.Auth(), "no-perms", testRole,
	)
	require.NoError(t, err)

	client, err := srv.NewClient(auth.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
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

	tests := []struct {
		name string
		user string
		req  *machineidv1pb.CreateBotRequest

		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.Bot
		wantUser    *types.UserV2
		wantRole    *types.RoleV6
	}{
		{
			name: "success",
			user: botCreator.GetName(),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "success",
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
							{
								Name:   constants.TraitKubeUsers,
								Values: []string{},
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
					Name: "success",
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
						MaxSessionTTL: types.Duration(12 * time.Hour),
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
			name: "success with expiry",
			user: botCreator.GetName(),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
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
						MaxSessionTTL: types.Duration(12 * time.Hour),
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
			name: "bot already exists",
			user: botCreator.GetName(),
			req: &machineidv1pb.CreateBotRequest{
				Bot: preExistingBot,
			},

			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAlreadyExists(err), "error should be already exists")
			},
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req:  &machineidv1pb.CreateBotRequest{},

			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "validation - nil bot",
			user: botCreator.GetName(),
			req: &machineidv1pb.CreateBotRequest{
				Bot: nil,
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - nil metadata",
			user: botCreator.GetName(),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: nil,
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no name",
			user: botCreator.GetName(),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - nil spec",
			user: botCreator.GetName(),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "terminator",
					},
					Spec: nil,
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "spec: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - empty role",
			user: botCreator.GetName(),
			req: &machineidv1pb.CreateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "empty-string-role",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles:  []string{"foo", "", "bar"},
						Traits: []*machineidv1pb.Trait{},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "spec.roles: must not contain empty strings")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
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

	botUpdaterUser, _, err := auth.CreateUserAndRole(srv.Auth(), "bot-updater", []string{}, []types.Rule{
		{
			Resources: []string{types.KindBot},
			Verbs:     []string{types.VerbUpdate},
		},
	})
	require.NoError(t, err)
	beforeRole, err := auth.CreateRole(ctx, srv.Auth(), "before-role", types.RoleSpecV6{})
	require.NoError(t, err)
	afterRole, err := auth.CreateRole(ctx, srv.Auth(), "after-role", types.RoleSpecV6{})
	require.NoError(t, err)
	unprivilegedUser, err := auth.CreateUser(ctx, srv.Auth(), "no-perms", beforeRole)
	require.NoError(t, err)

	// Create a pre-existing bot so we can check you can update an existing bot.
	client, err := srv.NewClient(auth.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Metadata: &headerv1.Metadata{
				Name: "pre-existing",
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
					Metadata: &headerv1.Metadata{
						Name: preExistingBot.Metadata.Name,
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
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles", "spec.traits"},
				},
			},

			assertError: require.NoError,
			want: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: preExistingBot.Metadata.Name,
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
					Name:      preExistingBot.Status.UserName,
					Namespace: defaults.Namespace,
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
						MaxSessionTTL: types.Duration(12 * time.Hour),
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
			req:  &machineidv1pb.UpdateBotRequest{},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
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
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bot: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - nil bot spec",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "bernard-lowe",
					},
					Spec: nil,
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles"},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bot.spec: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - nil metadata",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles"},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bot.metadata: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no name",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.roles"},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bot.metadata.name: must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no update mask",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "foo",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
				UpdateMask: nil,
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "update_mask: must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no update mask paths",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "foo",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{beforeRole.GetName()},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "update_mask.paths: must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - empty string role",
			user: botUpdaterUser.GetName(),
			req: &machineidv1pb.UpdateBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: preExistingBot.Metadata.Name,
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
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "spec.roles: must not contain empty strings")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
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
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botCreator, _, err := auth.CreateUserAndRole(srv.Auth(), "bot-creator", []string{}, []types.Rule{
		{
			Resources: []string{types.KindBot},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate},
		},
	})
	require.NoError(t, err)
	testRole, err := auth.CreateRole(ctx, srv.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)
	unprivilegedUser, err := auth.CreateUser(ctx, srv.Auth(), "no-perms", testRole)
	require.NoError(t, err)

	// Create a pre-existing bot so we can check you can upsert over an existing bot.
	client, err := srv.NewClient(auth.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
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

	tests := []struct {
		name string
		user string
		req  *machineidv1pb.UpsertBotRequest

		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.Bot
		wantUser    *types.UserV2
		wantRole    *types.RoleV6
	}{
		{
			name: "new",
			user: botCreator.GetName(),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
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
						MaxSessionTTL: types.Duration(12 * time.Hour),
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
			name: "new with expiry",
			user: botCreator.GetName(),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
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
						MaxSessionTTL: types.Duration(12 * time.Hour),
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
			name: "already exists",
			user: botCreator.GetName(),
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
						MaxSessionTTL: types.Duration(12 * time.Hour),
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
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req:  &machineidv1pb.UpsertBotRequest{},

			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "validation - nil bot",
			user: botCreator.GetName(),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: nil,
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - nil metadata",
			user: botCreator.GetName(),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: nil,
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "must be non-nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no name",
			user: botCreator.GetName(),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{},
					Spec: &machineidv1pb.BotSpec{
						Roles: []string{testRole.GetName()},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - empty role",
			user: botCreator.GetName(),
			req: &machineidv1pb.UpsertBotRequest{
				Bot: &machineidv1pb.Bot{
					Metadata: &headerv1.Metadata{
						Name: "empty-string-role",
					},
					Spec: &machineidv1pb.BotSpec{
						Roles:  []string{"foo", "", "bar"},
						Traits: []*machineidv1pb.Trait{},
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "spec.roles: must not contain empty strings")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
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
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botGetterUser, _, err := auth.CreateUserAndRole(
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
	testRole, err := auth.CreateRole(
		ctx, srv.Auth(), "test-role", types.RoleSpecV6{},
	)
	require.NoError(t, err)
	unprivilegedUser, err := auth.CreateUser(
		ctx, srv.Auth(), "no-perms", testRole,
	)
	require.NoError(t, err)

	client, err := srv.NewClient(auth.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
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

	tests := []struct {
		name        string
		user        string
		req         *machineidv1pb.GetBotRequest
		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.Bot
	}{
		{
			name: "success",
			user: botGetterUser.GetName(),
			req: &machineidv1pb.GetBotRequest{
				BotName: preExistingBot.Metadata.Name,
			},

			assertError: require.NoError,
			want:        preExistingBot,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: &machineidv1pb.GetBotRequest{
				BotName: preExistingBot.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "validation - no bot name",
			user: botGetterUser.GetName(),
			req: &machineidv1pb.GetBotRequest{
				BotName: "",
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "bot doesnt exist",
			user: botGetterUser.GetName(),
			req: &machineidv1pb.GetBotRequest{
				BotName: "non-existent",
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
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
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botListerUser, _, err := auth.CreateUserAndRole(
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
	testRole, err := auth.CreateRole(
		ctx, srv.Auth(), "test-role", types.RoleSpecV6{},
	)
	require.NoError(t, err)
	unprivilegedUser, err := auth.CreateUser(
		ctx, srv.Auth(), "no-perms", testRole,
	)
	require.NoError(t, err)

	client, err := srv.NewClient(auth.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
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

	tests := []struct {
		name        string
		user        string
		req         *machineidv1pb.ListBotsRequest
		assertError require.ErrorAssertionFunc
		want        *machineidv1pb.ListBotsResponse
	}{
		{
			name:        "success",
			user:        botListerUser.GetName(),
			req:         &machineidv1pb.ListBotsRequest{},
			assertError: require.NoError,
			want: &machineidv1pb.ListBotsResponse{
				Bots: []*machineidv1pb.Bot{
					preExistingBot,
					preExistingBot2,
				},
			},
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req:  &machineidv1pb.ListBotsRequest{},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
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
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	botDeleterUser, _, err := auth.CreateUserAndRole(
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
	testRole, err := auth.CreateRole(
		ctx, srv.Auth(), "test-role", types.RoleSpecV6{},
	)
	require.NoError(t, err)
	unprivilegedUser, err := auth.CreateUser(
		ctx, srv.Auth(), "no-perms", testRole,
	)
	require.NoError(t, err)

	// Create a user/role with a bot-like name but that isn't a bot to ensure we
	// don't delete it
	_, err = auth.CreateUser(
		ctx, srv.Auth(), "bot-not-bot", testRole,
	)
	require.NoError(t, err)
	_, err = auth.CreateRole(
		ctx, srv.Auth(), "bot-not-bot", types.RoleSpecV6{},
	)
	require.NoError(t, err)

	client, err := srv.NewClient(auth.TestAdmin())
	require.NoError(t, err)
	preExistingBot, err := client.BotServiceClient().CreateBot(
		ctx,
		&machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
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

	tests := []struct {
		name                  string
		user                  string
		req                   *machineidv1pb.DeleteBotRequest
		assertError           require.ErrorAssertionFunc
		checkResourcesDeleted bool
	}{
		{
			name: "success",
			user: botDeleterUser.GetName(),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: preExistingBot.Metadata.Name,
			},
			assertError:           require.NoError,
			checkResourcesDeleted: true,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: preExistingBot3.Metadata.Name,
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "non existent",
			user: botDeleterUser.GetName(),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: "does-not-exist",
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
		{
			name: "non-bot role",
			user: botDeleterUser.GetName(),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: "not-bot",
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "missing bot label matching bot name")
			},
		},
		{
			name: "validation - no bot name",
			user: botDeleterUser.GetName(),
			req: &machineidv1pb.DeleteBotRequest{
				BotName: "",
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bot_name: must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be access denied")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			_, err = client.BotServiceClient().DeleteBot(ctx, tt.req)
			tt.assertError(t, err)
			if tt.checkResourcesDeleted {
				_, err := srv.Auth().GetUser(ctx, machineidv1.BotResourceName(tt.req.BotName), false)
				require.True(t, trace.IsNotFound(err), "bot user should be deleted")
				_, err = srv.Auth().GetRole(ctx, machineidv1.BotResourceName(tt.req.BotName))
				require.True(t, trace.IsNotFound(err), "bot role should be deleted")
			}
		})
	}
}

func newTestTLSServer(t testing.TB) (*auth.TestTLSServer, *eventstest.MockRecorderEmitter) {
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)

	emitter := &eventstest.MockRecorderEmitter{}
	srv, err := as.NewTestTLSServer(func(config *auth.TestTLSServerConfig) {
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
