/*
Copyright 2021 Gravitational, Inc.

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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtract(t *testing.T) {
	testCases := []struct {
		addr     string
		hostPort string
		host     string
		port     string
	}{
		{
			addr:     "example.com",
			hostPort: "example.com",
			host:     "example.com",
			port:     "",
		}, {
			addr:     "example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		}, {
			addr:     "http://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		}, {
			addr:     "https://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		}, {
			addr:     "tcp://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		}, {
			addr:     "file://host/path",
			hostPort: "",
			host:     "",
			port:     "",
		}, {
			addr:     "[::]:443",
			hostPort: "[::]:443",
			host:     "::",
			port:     "443",
		}, {
			addr:     "https://example.com:443/path?query=query#fragment",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.addr, func(t *testing.T) {
			hostPort, err := ExtractHostPort(tc.addr)
			// Expect err if expected value is empty
			require.True(t, (tc.hostPort == "") == (err != nil))
			require.Equal(t, tc.hostPort, hostPort)

			host, err := ExtractHost(tc.addr)
			// Expect err if expected value is empty
			require.True(t, (tc.host == "") == (err != nil))
			require.Equal(t, tc.host, host)

			port, err := ExtractPort(tc.addr)
			// Expect err if expected value is empty
			require.True(t, (tc.port == "") == (err != nil))
			require.Equal(t, tc.port, port)
		})
	}
}
