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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/clusterconfig/clusterconfigv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
)

type envConfig struct {
	authorizer        authz.Authorizer
	accessGraphConfig clusterconfigv1.AccessGraphConfig
}
type serviceOpt = func(config *envConfig)

func withAuthorizer(authz authz.Authorizer) serviceOpt {
	return func(config *envConfig) {
		config.authorizer = authz
	}
}

func withAccessGraphConfig(cfg clusterconfigv1.AccessGraphConfig) serviceOpt {
	return func(config *envConfig) {
		config.accessGraphConfig = cfg
	}
}

type env struct {
	*clusterconfigv1.Service
}

func newTestEnv(opts ...serviceOpt) (*env, error) {
	var cfg envConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	svc, err := clusterconfigv1.NewService(clusterconfigv1.ServiceConfig{
		Authorizer:  cfg.authorizer,
		AccessGraph: cfg.accessGraphConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating users service")
	}

	return &env{
		Service: svc,
	}, nil
}

func TestGetAccessGraphConfig(t *testing.T) {
	cfgEnabled := clusterconfigv1.AccessGraphConfig{
		Enabled:  true,
		Address:  "address",
		CA:       []byte("ca"),
		Insecure: true,
	}
	cases := []struct {
		name              string
		accessGraphConfig clusterconfigv1.AccessGraphConfig
		role              types.SystemRole
		testSetup         func(*testing.T)
		errorAssertion    require.ErrorAssertionFunc
		responseAssertion *clusterconfigpb.GetClusterAccessGraphConfigResponse
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
						Policy: modules.PolicyFeature{
							Enabled: true,
						},
					},
				}
				modules.SetTestModules(t, &m)
			},
			accessGraphConfig: cfgEnabled,
			errorAssertion:    require.NoError,
			responseAssertion: &clusterconfigpb.GetClusterAccessGraphConfigResponse{
				AccessGraph: &clusterconfigpb.AccessGraphConfig{
					Enabled:  true,
					Insecure: true,
					Address:  "address",
					Ca:       []byte("ca"),
				},
			},
		},
		{
			name: "authorized discovery with non empty access graph config; Policy module is enabled",
			role: types.RoleDiscovery,
			testSetup: func(t *testing.T) {
				m := modules.TestModules{
					TestFeatures: modules.Features{
						Policy: modules.PolicyFeature{
							Enabled: true,
						},
					},
				}
				modules.SetTestModules(t, &m)
			},
			accessGraphConfig: cfgEnabled,
			errorAssertion:    require.NoError,
			responseAssertion: &clusterconfigpb.GetClusterAccessGraphConfigResponse{
				AccessGraph: &clusterconfigpb.AccessGraphConfig{
					Enabled:  true,
					Insecure: true,
					Address:  "address",
					Ca:       []byte("ca"),
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
			env, err := newTestEnv(withAuthorizer(authorizer), withAccessGraphConfig(test.accessGraphConfig))
			require.NoError(t, err, "creating test service")

			got, err := env.GetClusterAccessGraphConfig(context.Background(), &clusterconfigpb.GetClusterAccessGraphConfigRequest{})
			test.errorAssertion(t, err)

			require.Empty(t, cmp.Diff(test.responseAssertion, got, protocmp.Transform()))
		})
	}
}
