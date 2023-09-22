/**
 * Copyright 2021 Gravitational, Inc.
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
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestExtractResourceAndValidate(t *testing.T) {
	goodContent := `kind: role
metadata:
  name: test
spec:
  allow:
    logins:
    - testing
version: v3`
	extractedResource, err := ExtractResourceAndValidate(goodContent)
	require.Nil(t, err)
	require.NotNil(t, extractedResource)

	// Test missing name.
	invalidContent := `kind: role
metadata:
  name:`
	extractedResource, err = ExtractResourceAndValidate(invalidContent)
	require.Nil(t, extractedResource)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "Name")
}

func TestCheckResourceUpsert(t *testing.T) {
	tests := []struct {
		desc                string
		httpMethod          string
		httpParams          httprouter.Params
		payloadResourceName string
		get                 getResource
		assertErr           require.ErrorAssertionFunc
	}{
		{
			desc:                "creating non-existing resource succeeds",
			httpMethod:          "POST",
			httpParams:          httprouter.Params{},
			payloadResourceName: "my-resource",
			get: func(ctx context.Context, name string) (types.Resource, error) {
				// Resource does not exist.
				return nil, trace.NotFound("")
			},
			assertErr: require.NoError,
		},
		{
			desc:                "creating existing resource fails",
			httpMethod:          "POST",
			httpParams:          httprouter.Params{},
			payloadResourceName: "my-resource",
			get: func(ctx context.Context, name string) (types.Resource, error) {
				// Resource does exist.
				return nil, nil
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsAlreadyExists(err))
			},
		},
		{
			desc:                "updating resource without name HTTP param fails",
			httpMethod:          "PUT",
			httpParams:          httprouter.Params{},
			payloadResourceName: "my-resource",
			get: func(ctx context.Context, name string) (types.Resource, error) {
				// Resource does exist.
				return nil, nil
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			},
		},
		{
			desc:                "updating non-existing resource fails",
			httpMethod:          "PUT",
			httpParams:          httprouter.Params{httprouter.Param{Key: "name", Value: "my-resource"}},
			payloadResourceName: "my-resource",
			get: func(ctx context.Context, name string) (types.Resource, error) {
				// Resource does not exist.
				return nil, trace.NotFound("")
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			desc:                "updating existing resource succeeds",
			httpMethod:          "PUT",
			httpParams:          httprouter.Params{httprouter.Param{Key: "name", Value: "my-resource"}},
			payloadResourceName: "my-resource",
			get: func(ctx context.Context, name string) (types.Resource, error) {
				// Resource does exist.
				return nil, nil
			},
			assertErr: require.NoError,
		},
		{
			desc:                "renaming existing resource fails",
			httpMethod:          "PUT",
			httpParams:          httprouter.Params{httprouter.Param{Key: "name", Value: "my-resource"}},
			payloadResourceName: "my-resource-new-name",
			get: func(ctx context.Context, name string) (types.Resource, error) {
				// Resource does exist.
				return nil, nil
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := CheckResourceUpsert(context.TODO(), tc.httpMethod, tc.httpParams, tc.payloadResourceName, tc.get)
			tc.assertErr(t, err)
		})
	}
}

func TestNewResourceItemGithub(t *testing.T) {
	const contents = `kind: github
metadata:
  name: githubName
spec:
  api_endpoint_url: ""
  client_id: ""
  client_secret: ""
  display: ""
  endpoint_url: ""
  redirect_url: ""
  teams_to_logins:
  - logins:
    - dummy
    organization: octocats
    team: dummy
  teams_to_roles: null
version: v3
`
	githubConn, err := types.NewGithubConnector("githubName", types.GithubConnectorSpecV3{
		TeamsToLogins: []types.TeamMapping{
			{
				Organization: "octocats",
				Team:         "dummy",
				Logins:       []string{"dummy"},
			},
		},
	})
	require.NoError(t, err)
	item, err := ui.NewResourceItem(githubConn)
	require.NoError(t, err)

	require.Equal(t, &ui.ResourceItem{
		ID:      "github:githubName",
		Kind:    types.KindGithubConnector,
		Name:    "githubName",
		Content: contents,
	}, item)
}

func TestNewResourceItemRole(t *testing.T) {
	const contents = `kind: role
metadata:
  name: roleName
spec:
  allow:
    app_labels:
      '*': '*'
    db_labels:
      '*': '*'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
    - kind: pod
      name: '*'
      namespace: '*'
    logins:
    - test
    node_labels:
      '*': '*'
  deny: {}
  options:
    cert_format: standard
    create_desktop_user: false
    desktop_clipboard: true
    desktop_directory_sharing: true
    enhanced_recording:
    - command
    - network
    forward_agent: false
    idp:
      saml:
        enabled: true
    max_session_ttl: 30h0m0s
    pin_source_ip: false
    port_forwarding: true
    record_session:
      default: best_effort
      desktop: true
    ssh_file_copy: true
version: v6
`
	role, err := types.NewRole("roleName", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:           []string{"test"},
			NodeLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
			AppLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard,
				},
			},
		},
	})
	require.NoError(t, err)

	item, err := ui.NewResourceItem(role)
	require.NoError(t, err)
	require.Equal(t, &ui.ResourceItem{
		ID:      "role:roleName",
		Kind:    types.KindRole,
		Name:    "roleName",
		Content: contents,
	}, item)
}

func TestNewResourceItemTrustedCluster(t *testing.T) {
	const contents = `kind: trusted_cluster
metadata:
  name: tcName
spec:
  enabled: false
  token: ""
  tunnel_addr: ""
  web_proxy_addr: ""
version: v2
`
	cluster, err := types.NewTrustedCluster("tcName", types.TrustedClusterSpecV2{})
	require.Nil(t, err)

	item, err := ui.NewResourceItem(cluster)
	require.Nil(t, err)
	require.Equal(t, item, &ui.ResourceItem{
		ID:      "trusted_cluster:tcName",
		Kind:    types.KindTrustedCluster,
		Name:    "tcName",
		Content: contents,
	})
}

func TestGetRoles(t *testing.T) {
	m := &mockedResourceAPIGetter{}

	m.mockGetRoles = func(ctx context.Context) ([]types.Role, error) {
		role, err := types.NewRole("test", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"test"},
			},
		})
		require.Nil(t, err)

		return []types.Role{role}, nil
	}

	// Test response is converted to ui objects.
	roles, err := getRoles(m)
	require.Nil(t, err)
	require.Len(t, roles, 1)
	require.Contains(t, roles[0].Content, "name: test")
}

func TestUpsertRole(t *testing.T) {
	m := &mockedResourceAPIGetter{}

	existingRoles := make(map[string]types.Role)
	m.mockUpsertRole = func(ctx context.Context, role types.Role) error {
		existingRoles[role.GetName()] = role
		return nil
	}
	m.mockGetRole = func(ctx context.Context, name string) (types.Role, error) {
		role, ok := existingRoles[name]
		if ok {
			return role, nil
		}
		return nil, trace.NotFound("")
	}

	// Test bad request kind.
	invalidKind := `kind: invalid-kind
metadata:
  name: test`
	role, err := upsertRole(context.Background(), m, invalidKind, "", httprouter.Params{})
	require.Nil(t, role)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "kind")

	goodContent := `kind: role
metadata:
  name: test-goodcontent
spec:
  allow:
    logins:
    - testing
version: v3`

	// Updating non-existing role fails.
	role, err = upsertRole(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, role)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Creating non-existing role succeeds.
	role, err = upsertRole(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.NoError(t, err)
	require.Contains(t, role.Content, "name: test-goodcontent")

	// Creating existing role fails.
	role, err = upsertRole(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.Nil(t, role)
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err))

	// Updating existing role succeeds.
	role, err = upsertRole(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.NoError(t, err)
	require.Contains(t, role.Content, "name: test-goodcontent")

	// Renaming existing role fails.
	goodContentRenamed := strings.ReplaceAll(goodContent, "test-goodcontent", "test-goodcontent-new-name")
	role, err = upsertRole(context.Background(), m, goodContentRenamed, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, role)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
}

func TestGetGithubConnectors(t *testing.T) {
	ctx := context.Background()
	m := &mockedResourceAPIGetter{}

	m.mockGetGithubConnectors = func(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
		connector, err := types.NewGithubConnector("test", types.GithubConnectorSpecV3{
			TeamsToLogins: []types.TeamMapping{
				{
					Organization: "octocats",
					Team:         "dummy",
					Logins:       []string{"dummy"},
				},
			},
		})
		require.NoError(t, err)

		return []types.GithubConnector{connector}, nil
	}

	// Test response is converted to ui objects.
	connectors, err := getGithubConnectors(ctx, m)
	require.Nil(t, err)
	require.Len(t, connectors, 1)
	require.Contains(t, connectors[0].Content, "name: test")
}

func TestUpsertGithubConnector(t *testing.T) {
	m := &mockedResourceAPIGetter{}

	existingConnectors := make(map[string]types.GithubConnector)
	m.mockUpsertGithubConnector = func(ctx context.Context, connector types.GithubConnector) error {
		existingConnectors[connector.GetName()] = connector
		return nil
	}
	m.mockGetGithubConnector = func(ctx context.Context, name string, withSecrets bool) (types.GithubConnector, error) {
		connector, ok := existingConnectors[name]
		if ok {
			return connector, nil
		}
		return nil, trace.NotFound("")
	}

	// Test bad request kind.
	invalidKind := `kind: invalid-kind
metadata:
  name: test`
	connector, err := upsertGithubConnector(context.Background(), m, invalidKind, "", httprouter.Params{})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "kind")

	goodContent := `kind: github
metadata:
  name: test-goodcontent
spec:
  client_id: <client-id>
  client_secret: <client-secret>
  display: Github
  redirect_url: https://<cluster-url>/v1/webapi/github/callback
  teams_to_logins:
  - logins:
    - admins
    organization: <github-org>
    team: admins
version: v3`

	// Updating non-existing connector fails.
	connector, err = upsertGithubConnector(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Creating non-existing connector succeeds.
	connector, err = upsertGithubConnector(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.NoError(t, err)
	require.Contains(t, connector.Content, "name: test-goodcontent")

	// Creating existing connector fails.
	connector, err = upsertGithubConnector(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err))

	// Updating existing connector succeeds.
	connector, err = upsertGithubConnector(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.NoError(t, err)
	require.Contains(t, connector.Content, "name: test-goodcontent")

	// Renaming existing connector fails.
	goodContentRenamed := strings.ReplaceAll(goodContent, "test-goodcontent", "test-goodcontent-new-name")
	connector, err = upsertGithubConnector(context.Background(), m, goodContentRenamed, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
}

func TestGetTrustedClusters(t *testing.T) {
	ctx := context.Background()
	m := &mockedResourceAPIGetter{}

	m.mockGetTrustedClusters = func(ctx context.Context) ([]types.TrustedCluster, error) {
		cluster, err := types.NewTrustedCluster("test", types.TrustedClusterSpecV2{})
		require.Nil(t, err)

		return []types.TrustedCluster{cluster}, nil
	}

	// Test response is converted to ui objects.
	tcs, err := getTrustedClusters(ctx, m)
	require.Nil(t, err)
	require.Len(t, tcs, 1)
	require.Contains(t, tcs[0].Content, "name: test")
}

func TestUpsertTrustedCluster(t *testing.T) {
	m := &mockedResourceAPIGetter{}

	existingTrustedClusters := make(map[string]types.TrustedCluster)
	m.mockUpsertTrustedCluster = func(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error) {
		existingTrustedClusters[tc.GetName()] = tc
		return tc, nil
	}
	m.mockGetTrustedCluster = func(ctx context.Context, name string) (types.TrustedCluster, error) {
		tc, ok := existingTrustedClusters[name]
		if ok {
			return tc, nil
		}
		return nil, trace.NotFound("")
	}

	// Test bad request kind.
	invalidKind := `kind: invalid-kind
metadata:
  name: test`
	tc, err := upsertTrustedCluster(context.Background(), m, invalidKind, "", httprouter.Params{})
	require.Nil(t, tc)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "kind")

	goodContent := `kind: trusted_cluster
metadata:
  name: test-goodcontent
spec:
  role_map:
  - local:
    - admin
    remote: admin
version: v2`

	// Updating non-existing trusted cluster fails.
	tc, err = upsertTrustedCluster(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, tc)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Creating non-existing trusted cluster succeeds.
	tc, err = upsertTrustedCluster(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.NoError(t, err)
	require.Contains(t, tc.Content, "name: test-goodcontent")

	// Creating existing trusted cluster fails.
	tc, err = upsertTrustedCluster(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.Nil(t, tc)
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err))

	// Updating existing trusted cluster succeeds.
	tc, err = upsertTrustedCluster(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.NoError(t, err)
	require.Contains(t, tc.Content, "name: test-goodcontent")

	// Renaming existing trusted cluster fails.
	goodContentRenamed := strings.ReplaceAll(goodContent, "test-goodcontent", "test-goodcontent-new-name")
	tc, err = upsertTrustedCluster(context.Background(), m, goodContentRenamed, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, tc)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
}

func TestListResources(t *testing.T) {
	t.Parallel()

	// Test parsing query params.
	testCases := []struct {
		name, url       string
		wantBadParamErr bool
		expected        proto.ListResourcesRequest
	}{
		{
			name: "decode complex query correctly",
			url:  "https://dev:3080/login?query=(labels%5B%60%22test%22%60%5D%20%3D%3D%20%22%2B%3A'%2C%23*~%25%5E%22%20%26%26%20!exists(labels.tier))%20%7C%7C%20resource.spec.description%20!%3D%20%22weird%20example%20https%3A%2F%2Ffoo.dev%3A3080%3Fbar%3Da%2Cb%26baz%3Dbanana%22",
			expected: proto.ListResourcesRequest{
				ResourceType:        types.KindNode,
				Limit:               defaults.MaxIterationLimit,
				PredicateExpression: "(labels[`\"test\"`] == \"+:',#*~%^\" && !exists(labels.tier)) || resource.spec.description != \"weird example https://foo.dev:3080?bar=a,b&baz=banana\"",
			},
		},
		{
			name: "all param defined and set",
			url:  `https://dev:3080/login?searchAsRoles=yes&query=labels.env%20%3D%3D%20%22prod%22&limit=50&startKey=banana&sort=foo:desc&search=foo%2Bbar+baz+foo%2Cbar+%22some%20phrase%22`,
			expected: proto.ListResourcesRequest{
				ResourceType:        types.KindNode,
				Limit:               50,
				StartKey:            "banana",
				SearchKeywords:      []string{"foo+bar", "baz", "foo,bar", "some phrase"},
				PredicateExpression: `labels.env == "prod"`,
				SortBy:              types.SortBy{Field: "foo", IsDesc: true},
				UseSearchAsRoles:    true,
			},
		},
		{
			name: "all query param defined but empty",
			url:  `https://dev:3080/login?query=&startKey=&search=&sort=&limit=&startKey=`,
			expected: proto.ListResourcesRequest{
				ResourceType: types.KindNode,
				Limit:        defaults.MaxIterationLimit,
			},
		},
		{
			name: "sort partially defined: fieldName",
			url:  `https://dev:3080/login?sort=foo`,
			expected: proto.ListResourcesRequest{
				ResourceType: types.KindNode,
				Limit:        defaults.MaxIterationLimit,
				SortBy:       types.SortBy{Field: "foo", IsDesc: false},
			},
		},
		{
			name: "sort partially defined: fieldName with colon",
			url:  `https://dev:3080/login?sort=foo:`,
			expected: proto.ListResourcesRequest{
				ResourceType: types.KindNode,
				Limit:        defaults.MaxIterationLimit,
				SortBy:       types.SortBy{Field: "foo", IsDesc: false},
			},
		},
		{
			name:            "invalid limit value",
			wantBadParamErr: true,
			url:             `https://dev:3080/login?limit=12invalid`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			httpReq, err := http.NewRequest("", tc.url, nil)
			require.NoError(t, err)

			m := &mockedResourceAPIGetter{}
			m.mockListResources = func(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
				if !tc.wantBadParamErr {
					require.Equal(t, tc.expected, req)
				}
				return nil, nil
			}

			_, err = listResources(m, httpReq, types.KindNode)
			if tc.wantBadParamErr {
				require.True(t, trace.IsBadParameter(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type mockedResourceAPIGetter struct {
	mockGetRole               func(ctx context.Context, name string) (types.Role, error)
	mockGetRoles              func(ctx context.Context) ([]types.Role, error)
	mockUpsertRole            func(ctx context.Context, role types.Role) error
	mockUpsertGithubConnector func(ctx context.Context, connector types.GithubConnector) error
	mockGetGithubConnectors   func(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	mockGetGithubConnector    func(ctx context.Context, id string, withSecrets bool) (types.GithubConnector, error)
	mockDeleteGithubConnector func(ctx context.Context, id string) error
	mockUpsertTrustedCluster  func(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error)
	mockGetTrustedCluster     func(ctx context.Context, name string) (types.TrustedCluster, error)
	mockGetTrustedClusters    func(ctx context.Context) ([]types.TrustedCluster, error)
	mockDeleteTrustedCluster  func(ctx context.Context, name string) error
	mockListResources         func(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

func (m *mockedResourceAPIGetter) GetRole(ctx context.Context, name string) (types.Role, error) {
	if m.mockGetRole != nil {
		return m.mockGetRole(ctx, name)
	}
	return nil, trace.NotImplemented("mockGetRole not implemented")
}

func (m *mockedResourceAPIGetter) GetRoles(ctx context.Context) ([]types.Role, error) {
	if m.mockGetRoles != nil {
		return m.mockGetRoles(ctx)
	}
	return nil, trace.NotImplemented("mockGetRoles not implemented")
}

func (m *mockedResourceAPIGetter) UpsertRole(ctx context.Context, role types.Role) error {
	if m.mockUpsertRole != nil {
		return m.mockUpsertRole(ctx, role)
	}

	return trace.NotImplemented("mockUpsertRole not implemented")
}

func (m *mockedResourceAPIGetter) UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error {
	if m.mockUpsertGithubConnector != nil {
		return m.mockUpsertGithubConnector(ctx, connector)
	}

	return trace.NotImplemented("mockUpsertGithubConnector not implemented")
}

func (m *mockedResourceAPIGetter) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
	if m.mockGetGithubConnectors != nil {
		return m.mockGetGithubConnectors(ctx, false)
	}

	return nil, trace.NotImplemented("mockGetGithubConnectors not implemented")
}

func (m *mockedResourceAPIGetter) GetGithubConnector(ctx context.Context, id string, withSecrets bool) (types.GithubConnector, error) {
	if m.mockGetGithubConnector != nil {
		return m.mockGetGithubConnector(ctx, id, false)
	}

	return nil, trace.NotImplemented("mockGetGithubConnector not implemented")
}

func (m *mockedResourceAPIGetter) DeleteGithubConnector(ctx context.Context, id string) error {
	if m.mockDeleteGithubConnector != nil {
		return m.mockDeleteGithubConnector(ctx, id)
	}

	return trace.NotImplemented("mockDeleteGithubConnector not implemented")
}

func (m *mockedResourceAPIGetter) UpsertTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error) {
	if m.mockUpsertTrustedCluster != nil {
		return m.mockUpsertTrustedCluster(ctx, tc)
	}

	return nil, trace.NotImplemented("mockUpsertTrustedCluster not implemented")
}

func (m *mockedResourceAPIGetter) GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error) {
	if m.mockGetTrustedCluster != nil {
		return m.mockGetTrustedCluster(ctx, name)
	}

	return nil, trace.NotImplemented("mockGetTrustedCluster not implemented")
}

func (m *mockedResourceAPIGetter) GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error) {
	if m.mockGetTrustedClusters != nil {
		return m.mockGetTrustedClusters(ctx)
	}

	return nil, trace.NotImplemented("mockGetTrustedClusters not implemented")
}

func (m *mockedResourceAPIGetter) DeleteTrustedCluster(ctx context.Context, name string) error {
	if m.mockDeleteTrustedCluster != nil {
		return m.mockDeleteTrustedCluster(ctx, name)
	}

	return trace.NotImplemented("mockDeleteTrustedCluster not implemented")
}

func (m *mockedResourceAPIGetter) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if m.mockListResources != nil {
		return m.mockListResources(ctx, req)
	}

	return nil, trace.NotImplemented("mockListResources not implemented")
}

func Test_newKubeListRequest(t *testing.T) {
	type args struct {
		query        string
		site         string
		resourceKind string
	}
	tests := []struct {
		name string
		args args
		want *kubeproto.ListKubernetesResourcesRequest
	}{
		{
			name: "list resources",
			args: args{
				query:        "kind=kind1",
				site:         "site1",
				resourceKind: "kind1",
			},
			want: &kubeproto.ListKubernetesResourcesRequest{
				TeleportCluster: "site1",
				ResourceType:    "kind1",
				SortBy:          &types.SortBy{},
				Limit:           defaults.MaxIterationLimit,
			},
		},
		{
			name: "list resources with sort and query",
			args: args{
				query:        "kind=kind1&query=foo&sort=bar:desc&limit=10",
				site:         "site1",
				resourceKind: "kind1",
			},
			want: &kubeproto.ListKubernetesResourcesRequest{
				TeleportCluster:     "site1",
				ResourceType:        "kind1",
				PredicateExpression: "foo",
				SortBy: &types.SortBy{
					Field:  "bar",
					IsDesc: true,
				},
				Limit: 10,
			},
		},
		{
			name: "list resources with search as roles",
			args: args{
				query:        "startKey=startK1&query=bar&sort=foo:asc&searchAsRoles=yes&limit=10&kubeCluster=cluster&kubeNamespace=namespace",
				site:         "site1",
				resourceKind: "kind1",
			},
			want: &kubeproto.ListKubernetesResourcesRequest{
				StartKey:            "startK1",
				KubernetesCluster:   "cluster",
				KubernetesNamespace: "namespace",
				TeleportCluster:     "site1",
				ResourceType:        "kind1",
				PredicateExpression: "bar",
				SortBy: &types.SortBy{
					Field:  "foo",
					IsDesc: false,
				},
				UseSearchAsRoles: true,
				Limit:            10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, err := url.ParseQuery(tt.args.query)
			require.NoError(t, err)
			got, err := newKubeListRequest(values, tt.args.site, tt.args.resourceKind)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
