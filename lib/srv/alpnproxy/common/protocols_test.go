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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithPingProtocols(t *testing.T) {
	require.Equal(t,
		[]Protocol{
			"teleport-tcp-ping",
			"teleport-redis-ping",
			"teleport-reversetunnel",
			"teleport-tcp",
			"teleport-redis",
			"h2",
		},
		WithPingProtocols([]Protocol{
			ProtocolReverseTunnel,
			ProtocolTCP,
			ProtocolRedisDB,
			ProtocolReverseTunnel,
			ProtocolHTTP2,
		}),
	)
}

func TestIsDBTLSProtocol(t *testing.T) {
	require.True(t, IsDBTLSProtocol("teleport-redis"))
	require.True(t, IsDBTLSProtocol("teleport-redis-ping"))
	require.False(t, IsDBTLSProtocol("teleport-tcp"))
	require.False(t, IsDBTLSProtocol(""))
}

func TestProtocolToStringsWithPing(t *testing.T) {
	require.Equal(t, []string{"teleport-proxy-grpc-mtls"}, ProtocolToStringsWithPing(ProtocolProxyGRPCSecure))
	require.Equal(t, []string{"teleport-mcp-ping", "teleport-mcp"}, ProtocolToStringsWithPing(ProtocolMCP))
}

// TestReorderHTTPNextProtos covers the per-app HTTP/2 prioritisation
// helper. The function swaps `http/1.1` and `h2` according to the
// flag and leaves all other entries (in particular `acme-tls/1`
// carried for ACME TLS-ALPN-01 challenges) at their existing
// indices.
func TestReorderHTTPNextProtos(t *testing.T) {
	const acmeProto = "acme-tls/1"
	const httpProto = string(ProtocolHTTP)
	const http2Proto = string(ProtocolHTTP2)

	tests := []struct {
		name            string
		input           []string
		prioritizeHTTP2 bool
		want            []string
	}{
		{
			name:            "default order, flag off, no change",
			input:           []string{httpProto, http2Proto},
			prioritizeHTTP2: false,
			want:            []string{httpProto, http2Proto},
		},
		{
			name:            "default order, flag on, h2 first",
			input:           []string{httpProto, http2Proto},
			prioritizeHTTP2: true,
			want:            []string{http2Proto, httpProto},
		},
		{
			name:            "h2 already first, flag off, restored to default",
			input:           []string{http2Proto, httpProto},
			prioritizeHTTP2: false,
			want:            []string{httpProto, http2Proto},
		},
		{
			name:            "acme-tls/1 preserved at index 0, flag on swaps HTTP entries only",
			input:           []string{acmeProto, httpProto, http2Proto},
			prioritizeHTTP2: true,
			want:            []string{acmeProto, http2Proto, httpProto},
		},
		{
			name:            "ACME shipping shape, flag off, unchanged (default)",
			input:           []string{acmeProto, httpProto, http2Proto},
			prioritizeHTTP2: false,
			want:            []string{acmeProto, httpProto, http2Proto},
		},
		{
			name:            "only one HTTP entry present, flag on, unchanged",
			input:           []string{acmeProto, http2Proto},
			prioritizeHTTP2: true,
			want:            []string{acmeProto, http2Proto},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputCopy := append([]string(nil), tc.input...)
			got := ReorderHTTPNextProtos(tc.input, tc.prioritizeHTTP2)
			require.Equal(t, tc.want, got)
			require.Equal(t, inputCopy, tc.input,
				"ReorderHTTPNextProtos must not mutate the input slice")
		})
	}
}
