// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

//go:build darwin
// +build darwin

package dns

import (
	"context"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

// TestOSUpstreamNameservers configures the DNS server to forward requests for all addresses to the OS's real
// upstream nameservers, to test that this logic is working correctly.
func TestOSUpstreamNameservers(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	resolver := &stubResolver{}
	upstreams, err := NewOSUpstreamNameserverSource()
	require.NoError(t, err)
	server, err := NewServer(resolver, upstreams)
	require.NoError(t, err)

	conn, err := net.ListenUDP("udp", udpLocalhost)
	require.NoError(t, err)

	utils.RunTestBackgroundTask(ctx, t, &utils.TestBackgroundTask{
		Name: "nameserver",
		Task: func(ctx context.Context) error {
			err := server.ListenAndServeUDP(ctx, conn)
			if err == nil || utils.IsOKNetworkError(err) {
				return nil
			}
			return trace.Wrap(err)
		},
		Terminate: conn.Close,
	})

	netResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Always dial the resolver under test.
			return net.Dial(network, conn.LocalAddr().String())
		},
	}

	for _, tc := range []struct {
		host string
	}{
		{"goteleport.com"},
		{"teleport.sh"},
		{"example.com"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			addrs, err := netResolver.LookupHost(ctx, tc.host)
			require.NoError(t, err)
			require.NotEmpty(t, addrs)
		})
	}
}
