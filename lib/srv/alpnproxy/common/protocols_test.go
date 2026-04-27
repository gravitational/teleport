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
	"slices"
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

// TestOrderedProtocols verifies that OrderedProtocols returns the same
// set of protocols as SupportedProtocols, with HTTP/HTTP2 ordered
// according to the prioritizeHTTP2 flag.
func TestOrderedProtocols(t *testing.T) {
	t.Run("default order keeps http/1.1 before h2", func(t *testing.T) {
		got := OrderedProtocols(false)
		httpIdx := slices.Index(got, ProtocolHTTP)
		http2Idx := slices.Index(got, ProtocolHTTP2)
		require.NotEqual(t, -1, httpIdx, "ProtocolHTTP missing from result")
		require.NotEqual(t, -1, http2Idx, "ProtocolHTTP2 missing from result")
		require.Less(t, httpIdx, http2Idx,
			"http/1.1 must come before h2 when prioritizeHTTP2 is false")
	})

	t.Run("prioritized order puts h2 before http/1.1", func(t *testing.T) {
		got := OrderedProtocols(true)
		httpIdx := slices.Index(got, ProtocolHTTP)
		http2Idx := slices.Index(got, ProtocolHTTP2)
		require.NotEqual(t, -1, httpIdx, "ProtocolHTTP missing from result")
		require.NotEqual(t, -1, http2Idx, "ProtocolHTTP2 missing from result")
		require.Less(t, http2Idx, httpIdx,
			"h2 must come before http/1.1 when prioritizeHTTP2 is true")
	})

	t.Run("set of protocols is preserved regardless of order", func(t *testing.T) {
		require.ElementsMatch(t, SupportedProtocols, OrderedProtocols(false),
			"OrderedProtocols(false) must contain the same protocols as SupportedProtocols")
		require.ElementsMatch(t, SupportedProtocols, OrderedProtocols(true),
			"OrderedProtocols(true) must contain the same protocols as SupportedProtocols")
	})
}
