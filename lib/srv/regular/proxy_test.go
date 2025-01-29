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

package regular

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/srv"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestParseProxyRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc, req string
		expected  proxySubsysRequest
	}{
		{
			desc: "proxy request for a host:port",
			req:  "proxy:host:22",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "host",
				port:        "22",
				clusterName: "",
			},
		},
		{
			desc: "similar request, just with '@' at the end (missing site)",
			req:  "proxy:host:22@",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "host",
				port:        "22",
				clusterName: "",
			},
		},
		{
			desc: "proxy request for just the sitename",
			req:  "proxy:@moon",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "",
				port:        "",
				clusterName: "moon",
			},
		},
		{
			desc: "proxy request for the host:port@sitename",
			req:  "proxy:station:100@moon",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "station",
				port:        "100",
				clusterName: "moon",
			},
		},
		{
			desc: "proxy request for the host:port@namespace@cluster",
			req:  "proxy:station:100@system@moon",
			expected: proxySubsysRequest{
				namespace:   "system",
				host:        "station",
				port:        "100",
				clusterName: "moon",
			},
		},
	}

	server := &Server{
		hostname:  "redhorse",
		proxyMode: true,
		logger:    slog.New(logutils.DiscardHandler{}),
	}

	for i, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			if tt.expected.namespace == "" {
				// test cases without a defined namespace are testing for
				// the presence of the default namespace; namespace should
				// never actually be empty.
				tt.expected.namespace = apidefaults.Namespace
			}
			req, err := server.parseProxySubsysRequest(context.Background(), tt.req)
			require.NoError(t, err, "Test case %d: req=%s, expected=%+v", i, tt.req, tt.expected)
			require.Equal(t, tt.expected, req, "Test case %d: req=%s, expected=%+v", i, tt.req, tt.expected)
		})
	}
}

func TestParseBadRequests(t *testing.T) {
	t.Parallel()

	server := &Server{
		hostname:  "redhorse",
		proxyMode: true,
		logger:    slog.New(logutils.DiscardHandler{}),
	}

	ctx := &srv.ServerContext{}

	testCases := []struct {
		desc  string
		input string
	}{
		{desc: "empty request", input: "proxy:"},
		{desc: "missing hostname", input: "proxy::80"},
		{desc: "missing hostname and missing cluster name", input: "proxy:@"},
		{desc: "just random string", input: "this is bad string"},
	}
	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			subsystem, err := server.parseProxySubsys(context.Background(), tt.input, ctx)
			require.Error(t, err, "test case: %q", tt.input)
			require.Nil(t, subsystem, "test case: %q", tt.input)
		})
	}
}
