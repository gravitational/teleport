/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

const validAccessMonitoringRuleYaml = `kind: access_monitoring_rule
metadata:
  name: foo
spec:
  condition: some-condition
  notification:
    name: mattermost
    recipients:
    - apple
  subjects:
  - access_request
version: v1
`

const validTokenYaml = `kind: token
metadata:
  name: test-name
spec:
  github:
    enterprise_server_host: test-server-host
    static_jwks: test-twks
    allow:
    - actor: test-actor
      environment: test-environment
      ref: test-ref
      ref_type: test-ref-type
      repository: test-repository
      repository_owner: test-owner
      workflow: test-workflow
      sub: test-sub
  join_method: github
  roles:
  - Node
version: v2
`

func getAccessMonitoringRuleResource() *accessmonitoringrulesv1.AccessMonitoringRule {
	return &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "foo",
		},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:  []string{types.KindAccessRequest},
			Condition: "some-condition",
			Notification: &accessmonitoringrulesv1.Notification{
				Name:       "mattermost",
				Recipients: []string{"apple"},
			},
		},
	}
}

func getTokenResource() *types.ProvisionTokenV2 {
	return &types.ProvisionTokenV2{
		Kind:    types.KindToken,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "test-name",
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleNode},
			JoinMethod: types.JoinMethodGitHub,
			GitHub: &types.ProvisionTokenSpecV2GitHub{
				EnterpriseServerHost: "test-server-host",
				StaticJWKS:           "test-twks",
				Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
					{
						Sub:             "test-sub",
						Repository:      "test-repository",
						RepositoryOwner: "test-owner",
						Workflow:        "test-workflow",
						Environment:     "test-environment",
						Actor:           "test-actor",
						Ref:             "test-ref",
						RefType:         "test-ref-type",
					},
				},
			},
		},
	}
}

func TestYAMLParse_Valid(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test@example.com", nil)

	testCases := []struct {
		name     string
		kind     string
		yaml     string
		expected any
	}{
		{
			name:     "AccessMonitoringRule",
			kind:     types.KindAccessMonitoringRule,
			yaml:     validAccessMonitoringRuleYaml,
			expected: getAccessMonitoringRuleResource(),
		},
		{
			name:     "Token",
			kind:     types.KindToken,
			yaml:     validTokenYaml,
			expected: getTokenResource(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := pack.clt.Endpoint("webapi", "yaml", "parse", tc.kind)
			re, err := pack.clt.PostJSON(context.Background(), endpoint, yamlParseRequest{
				YAML: tc.yaml,
			})
			require.NoError(t, err)

			var endpointResp yamlParseResponse
			require.NoError(t, json.Unmarshal(re.Bytes(), &endpointResp))

			// Can't cast a unmarshaled interface{} into the expected type, so
			// we are transforming the expected type to the same type as the
			// one we got as a response.
			b, err := json.Marshal(yamlParseResponse{Resource: tc.expected})
			require.NoError(t, err)
			var expectedResp yamlParseResponse
			require.NoError(t, json.Unmarshal(b, &expectedResp))

			require.Empty(t, cmp.Diff(expectedResp.Resource, endpointResp.Resource))
		})
	}
}

func TestYAMLParse_Errors(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test@example.com", nil)

	testCases := []struct {
		desc string
		yaml string
		kind string
	}{
		{
			desc: "unsupported kind",
			yaml: validAccessMonitoringRuleYaml,
			kind: "something-random",
		},
		{
			desc: "missing kind",
			yaml: validAccessMonitoringRuleYaml,
			kind: "",
		},
		{
			desc: "invalid yaml",
			yaml: "// 232@#$",
			kind: types.KindAccessMonitoringRule,
		},
		{
			desc: "invalid empty yaml",
			yaml: "",
			kind: types.KindAccessMonitoringRule,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			endpoint := pack.clt.Endpoint("webapi", "yaml", "parse", tc.kind)
			_, err := pack.clt.PostJSON(context.Background(), endpoint, yamlParseRequest{
				YAML: tc.yaml,
			})

			require.Error(t, err)
		})
	}
}

func TestYAMLStringify_Valid(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test@example.com", nil)

	req := struct {
		Resource *accessmonitoringrulesv1.AccessMonitoringRule `json:"resource"`
	}{
		Resource: getAccessMonitoringRuleResource(),
	}

	endpoint := pack.clt.Endpoint("webapi", "yaml", "stringify", types.KindAccessMonitoringRule)
	re, err := pack.clt.PostJSON(context.Background(), endpoint, req)
	require.NoError(t, err)

	var resp yamlStringifyResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Equal(t, validAccessMonitoringRuleYaml, resp.YAML)
}

func TestYAMLStringify_Errors(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test@example.com", nil)

	testCases := []struct {
		desc string
		kind string
	}{
		{
			desc: "unsupported kind",
			kind: "something-random",
		},
		{
			desc: "missing kind",
			kind: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			endpoint := pack.clt.Endpoint("webapi", "yaml", "stringify", tc.kind)
			_, err := pack.clt.PostJSON(context.Background(), endpoint, struct {
				Resource *accessmonitoringrulesv1.AccessMonitoringRule `json:"resource"`
			}{
				Resource: getAccessMonitoringRuleResource(),
			})

			require.Error(t, err)
		})
	}
}
