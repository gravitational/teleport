package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGithubConnectorCheckAndSetDefaults(t *testing.T) {
	tests := []struct {
		name              string
		connector         GithubConnector
		expectedConnector GithubConnector
		expectedErr       string
	}{
		{
			name: "all defaults",
			connector: GithubConnector{
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
			expectedConnector: GithubConnector{
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
			connector: GithubConnector{
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
			expectedConnector: GithubConnector{
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
			connector: GithubConnector{
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
			expectedConnector: GithubConnector{
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
