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

func TestYAMLParse_Valid(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test@example.com", nil)

	endpoint := pack.clt.Endpoint("webapi", "yaml", "parse", types.KindAccessMonitoringRule)
	re, err := pack.clt.PostJSON(context.Background(), endpoint, yamlParseRequest{
		YAML: validAccessMonitoringRuleYaml,
	})
	require.NoError(t, err)

	var endpointResp yamlParseResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &endpointResp))

	expectedResource := getAccessMonitoringRuleResource()

	// Can't cast a unmarshaled interface{} into the expected type, so
	// we are transforming the expected type to the same type as the
	// one we got as a response.
	b, err := json.Marshal(yamlParseResponse{Resource: expectedResource})
	require.NoError(t, err)
	var expectedResp yamlParseResponse
	require.NoError(t, json.Unmarshal(b, &expectedResp))

	require.Equal(t, expectedResp.Resource, endpointResp.Resource)
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
