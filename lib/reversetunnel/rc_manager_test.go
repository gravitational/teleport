package reversetunnel

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestRemoteClusterTunnelManagerSync(t *testing.T) {
	t.Parallel()

	var newAgentPoolErr error
	w := &RemoteClusterTunnelManager{
		pools: make(map[remoteClusterKey]*AgentPool),
		newAgentPool: func(ctx context.Context, cluster, addr string) (*AgentPool, error) {
			return &AgentPool{
				cfg:    AgentPoolConfig{Cluster: cluster, ProxyAddr: addr},
				cancel: func() {},
			}, newAgentPoolErr
		},
	}
	defer w.Close()

	tests := []struct {
		desc              string
		reverseTunnels    []services.ReverseTunnel
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
			reverseTunnels: []services.ReverseTunnel{
				&services.ReverseTunnelV2{
					Spec: services.ReverseTunnelSpecV2{
						ClusterName: "cluster-a",
						DialAddrs:   []string{"addr-a"},
					},
				},
			},
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-a"}},
			},
			assertErr: require.NoError,
		},
		{
			desc: "one reverse tunnel added with multiple addresses",
			reverseTunnels: []services.ReverseTunnel{
				&services.ReverseTunnelV2{
					Spec: services.ReverseTunnelSpecV2{
						ClusterName: "cluster-a",
						DialAddrs:   []string{"addr-a", "addr-b", "addr-c"},
					},
				},
			},
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-a"}},
				{cluster: "cluster-a", addr: "addr-b"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-b"}},
				{cluster: "cluster-a", addr: "addr-c"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-c"}},
			},
			assertErr: require.NoError,
		},
		{
			desc: "one reverse tunnel added and one removed",
			reverseTunnels: []services.ReverseTunnel{
				&services.ReverseTunnelV2{
					Spec: services.ReverseTunnelSpecV2{
						ClusterName: "cluster-b",
						DialAddrs:   []string{"addr-b"},
					},
				},
			},
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-b", addr: "addr-b"}: {cfg: AgentPoolConfig{Cluster: "cluster-b", ProxyAddr: "addr-b"}},
			},
			assertErr: require.NoError,
		},
		{
			desc: "multiple reverse tunnels",
			reverseTunnels: []services.ReverseTunnel{
				&services.ReverseTunnelV2{
					Spec: services.ReverseTunnelSpecV2{
						ClusterName: "cluster-a",
						DialAddrs:   []string{"addr-a", "addr-b", "addr-c"},
					},
				},
				&services.ReverseTunnelV2{
					Spec: services.ReverseTunnelSpecV2{
						ClusterName: "cluster-b",
						DialAddrs:   []string{"addr-b"},
					},
				},
			},
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-a"}},
				{cluster: "cluster-a", addr: "addr-b"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-b"}},
				{cluster: "cluster-a", addr: "addr-c"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-c"}},
				{cluster: "cluster-b", addr: "addr-b"}: {cfg: AgentPoolConfig{Cluster: "cluster-b", ProxyAddr: "addr-b"}},
			},
			assertErr: require.NoError,
		},
		{
			desc:              "GetReverseTunnels error, keep existing pools",
			reverseTunnelsErr: errors.New("nah"),
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-a"}},
				{cluster: "cluster-a", addr: "addr-b"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-b"}},
				{cluster: "cluster-a", addr: "addr-c"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-c"}},
				{cluster: "cluster-b", addr: "addr-b"}: {cfg: AgentPoolConfig{Cluster: "cluster-b", ProxyAddr: "addr-b"}},
			},
			assertErr: require.Error,
		},
		{
			desc: "AgentPool creation fails, keep existing pools",
			reverseTunnels: []services.ReverseTunnel{
				&services.ReverseTunnelV2{
					Spec: services.ReverseTunnelSpecV2{
						ClusterName: "cluster-a",
						DialAddrs:   []string{"addr-a", "addr-b", "addr-c"},
					},
				},
				&services.ReverseTunnelV2{
					Spec: services.ReverseTunnelSpecV2{
						ClusterName: "cluster-b",
						DialAddrs:   []string{"addr-b"},
					},
				},
				&services.ReverseTunnelV2{
					Spec: services.ReverseTunnelSpecV2{
						ClusterName: "cluster-c",
						DialAddrs:   []string{"addr-c1", "addr-c2"},
					},
				},
			},
			newAgentPoolErr: errors.New("nah"),
			wantPools: map[remoteClusterKey]*AgentPool{
				{cluster: "cluster-a", addr: "addr-a"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-a"}},
				{cluster: "cluster-a", addr: "addr-b"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-b"}},
				{cluster: "cluster-a", addr: "addr-c"}: {cfg: AgentPoolConfig{Cluster: "cluster-a", ProxyAddr: "addr-c"}},
				{cluster: "cluster-b", addr: "addr-b"}: {cfg: AgentPoolConfig{Cluster: "cluster-b", ProxyAddr: "addr-b"}},
			},
			assertErr: require.Error,
		},
	}

	ctx := context.TODO()
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			for _, rt := range tt.reverseTunnels {
				err := rt.CheckAndSetDefaults()
				require.NoError(t, err)
			}
			w.cfg.AuthClient = mockAuthClient{
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
					// Only check the supplied configs of AgentPools.
					return cmp.Equal(a.cfg, b.cfg)
				}),
			))
		})
	}
}

type mockAuthClient struct {
	auth.ClientI

	reverseTunnels    []services.ReverseTunnel
	reverseTunnelsErr error
}

func (c mockAuthClient) GetReverseTunnels(...services.MarshalOption) ([]services.ReverseTunnel, error) {
	return c.reverseTunnels, c.reverseTunnelsErr
}
