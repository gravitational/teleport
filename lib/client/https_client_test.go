/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPTransportProxy(t *testing.T) {
	proxyURL := "proxy.example.com"
	target := "target.example.com"
	tests := []struct {
		name             string
		env              map[string]string
		expectedProxyURL string
	}{
		{
			name: "use http proxy",
			env: map[string]string{
				"HTTPS_PROXY": proxyURL,
			},
			expectedProxyURL: "http://" + proxyURL,
		},
		{
			name: "ignore proxy when no_proxy is set",
			env: map[string]string{
				"HTTPS_PROXY": proxyURL,
				"NO_PROXY":    target,
			},
			expectedProxyURL: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			inputURL, err := url.Parse("https://" + target)
			require.NoError(t, err)
			outputURL, err := httpTransport(false, nil).Proxy(&http.Request{
				URL: inputURL,
			})
			require.NoError(t, err)
			if tc.expectedProxyURL != "" {
				require.NotNil(t, outputURL)
				require.Equal(t, tc.expectedProxyURL, outputURL.String())
			} else {
				require.Nil(t, outputURL)
			}
		})
	}
}
