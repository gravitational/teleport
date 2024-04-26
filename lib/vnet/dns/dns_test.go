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

package dns

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"

	"github.com/gravitational/teleport/lib/utils"
)

var (
	udpLocalhost = &net.UDPAddr{IP: net.ParseIP("127.0.0.1")}
)

func TestMain(m *testing.M) {
	utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
	os.Exit(m.Run())
}

func runInBackground(ctx context.Context, t *testing.T, name string, task func(context.Context) error) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := trace.Wrap(task(ctx))
		// Only care about the error if the task exited early, which we know is true if the context hasn't
		// been canceled.
		if ctx.Err() == nil {
			t.Errorf("%s exited early with error: %v", name, err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Errorf("%s didn't exit after 2 seconds", name)
		}
	})
}

// TestServer sets up a main DNS server and two upstream DNS servers, all using real UDP sockets, and tests
// that net.Resolver can successfully use the stack to lookup hosts.
func TestServer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	defaultIP4 := tcpip.AddrFrom4([4]byte{1, 2, 3, 4})
	defaultIP6 := tcpip.AddrFrom16([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	staticResolver := &staticResolver{Result{
		A:    defaultIP4.As4(),
		AAAA: defaultIP6.As16(),
	}}
	noUpstreams := &stubUpstreamNamservers{}

	// Create two upstream nameservers that are able to resolve A and AAAA records for all names.
	var upstreamAddrs []string
	for i := 0; i < 2; i++ {
		upstreamServer, err := NewServer(staticResolver, noUpstreams)
		require.NoError(t, err)
		conn, err := net.ListenUDP("udp", udpLocalhost)
		require.NoError(t, err)

		runInBackground(ctx, t, "upstream nameserver", func(ctx context.Context) error {
			return trace.Wrap(upstreamServer.ListenAndServeUDP(ctx, conn))
		})
		t.Cleanup(func() {
			conn.Close()
		})

		upstreamAddrs = append(upstreamAddrs, conn.LocalAddr().String())
	}

	// Create the nameserver under test.
	goTeleportIPv4 := tcpip.AddrFrom4([4]byte{1, 1, 1, 1})
	goTeleportIPv6 := tcpip.AddrFrom16([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
	teleportShIPv6 := tcpip.AddrFrom16([16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2})
	resolver := &stubResolver{
		aRecords: map[string]Result{
			"goteleport.com.": Result{
				A: goTeleportIPv4.As4(),
			},
			"teleport.sh.": Result{
				NoRecord: true,
			},
			"fake.example.com.": Result{
				NXDomain: true,
			},
		},
		aaaaRecords: map[string]Result{
			"goteleport.com.": Result{
				AAAA: goTeleportIPv6.As16(),
			},
			"teleport.sh.": Result{
				AAAA: teleportShIPv6.As16(),
			},
			"fake.example.com.": Result{
				NXDomain: true,
			},
		},
	}
	upstreams := &stubUpstreamNamservers{nameservers: upstreamAddrs}
	server, err := NewServer(resolver, upstreams)
	require.NoError(t, err)

	conn, err := net.ListenUDP("udp", udpLocalhost)
	require.NoError(t, err)

	runInBackground(ctx, t, "nameserver under test", func(ctx context.Context) error {
		return trace.Wrap(server.ListenAndServeUDP(ctx, conn))
	})
	t.Cleanup(func() {
		cancel()
		conn.Close()
	})

	netResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Always dial the resolver under test.
			return net.Dial(network, conn.LocalAddr().String())
		},
	}

	for _, tc := range []struct {
		desc        string
		host        string
		expectAddrs []string
		expectErr   string
	}{
		{
			desc:        "v4 and v6",
			host:        "goteleport.com.",
			expectAddrs: []string{goTeleportIPv4.String(), goTeleportIPv6.String()},
		},
		{
			desc:        "only v6",
			host:        "teleport.sh.",
			expectAddrs: []string{teleportShIPv6.String()},
		},
		{
			desc:        "forward to upstream",
			host:        "example.com.",
			expectAddrs: []string{defaultIP4.String(), defaultIP6.String()},
		},
		{
			desc:      "no domain",
			host:      "fake.example.com.",
			expectErr: "no such host",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			addrs, err := netResolver.LookupHost(ctx, tc.host)
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, tc.expectAddrs, addrs)
		})
	}
}

type stubResolver struct {
	aRecords    map[string]Result
	aaaaRecords map[string]Result
}

func (s *stubResolver) ResolveA(ctx context.Context, fqdn string) (Result, error) {
	return s.aRecords[fqdn], nil
}

func (s *stubResolver) ResolveAAAA(ctx context.Context, fqdn string) (Result, error) {
	return s.aaaaRecords[fqdn], nil
}

type staticResolver struct {
	result Result
}

func (s *staticResolver) ResolveA(ctx context.Context, fqdn string) (Result, error) {
	return s.result, nil
}

func (s *staticResolver) ResolveAAAA(ctx context.Context, fqdn string) (Result, error) {
	return s.result, nil
}

type stubUpstreamNamservers struct {
	nameservers []string
	err         error
}

func (s *stubUpstreamNamservers) UpstreamNameservers(ctx context.Context) ([]string, error) {
	return s.nameservers, s.err
}
