// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	authServer := env.server.Auth()

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	tests := []struct {
		name       string
		spec       *types.AuthPreferenceSpecV2
		assertResp func(cap types.AuthPreference, resp *webclient.PingResponse)
	}{
		{
			name: "OK local auth",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOptional,
				U2F: &types.U2F{
					AppID:  "https://example.com",
					Facets: []string{"https://example.com"},
				},
				Webauthn: &types.Webauthn{
					RPID: "example.com",
				},
			},
			assertResp: func(cap types.AuthPreference, resp *webclient.PingResponse) {
				require.Equal(t, cap.GetType(), resp.Auth.Type)
				require.Equal(t, cap.GetSecondFactor(), resp.Auth.SecondFactor)

				u2f, _ := cap.GetU2F()
				require.NotNil(t, resp.Auth.U2F)
				require.Equal(t, u2f.AppID, resp.Auth.U2F.AppID)

				webCfg, _ := cap.GetWebauthn()
				require.NotNil(t, resp.Auth.Webauthn)
				require.Equal(t, webCfg.RPID, resp.Auth.Webauthn.RPID)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cap, err := types.NewAuthPreference(*test.spec)
			require.NoError(t, err)
			require.NoError(t, authServer.SetAuthPreference(ctx, cap))

			resp, err := clt.Get(ctx, clt.Endpoint("webapi", "ping"), url.Values{})
			require.NoError(t, err)
			var pingResp webclient.PingResponse
			require.NoError(t, json.Unmarshal(resp.Bytes(), &pingResp))

			test.assertResp(cap, &pingResp)
		})
	}
}

// TestPing_multiProxyAddr makes sure ping endpoint can be called over any of
// the proxy's configured public addresses.
func TestPing_multiProxyAddr(t *testing.T) {
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	req, err := http.NewRequest(http.MethodGet, proxy.newClient(t).Endpoint("webapi", "ping"), nil)
	require.NoError(t, err)
	// Make sure ping endpoint can be reached over all proxy public addrs.
	for _, proxyAddr := range proxy.handler.handler.cfg.ProxyPublicAddrs {
		req.Host = proxyAddr.Host()
		resp, err := client.NewInsecureWebClient().Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
}
