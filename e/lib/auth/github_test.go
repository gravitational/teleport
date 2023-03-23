/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGithubConnectorCheckAndSetDefaults(t *testing.T) {
	tests := []struct {
		name              string
		connector         GithubConnectorE
		expectedConnector GithubConnectorE
		expectedErr       string
	}{
		{
			name: "all defaults",
			connector: GithubConnectorE{
				GithubConnectorV3: &types.GithubConnectorV3{
					Metadata: types.Metadata{
						Name: "test-connector",
					},
					Spec: types.GithubConnectorSpecV3{
						TeamsToRoles: []types.TeamRolesMapping{
							{
								Organization: "foo",
								Team:         "bar",
							},
						},
					},
				},
			},
			expectedConnector: GithubConnectorE{
				GithubConnectorV3: &types.GithubConnectorV3{
					Kind: types.KindGithubConnector,
					Metadata: types.Metadata{
						Name:      "test-connector",
						Namespace: "default",
					},
					Spec: types.GithubConnectorSpecV3{
						TeamsToRoles: []types.TeamRolesMapping{
							{
								Organization: "foo",
								Team:         "bar",
							},
						},
						EndpointURL:    "https://github.com",
						APIEndpointURL: "https://api.github.com",
					},
					Version: types.V3,
				},
			},
		},
		{
			name: "unset api_endpoint_url",
			connector: GithubConnectorE{
				GithubConnectorV3: &types.GithubConnectorV3{
					Metadata: types.Metadata{
						Name: "test-connector",
					},
					Spec: types.GithubConnectorSpecV3{
						TeamsToRoles: []types.TeamRolesMapping{
							{
								Organization: "foo",
								Team:         "bar",
							},
						},
						EndpointURL: "https://foo.bar",
					},
				},
			},
			expectedConnector: GithubConnectorE{
				GithubConnectorV3: &types.GithubConnectorV3{
					Kind: types.KindGithubConnector,
					Metadata: types.Metadata{
						Name:      "test-connector",
						Namespace: "default",
					},
					Spec: types.GithubConnectorSpecV3{
						TeamsToRoles: []types.TeamRolesMapping{
							{
								Organization: "foo",
								Team:         "bar",
							},
						},
						EndpointURL:    "https://foo.bar",
						APIEndpointURL: "https://api.foo.bar",
					},
					Version: types.V3,
				},
			},
		},
		{
			name: "endpoint_url explicitly set to default",
			connector: GithubConnectorE{
				GithubConnectorV3: &types.GithubConnectorV3{
					Metadata: types.Metadata{
						Name: "test-connector",
					},
					Spec: types.GithubConnectorSpecV3{
						TeamsToRoles: []types.TeamRolesMapping{
							{
								Organization: "foo",
								Team:         "bar",
							},
						},
						EndpointURL: "https://github.com",
					},
				},
			},
			expectedConnector: GithubConnectorE{
				GithubConnectorV3: &types.GithubConnectorV3{
					Kind: types.KindGithubConnector,
					Metadata: types.Metadata{
						Name:      "test-connector",
						Namespace: "default",
					},
					Spec: types.GithubConnectorSpecV3{
						TeamsToRoles: []types.TeamRolesMapping{
							{
								Organization: "foo",
								Team:         "bar",
							},
						},
						EndpointURL:    "https://github.com",
						APIEndpointURL: "https://api.github.com",
					},
					Version: types.V3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.connector.CheckAndSetDefaults()
			if tt.expectedErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.expectedConnector, tt.connector)
			} else {
				require.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}
