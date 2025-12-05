/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"github.com/gravitational/teleport/api/types"
)

const validAccessMonitoringRuleTerraform = `resource "teleport_access_monitoring_rule" "foo" {
  version = "v1"

  metadata = {
    name = "foo"
  }

  spec = {
    subjects  = ["access_request"]
    condition = "some-condition"
    notification = {
      name       = "mattermost"
      recipients = ["apple"]
    }
  }
}
`

func TestTerraformStringify_Valid(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test@example.com", nil)

	req := struct {
		Resource *accessmonitoringrulesv1.AccessMonitoringRule `json:"resource"`
	}{
		Resource: getAccessMonitoringRuleResource(),
	}

	endpoint := pack.clt.Endpoint("webapi", "terraform", "stringify", types.KindAccessMonitoringRule)
	re, err := pack.clt.PostJSON(context.Background(), endpoint, req)
	require.NoError(t, err)

	var resp terraformStringifyResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Equal(t, validAccessMonitoringRuleTerraform, resp.Terraform)
}

func TestTerraformStringify_Errors(t *testing.T) {
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
			endpoint := pack.clt.Endpoint("webapi", "terraform", "stringify", tc.kind)
			_, err := pack.clt.PostJSON(context.Background(), endpoint, struct {
				Resource *accessmonitoringrulesv1.AccessMonitoringRule `json:"resource"`
			}{
				Resource: getAccessMonitoringRuleResource(),
			})

			require.Error(t, err)
		})
	}
}
