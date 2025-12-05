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
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

type mockSSHClient struct {
	MockSendRequest       func(ctx context.Context, name string, wantReply bool, payload []byte) (bool, []byte, error)
	MockOpenChannel       func(ctx context.Context, name string, data []byte) (*tracessh.Channel, <-chan *ssh.Request, error)
	MockClose             func() error
	MockReply             func(*ssh.Request, bool, []byte) error
	MockPrincipals        []string
	MockGlobalRequests    chan *ssh.Request
	MockHandleChannelOpen chan ssh.NewChannel
}

func (m *mockSSHClient) User() string { return "" }

func (m *mockSSHClient) SessionID() []byte { return nil }

func (m *mockSSHClient) ClientVersion() []byte { return nil }

func (m *mockSSHClient) ServerVersion() []byte { return nil }

func (m *mockSSHClient) RemoteAddr() net.Addr { return nil }

func (m *mockSSHClient) LocalAddr() net.Addr { return nil }

func (m *mockSSHClient) SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, []byte, error) {
	if m.MockSendRequest != nil {
		return m.MockSendRequest(ctx, name, wantReply, payload)
	}
	return false, nil, trace.NotImplemented("")
}

func (m *mockSSHClient) OpenChannel(ctx context.Context, name string, data []byte) (*tracessh.Channel, <-chan *ssh.Request, error) {
	if m.MockOpenChannel != nil {
		return m.MockOpenChannel(ctx, name, data)
	}
	return nil, nil, trace.NotImplemented("")
}

func (m *mockSSHClient) Close() error {
	if m.MockClose != nil {
		return m.MockClose()
	}
	return nil
}

func (m *mockSSHClient) Wait() error {
	return nil
}

func (m *mockSSHClient) Principals() []string {
	return m.MockPrincipals
}

func (m *mockSSHClient) HandleChannelOpen(channelType string) <-chan ssh.NewChannel {
	return m.MockHandleChannelOpen
}

func (m *mockSSHClient) Reply(r *ssh.Request, ok bool, payload []byte) error {
	if m.MockReply != nil {
		return m.MockReply(r, ok, payload)
	}

	return trace.NotImplemented("")
}

func (m *mockSSHClient) GlobalRequests() <-chan *ssh.Request {
	return m.MockGlobalRequests
}

func (m *mockSSHClient) EnableWatchdog(timeout time.Duration) {
}

type fakeReaderWriter struct{}

func (n fakeReaderWriter) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (n fakeReaderWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

type mockSSHChannel struct {
	fakeReaderWriter
	MockSendRequest func(name string, wantReply bool, payload []byte) (bool, error)
}

func (m *mockSSHChannel) Close() error { return nil }

func (m *mockSSHChannel) CloseWrite() error { return nil }

func (m *mockSSHChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	if m.MockSendRequest != nil {
		return m.MockSendRequest(name, wantReply, payload)
	}

	return false, trace.NotImplemented("")
}

func (m *mockSSHChannel) Stderr() io.ReadWriter {
	return fakeReaderWriter{}
}

// mockAgentInjection implements several interfaces for injecting into an agent.
type mockAgentInjection struct {
	client SSHClient
}

func (m *mockAgentInjection) handleTransport(context.Context, ssh.Channel, <-chan *ssh.Request, apisshutils.Conn) {
}

func (m *mockAgentInjection) DialContext(context.Context, utils.NetAddr) (SSHClient, error) {
	return m.client, nil
}

func (m *mockAgentInjection) getVersion(context.Context) (string, error) {
	return teleport.Version, nil
}

func testAgent(t *testing.T, config agentConfig) (*agent, *mockSSHClient) {
	var err error

	if config.tracker == nil {
		config.tracker, err = track.New(track.Config{
			ClusterName: "test",
		})
		require.NoError(t, err)
	}

	config.addr = utils.NetAddr{Addr: "test-proxy-addr"}

	config.lease = config.tracker.TryAcquire()
	require.NotNil(t, config.lease)

	client := &mockSSHClient{
		MockPrincipals:        []string{"default"},
		MockGlobalRequests:    make(chan *ssh.Request),
		MockHandleChannelOpen: make(chan ssh.NewChannel),
	}

	inject := &mockAgentInjection{
		client: client,
	}

	if config.transportHandler == nil {
		config.transportHandler = inject
	}

	if config.sshDialer == nil {
		config.sshDialer = inject
	}

	if config.versionGetter == nil {
		config.versionGetter = inject
	}

	if config.keepAlive == 0 {
		config.keepAlive = time.Millisecond * 100
	}

	if config.keepAliveCount == 0 {
		config.keepAliveCount = 1
	}

	agent, err := newAgent(config)
	require.NoError(t, err, "Unexpected error during agent construction.")

	return agent, client
}

type callbackCounter struct {
	calls  int
	states []AgentState
	recv   chan struct{}
	mu     sync.Mutex
}

func newCallback() *callbackCounter {
	return &callbackCounter{
		recv: make(chan struct{}, 1),
	}
}

func (c *callbackCounter) callback(state AgentState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.states = append(c.states, state)

	select {
	case c.recv <- struct{}{}:
	default:
	}
}

func (c *callbackCounter) waitForCount(t *testing.T, count int) {
	timer := time.NewTimer(time.Second * 5)
	for {
		c.mu.Lock()
		if c.calls == count {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		select {
		case <-c.recv:
			continue
		case <-timer.C:
			require.FailNow(t, "timeout waiting for agent state changes")
		}
	}
}

// TestAgentFailedToClaimLease tests that an agent fails when it cannot claim a lease.
func TestAgentFailedToClaimLease(t *testing.T) {
	agent, client := testAgent(t, agentConfig{})
	claimedProxy := "claimed-proxy"

	callback := newCallback()
	agent.stateCallback = callback.callback

	agent.tracker.TrackExpected(track.Proxy{Name: claimedProxy}, track.Proxy{Name: "other-proxy"})
	lease := agent.tracker.TryAcquire()
	require.NotNil(t, lease)
	lease.Claim(claimedProxy)

	client.MockPrincipals = []string{claimedProxy}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := agent.Start(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), proxyAlreadyClaimedError, "Expected failed to claim proxy error.")

	callback.waitForCount(t, 2)
	require.Contains(t, callback.states, AgentConnecting)
	require.Contains(t, callback.states, AgentClosed)

	require.Equal(t, 2, callback.calls, "Unexpected number of state changes.")
	require.Equal(t, AgentClosed, agent.GetState())
}

// TestAgentStart tests an agent performs the necessary actions during startup.
func TestAgentStart(t *testing.T) {
	tests := []struct {
		name            string // description of this test case
		config          agentConfig
		srvSupportsHbV2 bool
		timeoutOnNth    int // Number of pings after the server times out, 0 == no timeouts
	}{

		{
			name:            "V2 supported",
			srvSupportsHbV2: true,
		},
		{
			name:            "V2 supported and disabled by env",
			srvSupportsHbV2: true,
			config: agentConfig{
				staleConnTimeoutDisabled: true,
			},
		},
		{
			name:            "V2 not supported",
			srvSupportsHbV2: false,
		},
		{
			name: "V2 not supported env disable has no effect",
			config: agentConfig{
				staleConnTimeoutDisabled: true,
			},
		},
		{
			name:            "heartbeat timeout on first ping",
			srvSupportsHbV2: true,
			timeoutOnNth:    1,
		},
		{
			name:            "heartbeat timeout on 3rd ping",
			srvSupportsHbV2: true,
			timeoutOnNth:    4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, client := testAgent(t, tt.config)

			callback := newCallback()
			agent.stateCallback = callback.callback

			openChannels := new(int32)
			sentPings := new(int32)
			versionReplies := new(int32)

			waitForVersion := make(chan struct{})
			go func() {
				client.MockGlobalRequests <- &ssh.Request{
					Type: versionRequest,
				}
			}()

			client.MockOpenChannel = func(ctx context.Context, name string, data []byte) (*tracessh.Channel, <-chan *ssh.Request, error) {
				// Block until the version request is handled to ensure we handle
				// global requests during startup.
				<-waitForVersion

				switch name {
				case chanHeartbeatV2:
					err := &ssh.OpenChannelError{Reason: ssh.UnknownChannelType}
					if tt.srvSupportsHbV2 {
						err = &ssh.OpenChannelError{Reason: RejectFeatureSupported}
					}
					return nil, nil, err

				case chanHeartbeat:
				default:
					assert.Fail(t, "unexpected channel name", name)
				}

				atomic.AddInt32(openChannels, 1)
				return tracessh.NewTraceChannel(
						&mockSSHChannel{
							MockSendRequest: func(name string, wantReply bool, payload []byte) (bool, error) {
								atomic.AddInt32(sentPings, 1)
								if !tt.srvSupportsHbV2 || tt.config.staleConnTimeoutDisabled {
									assert.False(t, wantReply, "Expected no reply wanted.")
								} else {
									assert.True(t, wantReply, "Expected reply wanted.")
								}
								assert.Equal(t, "ping", name, "Unexpected request name.")

								if atomic.LoadInt32(sentPings) == int32(tt.timeoutOnNth) {
									return false, trace.ConnectionProblem(nil, "no reply to heartbeat")
								}

								return false, nil
							},
						},
					),
					make(<-chan *ssh.Request),
					nil
			}

			client.MockReply = func(r *ssh.Request, b1 bool, b2 []byte) error {
				atomic.AddInt32(versionReplies, 1)

				// Unblock once we receive a version reply.
				close(waitForVersion)

				assert.Equal(t, versionRequest, r.Type, "Unexpected request type.")
				assert.Equal(t, teleport.Version, string(b2), "Unexpected version.")
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			switch {
			case tt.timeoutOnNth == 0:
				err := agent.Start(ctx)
				require.NoError(t, err)
				require.Equal(t, 1, int(atomic.LoadInt32(openChannels)), "Expected only heartbeat channel to be opened.")
				require.GreaterOrEqual(t, 1, int(atomic.LoadInt32(sentPings)), "Expected at least 1 ping to be sent.")
				require.Equal(t, 1, int(atomic.LoadInt32(versionReplies)), "Expected 1 version reply.")
				callback.waitForCount(t, 2)
				require.Contains(t, callback.states, AgentConnecting)
				require.Contains(t, callback.states, AgentConnected)
				require.Equal(t, 2, callback.calls, "Unexpected number of state changes.")
				require.Equal(t, AgentConnected, agent.GetState())

				err = agent.Stop()
				require.NoError(t, err)
				require.True(t, agent.lease.IsReleased(), "Expected lease to be released.")

				callback.waitForCount(t, 3)
				require.Contains(t, callback.states, AgentClosed)
				require.Equal(t, 3, callback.calls, "Unexpected number of state changes.")
				require.Equal(t, AgentClosed, agent.GetState())
			case tt.timeoutOnNth == 1:
				// special case, connect fails on the very first connection
				err := agent.Start(ctx)
				require.Error(t, err)
				require.True(t, trace.IsConnectionProblem(err))

			case tt.timeoutOnNth > 1:
				err := agent.Start(ctx)
				require.NoError(t, err)
				require.Equal(t, 1, int(atomic.LoadInt32(openChannels)), "Expected only heartbeat channel to be opened.")
				require.GreaterOrEqual(t, tt.timeoutOnNth-1, int(atomic.LoadInt32(sentPings)), "Expected at least (tt.timeoutOnNth-1) ping(s) to be sent.")
				require.Equal(t, 1, int(atomic.LoadInt32(versionReplies)), "Expected 1 version reply.")
				callback.waitForCount(t, 3)
				require.Contains(t, callback.states, AgentConnecting)
				require.Contains(t, callback.states, AgentConnected)
				require.Contains(t, callback.states, AgentClosed)
				require.Equal(t, 3, callback.calls, "Unexpected number of state changes.")
				require.Equal(t, AgentClosed, agent.GetState()) // timeout closes the agent
			}
		})
	}
}

func TestAgentStateTransitions(t *testing.T) {
	tests := []struct {
		start    AgentState
		noError  []AgentState
		yesError []AgentState
	}{
		{
			start:    AgentInitial,
			noError:  []AgentState{AgentConnecting, AgentClosed},
			yesError: []AgentState{AgentInitial, AgentConnected},
		},
		{
			start:    AgentConnecting,
			noError:  []AgentState{AgentConnected, AgentClosed},
			yesError: []AgentState{AgentInitial, AgentConnecting},
		},
		{
			start:    AgentConnected,
			noError:  []AgentState{AgentClosed},
			yesError: []AgentState{AgentInitial, AgentConnecting, AgentConnected},
		},
		{
			start:    AgentClosed,
			yesError: []AgentState{AgentInitial, AgentConnecting, AgentConnected, AgentClosed},
		},
	}

	agent, _ := testAgent(t, agentConfig{})
	for _, tc := range tests {
		msg := "failed testing agent state %s -> %s"
		for _, state := range tc.noError {
			agent.state = tc.start
			prev, err := agent.updateState(state)
			require.Equal(t, tc.start, prev, msg, tc.start, state)
			require.NoError(t, err, msg, tc.start, state)
		}
		for _, state := range tc.yesError {
			agent.state = tc.start
			_, err := agent.updateState(state)
			require.Error(t, err, msg, tc.start, state)
		}
	}
}

type mockHeartbeatInstance struct {
	sshServer        *sshutils.Server
	supportsV2       bool
	heartbeatCounter atomic.Int64
	pauseResponses   atomic.Bool
	t                *testing.T
}

func (m *mockHeartbeatInstance) Start() error {
	return trace.Wrap(m.sshServer.Start())
}

func (m *mockHeartbeatInstance) Stop() {
	m.sshServer.Close()
}

func (m *mockHeartbeatInstance) HandleNewChan(ctx context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
	switch nch.ChannelType() {
	case chanHeartbeat:
		go m.handleHeartbeat(ctx, ccx.NetConn, ccx.ServerConn, nch)
	case chanHeartbeatV2:
		if m.supportsV2 {
			nch.Reject(RejectFeatureSupported, "rejected")
			break
		}
		fallthrough
	default:
		nch.Reject(ssh.UnknownChannelType, "rejected")
	}
}

func (m *mockHeartbeatInstance) handleHeartbeat(ctx context.Context, conn net.Conn, _ *ssh.ServerConn, nch ssh.NewChannel) {
	ch, req, err := nch.Accept()
	assert.NoError(m.t, err, "Failed to accept channel")

	if err != nil {
		conn.Close()
		return
	}

	apisshutils.DiscardChannelData(ch)
	if ch != nil {
		defer func() {
			ch.Close()
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case r, ok := <-req:
			if !ok || r == nil {
				return
			}

			for m.pauseResponses.Load() {
				time.Sleep(10 * time.Millisecond)
			}

			r.Reply(false, nil)
			m.heartbeatCounter.Add(1)
		}
	}
}

func setupMockServerAndAgent(t *testing.T, tt *testAgentTimeoutCase) (*mockHeartbeatInstance, Agent) {
	t.Helper()
	ctx := t.Context()
	ca, err := apisshutils.MakeTestSSHCA()
	require.NoError(t, err)
	cert, err := apisshutils.MakeRealHostCert(ca)
	require.NoError(t, err)

	mock := &mockHeartbeatInstance{t: t, supportsV2: tt.supportsV2}

	sshServer, err := sshutils.NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		mock,
		sshutils.StaticHostSigners(cert),
		sshutils.AuthMethods{NoClient: true},
		sshutils.SetInsecureSkipHostValidation(),
	)
	require.NoError(t, err)

	mock.sshServer = sshServer
	require.NoError(t, sshServer.Start()) // Start here to obtain the port for the resolver.

	priv, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)

	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)

	resolver := func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
		return utils.MustParseAddr(sshServer.Addr()), types.ProxyListenerMode_Multiplex, nil
	}

	pool, err := NewAgentPool(ctx, AgentPoolConfig{
		Resolver:                 resolver,
		Client:                   &mockLocalClusterClient{},
		AccessPoint:              &fakeClient{caKey: ca.PublicKey()},
		Cluster:                  "test",
		AuthMethods:              []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostUUID:                 uuid.NewString(),
		StaleConnTimeoutDisabled: tt.staleConnTimeoutDisabled,
		HeartbeatTimeout:         tt.heartbeatTimeout,
	})
	require.NoError(t, err)

	pool.runtimeConfig.keepAliveInterval = tt.keepAliveInterval
	pool.runtimeConfig.keepAliveCount = tt.keepAliveCount

	tracker, err := track.New(track.Config{ClusterName: "test"})
	require.NoError(t, err)

	lease := tracker.TryAcquire()
	require.NotNil(t, lease)

	agent, err := pool.newAgent(ctx, tracker, lease)
	require.NoError(t, err)

	return mock, agent
}

type testAgentTimeoutCase struct {
	name                     string
	supportsV2               bool
	staleConnTimeoutDisabled bool
	keepAliveInterval        time.Duration
	heartbeatTimeout         time.Duration
	keepAliveCount           int
	testFn                   func(*testing.T, *mockHeartbeatInstance, Agent, time.Duration, time.Duration)
}

// TestAgentTimeout runs an agent against a mock SSH server and ensures that
// heartbeats time out as expected when the server does not respond.
func TestAgentTimeout(t *testing.T) {
	t.Parallel()
	cases := []testAgentTimeoutCase{
		{
			name:              "server supports V2",
			supportsV2:        true,
			keepAliveInterval: 100 * time.Millisecond,
			heartbeatTimeout:  50 * time.Millisecond,
			keepAliveCount:    3,
			testFn: func(t *testing.T, mock *mockHeartbeatInstance, a Agent, waitFor time.Duration, tick time.Duration) {
				mock.pauseResponses.Store(true)

				require.EventuallyWithT(t, func(collect *assert.CollectT) {
					assert.Equal(collect, AgentClosed, a.GetState())
				}, waitFor, tick, "Expected agent to enter disconnected state after heartbeat timeout.")
			},
		},
		{
			name:              "server supports V2, recover before disconnect",
			supportsV2:        true,
			keepAliveInterval: 100 * time.Millisecond,
			heartbeatTimeout:  50 * time.Millisecond,
			keepAliveCount:    5,
			testFn: func(t *testing.T, mock *mockHeartbeatInstance, a Agent, waitFor time.Duration, tick time.Duration) {
				mock.pauseResponses.Store(true)
				count := mock.heartbeatCounter.Load()

				require.Never(t, func() bool {
					return mock.heartbeatCounter.Load() > count
				}, 150*time.Millisecond, tick, "no further heartbeats expected")

				// Restore server back to normal
				mock.pauseResponses.Store(false)

				require.EventuallyWithT(t, func(collect *assert.CollectT) {
					assert.GreaterOrEqual(collect, mock.heartbeatCounter.Load(), count+4, "heartbeats resume as normal")
				}, waitFor, tick, "Expected heartbeats to resume after recovery")
				assert.Equal(t, AgentConnected, a.GetState(), "expected agent to stay connected")
			},
		},

		{
			name:                     "server supports V2 but env override set",
			supportsV2:               true,
			staleConnTimeoutDisabled: true,
			keepAliveInterval:        100 * time.Millisecond,
			heartbeatTimeout:         50 * time.Millisecond,
			keepAliveCount:           3,

			testFn: func(t *testing.T, mock *mockHeartbeatInstance, a Agent, waitFor time.Duration, tick time.Duration) {
				mock.pauseResponses.Store(true)
				require.Never(t, func() bool {
					return a.GetState() != AgentConnected
				}, waitFor, tick, "Expected agent to remain connected.")
			},
		},
		{
			name:              "server does not support V2",
			supportsV2:        false,
			keepAliveInterval: 100 * time.Millisecond,
			heartbeatTimeout:  50 * time.Millisecond,
			keepAliveCount:    5,
			testFn: func(t *testing.T, mock *mockHeartbeatInstance, a Agent, waitFor time.Duration, tick time.Duration) {
				mock.pauseResponses.Store(true)

				require.Never(t, func() bool {
					return a.GetState() != AgentConnected
				}, waitFor, tick, "Expected agent to remain connected.")
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			mock, agent := setupMockServerAndAgent(t, &tt)
			require.NoError(t, mock.Start())
			t.Cleanup(func() { mock.Stop() })

			require.NoError(t, agent.Start(ctx))
			t.Cleanup(func() { mock.pauseResponses.Store(false) })
			t.Cleanup(func() { agent.Stop() })
			defaultDisconnectTimeout := (tt.keepAliveInterval * time.Duration(tt.keepAliveCount+1))
			defaultTickTime := tt.keepAliveInterval / 2

			// Wait for some successful heartbeats to be received.
			require.Equal(t, AgentConnected, agent.GetState())
			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				assert.GreaterOrEqual(collect, mock.heartbeatCounter.Load(), int64(2))
			}, defaultDisconnectTimeout, defaultTickTime, "Expected at least 2 heartbeats to be received.")

			tt.testFn(t, mock, agent, defaultDisconnectTimeout, defaultTickTime)
		})
	}

}
