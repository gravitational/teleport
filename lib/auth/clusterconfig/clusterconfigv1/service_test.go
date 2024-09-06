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
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/constants"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/clusterconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/clusterconfig/clusterconfigv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
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
				require.NoError(t, err, "got (%v), expected auth role to create auth mutator", err)
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
				require.NoError(t, err, "got (%v), expected auth role to create auth mutator", err)
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
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

type failingConfigService struct {
	services.ClusterConfiguration
}

func (failingConfigService) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}

func (failingConfigService) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	return types.DefaultClusterNetworkingConfig(), nil
}

func (failingConfigService) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	return types.DefaultSessionRecordingConfig(), nil
}

func (failingConfigService) CreateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error) {
	return nil, errors.New("fail")
}
func (failingConfigService) UpdateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error) {
	return nil, errors.New("fail")
}
func (failingConfigService) UpsertAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error) {
	return nil, errors.New("fail")
}

func (failingConfigService) CreateClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	return nil, errors.New("fail")
}
func (failingConfigService) UpdateClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	return nil, errors.New("fail")
}
func (failingConfigService) UpsertClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	return nil, errors.New("fail")
}

func (failingConfigService) CreateSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	return nil, errors.New("fail")
}
func (failingConfigService) UpdateSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	return nil, errors.New("fail")
}
func (failingConfigService) UpsertSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	return nil, errors.New("fail")
}

func TestAuditEventsEmitted(t *testing.T) {
	ctx := context.Background()

	t.Run("successful events", func(t *testing.T) {
		env, err := newTestEnv(
			withAuthorizer(authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{
							types.KindSessionRecordingConfig:  {types.VerbUpdate, types.VerbCreate, types.VerbRead},
							types.KindClusterAuthPreference:   {types.VerbUpdate, types.VerbCreate, types.VerbRead},
							types.KindClusterNetworkingConfig: {types.VerbUpdate, types.VerbCreate, types.VerbRead},
						},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
				}, nil
			})),
			withDefaultRecordingConfig(types.DefaultSessionRecordingConfig()),
			withDefaultAuthPreference(types.DefaultAuthPreference()),
			withDefaultClusterNetworkingConfig(types.DefaultClusterNetworkingConfig()),
		)
		require.NoError(t, err, "creating test service")

		t.Run("auth preference", func(t *testing.T) {
			mfaUnchangedEvent := apievents.AuthPreferenceUpdate{
				Metadata: apievents.Metadata{
					Type: events.AuthPreferenceUpdateEvent,
					Code: events.AuthPreferenceUpdateCode,
				},
				Status: apievents.Status{
					Success: true,
				},
				UserMetadata: apievents.UserMetadata{
					User:     "llama",
					UserKind: apievents.UserKind_USER_KIND_HUMAN,
				},
				AdminActionsMFA: apievents.AdminActionsMFAStatus_ADMIN_ACTIONS_MFA_STATUS_UNCHANGED,
			}

			mfaEnabledEvent := mfaUnchangedEvent
			mfaEnabledEvent.AdminActionsMFA = apievents.AdminActionsMFAStatus_ADMIN_ACTIONS_MFA_STATUS_ENABLED

			mfaDisabledEvent := mfaUnchangedEvent
			mfaDisabledEvent.AdminActionsMFA = apievents.AdminActionsMFAStatus_ADMIN_ACTIONS_MFA_STATUS_DISABLED

			p, err := env.ResetAuthPreference(ctx, &clusterconfigpb.ResetAuthPreferenceRequest{})
			require.NoError(t, err)

			evt := <-env.emitter.C()
			require.Empty(t, cmp.Diff(&mfaUnchangedEvent, evt))

			p.SetLockingMode(constants.LockingModeStrict)

			p, err = env.UpdateAuthPreference(ctx, &clusterconfigpb.UpdateAuthPreferenceRequest{AuthPreference: p})
			require.NoError(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(&mfaUnchangedEvent, evt))

			_, err = env.UpsertAuthPreference(ctx, &clusterconfigpb.UpsertAuthPreferenceRequest{AuthPreference: p})
			require.NoError(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(&mfaUnchangedEvent, evt))

			p.Spec.SecondFactor = constants.SecondFactorWebauthn
			p.Spec.Webauthn = &types.Webauthn{
				RPID: "example.com",
			}

			p, err = env.UpdateAuthPreference(ctx, &clusterconfigpb.UpdateAuthPreferenceRequest{AuthPreference: p})
			require.NoError(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(&mfaEnabledEvent, evt))

			p.Spec.SecondFactor = constants.SecondFactorOTP

			_, err = env.UpsertAuthPreference(ctx, &clusterconfigpb.UpsertAuthPreferenceRequest{AuthPreference: p})
			require.NoError(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(&mfaDisabledEvent, evt))
		})

		t.Run("cluster networking config", func(t *testing.T) {
			expectedEvent := &apievents.ClusterNetworkingConfigUpdate{
				Metadata: apievents.Metadata{
					Type: events.ClusterNetworkingConfigUpdateEvent,
					Code: events.ClusterNetworkingConfigUpdateCode,
				},
				Status: apievents.Status{
					Success: true,
				},
				UserMetadata: apievents.UserMetadata{
					User:     "llama",
					UserKind: apievents.UserKind_USER_KIND_HUMAN,
				},
			}

			cfg, err := env.ResetClusterNetworkingConfig(ctx, &clusterconfigpb.ResetClusterNetworkingConfigRequest{})
			require.NoError(t, err)

			evt := <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			cfg.SetRoutingStrategy(types.RoutingStrategy_MOST_RECENT)

			cfg, err = env.UpdateClusterNetworkingConfig(ctx, &clusterconfigpb.UpdateClusterNetworkingConfigRequest{ClusterNetworkConfig: cfg})
			require.NoError(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			_, err = env.UpsertClusterNetworkingConfig(ctx, &clusterconfigpb.UpsertClusterNetworkingConfigRequest{ClusterNetworkConfig: cfg})
			require.NoError(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))
		})

		t.Run("session recording config", func(t *testing.T) {
			expectedEvent := &apievents.SessionRecordingConfigUpdate{
				Metadata: apievents.Metadata{
					Type: events.SessionRecordingConfigUpdateEvent,
					Code: events.SessionRecordingConfigUpdateCode,
				},
				Status: apievents.Status{
					Success: true,
				},
				UserMetadata: apievents.UserMetadata{
					User:     "llama",
					UserKind: apievents.UserKind_USER_KIND_HUMAN,
				},
			}

			cfg, err := env.ResetSessionRecordingConfig(ctx, &clusterconfigpb.ResetSessionRecordingConfigRequest{})
			require.NoError(t, err)

			evt := <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			cfg.SetMode(types.RecordAtProxy)

			cfg, err = env.UpdateSessionRecordingConfig(ctx, &clusterconfigpb.UpdateSessionRecordingConfigRequest{SessionRecordingConfig: cfg})
			require.NoError(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			_, err = env.UpsertSessionRecordingConfig(ctx, &clusterconfigpb.UpsertSessionRecordingConfigRequest{SessionRecordingConfig: cfg})
			require.NoError(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))
		})
	})

	t.Run("failed events", func(t *testing.T) {
		env, err := newTestEnv(
			withClusterConfigurationService(failingConfigService{}),
			withAuthorizer(authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{
							types.KindSessionRecordingConfig:  {types.VerbUpdate, types.VerbCreate, types.VerbRead},
							types.KindClusterAuthPreference:   {types.VerbUpdate, types.VerbCreate, types.VerbRead},
							types.KindClusterNetworkingConfig: {types.VerbUpdate, types.VerbCreate, types.VerbRead},
						},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
				}, nil
			})),
		)
		require.NoError(t, err, "creating test service")

		t.Run("auth preference", func(t *testing.T) {
			expectedEvent := &apievents.AuthPreferenceUpdate{
				Metadata: apievents.Metadata{
					Type: events.AuthPreferenceUpdateEvent,
					Code: events.AuthPreferenceUpdateCode,
				},
				Status: apievents.Status{
					Success:     false,
					Error:       "fail",
					UserMessage: "fail",
				},
				UserMetadata: apievents.UserMetadata{
					User:     "llama",
					UserKind: apievents.UserKind_USER_KIND_HUMAN,
				},
				AdminActionsMFA: apievents.AdminActionsMFAStatus_ADMIN_ACTIONS_MFA_STATUS_UNCHANGED,
			}

			_, err := env.ResetAuthPreference(ctx, &clusterconfigpb.ResetAuthPreferenceRequest{})
			require.Error(t, err)

			evt := <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			_, err = env.UpdateAuthPreference(ctx, &clusterconfigpb.UpdateAuthPreferenceRequest{AuthPreference: types.DefaultAuthPreference().(*types.AuthPreferenceV2)})
			require.Error(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			_, err = env.UpsertAuthPreference(ctx, &clusterconfigpb.UpsertAuthPreferenceRequest{AuthPreference: types.DefaultAuthPreference().(*types.AuthPreferenceV2)})
			require.Error(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))
		})

		t.Run("cluster networking config", func(t *testing.T) {
			expectedEvent := &apievents.ClusterNetworkingConfigUpdate{
				Metadata: apievents.Metadata{
					Type: events.ClusterNetworkingConfigUpdateEvent,
					Code: events.ClusterNetworkingConfigUpdateCode,
				},
				Status: apievents.Status{
					Success:     false,
					Error:       "fail",
					UserMessage: "fail",
				},
				UserMetadata: apievents.UserMetadata{
					User:     "llama",
					UserKind: apievents.UserKind_USER_KIND_HUMAN,
				},
			}

			_, err := env.ResetClusterNetworkingConfig(ctx, &clusterconfigpb.ResetClusterNetworkingConfigRequest{})
			require.Error(t, err)

			evt := <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			_, err = env.UpdateClusterNetworkingConfig(ctx, &clusterconfigpb.UpdateClusterNetworkingConfigRequest{ClusterNetworkConfig: types.DefaultClusterNetworkingConfig().(*types.ClusterNetworkingConfigV2)})
			require.Error(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			_, err = env.UpsertClusterNetworkingConfig(ctx, &clusterconfigpb.UpsertClusterNetworkingConfigRequest{ClusterNetworkConfig: types.DefaultClusterNetworkingConfig().(*types.ClusterNetworkingConfigV2)})
			require.Error(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))
		})

		t.Run("session recording config", func(t *testing.T) {
			expectedEvent := &apievents.SessionRecordingConfigUpdate{
				Metadata: apievents.Metadata{
					Type: events.SessionRecordingConfigUpdateEvent,
					Code: events.SessionRecordingConfigUpdateCode,
				},
				Status: apievents.Status{
					Success:     false,
					Error:       "fail",
					UserMessage: "fail",
				},
				UserMetadata: apievents.UserMetadata{
					User:     "llama",
					UserKind: apievents.UserKind_USER_KIND_HUMAN,
				},
			}

			_, err := env.ResetSessionRecordingConfig(ctx, &clusterconfigpb.ResetSessionRecordingConfigRequest{})
			require.Error(t, err)

			evt := <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			_, err = env.UpdateSessionRecordingConfig(ctx, &clusterconfigpb.UpdateSessionRecordingConfigRequest{SessionRecordingConfig: types.DefaultSessionRecordingConfig().(*types.SessionRecordingConfigV2)})
			require.Error(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))

			_, err = env.UpsertSessionRecordingConfig(ctx, &clusterconfigpb.UpsertSessionRecordingConfigRequest{SessionRecordingConfig: types.DefaultSessionRecordingConfig().(*types.SessionRecordingConfigV2)})
			require.Error(t, err)

			evt = <-env.emitter.C()
			require.Empty(t, cmp.Diff(expectedEvent, evt))
		})
	})
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
	authorizer                 authz.Authorizer
	emitter                    apievents.Emitter
	defaultAuthPreference      types.AuthPreference
	defaultNetworkingConfig    types.ClusterNetworkingConfig
	defaultRecordingConfig     types.SessionRecordingConfig
	service                    services.ClusterConfiguration
	accessGraphConfig          clusterconfigv1.AccessGraphConfig
	defaultAccessGraphSettings *clusterconfigpb.AccessGraphSettings
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

func withClusterConfigurationService(svc services.ClusterConfiguration) serviceOpt {
	return func(config *envConfig) {
		config.service = svc
	}
}

func withAccessGraphConfig(cfg clusterconfigv1.AccessGraphConfig) serviceOpt {
	return func(config *envConfig) {
		config.accessGraphConfig = cfg
	}
}

func withAccessGraphSettings(cfg *clusterconfigpb.AccessGraphSettings) serviceOpt {
	return func(config *envConfig) {
		config.defaultAccessGraphSettings = cfg
	}
}

type env struct {
	*clusterconfigv1.Service
	emitter                    *eventstest.ChannelEmitter
	defaultPreference          types.AuthPreference
	defaultNetworkingConfig    types.ClusterNetworkingConfig
	defaultRecordingConfig     types.SessionRecordingConfig
	defaultAccessGraphSettings *clusterconfigpb.AccessGraphSettings
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

	emitter := eventstest.NewChannelEmitter(10)
	cfg := envConfig{
		emitter: emitter,
		service: struct{ services.ClusterConfiguration }{ClusterConfiguration: storage},
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	svc, err := clusterconfigv1.NewService(clusterconfigv1.ServiceConfig{
		Cache:       cfg.service,
		Backend:     cfg.service,
		Authorizer:  cfg.authorizer,
		Emitter:     cfg.emitter,
		AccessGraph: cfg.accessGraphConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating users service")
	}

	ctx := context.Background()
	var defaultPreference types.AuthPreference
	if cfg.defaultAuthPreference != nil {
		defaultPreference, err = cfg.service.CreateAuthPreference(ctx, cfg.defaultAuthPreference)
		if err != nil {
			return nil, trace.Wrap(err, "creating default auth mutator")
		}
	}

	var defaultNetworkingConfig types.ClusterNetworkingConfig
	if cfg.defaultNetworkingConfig != nil {
		defaultNetworkingConfig, err = cfg.service.CreateClusterNetworkingConfig(ctx, cfg.defaultNetworkingConfig)
		if err != nil {
			return nil, trace.Wrap(err, "creating default networking config")
		}
	}

	var defaultSessionRecordingConfig types.SessionRecordingConfig
	if cfg.defaultRecordingConfig != nil {
		defaultSessionRecordingConfig, err = cfg.service.CreateSessionRecordingConfig(ctx, cfg.defaultRecordingConfig)
		if err != nil {
			return nil, trace.Wrap(err, "creating session recording config")
		}
	}

	var defaultAccessGraphSettings *clusterconfigpb.AccessGraphSettings
	if cfg.defaultAccessGraphSettings != nil {
		defaultAccessGraphSettings, err = cfg.service.CreateAccessGraphSettings(ctx, cfg.defaultAccessGraphSettings)
		if err != nil {
			return nil, trace.Wrap(err, "creating access graph settings")
		}
	}

	return &env{
		Service:                    svc,
		defaultPreference:          defaultPreference,
		defaultNetworkingConfig:    defaultNetworkingConfig,
		defaultRecordingConfig:     defaultSessionRecordingConfig,
		defaultAccessGraphSettings: defaultAccessGraphSettings,
		emitter:                    emitter,
	}, nil
}

func TestGetAccessGraphConfig(t *testing.T) {

	settings, err := clusterconfig.NewAccessGraphSettings(
		&clusterconfigpb.AccessGraphSettingsSpec{
			SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
		},
	)
	require.NoError(t, err)

	cfgEnabled := clusterconfigv1.AccessGraphConfig{
		Enabled:  true,
		Address:  "address",
		CA:       []byte("ca"),
		Insecure: true,
	}
	cases := []struct {
		name                string
		accessGraphConfig   clusterconfigv1.AccessGraphConfig
		role                types.SystemRole
		testSetup           func(*testing.T)
		errorAssertion      require.ErrorAssertionFunc
		responseAssertion   *clusterconfigpb.GetClusterAccessGraphConfigResponse
		accessGraphSettings *clusterconfigpb.AccessGraphSettings
	}{
		{
			name:              "authorized proxy with non empty access graph config; Policy module is disabled",
			role:              types.RoleProxy,
			testSetup:         func(t *testing.T) {},
			accessGraphConfig: cfgEnabled,
			errorAssertion:    require.NoError,
			responseAssertion: &clusterconfigpb.GetClusterAccessGraphConfigResponse{
				AccessGraph: &clusterconfigpb.AccessGraphConfig{
					Enabled: false,
				},
			},
		},
		{
			name: "authorized proxy with non empty access graph config; Policy module is enabled",
			role: types.RoleProxy,
			testSetup: func(t *testing.T) {
				m := modules.TestModules{
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Policy: {Enabled: true},
						},
					},
				}
				modules.SetTestModules(t, &m)
			},
			accessGraphConfig: cfgEnabled,
			errorAssertion:    require.NoError,
			responseAssertion: &clusterconfigpb.GetClusterAccessGraphConfigResponse{
				AccessGraph: &clusterconfigpb.AccessGraphConfig{
					Enabled:           true,
					Insecure:          true,
					Address:           "address",
					Ca:                []byte("ca"),
					SecretsScanConfig: &clusterconfigpb.AccessGraphSecretsScanConfiguration{},
				},
			},
		},
		{
			name: "authorized discovery with non empty access graph config; Policy module is enabled",
			role: types.RoleDiscovery,
			testSetup: func(t *testing.T) {
				m := modules.TestModules{
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Policy: {Enabled: true},
						},
					},
				}
				modules.SetTestModules(t, &m)
			},
			accessGraphConfig: cfgEnabled,
			errorAssertion:    require.NoError,
			responseAssertion: &clusterconfigpb.GetClusterAccessGraphConfigResponse{
				AccessGraph: &clusterconfigpb.AccessGraphConfig{
					Enabled:           true,
					Insecure:          true,
					Address:           "address",
					Ca:                []byte("ca"),
					SecretsScanConfig: &clusterconfigpb.AccessGraphSecretsScanConfiguration{},
				},
			},
		},
		{
			name: "Policy module is enabled with secrets scan option",
			role: types.RoleDiscovery,
			testSetup: func(t *testing.T) {
				m := modules.TestModules{
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Policy: {Enabled: true},
						},
					},
				}
				modules.SetTestModules(t, &m)
			},
			accessGraphConfig:   cfgEnabled,
			accessGraphSettings: settings,
			errorAssertion:      require.NoError,
			responseAssertion: &clusterconfigpb.GetClusterAccessGraphConfigResponse{
				AccessGraph: &clusterconfigpb.AccessGraphConfig{
					Enabled:  true,
					Insecure: true,
					Address:  "address",
					Ca:       []byte("ca"),
					SecretsScanConfig: &clusterconfigpb.AccessGraphSecretsScanConfiguration{
						SshScanEnabled: true,
					},
				},
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			test.testSetup(t)

			authRoleContext, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
				Role:     test.role,
				Username: string(test.role),
			}, nil)
			require.NoError(t, err, "creating auth role context")
			authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return authRoleContext, nil
			})

			env, err := newTestEnv(withAuthorizer(authorizer), withAccessGraphConfig(test.accessGraphConfig), withAccessGraphSettings(test.accessGraphSettings))
			require.NoError(t, err, "creating test service")

			got, err := env.GetClusterAccessGraphConfig(context.Background(), &clusterconfigpb.GetClusterAccessGraphConfigRequest{})
			test.errorAssertion(t, err)

			require.Empty(t, cmp.Diff(test.responseAssertion, got, protocmp.Transform()))
		})
	}
}

func TestGetAccessGraphSettings(t *testing.T) {
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
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to be prevented from getting access graph settings", err)
			},
		}, {
			name: "authorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbRead}},
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
			settings, err := clusterconfig.NewAccessGraphSettings(
				&clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
				},
			)
			require.NoError(t, err)
			env, err := newTestEnv(withAuthorizer(test.authorizer), withAccessGraphSettings(settings))
			require.NoError(t, err, "creating test service")

			got, err := env.GetAccessGraphSettings(context.Background(), &clusterconfigpb.GetAccessGraphSettingsRequest{})
			test.assertion(t, err)
			if err == nil {
				require.Empty(t, cmp.Diff(settings, got, cmpopts.IgnoreFields(types.Metadata{}, "Revision"), protocmp.Transform()))
			}
		})
	}
}

func TestUpdateAccessGraphSettings(t *testing.T) {
	cases := []struct {
		name       string
		mutator    func(p *clusterconfigpb.AccessGraphSettings)
		authorizer authz.Authorizer
		testSetup  func(*testing.T)
		assertion  func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent updating access graph settings", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbUpdate}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent updating access graph settings", err)
			},
		},
		{
			name: "update without access graph being enabled",

			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
				}, nil
			}),
			mutator: func(p *clusterconfigpb.AccessGraphSettings) {
				p.Spec.SecretsScanConfig = clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED
			},
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "updated",
			testSetup: func(t *testing.T) {
				m := modules.TestModules{
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Policy: {Enabled: true},
						},
					},
				}
				modules.SetTestModules(t, &m)
			},
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
				}, nil
			}),
			mutator: func(p *clusterconfigpb.AccessGraphSettings) {
				p.Spec.SecretsScanConfig = clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED
			},
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.NoError(t, err)
				require.Equal(t, clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED, updated.GetSpec().GetSecretsScanConfig())
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.testSetup != nil {
				test.testSetup(t)
			}
			settings, err := clusterconfig.NewAccessGraphSettings(
				&clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
				},
			)
			require.NoError(t, err)
			env, err := newTestEnv(withAuthorizer(test.authorizer), withAccessGraphSettings(settings))
			require.NoError(t, err, "creating test service")

			// Set revisions to allow the update to succeed.
			pref := env.defaultAccessGraphSettings
			if test.mutator != nil {
				test.mutator(pref)
			}

			updated, err := env.UpdateAccessGraphSettings(context.Background(), &clusterconfigpb.UpdateAccessGraphSettingsRequest{AccessGraphSettings: pref})
			test.assertion(t, updated, err)
		})
	}
}

func TestUpsertAccessGraphSettings(t *testing.T) {
	cases := []struct {
		name       string
		testSetup  func(*testing.T)
		mutator    func(p *clusterconfigpb.AccessGraphSettings)
		authorizer authz.Authorizer
		assertion  func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent upserting access graph settings", err)
			},
		},
		{
			name: "access prevented",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}),
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent upserting access graph settings", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbCreate, types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthUnauthorized,
				}, nil
			}),
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent upserting access graph settings", err)
			},
		},
		{
			name: "policy not enabled",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbCreate, types.VerbUpdate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
				}, nil
			}),
			mutator: func(p *clusterconfigpb.AccessGraphSettings) {

			},
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.True(t, trace.IsAccessDenied(err), "got (%v), upserting access graph settings must fail when policy isn't enabled", err)
			},
		},

		{
			name: "upserted",
			testSetup: func(t *testing.T) {
				m := modules.TestModules{
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Policy: {Enabled: true},
						},
					},
				}
				modules.SetTestModules(t, &m)
			},
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbUpdate, types.VerbCreate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
				}, nil
			}),
			mutator: func(p *clusterconfigpb.AccessGraphSettings) {
				p.Spec.SecretsScanConfig = clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED
			},
			assertion: func(t *testing.T, updated *clusterconfigpb.AccessGraphSettings, err error) {
				require.NoError(t, err)
				require.Equal(t, clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED, updated.Spec.SecretsScanConfig)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.testSetup != nil {
				test.testSetup(t)
			}
			settings, err := clusterconfig.NewAccessGraphSettings(
				&clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
				})

			require.NoError(t, err)

			env, err := newTestEnv(withAuthorizer(test.authorizer), withAccessGraphSettings(settings))
			require.NoError(t, err, "creating test service")

			// Discard revisions to allow the update to succeed.
			pref := settings
			if test.mutator != nil {
				test.mutator(pref)
			}

			updated, err := env.UpsertAccessGraphSettings(context.Background(), &clusterconfigpb.UpsertAccessGraphSettingsRequest{AccessGraphSettings: pref})
			test.assertion(t, updated, err)
		})
	}
}

func TestResetAccessGraphSettings(t *testing.T) {
	cases := []struct {
		name       string
		authorizer authz.Authorizer
		testSetup  func(*testing.T)
		assertion  func(t *testing.T, reset *clusterconfigpb.AccessGraphSettings, err error)
	}{
		{
			name: "unauthorized",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{},
				}, nil
			}),
			assertion: func(t *testing.T, reset *clusterconfigpb.AccessGraphSettings, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected unauthorized user to prevent resetting access graph settings", err)
			},
		},
		{
			name: "no admin action",
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbUpdate}},
					},
				}, nil
			}),
			assertion: func(t *testing.T, reset *clusterconfigpb.AccessGraphSettings, err error) {
				assert.Nil(t, reset)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected lack of admin action to prevent resetting access graph settings", err)
			},
		},
		{
			name: "reset",
			testSetup: func(t *testing.T) {
				m := modules.TestModules{
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Policy: {Enabled: true},
						},
					},
				}
				modules.SetTestModules(t, &m)
			},
			authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Checker: fakeChecker{
						rules: map[string][]string{types.KindAccessGraphSettings: {types.VerbUpdate, types.VerbCreate}},
					},
					AdminActionAuthState: authz.AdminActionAuthMFAVerified,
					Identity: authz.LocalUser{
						Username: "llama",
						Identity: tlsca.Identity{Username: "llama"},
					},
				}, nil
			}),
			assertion: func(t *testing.T, reset *clusterconfigpb.AccessGraphSettings, err error) {
				require.NoError(t, err)
				require.Equal(t, clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED, reset.GetSpec().GetSecretsScanConfig())
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.testSetup != nil {
				test.testSetup(t)
			}
			settings, err := clusterconfig.NewAccessGraphSettings(
				&clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
				})

			require.NoError(t, err)

			env, err := newTestEnv(withAuthorizer(test.authorizer), withAccessGraphSettings(settings))
			require.NoError(t, err, "creating test service")

			reset, err := env.ResetAccessGraphSettings(context.Background(), &clusterconfigpb.ResetAccessGraphSettingsRequest{})
			test.assertion(t, reset, err)
		})
	}
}
