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

package reversetunnel

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// discoveryRequestRaw is the legacy type that was used
// as the payload for discoveryRequests. It exists
// here for the sake of ensuring backward compatibility.
type discoveryRequestRaw struct {
	ClusterName string            `json:"cluster_name"`
	Type        string            `json:"type"`
	Proxies     []json.RawMessage `json:"proxies"`
}

// marshalDiscoveryRequest is the legacy method of marshaling a discoveryRequest
func marshalDiscoveryRequest(proxies []types.Server) ([]byte, error) {
	out := discoveryRequestRaw{
		Proxies: make([]json.RawMessage, 0, len(proxies)),
	}
	for _, p := range proxies {
		// Clone the server value to avoid a potential race
		// since the proxies are shared.
		// Marshaling attempts to enforce defaults which modifies
		// the original value.
		p = p.DeepCopy()
		data, err := services.MarshalServer(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out.Proxies = append(out.Proxies, data)
	}

	return json.Marshal(out)
}

// unmarshalDiscoveryRequest exercises the legacy method of unmarshaling a
// discoveryRequest, returning a slice with the names of the unmarshaled
// types.Server resources.
func unmarshalDiscoveryRequest(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing payload in discovery request")
	}

	var raw discoveryRequestRaw
	if err := utils.FastUnmarshal(data, &raw); err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]string, 0, len(raw.Proxies))
	for _, bytes := range raw.Proxies {
		var v struct {
			Version string `json:"version"`
		}
		if err := utils.FastUnmarshal(bytes, &v); err != nil {
			return nil, trace.Wrap(err)
		}

		if v.Version != types.V2 {
			return nil, trace.BadParameter("server resource version %q is not supported", v.Version)
		}

		proxy, err := services.UnmarshalServer(bytes, types.KindProxy)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, proxy.GetName())
	}

	return out, nil
}

func TestDiscoveryRequestMarshalling(t *testing.T) {
	const proxyCount = 10

	// prepare some random proxies for the discovery request
	proxies := make([]types.Server, 0, proxyCount)
	for range proxyCount {
		p, err := types.NewServer(uuid.New().String(), types.KindProxy, types.ServerSpecV2{})
		require.NoError(t, err)
		proxies = append(proxies, p)
	}

	// create the request
	var req discoveryRequest
	req.SetProxies(proxies)

	// test marshaling the request with the legacy mechanism and unmarshaling
	// with the new mechanism
	t.Run("marshal=legacy unmarshal=new", func(t *testing.T) {
		payload, err := marshalDiscoveryRequest(proxies)
		require.NoError(t, err)

		var got discoveryRequest
		require.NoError(t, json.Unmarshal(payload, &got))

		require.Empty(t, cmp.Diff(req.ProxyNames(), got.ProxyNames()))
	})

	// test marshaling the request with the new mechanism and unmarshaling
	// with the legacy mechanism
	t.Run("marshal=new unmarshal=legacy", func(t *testing.T) {
		payload, err := json.Marshal(req)
		require.NoError(t, err)

		got, err := unmarshalDiscoveryRequest(payload)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(req.ProxyNames(), got))
	})

	// test marshaling and unmarshaling the request with the new mechanism
	t.Run("marshal=new unmarshal=new", func(t *testing.T) {
		payload, err := json.Marshal(req)
		require.NoError(t, err)

		var got discoveryRequest
		require.NoError(t, json.Unmarshal(payload, &got))

		require.Empty(t, cmp.Diff(req.ProxyNames(), got.ProxyNames()))
	})

	// test marshaling and unmarshaling the request with the legacy mechanism
	t.Run("marshal=legacy unmarshal=legacy", func(t *testing.T) {
		payload, err := marshalDiscoveryRequest(proxies)
		require.NoError(t, err)

		got, err := unmarshalDiscoveryRequest(payload)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(req.ProxyNames(), got))
	})
}

func TestTrackProxiesPreservesTTL(t *testing.T) {
	req := discoveryRequest{
		Proxies: []discoveryProxy{
			{
				Version:              types.V2,
				ProxyGroupID:         "group-a",
				ProxyGroupGeneration: 7,
				TTL:                  42 * time.Second,
			},
		},
	}
	req.Proxies[0].Metadata.Name = "proxy-a"

	got := req.TrackProxies()
	require.Len(t, got, 1)
	require.Equal(t, "proxy-a", got[0].Name)
	require.Equal(t, "group-a", got[0].Group)
	require.Equal(t, uint64(7), got[0].Generation)
	require.Equal(t, 42*time.Second, got[0].TTL)
}

func TestProxyPubSub(t *testing.T) {
	type proxy struct {
		name         string
		expiryOffset time.Duration
	}
	type update struct {
		expiryAdvance time.Duration
		update        []proxy
		wantGet       []string
		wantAll       []string
	}
	tests := []struct {
		name           string
		compact        bool
		initial        []proxy
		wantInitialGet []string
		updates        []update
	}{
		{
			name:    "initial state triggers wait and get",
			compact: false,
			initial: []proxy{
				{name: "proxy-a"},
				{name: "proxy-b"},
			},
			wantInitialGet: []string{"proxy-a", "proxy-b"},
		},
		{
			name:    "default get returns full proxy set",
			compact: false,
			initial: []proxy{
				{name: "proxy-a"},
				{name: "proxy-b"},
			},
			wantInitialGet: []string{"proxy-a", "proxy-b"},
			updates: []update{
				{
					update: []proxy{
						{name: "proxy-a"},
						{name: "proxy-b"},
					},
					wantGet: []string{"proxy-a", "proxy-b"},
					wantAll: []string{"proxy-a", "proxy-b"},
				},
			},
		},
		{
			name:    "compact get returns only updated proxies",
			compact: true,
			initial: []proxy{
				{name: "proxy-a"},
				{name: "proxy-b"},
			},
			wantInitialGet: []string{"proxy-a", "proxy-b"},
			updates: []update{
				{
					update: []proxy{
						{name: "proxy-a", expiryOffset: time.Second},
						{name: "proxy-b"},
					},
					wantGet: []string{"proxy-a"},
					wantAll: []string{"proxy-a", "proxy-b"},
				},
			},
		},
		{
			name:    "compact get removes expired proxies",
			compact: true,
			initial: []proxy{
				{name: "proxy-a"},
				{name: "proxy-b"},
			},
			wantInitialGet: []string{"proxy-a", "proxy-b"},
			updates: []update{
				{
					expiryAdvance: defaults.ProxyAnnounceTTL() + time.Second,
					update: []proxy{
						{name: "proxy-a", expiryOffset: defaults.ProxyAnnounceTTL() + time.Second},
						{name: "proxy-b"},
					},
					wantGet: []string{"proxy-a"},
					wantAll: []string{"proxy-a"},
				},
			},
		},
		{
			name:    "default get ignores expiry and version",
			compact: false,
			initial: []proxy{
				{name: "proxy-a"},
				{name: "proxy-b"},
			},
			wantInitialGet: []string{"proxy-a", "proxy-b"},
			updates: []update{
				{
					expiryAdvance: defaults.ProxyAnnounceTTL() + time.Second,
					update: []proxy{
						{name: "proxy-a", expiryOffset: defaults.ProxyAnnounceTTL() + time.Second},
						{name: "proxy-b"},
					},
					wantGet: []string{"proxy-a", "proxy-b"},
					wantAll: []string{"proxy-a", "proxy-b"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				clock := clockwork.NewFakeClock()
				baseTime := clock.Now()
				mkServers := func(states []proxy) []types.Server {
					servers := make([]types.Server, 0, len(states))
					for _, state := range states {
						s, err := types.NewServer(state.name, types.KindProxy, types.ServerSpecV2{})
						require.NoError(t, err)
						if state.expiryOffset != 0 {
							s.SetExpiry(baseTime.Add(state.expiryOffset))
						} else {
							s.SetExpiry(baseTime)
						}
						servers = append(servers, s)
					}
					return servers
				}
				proxyNames := func(proxies []discoveryProxy) []string {
					names := make([]string, 0, len(proxies))
					for _, proxy := range proxies {
						names = append(names, proxy.Metadata.Name)
					}
					slices.Sort(names)
					return names
				}
				stateNames := func(states []proxy) []string {
					names := make([]string, 0, len(states))
					for _, state := range states {
						names = append(names, state.name)
					}
					slices.Sort(names)
					return names
				}

				client := &mockLocalClusterClient{
					proxies: mkServers(tt.initial),
				}
				watcher, err := services.NewProxyWatcher(ctx, services.ProxyWatcherConfig{
					ResourceWatcherConfig: services.ResourceWatcherConfig{
						Component: "test",
						Clock:     clock,
						Logger:    logtest.NewLogger(),
						Client:    client,
					},
					ProxyGetter: client,
					ProxiesC:    make(chan []types.Server, len(tt.updates)+1),
				})
				require.NoError(t, err)
				require.NoError(t, watcher.WaitInitialization())

				pb := newDiscoPub(ctx, watcher)
				pb.compact = tt.compact
				t.Cleanup(pb.Close)

				sub := pb.Subscribe()
				t.Cleanup(sub.Close)

				// Wait until discoPub is blocked before continuing
				synctest.Wait()
				select {
				case <-sub.Wait():
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for initial proxy state")
				}

				require.ElementsMatch(t, tt.wantInitialGet, proxyNames(sub.Get()))
				require.ElementsMatch(t, stateNames(tt.initial), proxyNames(sub.GetAll()))

				for _, update := range tt.updates {
					clock.Advance(update.expiryAdvance)
					if len(update.update) > 0 {
						// Wait until discoPub is blocked before continuing
						synctest.Wait()
						watcher.ResourcesC <- mkServers(update.update)

						select {
						case <-sub.Wait():
						case <-time.After(5 * time.Second):
							t.Fatal("timed out waiting for proxy update")
						}
					}

					require.ElementsMatch(t, update.wantGet, proxyNames(sub.Get()))
					require.ElementsMatch(t, update.wantAll, proxyNames(sub.GetAll()))
				}
			})
		})
	}
}
