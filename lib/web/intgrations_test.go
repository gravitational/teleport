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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestIntegrationsCreateWithAudience(t *testing.T) {
	t.Parallel()
	wPack := newWebPack(t, 1 /* proxies */)
	proxy := wPack.proxies[0]
	authPack := proxy.authPack(t, "user", []types.Role{services.NewPresetEditorRole()})
	ctx := context.Background()

	const integrationName = "test-integration"
	cases := []struct {
		name     string
		audience string
	}{
		{
			name:     "without audiences",
			audience: types.IntegrationAWSOIDCAudienceUnspecified,
		},
		{
			name:     "with audiences",
			audience: types.IntegrationAWSOIDCAudienceAWSIdentityCenter,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			createData := ui.Integration{
				Name:    integrationName,
				SubKind: "aws-oidc",
				AWSOIDC: &ui.IntegrationAWSOIDCSpec{
					RoleARN:  "arn:aws:iam::026090554232:role/testrole",
					Audience: test.audience,
				},
			}
			createEndpoint := authPack.clt.Endpoint("webapi", "sites", wPack.server.ClusterName(), "integrations")
			createResp, err := authPack.clt.PostJSON(ctx, createEndpoint, createData)
			require.NoError(t, err)
			require.Equal(t, 200, createResp.Code())

			// check origin label stored in backend
			intgrationResource, err := wPack.server.Auth().GetIntegration(ctx, integrationName)
			require.NoError(t, err)
			require.Equal(t, test.audience, intgrationResource.GetAWSOIDCIntegrationSpec().Audience)

			// check origin label returned in the web api
			getEndpoint := authPack.clt.Endpoint("webapi", "sites", wPack.server.ClusterName(), "integrations", integrationName)
			getResp, err := authPack.clt.Get(ctx, getEndpoint, nil)
			require.NoError(t, err)
			require.Equal(t, 200, getResp.Code())

			var resp ui.Integration
			err = json.Unmarshal(getResp.Bytes(), &resp)
			require.NoError(t, err)
			require.Equal(t, createData, resp)

			err = wPack.server.Auth().DeleteIntegration(ctx, integrationName)
			require.NoError(t, err)
		})
	}
}
