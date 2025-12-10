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

func TestAccessMonitoringRuleGenerateTerraform_Valid(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test@example.com", nil)

	req := accessMonitoringRuleGenerateTerraformRequest{
		Resource: getAccessMonitoringRuleResource(),
	}

	endpoint := pack.clt.Endpoint("webapi", "access-monitoring-rule", "terraform")
	re, err := pack.clt.PostJSON(context.Background(), endpoint, req)
	require.NoError(t, err)

	var resp accessMonitoringRuleGenerateTerraformResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Equal(t, validAccessMonitoringRuleTerraform, resp.Terraform)
}
