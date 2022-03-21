// Copyright 2022 Gravitational, Inc
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

package reversetunnel

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestStaticResolver(t *testing.T) {
	cases := []struct {
		name             string
		address          string
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
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := StaticResolver(tt.address)()
			tt.errorAssertionFn(t, err)
			if err != nil {
				return
			}

			require.Empty(t, cmp.Diff(tt.expected, addr))
		})
	}
}

func TestResolveViaWebClient(t *testing.T) {

	fakeAddr := utils.NetAddr{}

	cases := []struct {
		name             string
		addrs            []utils.NetAddr
		address          string
		errorAssertionFn require.ErrorAssertionFunc
		expected         *utils.NetAddr
	}{
		{
			name:             "no addrs yields no results",
			errorAssertionFn: require.NoError,
		},
		{
			name:             "unreachable proxy yields errors",
			addrs:            []utils.NetAddr{fakeAddr},
			address:          "",
			errorAssertionFn: require.Error,
		},
		{
			name:             "invalid address yields errors",
			addrs:            []utils.NetAddr{fakeAddr},
			address:          "fake://test",
			errorAssertionFn: require.Error,
		},
		{
			name:             "valid address yields NetAddr",
			addrs:            []utils.NetAddr{fakeAddr},
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
			os.Setenv(defaults.TunnelPublicAddrEnvar, tt.address)
			t.Cleanup(func() { os.Unsetenv(defaults.TunnelPublicAddrEnvar) })

			addr, err := WebClientResolver(context.Background(), tt.addrs, true)()
			tt.errorAssertionFn(t, err)
			if err != nil {
				return
			}

			require.Empty(t, cmp.Diff(tt.expected, addr))
		})
	}
}

func TestCachingResolver(t *testing.T) {
	randomResolver := func() (*utils.NetAddr, error) {
		return &utils.NetAddr{
			Addr:        uuid.New().String(),
			AddrNetwork: uuid.New().String(),
			Path:        uuid.New().String(),
		}, nil
	}

	clock := clockwork.NewFakeClock()
	resolver, err := CachingResolver(randomResolver, clock)
	require.NoError(t, err)

	addr, err := resolver()
	require.NoError(t, err)

	addr2, err := resolver()
	require.NoError(t, err)

	require.Equal(t, addr, addr2)

	clock.Advance(time.Hour)

	addr3, err := resolver()
	require.NoError(t, err)

	require.NotEqual(t, addr2, addr3)

	addr4, err := resolver()
	require.NoError(t, err)

	require.Equal(t, addr3, addr4)
}
