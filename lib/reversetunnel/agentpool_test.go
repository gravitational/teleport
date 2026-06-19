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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	apissh "github.com/gravitational/teleport/api/ssh"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/utils"
)

type mockAgent struct {
	Agent
	proxyID      string
	rtt          time.Duration
	stopCalls    int
	mockStart    func(ctx context.Context) error
	mockStop     func() error
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
	if m.proxyID == "" {
		return "", false
	}
	return m.proxyID, true
}

func (m *mockAgent) Stop() error {
	m.stopCalls++
	if m.mockStop != nil {
		return m.mockStop()
	}
	return nil
}

func (m *mockAgent) RTT() (time.Duration, bool) {
	if m.rtt == 0 {
		return 0, false
	}
	return m.rtt, true
}

type mockClient struct {
	authclient.ClientI

	mockGetClusterNetworkingConfig func(context.Context) (types.ClusterNetworkingConfig, error)
}

func (c *mockClient) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	if c.mockGetClusterNetworkingConfig != nil {
		return c.mockGetClusterNetworkingConfig(ctx)
	}
	return nil, trace.NotImplemented("")
}

func setupTestAgentPool(t *testing.T) (*AgentPool, *mockClient) {
	client := &mockClient{}

	pool, err := NewAgentPool(context.Background(), AgentPoolConfig{
		Client:      client,
		AccessPoint: client,
		PublicKeyAuth: apissh.PublicKeyAuthConfig{
			Signers: func() ([]ssh.Signer, error) {
				return []ssh.Signer{mockSigner{}}, nil
			},
		},
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

	pool.tracker.TrackExpected(
		track.Proxy{Name: "proxy-1"},
		track.Proxy{Name: "proxy-2"},
		track.Proxy{Name: "proxy-3"},
	)
	pool.newAgentFunc = func(ctx context.Context, tracker *track.Tracker, l *track.Lease) (Agent, error) {
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

	require.NoError(t, pool.Start())
	t.Cleanup(pool.Stop)

	require.Eventually(t, func() bool {
		return pool.active.len() == 1
	}, time.Second*5, time.Millisecond*10, "wait for agent pool")

	require.Nil(t, pool.tracker.TryAcquire())
	require.Equal(t, 1, pool.Count())

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

	require.NoError(t, pool.Start())
	t.Cleanup(pool.Stop)

	require.Eventually(t, func() bool {
		return pool.Count() == 3
	}, time.Second*5, time.Millisecond*10)

	require.Nil(t, pool.tracker.TryAcquire())
	require.Equal(t, 3, pool.Count())
}

type mockSigner struct {
	ssh.Signer
}

func TestAgentPoolTryDisconnect(t *testing.T) {
	const disconnectThreshold = time.Minute
	newTestPool := func(t *testing.T) *AgentPool {
		client := &mockClient{}
		pool, err := NewAgentPool(context.Background(), AgentPoolConfig{
			Client:      client,
			AccessPoint: client,
			PublicKeyAuth: apissh.PublicKeyAuthConfig{
				Signers: func() ([]ssh.Signer, error) {
					return []ssh.Signer{mockSigner{}}, nil
				},
			},
			HostUUID:     "test-uuid",
			LocalCluster: "test-cluster",
			Cluster:      "test-cluster",
			Resolver: func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
				return &utils.NetAddr{}, types.ProxyListenerMode_Separate, nil
			},
		})
		require.NoError(t, err)
		pool.runtimeConfig.disconnectThreshold = disconnectThreshold
		pool.lastConnectivityChange = time.Now()
		return pool
	}

	trackedProxiesForAgents := func(agents []*mockAgent) []track.Proxy {
		proxies := make([]track.Proxy, 0, len(agents))
		for _, agent := range agents {
			if agent.proxyID == "" {
				continue
			}
			proxies = append(proxies, track.Proxy{Name: agent.proxyID})
		}
		return proxies
	}

	type testUpdate struct {
		apply  func(*testing.T, *AgentPool)
		assert func(*testing.T, *AgentPool, []*mockAgent)
	}

	tests := []struct {
		name   string
		agents []*mockAgent
		update []testUpdate
	}{
		{
			name: "wait for all agents to report rtt",
			agents: []*mockAgent{
				{proxyID: "proxy-1", rtt: 10 * time.Millisecond},
				{proxyID: "proxy-2"},
				{proxyID: "proxy-3", rtt: 30 * time.Millisecond},
			},
			update: []testUpdate{
				{
					apply: func(t *testing.T, pool *AgentPool) {
						pool.tracker.SetConnectionCount(2)
						time.Sleep(disconnectThreshold + time.Second)
					},
					assert: func(t *testing.T, pool *AgentPool, agents []*mockAgent) {
						require.Zero(t, agents[0].stopCalls)
						require.Zero(t, agents[1].stopCalls)
						require.Zero(t, agents[2].stopCalls)
						require.NotEqual(t, time.Now(), pool.lastConnectivityChange)
					},
				},
			},
		},
		{
			name: "disconnect agent with highest rtt",
			agents: []*mockAgent{
				{proxyID: "proxy-1", rtt: 10 * time.Millisecond},
				{proxyID: "proxy-2", rtt: 70 * time.Millisecond},
				{proxyID: "proxy-3", rtt: 30 * time.Millisecond},
			},
			update: []testUpdate{
				{
					apply: func(t *testing.T, pool *AgentPool) {
						pool.tracker.SetConnectionCount(2)
						time.Sleep(disconnectThreshold + time.Second)
					},
					assert: func(t *testing.T, pool *AgentPool, agents []*mockAgent) {
						require.Zero(t, agents[0].stopCalls)
						require.Equal(t, 1, agents[1].stopCalls)
						require.Zero(t, agents[2].stopCalls)
						require.Equal(t, time.Now(), pool.lastConnectivityChange)
					},
				},
			},
		},
		{
			name: "wait for disconnect threshold after last connectivity change",
			agents: []*mockAgent{
				{proxyID: "proxy-1", rtt: 10 * time.Millisecond},
				{proxyID: "proxy-2", rtt: 70 * time.Millisecond},
				{proxyID: "proxy-3", rtt: 30 * time.Millisecond},
			},
			update: []testUpdate{
				{
					apply: func(t *testing.T, pool *AgentPool) {
						pool.tracker.SetConnectionCount(2)
						time.Sleep(disconnectThreshold + time.Second)
						pool.lastConnectivityChange = time.Now()
					},
					assert: func(t *testing.T, pool *AgentPool, agents []*mockAgent) {
						require.Zero(t, agents[0].stopCalls)
						require.Zero(t, agents[1].stopCalls)
						require.Zero(t, agents[2].stopCalls)
						require.Equal(t, time.Now(), pool.lastConnectivityChange)
					},
				},
				{
					apply: func(t *testing.T, pool *AgentPool) {
						time.Sleep(disconnectThreshold + time.Second)
					},
					assert: func(t *testing.T, pool *AgentPool, agents []*mockAgent) {
						require.Zero(t, agents[0].stopCalls)
						require.Equal(t, 1, agents[1].stopCalls)
						require.Zero(t, agents[2].stopCalls)
						require.Equal(t, time.Now(), pool.lastConnectivityChange)
					},
				},
			},
		},
		{
			name: "wait for disconnect threshold after last topology change",
			agents: []*mockAgent{
				{proxyID: "proxy-1", rtt: 10 * time.Millisecond},
				{proxyID: "proxy-2", rtt: 70 * time.Millisecond},
				{proxyID: "proxy-3", rtt: 30 * time.Millisecond},
			},
			update: []testUpdate{
				{
					apply: func(t *testing.T, pool *AgentPool) {
						pool.tracker.SetConnectionCount(2)
						time.Sleep(disconnectThreshold + time.Second)
						pool.tracker.TrackExpected(track.Proxy{Name: "proxy-4"})
					},
					assert: func(t *testing.T, pool *AgentPool, agents []*mockAgent) {
						require.Zero(t, agents[0].stopCalls)
						require.Zero(t, agents[1].stopCalls)
						require.Zero(t, agents[2].stopCalls)
						require.NotEqual(t, time.Now(), pool.lastConnectivityChange)
						require.Equal(t, time.Now(), pool.tracker.Snapshot().LastTopologyChange)
					},
				},
				{
					apply: func(t *testing.T, pool *AgentPool) {
						time.Sleep(disconnectThreshold + time.Second)
					},
					assert: func(t *testing.T, pool *AgentPool, agents []*mockAgent) {
						require.Zero(t, agents[0].stopCalls)
						require.Equal(t, 1, agents[1].stopCalls)
						require.Zero(t, agents[2].stopCalls)
						require.NotEqual(t, time.Now(), pool.tracker.Snapshot().LastTopologyChange)
						require.Equal(t, time.Now(), pool.lastConnectivityChange)
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				pool := newTestPool(t)
				for _, agent := range tt.agents {
					pool.active.add(agent)
				}
				pool.tracker.TrackExpected(trackedProxiesForAgents(tt.agents)...)
				for _, update := range tt.update {
					update.apply(t, pool)
					pool.tryDisconnect(t.Context())
					synctest.Wait()
					update.assert(t, pool, tt.agents)
				}
			})
		})
	}
}

func TestAgentKeepAliveCountTimeout(t *testing.T) {
	cases := []struct {
		name           string
		keepAliveCount int
		expectTimeout  time.Duration
	}{
		{
			name:           "count=3 produces 300ms watchdog timeout",
			keepAliveCount: 3,
			expectTimeout:  300 * time.Millisecond,
		},
		{
			name:           "count=7 produces 700ms watchdog timeout",
			keepAliveCount: 7,
			expectTimeout:  700 * time.Millisecond,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var (
					watchdogOnce    sync.Once
					watchdogTimeout time.Duration
				)

				requests := make(chan *ssh.Request)

				agent, client := testAgent(t, agentConfig{
					keepAlive:      100 * time.Millisecond,
					keepAliveCount: tt.keepAliveCount,
				})

				client.MockEnableWatchdog = func(timeout time.Duration) {
					watchdogOnce.Do(func() {
						watchdogTimeout = timeout
					})
				}

				client.MockSendRequest = func(ctx context.Context, name string, wantReply bool, payload []byte) (bool, []byte, error) {
					return true, nil, nil
				}

				client.MockOpenChannel = func(ctx context.Context, name string, data []byte) (*tracessh.Channel, <-chan *ssh.Request, error) {
					return tracessh.NewTraceChannel(
						&mockSSHChannel{
							MockSendRequest: func(name string, wantReply bool, payload []byte) (bool, error) {
								return true, nil
							},
						},
					), requests, nil
				}

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				require.NoError(t, agent.Start(ctx))

				// Wait for all goroutines in the bubble to block, at which
				// point sendKeepalives has fired its timer(0) and called
				// EnableWatchdog.
				synctest.Wait()

				// Stop the agent and drain the requests channel so
				// DiscardRequests can exit, leaving no blocked goroutines.
				agent.Stop()
				close(requests)

				require.Equal(t, tt.expectTimeout, watchdogTimeout,
					"watchdog timeout should equal keepAlive * keepAliveCount")
			})
		})
	}
}

func TestAgentPoolKeepAliveCountUpdatesAgent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		watchdogTimeouts := make(chan time.Duration, 2)

		var mu sync.Mutex
		currentRequests := make(chan *ssh.Request)

		pool, client := setupTestAgentPool(t)

		// Use a fresh tracker with no pre-tracked proxies so lease.Claim
		// always succeeds regardless of principal name.
		var err error
		pool.tracker, err = track.New(track.Config{ClusterName: "test-cluster"})
		require.NoError(t, err)
		pool.tracker.TrackExpected(track.Proxy{Name: "proxy-1"})

		agentCount := 0
		var keepAliveCountMax atomic.Int64 // starting at 0 will default to apidefaults.KeepAliveCountMax

		pool.newAgentFunc = func(ctx context.Context, tracker *track.Tracker, lease *track.Lease) (Agent, error) {
			agentCount++
			// Each agent gets a unique principal so it never collides with a
			// previously claimed proxy.
			principal := fmt.Sprintf("proxy-%d", agentCount)

			mu.Lock()
			reqs := currentRequests
			mu.Unlock()

			mockClient := &mockSSHClient{
				MockPrincipals:        []string{principal},
				MockGlobalRequests:    make(chan *ssh.Request),
				MockHandleChannelOpen: make(chan ssh.NewChannel),
				MockSendRequest: func(ctx context.Context, name string, wantReply bool, payload []byte) (bool, []byte, error) {
					return true, nil, nil
				},
				MockOpenChannel: func(ctx context.Context, name string, data []byte) (*tracessh.Channel, <-chan *ssh.Request, error) {
					return tracessh.NewTraceChannel(
						&mockSSHChannel{
							MockSendRequest: func(name string, wantReply bool, payload []byte) (bool, error) {
								return true, nil
							},
						},
					), reqs, nil
				},
			}
			mockClient.MockEnableWatchdog = func(timeout time.Duration) {
				select {
				case watchdogTimeouts <- timeout:
				default:
				}
			}

			inject := &mockAgentInjection{client: mockClient}
			a, err := newAgent(agentConfig{
				addr:             utils.NetAddr{Addr: "test-proxy-addr"},
				keepAlive:        pool.runtimeConfig.keepAliveInterval,
				keepAliveCount:   pool.runtimeConfig.keepAliveCount,
				sshDialer:        inject,
				transportHandler: inject,
				versionGetter:    inject,
				tracker:          tracker,
				lease:            lease,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			a.stateCallback = pool.getStateCallback(a)
			return a, nil
		}

		client.mockGetClusterNetworkingConfig = func(ctx context.Context) (types.ClusterNetworkingConfig, error) {
			config := types.DefaultClusterNetworkingConfig()
			config.SetKeepAliveInterval(100 * time.Millisecond)
			config.SetKeepAliveCountMax(keepAliveCountMax.Load())
			return config, nil
		}

		require.NoError(t, pool.Start())
		defer pool.Stop()

		synctest.Wait()

		require.Equal(t, 300*time.Millisecond, <-watchdogTimeouts,
			"first agent: watchdog timeout should be keepAlive(100ms) * keepAliveCount(3)")

		keepAliveCountMax.Store(7)

		newReqs := make(chan *ssh.Request)
		mu.Lock()
		oldReqs := currentRequests
		currentRequests = newReqs
		mu.Unlock()

		// Closing oldReqs unblocks the first agent's DiscardRequests goroutine,
		// which causes the agent to close and the pool to reconnect with the
		// updated keepAliveCount.
		close(oldReqs)

		synctest.Wait()

		require.Equal(t, 700*time.Millisecond, <-watchdogTimeouts,
			"second agent: watchdog timeout should be keepAlive(100ms) * keepAliveCount(7)")

		close(newReqs)
	})
}
