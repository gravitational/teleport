/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"context"
	_ "embed"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/modules"
)

func TestGetAccessGraph(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{IdentityGovernanceSecurity: true, Policy: modules.PolicyFeature{Enabled: true}},
	})

	tests := []struct {
		name             string
		features         string
		validation       func(*testing.T, *webSuite)
		assertQueryError require.ErrorAssertionFunc
		grantAccessToTag bool
	}{
		{
			name: "server doesn't support HTTP: user without access graph access",
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.GreaterOrEqual(t, 2, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.Error,
		},
		{
			name: "server doesn't support HTTP: authenticated requests with proper role",
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.GreaterOrEqual(t, 2, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 1, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.NoError,
			grantAccessToTag: true,
		},
		{
			name:     "server supports HTTP: user without access graph access",
			features: features,
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.Equal(t, 1, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 2, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.Error,
		},
		{
			name:     "server supports HTTP: authenticated requests with proper role",
			features: features,
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.Equal(t, 1, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 2, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 1, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.NoError,
			grantAccessToTag: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s := newWebSuite(t, withAccessGraphFeatures(test.features))
			var opts []webSuiteOpts
			if test.grantAccessToTag {
				opts = append(opts, withExtraRules(types.Rule{
					Resources: []string{types.KindAccessGraph},
					Verbs:     []string{types.ActionRead},
				}))
			}
			webPack := s.newAuthWebPack(t, "foo", opts...)

			endpoint := webPack.clt.Endpoint("enterprise", "accessgraph", "static", "features.json")
			_, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
			require.NoError(t, err)

			endpoint = webPack.clt.Endpoint("enterprise", "accessgraph", "query")
			q := url.Values{}
			q.Set("query", "foo")
			_, err = webPack.clt.Get(s.ctx, endpoint, q)
			test.assertQueryError(t, err)

			test.validation(t, s)

		})
	}
}

//go:embed testdata/access_graph_integrations_response.json
var expectedListIntegrationsResponse string

func TestGetAccessGraphIntegrations(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{IdentityGovernanceSecurity: true, Policy: modules.PolicyFeature{Enabled: true}},
	})

	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	ctx := context.Background()

	disConfig1, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "discovery-config-1"}, discoveryconfig.Spec{
			DiscoveryGroup: "discovery-group-1",
			AccessGraph: &types.AccessGraphSync{
				AWS: []*types.AccessGraphAWSSync{
					{
						Regions: []string{"us-west-1"},
						AssumeRole: &types.AssumeRole{
							RoleARN: "arn:aws:iam::123456789012:role/role-name",
						},
					},
				},
			},
		},
	)
	require.NoError(t, err)

	_, err = authClient.DiscoveryConfigClient().UpsertDiscoveryConfig(ctx, disConfig1)
	require.NoError(t, err)

	disConfig2, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "discovery-config-2"}, discoveryconfig.Spec{
			DiscoveryGroup: "discovery-group-1",
			AccessGraph: &types.AccessGraphSync{
				AWS: []*types.AccessGraphAWSSync{
					{
						Integration: "integration-1",
						Regions:     []string{"us-west-1"},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	_, err = authClient.DiscoveryConfigClient().UpsertDiscoveryConfig(ctx, disConfig2)
	require.NoError(t, err)

	_, err = authClient.CreateIntegration(ctx, &types.IntegrationV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "integration-1",
			},
			SubKind: types.IntegrationSubKindAWSOIDC,
		},
		Spec: types.IntegrationSpecV1{
			SubKindSpec: &types.IntegrationSpecV1_AWSOIDC{
				AWSOIDC: &types.AWSOIDCIntegrationSpecV1{
					RoleARN: "arn:aws:iam::0987654321:role/role-name",
				},
			},
		},
	})

	require.NoError(t, err)
	_, err = authClient.PluginsClient().CreatePlugin(ctx, &pluginsv1.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Kind:    types.KindPlugin,
			SubKind: types.PluginSubkindAccessGraph,
			Metadata: types.Metadata{
				Name:   "gitlab",
				Labels: map[string]string{},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Gitlab{
					Gitlab: &types.PluginGitlabSettings{
						ApiEndpoint: "https://gitlab.com",
					},
				},
			},
			Status: types.PluginStatusV1{
				Code:         types.PluginStatusCode_RUNNING,
				ErrorMessage: "fake error",
				LastSyncTime: s.clock.Now(),
				Details: &types.PluginStatusV1_Gitlab{
					Gitlab: &types.PluginGitlabStatusV1{
						ImportedGroups:   100,
						ImportedUsers:    200,
						ImportedProjects: 500,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "integration-1",
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "token",
				},
			},
		},
	})

	require.NoError(t, err)

	_, err = authClient.PluginsClient().CreatePlugin(ctx, &pluginsv1.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Kind: types.KindPlugin,
			Metadata: types.Metadata{
				Name:   "okta",
				Labels: map[string]string{},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Okta{
					Okta: &types.PluginOktaSettings{
						OrgUrl: "https://ultraorg.okta.com",
					},
				},
			},
			Status: types.PluginStatusV1{
				LastSyncTime: s.clock.Now(),
				Code:         types.PluginStatusCode_RUNNING,
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "integration-2",
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "token",
				},
			},
		},
	})

	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "accessgraph", "integrations")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	require.JSONEq(t, expectedListIntegrationsResponse, string(resp.Bytes()))
}
