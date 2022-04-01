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
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type mockSSHClient struct {
	MockSendRequest       func(name string, wantReply bool, payload []byte) (bool, []byte, error)
	MockOpenChannel       func(name string, data []byte) (ssh.Channel, <-chan *ssh.Request, error)
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

func (m *mockSSHClient) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	if m.MockSendRequest != nil {
		return m.MockSendRequest(name, wantReply, payload)
	}
	return false, nil, trace.NotImplemented("")
}

func (m *mockSSHClient) OpenChannel(name string, data []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	if m.MockOpenChannel != nil {
		return m.MockOpenChannel(name, data)
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

type mockSSHChannel struct {
	MockSendRequest func(name string, wantReply bool, payload []byte) (bool, error)
}

func (m *mockSSHChannel) Read(data []byte) (int, error) {
	return 0, trace.NotImplemented("")
}

func (m *mockSSHChannel) Write(data []byte) (int, error) {
	return 0, trace.NotImplemented("")
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
	return nil
}

// mockAgentInjection impements several interaces for injecting into an agent.
type mockAgentInjection struct {
	client SSHClient
}

func (m *mockAgentInjection) transport(context.Context, ssh.Channel, <-chan *ssh.Request, ssh.Conn) *transport {
	return &transport{}
}

func (m *mockAgentInjection) DialContext(context.Context, utils.NetAddr) (SSHClient, error) {
	return m.client, nil
}

func (m *mockAgentInjection) getVersion(context.Context) (string, error) {
	return teleport.Version, nil
}

func testAgent(t *testing.T) (*Agent, *mockSSHClient) {
	tracker, err := track.New(context.Background(), track.Config{
		ClusterName: "test",
	})
	require.NoError(t, err)

	addr := utils.NetAddr{Addr: "test-proxy-addr"}
	tracker.Start()

	lease := <-tracker.Acquire()

	client := &mockSSHClient{
		MockPrincipals:        []string{"default"},
		MockGlobalRequests:    make(chan *ssh.Request),
		MockHandleChannelOpen: make(chan ssh.NewChannel),
	}

	inject := &mockAgentInjection{
		client: client,
	}

	agent, err := NewAgent(&agentConfig{
		keepAlive:     time.Minute,
		addr:          addr,
		transporter:   inject,
		sshDialer:     inject,
		versionGetter: inject,
		tracker:       tracker,
		lease:         lease,
	})
	require.NoError(t, err, "Unexpected error during agent construction.")

	return agent, client
}

// TestAgentFailedToClaimLease tests that an agent fails when it cannot claim a lease.
func TestAgentFailedToClaimLease(t *testing.T) {
	agent, client := testAgent(t)
	claimedProxy := "claimed-proxy"

	var calls int
	agent.stateCallback = func(a *Agent) {
		calls++
	}

	agent.tracker.Claim(claimedProxy)

	client.MockPrincipals = []string{claimedProxy}
	err := agent.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to claim proxy", "Expected failed to claim proxy error.")
	require.Equal(t, 0, calls, "Unexpected number of state changes.")

	err = agent.Stop()
	require.NoError(t, err)
}

// TestAgentStart tests an agent performs the necessary actions during startup.
func TestAgentStart(t *testing.T) {
	agent, client := testAgent(t)

	var calls int
	states := make(chan AgentState)
	agent.stateCallback = func(a *Agent) {
		calls++
		states <- a.GetState()
	}

	openChannels := 0
	sentPings := 0

	waitForVersion := make(chan struct{})
	go func() {
		client.MockGlobalRequests <- &ssh.Request{
			Type: versionRequest,
		}
	}()

	client.MockOpenChannel = func(name string, data []byte) (ssh.Channel, <-chan *ssh.Request, error) {
		// Block until the version request is handled to ensure we handle
		// global requests during startup.
		<-waitForVersion

		openChannels++
		require.Equal(t, name, chanHeartbeat, "Unexpected channel opened during startup.")
		return &mockSSHChannel{MockSendRequest: func(name string, wantReply bool, payload []byte) (bool, error) {
			sentPings++

			require.Equal(t, name, "ping", "Unexpected request name.")
			require.False(t, wantReply, "Expected no reply wanted.")
			return true, nil
		}}, make(<-chan *ssh.Request), nil
	}

	versionReplies := 0
	client.MockReply = func(r *ssh.Request, b1 bool, b2 []byte) error {
		// Unblock once we receive a version reply.
		close(waitForVersion)
		versionReplies++

		require.Equal(t, versionRequest, r.Type, "Unexpected request type.")
		require.Equal(t, teleport.Version, string(b2), "Unexpected version.")
		return nil
	}

	err := agent.Start(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, openChannels, "Expected only heartbeat channel to be opened.")
	require.GreaterOrEqual(t, 1, sentPings, "Expected at least 1 ping to be sent.")
	require.Equal(t, 1, versionReplies, "Expected 1 version reply.")

	state := <-states
	require.Equal(t, AgentConnected, state, "Unexpected state change")
	require.Equal(t, 1, calls, "Unexpected number of state changes.")

	unclaimed := false
	agent.unclaim = func() {
		unclaimed = true
	}

	err = agent.Stop()
	require.NoError(t, err)
	require.True(t, unclaimed, "Expected unclaim to be called.")
	state = <-states
	require.Equal(t, AgentClosed, state, "Unexpected state change")
	require.Equal(t, 2, calls, "Unexpected number of state changes.")
}
