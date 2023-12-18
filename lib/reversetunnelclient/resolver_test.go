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

package reversetunnelclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func TestStaticResolver(t *testing.T) {
	cases := []struct {
		name             string
		address          string
		mode             types.ProxyListenerMode
		errorAssertionFn require.ErrorAssertionFunc
		expected         *utils.NetAddr
	}{
		{
			name:             "invalid address yields error",
			address:          "",
			errorAssertionFn: require.Error,
		},
		{
			name:             "valid address yields NetAddr",
			address:          "localhost:80",
			errorAssertionFn: require.NoError,
			expected: &utils.NetAddr{
				Addr:        "localhost:80",
				AddrNetwork: "tcp",
				Path:        "",
			},
			mode: types.ProxyListenerMode_Multiplex,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			addr, mode, err := StaticResolver(tt.address, tt.mode)(context.Background())
			tt.errorAssertionFn(t, err)
			if err != nil {
				return
			}

			require.Empty(t, cmp.Diff(tt.expected, addr))
			require.Equal(t, tt.mode, mode)
		})
	}
}

func TestResolveViaWebClient(t *testing.T) {

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(&webclient.PingResponse{})
	}))
	t.Cleanup(srv.Close)

	fakeAddr := utils.NetAddr{}

	cases := []struct {
		name             string
		proxyAddr        utils.NetAddr
		address          string
		errorAssertionFn require.ErrorAssertionFunc
		expected         *utils.NetAddr
	}{
		{
			name:             "no addrs yields errors",
			errorAssertionFn: require.Error,
		},
		{
			name:             "unreachable proxy yields errors",
			proxyAddr:        fakeAddr,
			address:          "",
			errorAssertionFn: require.Error,
		},
		{
			name:             "invalid address yields errors",
			proxyAddr:        fakeAddr,
			address:          "fake://test",
			errorAssertionFn: require.Error,
		},
		{
			name:             "valid address yields NetAddr",
			proxyAddr:        utils.NetAddr{Addr: srv.Listener.Addr().String()},
			address:          "localhost:80",
			errorAssertionFn: require.NoError,
			expected: &utils.NetAddr{
				Addr:        "localhost:80",
				AddrNetwork: "tcp",
				Path:        "",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(defaults.TunnelPublicAddrEnvar, tt.address)
			resolver := WebClientResolver(&webclient.Config{
				Context:   context.Background(),
				ProxyAddr: tt.proxyAddr.String(),
				Insecure:  true,
				Timeout:   time.Second,
			})

			addr, _, err := resolver(context.Background())
			tt.errorAssertionFn(t, err)
			if err != nil {
				return
			}

			require.Empty(t, cmp.Diff(tt.expected, addr))
		})
	}
}

func TestCachingResolver(t *testing.T) {
	randomResolver := func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
		return &utils.NetAddr{
			Addr:        uuid.New().String(),
			AddrNetwork: uuid.New().String(),
			Path:        uuid.New().String(),
		}, types.ProxyListenerMode_Multiplex, nil
	}

	clock := clockwork.NewFakeClock()
	resolver, err := CachingResolver(context.Background(), randomResolver, clock)
	require.NoError(t, err)

	// This is a data race check.
	// We start a goroutine that mutates the underlying NetAddr, but without invalidating the cache.
	// The caching resolver must return a pointer to a copy of the NetAddr to avoid a data race.
	go func() {
		addr, _, _ := resolver(context.Background())
		// data race check: write to *addr
		addr.Addr = ""
	}()

	addr, mode, err := resolver(context.Background())
	require.NoError(t, err)

	addr2, mode2, err := resolver(context.Background())
	require.NoError(t, err)

	// data race check: read from *addr
	require.Equal(t, addr, addr2)
	require.Equal(t, mode, mode2)

	clock.Advance(time.Hour)

	addr3, mode3, err := resolver(context.Background())
	require.NoError(t, err)

	require.NotEqual(t, addr2, addr3)
	require.Equal(t, mode2, mode3)

	addr4, mode4, err := resolver(context.Background())
	require.NoError(t, err)

	require.Equal(t, addr3, addr4)
	require.Equal(t, mode3, mode4)
}
