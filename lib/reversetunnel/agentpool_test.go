/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reversetunnel

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type mockAgent struct {
	Agent
	mockStart    func(ctx context.Context) error
	mockGetState func() AgentState
}

func (m *mockAgent) Start(ctx context.Context) error {
	if m.mockStart != nil {
		return m.mockStart(ctx)
	}
	return trace.NotImplemented("")
}

func (m *mockAgent) GetState() AgentState {
	if m.mockGetState != nil {
		return m.mockGetState()
	}
	return AgentInitial
}

func (m *mockAgent) GetProxyID() (string, bool) {
	return "test-id", true
}

type mockClient struct {
	auth.ClientI
	ssh.Signer
	mockGetClusterNetworkingConfig func(context.Context) (types.ClusterNetworkingConfig, error)
}

func (c *mockClient) GetClusterNetworkingConfig(ctx context.Context, _ ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	if c.mockGetClusterNetworkingConfig != nil {
		return c.mockGetClusterNetworkingConfig(ctx)
	}
	return nil, trace.NotImplemented("")
}

func setupTestAgentPool(t *testing.T) (*AgentPool, *mockClient) {
	client := &mockClient{}

	pool, err := NewAgentPool(context.Background(), AgentPoolConfig{
		Client:       client,
		AccessPoint:  client,
		HostSigner:   client,
		HostUUID:     "test-uuid",
		LocalCluster: "test-cluster",
		Cluster:      "test-cluster",
		Resolver: func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
			return &utils.NetAddr{}, types.ProxyListenerMode_Separate, nil
		},
	})
	require.NoError(t, err)

	pool.backoff, err = retryutils.NewLinear(retryutils.LinearConfig{
		Step: time.Millisecond,
		Max:  time.Millisecond,
	})
	require.NoError(t, err)

	pool.tracker.TrackExpected([]string{"proxy-1", "proxy-2", "proxy-3"}...)
	pool.newAgentFunc = func(ctx context.Context, tracker *track.Tracker, l track.Lease) (Agent, error) {
		agent := &mockAgent{}
		agent.mockStart = func(ctx context.Context) error {
			return nil
		}

		go func() {
			<-pool.ctx.Done()
			agent.mockGetState = func() AgentState {
				return AgentClosed
			}
			callback := pool.getStateCallback(agent)
			callback(AgentClosed)
		}()

		return agent, nil
	}

	return pool, client
}

// TestAgentPoolConnectionCount ensures that an agent pool creates the desired
// number of connections based on the runtime config.
func TestAgentPoolConnectionCount(t *testing.T) {
	pool, client := setupTestAgentPool(t)
	client.mockGetClusterNetworkingConfig = func(ctx context.Context) (types.ClusterNetworkingConfig, error) {
		config := types.DefaultClusterNetworkingConfig()
		config.SetTunnelStrategy(&types.TunnelStrategyV1{
			Strategy: &types.TunnelStrategyV1_ProxyPeering{
				ProxyPeering: types.DefaultProxyPeeringTunnelStrategy(),
			},
		})
		return config, nil
	}

	err := pool.Start()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return pool.active.len() == 1
	}, time.Second*5, time.Millisecond*10, "wait for agent pool")

	require.False(t, pool.isAgentRequired())
	require.Equal(t, pool.Count(), 1)

	pool.Stop()

	pool, client = setupTestAgentPool(t)
	client.mockGetClusterNetworkingConfig = func(ctx context.Context) (types.ClusterNetworkingConfig, error) {
		config := types.DefaultClusterNetworkingConfig()
		config.SetTunnelStrategy(&types.TunnelStrategyV1{
			Strategy: &types.TunnelStrategyV1_AgentMesh{
				AgentMesh: types.DefaultAgentMeshTunnelStrategy(),
			},
		})
		return config, nil
	}

	err = pool.Start()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return pool.Count() == 3
	}, time.Second*5, time.Millisecond*10)

	select {
	case <-pool.tracker.Acquire():
		require.FailNow(t, "expected all leases to be acquired")
	default:
	}

	require.True(t, pool.isAgentRequired())
	require.Equal(t, pool.Count(), 3)

	pool.Stop()
}
