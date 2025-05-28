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
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestBuildAWSRATrustAnchorConfigureScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)

	// Unauthenticated client for script downloading.
	publicClt := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"awsra-trust-anchor.sh",
	}
	scriptEndpoint := publicClt.Endpoint(pathVars...)

	baseQueryParams := func() url.Values {
		return url.Values{
			"integrationName": []string{"myintegration"},
			"trustAnchor":     []string{"my-trust-anchor"},
			"syncProfile":     []string{"my-profile-for-sync"},
			"syncRole":        []string{"my-role-for-sync"},
		}
	}

	tests := []struct {
		name                 string
		reqRelativeURL       string
		reqQuery             func() url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name:     "valid",
			reqQuery: baseQueryParams,
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure awsra-trust-anchor " +
				"--cluster=localhost " +
				"--name=myintegration " +
				"--trust-anchor=my-trust-anchor " +
				"--sync-profile=my-profile-for-sync " +
				"--sync-role=my-role-for-sync ",
		},
		{
			name: "missing integration name",
			reqQuery: func() url.Values {
				q := baseQueryParams()
				q.Del("integrationName")
				return q
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing trust anchor",
			reqQuery: func() url.Values {
				q := baseQueryParams()
				q.Del("trustAnchor")
				return q
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing sync profile name",
			reqQuery: func() url.Values {
				q := baseQueryParams()
				q.Del("syncProfile")
				return q
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing sync role name",
			reqQuery: func() url.Values {
				q := baseQueryParams()
				q.Del("syncRole")
				return q
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, scriptEndpoint, tc.reqQuery())
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()), "entrypointArgs='"+tc.expectedTeleportArgs)
		})
	}
}
