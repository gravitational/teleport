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
