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

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
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
	require.NoError(t, err)
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
	ctx := context.Background()
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
			err := CheckResourceUpsert(ctx, tc.httpMethod, tc.httpParams, tc.payloadResourceName, tc.get)
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
    create_db_user: false
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
    record_session:
      default: best_effort
      desktop: true
    ssh_file_copy: true
version: v7
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
	require.NoError(t, err)

	item, err := ui.NewResourceItem(cluster)
	require.NoError(t, err)
	require.Equal(t, &ui.ResourceItem{
		ID:      "trusted_cluster:tcName",
		Kind:    types.KindTrustedCluster,
		Name:    "tcName",
		Content: contents,
	}, item)
}

func TestGetRoles(t *testing.T) {
	m := &mockedResourceAPIGetter{}

	m.mockListRoles = func(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error) {
		role, err := types.NewRole("test", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"test"},
			},
		})
		require.NoError(t, err)

		return &proto.ListRolesResponse{
			Roles:   []*types.RoleV6{role.(*types.RoleV6)},
			NextKey: "",
		}, nil
	}

	// Test response is converted to ui objects.
	roles, err := listRoles(m, url.Values{})
	require.NoError(t, err)
	require.Len(t, roles.Items, 1)
	require.Contains(t, roles.Items.([]ui.ResourceItem)[0].Content, "name: test")
}

func TestRoleCRUD(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)

	proxy := env.proxies[0]

	// Authenticate to get a session token and cookies.
	pack := proxy.authPack(t, "test-user@example.com", nil)

	expected, err := types.NewRole("test-role", types.RoleSpecV6{})
	require.NoError(t, err, "creating initial role resource")

	createPayload := func(r types.Role) ui.ResourceItem {
		raw, err := services.MarshalRole(r, services.PreserveRevision())
		require.NoError(t, err, "marshaling role")

		return ui.ResourceItem{
			Kind:    types.KindRole,
			Name:    r.GetName(),
			Content: string(raw),
		}
	}

	unmarshalResponse := func(resp []byte) types.Role {
		var item ui.ResourceItem
		require.NoError(t, json.Unmarshal(resp, &item), "response from server contained an invalid resource item")

		var r types.RoleV6
		require.NoError(t, yaml.Unmarshal([]byte(item.Content), &r), "resource item content was not a role")
		return &r
	}

	// Create the initial role.
	resp, err := pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "roles"), createPayload(expected))
	require.NoError(t, err, "expected creating the initial role to succeed")
	require.Equal(t, http.StatusOK, resp.Code(), "unexpected status code creating role")

	created := unmarshalResponse(resp.Bytes())

	// Validate that creating the role again fails.
	resp, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "roles"), createPayload(expected))
	assert.Error(t, err, "expected an error creating a duplicate role")
	assert.True(t, trace.IsAlreadyExists(err), "expected an already exists error got %T", err)
	assert.Equal(t, http.StatusConflict, resp.Code(), "unexpected status code creating duplicate role")

	// Update the role.
	created.SetLogins(types.Allow, []string{"test"})
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "roles", expected.GetName()), createPayload(created))
	require.NoError(t, err, "unexpected error updating the role")
	require.Equal(t, http.StatusOK, resp.Code(), "unexpected status code updating the role")

	updated := unmarshalResponse(resp.Bytes())

	require.Empty(t, cmp.Diff(created, updated,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.RoleConditions{}, "Namespaces"),
	))
	require.NotEqual(t, expected.GetLogins(types.Allow), updated.GetLogins(types.Allow), "expected update to modify the logins")
	require.Equal(t, []string{"test"}, updated.GetLogins(types.Allow), "logins should have been updated to test. got %s", updated.GetLogins(types.Allow))

	// Validate that a stale revision prevents updates.
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "roles", expected.GetName()), createPayload(expected))
	assert.Error(t, err, "expected an error updating a role with a stale revision")
	assert.True(t, trace.IsCompareFailed(err), "expected a compare failed error got %T", err)
	assert.Equal(t, http.StatusPreconditionFailed, resp.Code(), "unexpected status code updating the role")

	// Validate that renaming the role prevents updates.
	updated.SetName(uuid.NewString())
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "roles", expected.GetName()), createPayload(updated))
	assert.Error(t, err, "expected and error when renaming a role")
	assert.True(t, trace.IsBadParameter(err), "expected a bad parameter error got %T", err)
	assert.Equal(t, http.StatusBadRequest, resp.Code(), "unexpected status code updating the role")

	// Validate that updating a nonexistent role fails.
	updated.SetName(uuid.NewString())
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "roles", updated.GetName()), createPayload(updated))
	assert.Error(t, err, "expected updating a nonexistent role to fail")
	assert.True(t, trace.IsCompareFailed(err), "expected a compare failed error got %T", err)
	assert.Equal(t, http.StatusPreconditionFailed, resp.Code(), "unexpected status code updating the role")

	// Validate that the role can be deleted
	_, err = pack.clt.Delete(ctx, pack.clt.Endpoint("webapi", "roles", expected.GetName()))
	require.NoError(t, err, "unexpected error deleting role")

	resp, err = pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "roles"), url.Values{"limit": []string{"15"}})
	assert.NoError(t, err, "unexpected error listing role")

	var getResponse listResourcesWithoutCountGetResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &getResponse), "invalid resource item received")
	assert.Equal(t, http.StatusOK, resp.Code(), "unexpected status code getting roles")

	assert.Equal(t, "", getResponse.StartKey)
	for _, item := range getResponse.Items.([]interface{}) {
		assert.NotEqual(t, "test-role", item.(map[string]interface{})["name"], "expected test-role to be deleted")
	}
}

func TestGithubConnectorsCRUD(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	pack := proxy.authPack(t, "test-user@example.com", nil)

	tests := []struct {
		name              string
		connectors        []types.GithubConnector
		setDefaultReq     *ui.SetDefaultAuthConnectorRequest
		wantConnectorName string
		wantConnectorType string
	}{
		{
			name:              "no connectors defaults to local auth",
			connectors:        []types.GithubConnector{},
			wantConnectorName: "",
			wantConnectorType: constants.Local,
		},
		{
			name: "default connector exists in list",
			connectors: []types.GithubConnector{
				makeGithubConnector(t, "github-1"),
			},
			setDefaultReq: &ui.SetDefaultAuthConnectorRequest{
				Name: "github-1",
				Type: constants.Github,
			},
			wantConnectorName: "github-1",
			wantConnectorType: constants.Github,
		},
		{
			name: "default connector missing defaults to last in list",
			connectors: []types.GithubConnector{
				makeGithubConnector(t, "github-1"),
				makeGithubConnector(t, "github-2"),
			},
			setDefaultReq: &ui.SetDefaultAuthConnectorRequest{
				Name: "missing",
				Type: constants.Github,
			},
			wantConnectorName: "github-2",
			wantConnectorType: constants.Github,
		},
		{
			name: "local auth type always defaults to local",
			connectors: []types.GithubConnector{
				makeGithubConnector(t, "github-1"),
			},
			setDefaultReq: &ui.SetDefaultAuthConnectorRequest{
				Name: "local",
				Type: constants.Local,
			},
			wantConnectorName: "",
			wantConnectorType: constants.Local,
		},
		{
			name:       "missing default with no connectors defaults to local",
			connectors: []types.GithubConnector{},
			setDefaultReq: &ui.SetDefaultAuthConnectorRequest{
				Name: "missing",
				Type: constants.Github,
			},
			wantConnectorName: "",
			wantConnectorType: constants.Local,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup initial connectors
			for _, conn := range tt.connectors {
				raw, err := services.MarshalGithubConnector(conn)
				require.NoError(t, err)
				resp, err := pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "github"), ui.ResourceItem{
					Kind:    types.KindGithubConnector,
					Name:    conn.GetName(),
					Content: string(raw),
				})
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
			}

			// Set default connector if specified
			if tt.setDefaultReq != nil {
				resp, err := pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "authconnector", "default"), tt.setDefaultReq)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
			}

			// Get connectors and verify response
			resp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "github"), url.Values{})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var connResponse ui.ListAuthConnectorsResponse
			err = json.Unmarshal(resp.Bytes(), &connResponse)
			require.NoError(t, err)

			// Verify connector name and type
			assert.Equal(t, tt.wantConnectorName, connResponse.DefaultConnectorName)
			assert.Equal(t, tt.wantConnectorType, connResponse.DefaultConnectorType)

			// Verify connectors list
			require.Equal(t, len(tt.connectors), len(connResponse.Connectors))
			for i, conn := range tt.connectors {
				expectedItem, err := ui.NewResourceItem(conn)
				require.NoError(t, err)
				require.Equal(t, expectedItem.Name, connResponse.Connectors[i].Name)
			}

			// Cleanup connectors
			for _, conn := range tt.connectors {
				_, err := pack.clt.Delete(ctx, pack.clt.Endpoint("webapi", "github", conn.GetName()))
				require.NoError(t, err)
			}
		})
	}
}

func TestGetTrustedClusters(t *testing.T) {
	ctx := context.Background()
	m := &mockedResourceAPIGetter{}

	m.mockGetTrustedClusters = func(ctx context.Context) ([]types.TrustedCluster, error) {
		cluster, err := types.NewTrustedCluster("test", types.TrustedClusterSpecV2{})
		require.NoError(t, err)

		return []types.TrustedCluster{cluster}, nil
	}

	// Test response is converted to ui objects.
	tcs, err := getTrustedClusters(ctx, m)
	require.NoError(t, err)
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

			_, err = convertListResourcesRequest(httpReq, types.KindNode)
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
	mockListRoles             func(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error)
	mockUpsertRole            func(ctx context.Context, role types.Role) (types.Role, error)
	mockGetGithubConnectors   func(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	mockGetGithubConnector    func(ctx context.Context, id string, withSecrets bool) (types.GithubConnector, error)
	mockDeleteGithubConnector func(ctx context.Context, id string) error
	mockUpsertTrustedCluster  func(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error)
	mockGetTrustedCluster     func(ctx context.Context, name string) (types.TrustedCluster, error)
	mockGetTrustedClusters    func(ctx context.Context) ([]types.TrustedCluster, error)
	mockDeleteTrustedCluster  func(ctx context.Context, name string) error
	mockListResources         func(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
	mockGetAuthPreference     func(ctx context.Context) (types.AuthPreference, error)
	mockUpsertAuthPreference  func(ctx context.Context, pref types.AuthPreference) (types.AuthPreference, error)
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

func (m *mockedResourceAPIGetter) ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error) {
	if m.mockListRoles != nil {
		return m.mockListRoles(ctx, req)
	}
	return nil, trace.NotImplemented("mockListRoles not implemented")
}

func (m *mockedResourceAPIGetter) UpsertRole(ctx context.Context, role types.Role) (types.Role, error) {
	if m.mockUpsertRole != nil {
		return m.mockUpsertRole(ctx, role)
	}

	return nil, trace.NotImplemented("mockUpsertRole not implemented")
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

// Add new mock methods
func (m *mockedResourceAPIGetter) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	if m.mockGetAuthPreference != nil {
		return m.mockGetAuthPreference(ctx)
	}
	return nil, trace.NotImplemented("mockGetAuthPreference not implemented")
}

func (m *mockedResourceAPIGetter) UpsertAuthPreference(ctx context.Context, pref types.AuthPreference) (types.AuthPreference, error) {
	if m.mockUpsertAuthPreference != nil {
		return m.mockUpsertAuthPreference(ctx, pref)
	}
	return nil, trace.NotImplemented("mockUpsertAuthPreference not implemented")
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

func makeGithubConnector(t *testing.T, name string) types.GithubConnector {
	connector, err := types.NewGithubConnector(name, types.GithubConnectorSpecV3{
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "octocats",
				Team:         "dummy",
				Roles:        []string{"dummy"},
			},
		},
	})
	require.NoError(t, err)
	return connector
}
