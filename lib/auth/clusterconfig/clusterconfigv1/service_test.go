// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clusterconfigv1_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/clusterconfig/clusterconfigv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestCreateAuthPreference(t *testing.T) {
	authRoleContext, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleAuth,
		Username: string(types.RoleAuth),
	}, nil)
	require.NoError(t, err, "creating auth role context")

	cases := []struct {
		name       string
		modules    modules.Modules
		authorizer authz.Authorizer
		preference func(p types.AuthPreference)
		assertion  func(t *testing.T, created types.AuthPreference, err error)
	}{
		{
			name: "unauthorized built in role",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authz.ContextForBuiltinRole(authz.BuiltinRole{
					Role:     types.RoleProxy,
					Username: string(types.RoleProxy),
				}, nil)
			}),
			assertion: func(t *testing.T, created types.AuthPreference, err error) {
				assert.Nil(t, created)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected proxy role to be prevented from creating auth preferences", err)
			},
		},
		{
			name: "authorized built in auth",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			}),
			assertion: func(t *testing.T, created types.AuthPreference, err error) {
				require.NoError(t, err, "got (%v), expected auth role to create auth preference", err)
				require.NotNil(t, created)
			},
		},
		{
			name: "creation prevented when hardware key policy is set in open source",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			}),
			preference: func(p types.AuthPreference) {
				pp := p.(*types.AuthPreferenceV2)
				pp.Spec.RequireMFAType = types.RequireMFAType_HARDWARE_KEY_PIN
			},
			assertion: func(t *testing.T, created types.AuthPreference, err error) {
				assert.Nil(t, created)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected hardware key policy to be rejected in OSS", err)
			},
		},
		{
			name: "creation allowed when hardware key policy is set in enterprise",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			}),
			modules: &modules.TestModules{TestBuildType: modules.BuildEnterprise},
			preference: func(p types.AuthPreference) {
				pp := p.(*types.AuthPreferenceV2)
				pp.Spec.RequireMFAType = types.RequireMFAType_HARDWARE_KEY_PIN
			},
			assertion: func(t *testing.T, created types.AuthPreference, err error) {
				require.NoError(t, err, "got (%v), expected auth role to create auth preference", err)
				require.NotNil(t, created)
			},
		},
		{
			name: "creation prevented when hardware key policy is set in open source",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			}),
			preference: func(p types.AuthPreference) {
				p.SetDeviceTrust(&types.DeviceTrust{
					Mode: constants.DeviceTrustModeRequired,
				})
			},
			assertion: func(t *testing.T, created types.AuthPreference, err error) {
				assert.Nil(t, created)
				require.True(t, trace.IsBadParameter(err), "got (%v), expected device trust mode conflict to prevent creation", err)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.modules != nil {
				modules.SetTestModules(t, test.modules)
			}

			var opts []serviceOpt
			if test.authorizer != nil {
				opts = append(opts, withAuthorizer(test.authorizer))
			}

			env, err := newTestEnv(opts...)
			require.NoError(t, err, "creating test service")

			pref := types.DefaultAuthPreference()
			if test.preference != nil {
				test.preference(pref)
			}

			created, err := env.CreateAuthPreference(context.Background(), pref)
			test.assertion(t, created, err)
		})
	}
}

func TestGetAuthPreference(t *testing.T) {
	cases := []struct {
		name       string
		authorizer authz.Authorizer
		assertion  func(t *testing.T, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to be prevented from getting auth preferences", err)
			},
		}, {
			name: "authorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbRead}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultAuthPreference(types.DefaultAuthPreference()))
			require.NoError(t, err, "creating test service")

			got, err := env.GetAuthPreference(context.Background(), &clusterconfigpb.GetAuthPreferenceRequest{})
			test.assertion(t, err)
			if err == nil {
				require.Empty(t, cmp.Diff(types.DefaultAuthPreference(), got, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			}
		})
	}
}

func TestUpdateAuthPreference(t *testing.T) {
	cases := []struct {
		name       string
		preference func(p types.AuthPreference)
		authorizer authz.Authorizer
		assertion  func(t *testing.T, updated types.AuthPreference, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent updating auth preferences", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent updating auth preferences", err)
			},
		},
		{
			name: "oss hardware key policy",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			preference: func(p types.AuthPreference) {
				p.(*types.AuthPreferenceV2).Spec.RequireMFAType = types.RequireMFAType_HARDWARE_KEY_PIN
			},
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected enterprise only features to prevent updating auth preferences", err)
			},
		},
		{
			name: "invalid device trust settings",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			preference: func(p types.AuthPreference) {
				p.(*types.AuthPreferenceV2).Spec.DeviceTrust = &types.DeviceTrust{Mode: constants.DeviceTrustModeRequired}
			},
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsBadParameter(err), "got (%v), expected conflicting device trust settings to prevent updating auth preferences", err)
			},
		},
		{
			name: "updated",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			preference: func(p types.AuthPreference) {
				p.(*types.AuthPreferenceV2).Spec.LockingMode = constants.LockingModeStrict
				p.SetOrigin("test-origin")
			},
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.NoError(t, err)
				require.Equal(t, constants.LockingModeStrict, updated.GetLockingMode())
				require.Equal(t, types.OriginDynamic, updated.Origin())
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultAuthPreference(types.DefaultAuthPreference()))
			require.NoError(t, err, "creating test service")

			// Set revisions to allow the update to succeed.
			pref := env.defaultPreference
			if test.preference != nil {
				test.preference(pref)
			}

			updated, err := env.UpdateAuthPreference(context.Background(), &clusterconfigpb.UpdateAuthPreferenceRequest{AuthPreference: pref.(*types.AuthPreferenceV2)})
			test.assertion(t, updated, err)
		})
	}
}

func TestUpsertAuthPreference(t *testing.T) {
	cases := []struct {
		name       string
		preference func(p types.AuthPreference)
		authorizer authz.Authorizer
		assertion  func(t *testing.T, updated types.AuthPreference, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent upserting auth preferences", err)
			},
		},
		{
			name: "access prevented",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}),
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent upserting auth preferences", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbCreate, types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}),
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent upserting auth preferences", err)
			},
		},
		{
			name: "oss hardware key policy",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbCreate, types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			preference: func(p types.AuthPreference) {
				p.(*types.AuthPreferenceV2).Spec.RequireMFAType = types.RequireMFAType_HARDWARE_KEY_PIN
			},
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected enterprise only features to prevent upserting auth preferences", err)
			},
		},
		{
			name: "invalid device trust settings",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbCreate, types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			preference: func(p types.AuthPreference) {
				p.(*types.AuthPreferenceV2).Spec.DeviceTrust = &types.DeviceTrust{Mode: constants.DeviceTrustModeRequired}
			},
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.True(t, trace.IsBadParameter(err), "got (%v), expected conflicting device trust settings to prevent upserting auth preferences", err)
			},
		},
		{
			name: "upserted",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate, types.VerbCreate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			preference: func(p types.AuthPreference) {
				p.(*types.AuthPreferenceV2).Spec.LockingMode = constants.LockingModeStrict
				p.SetOrigin("test-origin")
			},
			assertion: func(t *testing.T, updated types.AuthPreference, err error) {
				require.NoError(t, err)
				require.Equal(t, constants.LockingModeStrict, updated.GetLockingMode())
				require.Equal(t, types.OriginDynamic, updated.Origin())
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultAuthPreference(types.DefaultAuthPreference()))
			require.NoError(t, err, "creating test service")

			// Set revisions to allow the update to succeed.
			pref := env.defaultPreference
			if test.preference != nil {
				test.preference(pref)
			}

			updated, err := env.UpsertAuthPreference(context.Background(), &clusterconfigpb.UpsertAuthPreferenceRequest{AuthPreference: pref.(*types.AuthPreferenceV2)})
			test.assertion(t, updated, err)
		})
	}
}

func TestResetAuthPreference(t *testing.T) {
	cases := []struct {
		name       string
		authorizer authz.Authorizer
		modules    modules.Modules
		preference types.AuthPreference
		assertion  func(t *testing.T, reset types.AuthPreference, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, reset types.AuthPreference, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent resetting auth preferences", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, reset types.AuthPreference, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent resetting auth preferences", err)
			},
		},
		{
			name: "config file origin prevents reset",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			preference: func() types.AuthPreference {
				p := types.DefaultAuthPreference()
				p.SetOrigin(types.OriginConfigFile)
				return p
			}(),
			assertion: func(t *testing.T, reset types.AuthPreference, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsBadParameter(err), "got (%v), expected config file origin to prevent resetting auth preferences", err)
			},
		},
		{
			name: "reset",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterAuthPreference: {types.VerbUpdate, types.VerbCreate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			assertion: func(t *testing.T, reset types.AuthPreference, err error) {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(types.DefaultAuthPreference(), reset, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			p := types.DefaultAuthPreference()
			if test.preference != nil {
				p = test.preference
			}
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultAuthPreference(p))
			require.NoError(t, err, "creating test service")

			reset, err := env.ResetAuthPreference(context.Background(), &clusterconfigpb.ResetAuthPreferenceRequest{})
			test.assertion(t, reset, err)
		})
	}
}

func TestCreateClusterNetworkingConfig(t *testing.T) {
	authRoleContext, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleAuth,
		Username: string(types.RoleAuth),
	}, nil)
	require.NoError(t, err, "creating auth role context")

	cases := []struct {
		name       string
		modules    modules.Modules
		authorizer authz.Authorizer
		config     func(p types.ClusterNetworkingConfig)
		assertion  func(t *testing.T, created types.ClusterNetworkingConfig, err error)
	}{
		{
			name: "unauthorized built in role",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authz.ContextForBuiltinRole(authz.BuiltinRole{
					Role:     types.RoleProxy,
					Username: string(types.RoleProxy),
				}, nil)
			}),
			assertion: func(t *testing.T, created types.ClusterNetworkingConfig, err error) {
				assert.Nil(t, created)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected proxy role to be prevented from creating networking config", err)
			},
		},
		{
			name: "authorized built in auth",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			}),
			assertion: func(t *testing.T, created types.ClusterNetworkingConfig, err error) {
				require.NoError(t, err, "got (%v), expected auth role to create networking config", err)
				require.NotNil(t, created)
			},
		},
		{
			name: "creation prevented when proxy peering is set in open source",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			}),
			config: func(p types.ClusterNetworkingConfig) {
				p.SetTunnelStrategy(&types.TunnelStrategyV1{
					Strategy: &types.TunnelStrategyV1_ProxyPeering{
						ProxyPeering: types.DefaultProxyPeeringTunnelStrategy(),
					},
				})
			},
			assertion: func(t *testing.T, created types.ClusterNetworkingConfig, err error) {
				assert.Nil(t, created)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected proxy peering to be rejected in OSS", err)
			},
		},
		{
			name: "creation allowed when proxy peering is set in enterprise",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			}),
			modules: &modules.TestModules{TestBuildType: modules.BuildEnterprise},
			config: func(p types.ClusterNetworkingConfig) {
				p.SetTunnelStrategy(&types.TunnelStrategyV1{
					Strategy: &types.TunnelStrategyV1_ProxyPeering{
						ProxyPeering: types.DefaultProxyPeeringTunnelStrategy(),
					},
				})
			},
			assertion: func(t *testing.T, created types.ClusterNetworkingConfig, err error) {
				require.NoError(t, err, "got (%v), expected auth role to create networking config", err)
				require.NotNil(t, created)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.modules != nil {
				modules.SetTestModules(t, test.modules)
			}

			var opts []serviceOpt
			if test.authorizer != nil {
				opts = append(opts, withAuthorizer(test.authorizer))
			}

			env, err := newTestEnv(opts...)
			require.NoError(t, err, "creating test service")

			cfg := types.DefaultClusterNetworkingConfig()
			if test.config != nil {
				test.config(cfg)
			}

			created, err := env.CreateClusterNetworkingConfig(context.Background(), cfg)
			test.assertion(t, created, err)
		})
	}
}

func TestGetClusterNetworkingConfig(t *testing.T) {
	cases := []struct {
		name       string
		authorizer authz.Authorizer
		assertion  func(t *testing.T, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to be prevented from getting auth preferences", err)
			},
		}, {
			name: "authorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbRead}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultClusterNetworkingConfig(types.DefaultClusterNetworkingConfig()))
			require.NoError(t, err, "creating test service")

			got, err := env.GetClusterNetworkingConfig(context.Background(), &clusterconfigpb.GetClusterNetworkingConfigRequest{})
			test.assertion(t, err)
			if err == nil {
				require.Empty(t, cmp.Diff(types.DefaultClusterNetworkingConfig(), got, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			}
		})
	}
}

func TestUpdateClusterNetworkingConfig(t *testing.T) {
	cases := []struct {
		name       string
		config     func(p types.ClusterNetworkingConfig)
		authorizer authz.Authorizer
		assertion  func(t *testing.T, updated types.ClusterNetworkingConfig, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent updating networking config", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbUpdate}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent updating networking config", err)
			},
		},
		{
			name: "oss proxy peering",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			config: func(p types.ClusterNetworkingConfig) {
				p.SetTunnelStrategy(&types.TunnelStrategyV1{
					Strategy: &types.TunnelStrategyV1_ProxyPeering{
						ProxyPeering: types.DefaultProxyPeeringTunnelStrategy(),
					},
				})
			},
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected enterprise only features to prevent updating networking config", err)
			},
		},
		{
			name: "updated",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			config: func(p types.ClusterNetworkingConfig) {
				p.SetRoutingStrategy(types.RoutingStrategy_MOST_RECENT)
				p.SetOrigin("test-origin")
			},
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.NoError(t, err)
				require.Equal(t, types.RoutingStrategy_MOST_RECENT, updated.GetRoutingStrategy())
				require.Equal(t, types.OriginDynamic, updated.Origin())
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultClusterNetworkingConfig(types.DefaultClusterNetworkingConfig()))
			require.NoError(t, err, "creating test service")

			// Set revisions to allow the update to succeed.
			cfg := env.defaultNetworkingConfig
			if test.config != nil {
				test.config(cfg)
			}

			updated, err := env.UpdateClusterNetworkingConfig(context.Background(), &clusterconfigpb.UpdateClusterNetworkingConfigRequest{ClusterNetworkConfig: cfg.(*types.ClusterNetworkingConfigV2)})
			test.assertion(t, updated, err)
		})
	}
}

func TestUpsertClusterNetworkingConfig(t *testing.T) {
	cases := []struct {
		name       string
		config     func(p types.ClusterNetworkingConfig)
		authorizer authz.Authorizer
		assertion  func(t *testing.T, updated types.ClusterNetworkingConfig, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent upserting network config", err)
			},
		},
		{
			name: "access prevented",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}),
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent upserting network config", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbCreate, types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}),
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent upserting network config", err)
			},
		},
		{
			name: "oss proxy peering",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbCreate, types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			config: func(p types.ClusterNetworkingConfig) {
				p.SetTunnelStrategy(&types.TunnelStrategyV1{
					Strategy: &types.TunnelStrategyV1_ProxyPeering{
						ProxyPeering: types.DefaultProxyPeeringTunnelStrategy(),
					},
				})
			},
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected enterprise only features to prevent upserting network config", err)
			},
		},
		{
			name: "upserted",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbUpdate, types.VerbCreate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			config: func(p types.ClusterNetworkingConfig) {
				p.SetRoutingStrategy(types.RoutingStrategy_MOST_RECENT)
				p.SetOrigin("test-origin")
			},
			assertion: func(t *testing.T, updated types.ClusterNetworkingConfig, err error) {
				require.NoError(t, err)
				require.Equal(t, types.RoutingStrategy_MOST_RECENT, updated.GetRoutingStrategy())
				require.Equal(t, types.OriginDynamic, updated.Origin())
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultClusterNetworkingConfig(types.DefaultClusterNetworkingConfig()))
			require.NoError(t, err, "creating test service")

			// Set revisions to allow the update to succeed.
			cfg := env.defaultNetworkingConfig
			if test.config != nil {
				test.config(cfg)
			}

			updated, err := env.UpsertClusterNetworkingConfig(context.Background(), &clusterconfigpb.UpsertClusterNetworkingConfigRequest{ClusterNetworkConfig: cfg.(*types.ClusterNetworkingConfigV2)})
			test.assertion(t, updated, err)
		})
	}
}

func TestResetClusterNetworkingConfig(t *testing.T) {
	cases := []struct {
		name       string
		authorizer authz.Authorizer
		modules    modules.Modules
		config     types.ClusterNetworkingConfig
		assertion  func(t *testing.T, reset types.ClusterNetworkingConfig, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, reset types.ClusterNetworkingConfig, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent resetting network config", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbUpdate}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, reset types.ClusterNetworkingConfig, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent resetting network config", err)
			},
		},
		{
			name: "config file origin prevents reset",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			config: func() types.ClusterNetworkingConfig {
				cfg := types.DefaultClusterNetworkingConfig()
				cfg.SetOrigin(types.OriginConfigFile)
				return cfg
			}(),
			assertion: func(t *testing.T, reset types.ClusterNetworkingConfig, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsBadParameter(err), "got (%v), expected config file origin to prevent resetting network config", err)
			},
		},
		{
			name: "reset",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindClusterNetworkingConfig: {types.VerbUpdate, types.VerbCreate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			assertion: func(t *testing.T, reset types.ClusterNetworkingConfig, err error) {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(types.DefaultClusterNetworkingConfig(), reset, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cfg := types.DefaultClusterNetworkingConfig()
			if test.config != nil {
				cfg = test.config
			}
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultClusterNetworkingConfig(cfg))
			require.NoError(t, err, "creating test service")

			reset, err := env.ResetClusterNetworkingConfig(context.Background(), &clusterconfigpb.ResetClusterNetworkingConfigRequest{})
			test.assertion(t, reset, err)
		})
	}
}

func TestCreateSessionRecordingConfig(t *testing.T) {
	authRoleContext, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleAuth,
		Username: string(types.RoleAuth),
	}, nil)
	require.NoError(t, err, "creating auth role context")

	cases := []struct {
		name       string
		modules    modules.Modules
		authorizer authz.Authorizer
		assertion  func(t *testing.T, created types.SessionRecordingConfig, err error)
	}{
		{
			name: "unauthorized built in role",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authz.ContextForBuiltinRole(authz.BuiltinRole{
					Role:     types.RoleProxy,
					Username: string(types.RoleProxy),
				}, nil)
			}),
			assertion: func(t *testing.T, created types.SessionRecordingConfig, err error) {
				assert.Nil(t, created)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected proxy role to be prevented from creating recording config", err)
			},
		},
		{
			name: "authorized built in auth",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			}),
			assertion: func(t *testing.T, created types.SessionRecordingConfig, err error) {
				require.NoError(t, err, "got (%v), expected auth role to create recording config", err)
				require.NotNil(t, created)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.modules != nil {
				modules.SetTestModules(t, test.modules)
			}

			var opts []serviceOpt
			if test.authorizer != nil {
				opts = append(opts, withAuthorizer(test.authorizer))
			}

			env, err := newTestEnv(opts...)
			require.NoError(t, err, "creating test service")

			created, err := env.CreateSessionRecordingConfig(context.Background(), types.DefaultSessionRecordingConfig())
			test.assertion(t, created, err)
		})
	}
}

func TestGetSessionRecordingConfig(t *testing.T) {
	cases := []struct {
		name       string
		authorizer authz.Authorizer
		assertion  func(t *testing.T, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to be prevented from getting recording config", err)
			},
		}, {
			name: "authorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbRead}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultRecordingConfig(types.DefaultSessionRecordingConfig()))
			require.NoError(t, err, "creating test service")

			got, err := env.GetSessionRecordingConfig(context.Background(), &clusterconfigpb.GetSessionRecordingConfigRequest{})
			test.assertion(t, err)
			if err == nil {
				require.Empty(t, cmp.Diff(types.DefaultSessionRecordingConfig(), got, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			}
		})
	}
}

func TestUpdateSessionRecordingConfig(t *testing.T) {
	cases := []struct {
		name       string
		config     func(p types.SessionRecordingConfig)
		authorizer authz.Authorizer
		assertion  func(t *testing.T, updated types.SessionRecordingConfig, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.SessionRecordingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent updating recording config", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbUpdate}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.SessionRecordingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent updating recording config", err)
			},
		},
		{
			name: "updated",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			config: func(p types.SessionRecordingConfig) {
				p.SetProxyChecksHostKeys(false)
				p.SetOrigin("test-origin")
			},
			assertion: func(t *testing.T, updated types.SessionRecordingConfig, err error) {
				require.NoError(t, err)
				require.False(t, updated.GetProxyChecksHostKeys())
				require.Equal(t, types.OriginDynamic, updated.Origin())
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultRecordingConfig(types.DefaultSessionRecordingConfig()))
			require.NoError(t, err, "creating test service")

			// Set revisions to allow the update to succeed.
			cfg := env.defaultRecordingConfig
			if test.config != nil {
				test.config(cfg)
			}

			updated, err := env.UpdateSessionRecordingConfig(context.Background(), &clusterconfigpb.UpdateSessionRecordingConfigRequest{SessionRecordingConfig: cfg.(*types.SessionRecordingConfigV2)})
			test.assertion(t, updated, err)
		})
	}
}

func TestUpsertSessionRecordingConfig(t *testing.T) {
	cases := []struct {
		name       string
		config     func(p types.SessionRecordingConfig)
		authorizer authz.Authorizer
		assertion  func(t *testing.T, updated types.SessionRecordingConfig, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, updated types.SessionRecordingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent upserting recording config", err)
			},
		},
		{
			name: "access prevented",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}),
			assertion: func(t *testing.T, updated types.SessionRecordingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent upserting recording config", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbCreate, types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}),
			assertion: func(t *testing.T, updated types.SessionRecordingConfig, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent upserting recording config", err)
			},
		},
		{
			name: "upserted",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbUpdate, types.VerbCreate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			config: func(p types.SessionRecordingConfig) {
				p.SetProxyChecksHostKeys(false)
				p.SetOrigin("test-origin")
			},
			assertion: func(t *testing.T, updated types.SessionRecordingConfig, err error) {
				require.NoError(t, err)
				require.False(t, updated.GetProxyChecksHostKeys())
				require.Equal(t, types.OriginDynamic, updated.Origin())
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultRecordingConfig(types.DefaultSessionRecordingConfig()))
			require.NoError(t, err, "creating test service")

			// Set revisions to allow the update to succeed.
			cfg := env.defaultRecordingConfig
			if test.config != nil {
				test.config(cfg)
			}

			updated, err := env.UpsertSessionRecordingConfig(context.Background(), &clusterconfigpb.UpsertSessionRecordingConfigRequest{SessionRecordingConfig: cfg.(*types.SessionRecordingConfigV2)})
			test.assertion(t, updated, err)
		})
	}
}

func TestResetSessionRecordingConfig(t *testing.T) {
	cases := []struct {
		name       string
		authorizer authz.Authorizer
		modules    modules.Modules
		config     types.SessionRecordingConfig
		assertion  func(t *testing.T, reset types.SessionRecordingConfig, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, reset types.SessionRecordingConfig, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent resetting recording config", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbUpdate}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, reset types.SessionRecordingConfig, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent resetting recording config", err)
			},
		},
		{
			name: "config file origin prevents reset",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			config: func() types.SessionRecordingConfig {
				cfg := types.DefaultSessionRecordingConfig()
				cfg.SetOrigin(types.OriginConfigFile)
				return cfg
			}(),
			assertion: func(t *testing.T, reset types.SessionRecordingConfig, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsBadParameter(err), "got (%v), expected config file origin to prevent resetting recording config", err)
			},
		},
		{
			name: "reset",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindSessionRecordingConfig: {types.VerbUpdate, types.VerbCreate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			assertion: func(t *testing.T, reset types.SessionRecordingConfig, err error) {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(types.DefaultSessionRecordingConfig(), reset, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cfg := types.DefaultSessionRecordingConfig()
			if test.config != nil {
				cfg = test.config
			}
			env, err := newTestEnv(withAuthorizer(test.authorizer), withDefaultRecordingConfig(cfg))
			require.NoError(t, err, "creating test service")

			reset, err := env.ResetSessionRecordingConfig(context.Background(), &clusterconfigpb.ResetSessionRecordingConfigRequest{})
			test.assertion(t, reset, err)
		})
	}
}

type fakeChecker struct {
	services.AccessChecker
	rules map[string][]string
}

func (f fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, kind string, verb string) error {
	verbs, ok := f.rules[kind]
	if !ok {
		return trace.AccessDenied("no allow rules for kind")
	}

	if !slices.Contains(verbs, verb) {
		return trace.AccessDenied("verb %s not allowed", verb)
	}

	return nil
}

type envConfig struct {
	authorizer              authz.Authorizer
	defaultAuthPreference   types.AuthPreference
	defaultNetworkingConfig types.ClusterNetworkingConfig
	defaultRecordingConfig  types.SessionRecordingConfig
}
type serviceOpt = func(config *envConfig)

func withAuthorizer(authz authz.Authorizer) serviceOpt {
	return func(config *envConfig) {
		config.authorizer = authz
	}
}

func withDefaultAuthPreference(p types.AuthPreference) serviceOpt {
	return func(config *envConfig) {
		config.defaultAuthPreference = p
	}
}

func withDefaultClusterNetworkingConfig(c types.ClusterNetworkingConfig) serviceOpt {
	return func(config *envConfig) {
		config.defaultNetworkingConfig = c
	}
}

func withDefaultRecordingConfig(c types.SessionRecordingConfig) serviceOpt {
	return func(config *envConfig) {
		config.defaultRecordingConfig = c
	}
}

type env struct {
	*clusterconfigv1.Service
	backend                 clusterconfigv1.Backend
	defaultPreference       types.AuthPreference
	defaultNetworkingConfig types.ClusterNetworkingConfig
	defaultRecordingConfig  types.SessionRecordingConfig
}

func newTestEnv(opts ...serviceOpt) (*env, error) {
	bk, err := memory.New(memory.Config{})
	if err != nil {
		return nil, trace.Wrap(err, "creating memory backend")
	}

	storage, err := local.NewClusterConfigurationService(bk)
	if err != nil {
		return nil, trace.Wrap(err, "created cluster configuration storage service")
	}

	var cfg envConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	service := struct{ services.ClusterConfiguration }{ClusterConfiguration: storage}
	svc, err := clusterconfigv1.NewService(clusterconfigv1.ServiceConfig{
		Cache:      service,
		Backend:    service,
		Authorizer: cfg.authorizer,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating users service")
	}

	ctx := context.Background()
	var defaultPreference types.AuthPreference
	if cfg.defaultAuthPreference != nil {
		defaultPreference, err = service.CreateAuthPreference(ctx, cfg.defaultAuthPreference)
		if err != nil {
			return nil, trace.Wrap(err, "creating default auth preference")
		}
	}

	var defaultNetworkingConfig types.ClusterNetworkingConfig
	if cfg.defaultNetworkingConfig != nil {
		defaultNetworkingConfig, err = service.CreateClusterNetworkingConfig(ctx, cfg.defaultNetworkingConfig)
		if err != nil {
			return nil, trace.Wrap(err, "creating default networking config")
		}
	}

	var defaultSessionRecordingConfig types.SessionRecordingConfig
	if cfg.defaultRecordingConfig != nil {
		defaultSessionRecordingConfig, err = service.CreateSessionRecordingConfig(ctx, cfg.defaultRecordingConfig)
		if err != nil {
			return nil, trace.Wrap(err, "creating session recording config")
		}
	}

	return &env{
		Service:                 svc,
		backend:                 service,
		defaultPreference:       defaultPreference,
		defaultNetworkingConfig: defaultNetworkingConfig,
		defaultRecordingConfig:  defaultSessionRecordingConfig,
	}, nil
}
