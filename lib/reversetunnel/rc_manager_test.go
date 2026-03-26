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
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
)

func TestRemoteClusterTunnelManagerSync(t *testing.T) {
	t.Parallel()

	resolverFn := func(addr string) reversetunnelclient.Resolver {
		return func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
			return &utils.NetAddr{
				Addr:        addr,
				AddrNetwork: "tcp",
				Path:        "",
			}, types.ProxyListenerMode_Separate, nil
		}
	}

	var newAgentPoolErr error
	w := &RemoteClusterTunnelManager{
		pools: make(map[remoteClusterKey]*AgentPool),
		newAgentPool: func(ctx context.Context, cfg RemoteClusterTunnelManagerConfig, cluster, addr string) (*AgentPool, error) {
			return &AgentPool{
				AgentPoolConfig: AgentPoolConfig{Cluster: cluster, Resolver: resolverFn(addr)},
				cancel:          func() {},
			}, newAgentPoolErr
		},
	}
	t.Cleanup(func() { require.NoError(t, w.Close()) })

	tests := []struct {
		desc              string
		reverseTunnels    []types.ReverseTunnel
		reverseTunnelsErr error
		newAgentPoolErr   error
		wantPools         map[remoteClusterKey]*AgentPool
		assertErr         require.ErrorAssertionFunc
	}{
		{
			desc:      "no reverse tunnels",
			wantPools: map[remoteClusterKey]*AgentPool{},
			assertErr: require.NoError,
		},
		{
			desc: "one reverse tunnel with one address",
			reverseTunnels: []types.ReverseTunnel{
				mustNewReverseTunnel(t, "cluster-a", []string{"addr-a"}),
			},
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-a")}},
			},
			assertErr: require.NoError,
		},
		{
			desc: "one reverse tunnel added with multiple addresses",
			reverseTunnels: []types.ReverseTunnel{
				mustNewReverseTunnel(t, "cluster-a", []string{"addr-a", "addr-b", "addr-c"}),
			},
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-a")}},
				{cluster: "cluster-a", addr: "addr-b"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-b")}},
				{cluster: "cluster-a", addr: "addr-c"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-c")}},
			},
			assertErr: require.NoError,
		},
		{
			desc: "one reverse tunnel added and one removed",
			reverseTunnels: []types.ReverseTunnel{
				mustNewReverseTunnel(t, "cluster-b", []string{"addr-b"}),
			},
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-b", addr: "addr-b"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-b", Resolver: resolverFn("addr-b")}},
			},
			assertErr: require.NoError,
		},
		{
			desc: "multiple reverse tunnels",
			reverseTunnels: []types.ReverseTunnel{
				mustNewReverseTunnel(t, "cluster-a", []string{"addr-a", "addr-b", "addr-c"}),
				mustNewReverseTunnel(t, "cluster-b", []string{"addr-b"}),
			},
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-a")}},
				{cluster: "cluster-a", addr: "addr-b"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-b")}},
				{cluster: "cluster-a", addr: "addr-c"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-c")}},
				{cluster: "cluster-b", addr: "addr-b"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-b", Resolver: resolverFn("addr-b")}},
			},
			assertErr: require.NoError,
		},
		{
			desc:              "GetReverseTunnels error, keep existing pools",
			reverseTunnelsErr: errors.New("nah"),
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-a")}},
				{cluster: "cluster-a", addr: "addr-b"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-b")}},
				{cluster: "cluster-a", addr: "addr-c"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-c")}},
				{cluster: "cluster-b", addr: "addr-b"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-b", Resolver: resolverFn("addr-b")}},
			},
			assertErr: require.Error,
		},
		{
			desc: "AgentPool creation fails, keep existing pools",
			reverseTunnels: []types.ReverseTunnel{
				mustNewReverseTunnel(t, "cluster-a", []string{"addr-a", "addr-b", "addr-c"}),
				mustNewReverseTunnel(t, "cluster-b", []string{"addr-b"}),
				mustNewReverseTunnel(t, "cluster-c", []string{"addr-c1", "addr-c2"}),
			},
			newAgentPoolErr: errors.New("nah"),
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-a")}},
				{cluster: "cluster-a", addr: "addr-b"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-b")}},
				{cluster: "cluster-a", addr: "addr-c"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-a", Resolver: resolverFn("addr-c")}},
				{cluster: "cluster-b", addr: "addr-b"}: {AgentPoolConfig: AgentPoolConfig{Cluster: "cluster-b", Resolver: resolverFn("addr-b")}},
			},
			assertErr: require.Error,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			w.cfg.AccessPoint = mockAuthClient{
				reverseTunnels:    tt.reverseTunnels,
				reverseTunnelsErr: tt.reverseTunnelsErr,
			}
			newAgentPoolErr = tt.newAgentPoolErr

			err := w.Sync(ctx)
			tt.assertErr(t, err)

			require.Empty(t, cmp.Diff(
				w.pools,
				tt.wantPools,
				// Tweaks to get comparison working with our complex types.
				cmp.AllowUnexported(remoteClusterKey{}),
				cmp.Comparer(func(a, b *AgentPool) bool {
					aAddr, aMode, aErr := a.AgentPoolConfig.Resolver(context.Background())
					bAddr, bMode, bErr := b.AgentPoolConfig.Resolver(context.Background())
					if aAddr != bAddr && aMode != bMode && !errors.Is(bErr, aErr) {
						return false
					}

					// Only check the supplied configs of AgentPools.
					return cmp.Equal(
						a.AgentPoolConfig,
						b.AgentPoolConfig,
						cmpopts.IgnoreFields(AgentPoolConfig{}, "Resolver"))
				}),
			))
		})
	}
}

type fakeRetry struct {
	retryutils.Retry
}

func (fakeRetry) Reset() {}

func (fakeRetry) After() <-chan time.Time {
	return make(chan time.Time)
}

func TestRemoteClusterTunnelManager(t *testing.T) {
	t.Parallel()

	events := make(chan types.Event, 1)

	w := &RemoteClusterTunnelManager{
		pools: make(map[remoteClusterKey]*AgentPool),
		retry: fakeRetry{},
		cfg: RemoteClusterTunnelManagerConfig{
			Logger: slog.New(slog.DiscardHandler),
			AccessPoint: mockAuthClient{
				tunnelEvents: events,
				reverseTunnels: []types.ReverseTunnel{
					mustNewReverseTunnel(t, "cluster-a", []string{"addr-a", "addr-b", "addr-c"}),
					mustNewReverseTunnel(t, "cluster-b", []string{"addr-b"}),
					mustNewReverseTunnel(t, "cluster-c", []string{"addr-c1", "addr-c2"}),
				},
			},
		},
		newAgentPool: func(ctx context.Context, cfg RemoteClusterTunnelManagerConfig, cluster, addr string) (*AgentPool, error) {
			resolverFn := func(addr string) reversetunnelclient.Resolver {
				return func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
					return &utils.NetAddr{
						Addr:        addr,
						AddrNetwork: "tcp",
						Path:        "",
					}, types.ProxyListenerMode_Multiplex, nil
				}
			}

			return &AgentPool{
				AgentPoolConfig: AgentPoolConfig{Cluster: cluster, Resolver: resolverFn(addr)},
				cancel:          func() {},
			}, nil
		},
	}
	t.Cleanup(func() { require.NoError(t, w.Close()) })

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	// Events are queued up in a goroutine to be processed when
	// Run has established its watcher. The last event sent is a
	// bogus event so that we ensure all events that we care about
	// are processed before exiting this goroutine. On completion of the
	// goroutine, the context is canceled which unblocks Run below.
	go func() {
		defer cancel()

		events <- types.Event{
			Type: types.OpInit,
		}

		events <- types.Event{
			Type: types.OpDelete,
			Resource: &types.ResourceHeader{
				Kind:    types.KindReverseTunnel,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "cluster-a",
				},
			},
		}

		events <- types.Event{
			Type:     types.OpPut,
			Resource: mustNewReverseTunnel(t, "cluster-b", []string{"addr-c", "addr-d"}),
		}

		events <- types.Event{
			Type: types.OpUnreliable,
		}

	}()

	w.Run(ctx)

	assert.Len(t, w.pools, 4)
	assert.NotNil(t, w.pools[remoteClusterKey{cluster: "cluster-c", addr: "addr-c1"}])
	assert.NotNil(t, w.pools[remoteClusterKey{cluster: "cluster-c", addr: "addr-c2"}])
	assert.NotNil(t, w.pools[remoteClusterKey{cluster: "cluster-b", addr: "addr-c"}])
	assert.NotNil(t, w.pools[remoteClusterKey{cluster: "cluster-b", addr: "addr-d"}])

}

type fakeWatcher struct {
	done     chan struct{}
	events   chan types.Event
	doneOnce sync.Once
}

func (f *fakeWatcher) Events() <-chan types.Event {
	return f.events
}

func (f *fakeWatcher) Close() error {
	f.doneOnce.Do(func() {
		close(f.done)
		close(f.events)
	})
	return nil
}

func (f *fakeWatcher) Error() error {
	return nil
}

func (f *fakeWatcher) Done() <-chan struct{} {
	return f.done
}

type mockAuthClient struct {
	authclient.ClientI

	reverseTunnels    []types.ReverseTunnel
	reverseTunnelsErr error
	tunnelEvents      chan types.Event
}

func (c mockAuthClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	w := &fakeWatcher{
		done:   make(chan struct{}),
		events: c.tunnelEvents,
	}

	return w, nil
}

func (c mockAuthClient) ListReverseTunnels(
	ctx context.Context, pageSize int, token string,
) ([]types.ReverseTunnel, string, error) {
	return c.reverseTunnels, "", c.reverseTunnelsErr
}

func mustNewReverseTunnel(t *testing.T, clusterName string, dialAddrs []string) types.ReverseTunnel {
	tun, err := types.NewReverseTunnel(clusterName, dialAddrs)
	require.NoError(t, err)
	return tun
}
