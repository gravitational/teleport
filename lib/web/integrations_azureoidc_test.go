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
	"fmt"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestAzureOIDCConfigureScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)

	proxyAddr := env.proxies[0].webURL.String()

	// Unauthenticated client for script downloading.
	publicClt := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"azureoidc.sh",
	}
	endpoint := publicClt.Endpoint(pathVars...)

	tests := []struct {
		name                 string
		reqQuery             url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name: "valid",
			reqQuery: url.Values{
				"authConnectorName": []string{"myconnector"},
				"accessGraph":       []string{"true"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure azure-oidc " +
				`--proxy-public-addr=` + proxyAddr + ` ` +
				`--auth-connector-name=myconnector ` +
				`--access-graph`,
		},
		{
			name: "valid without accessGraph",
			reqQuery: url.Values{
				"authConnectorName": []string{"myconnector"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure azure-oidc " +
				`--proxy-public-addr=` + proxyAddr + ` ` +
				`--auth-connector-name=myconnector`,
		},
		{
			name: "authConnectorName invalid",
			reqQuery: url.Values{
				"integrationName":   []string{"myintegration"},
				"authConnectorName": []string{"myconnector;"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "authConnectorName missing",
			reqQuery: url.Values{
				"integrationName": []string{"myintegration"},
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, endpoint, tc.reqQuery)
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()),
				fmt.Sprintf("entrypointArgs='%s'\n", tc.expectedTeleportArgs),
			)
		})
	}
}
