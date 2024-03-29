/*
 * Copyright 2023 Gravitational, Inc.
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
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
)

func TestGetAccessGraph(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{IdentityGovernanceSecurity: true, Policy: modules.PolicyFeature{Enabled: true}},
	})

	tests := []struct {
		name             string
		features         string
		validation       func(*testing.T, *webSuite)
		assertQueryError require.ErrorAssertionFunc
		grantAccessToTag bool
	}{
		{
			name: "server doesn't support HTTP: user without access graph access",
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.GreaterOrEqual(t, 2, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.Error,
		},
		{
			name: "server doesn't support HTTP: authenticated requests with proper role",
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.GreaterOrEqual(t, 2, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 1, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.NoError,
			grantAccessToTag: true,
		},
		{
			name:     "server supports HTTP: user without access graph access",
			features: features,
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.Equal(t, 1, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 2, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.Error,
		},
		{
			name:     "server supports HTTP: authenticated requests with proper role",
			features: features,
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.Equal(t, 1, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 2, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 1, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.NoError,
			grantAccessToTag: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s := newWebSuite(t, withAccessGraphFeatures(test.features))
			var opts []webSuiteOpts
			if test.grantAccessToTag {
				opts = append(opts, withExtraRules(types.Rule{
					Resources: []string{types.KindAccessGraph},
					Verbs:     []string{types.ActionRead},
				}))
			}
			webPack := s.newAuthWebPack(t, "foo", opts...)

			endpoint := webPack.clt.Endpoint("enterprise", "accessgraph", "static", "features.json")
			_, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
			require.NoError(t, err)

			endpoint = webPack.clt.Endpoint("enterprise", "accessgraph", "query")
			q := url.Values{}
			q.Set("query", "foo")
			_, err = webPack.clt.Get(s.ctx, endpoint, q)
			test.assertQueryError(t, err)

			test.validation(t, s)

		})
	}
}
