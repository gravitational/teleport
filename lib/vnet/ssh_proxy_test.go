// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package vnet

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

// TestProxySSHConnection exercises [proxySSHConnection] to test that it
// transparently proxies SSH channels and requests.
//
// The test starts a target SSH server implemented in this file that handles
// channels and requests of type "echo", each handler echos input back to
// output.
//
// The test also starts a proxy server that proxies incoming SSH connections to
// the target server using [proxySSHConnection].
//
// The test asserts that connecting directly to the target server or to the
// proxy appears identical to the client.
func TestProxySSHConnection(t *testing.T) {
	ctx := context.Background()

	proxyListener := bufconn.Listen(100)
	serverListener := bufconn.Listen(100)

	targetServerConfig := sshServerConfig(t)
	proxyServerConfig := sshServerConfig(t)

	proxyClientConfig := sshClientConfig(t)

	testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
		Name: "target server",
		Task: func(ctx context.Context) error {
			return runTestSSHServer(serverListener, targetServerConfig)
		},
		Terminate: func() error {
			return trace.Wrap(serverListener.Close())
		},
	})
	testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
		Name: "proxy server",
		Task: func(ctx context.Context) error {
			return runTestSSHProxy(ctx,
				proxyListener,
				proxyServerConfig,
				serverListener,
				proxyClientConfig,
			)
		},
		Terminate: func() error {
			return trace.Wrap(proxyListener.Close())
		},
	})

	// Test with a direct connection to the test server and a proxied connection
	// to make sure the behavior is indistinguishable.
	t.Run("direct", func(t *testing.T) {
		testSSHConnection(t, serverListener)
	})
	for i := range 4 {
		// Run the proxied test multiple times to make sure proxySSHConnection
		// actually returns when the connection ends.
		t.Run(fmt.Sprintf("proxied_%d", i), func(t *testing.T) {
			testSSHConnection(t, proxyListener)
		})
	}
}

func testSSHConnection(t *testing.T, dial dialer) {
	tcpConn, err := dial.Dial()
	require.NoError(t, err)
	defer tcpConn.Close()

	clientConfig := sshClientConfig(t)
	sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, "localhost", clientConfig)
	require.NoError(t, err)
	defer sshConn.Close()

	testConnectionToSshEchoServer(t, sshConn, chans, reqs)
}

func testConnectionToSshEchoServer(t *testing.T, sshConn ssh.Conn, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	go ssh.DiscardRequests(reqs)
	go func() {
		for newChan := range chans {
			newChan.Reject(ssh.Prohibited, "test")
		}
	}()

	// Try sending some global requests.
	t.Run("global requests", func(t *testing.T) {
		testGlobalRequests(t, sshConn)
	})

	// Try opening a channel that the target server will reject.
	t.Run("unexpected channel", func(t *testing.T) {
		_, _, err := sshConn.OpenChannel("unexpected", nil)
		require.Error(t, err)
		require.ErrorAs(t, err, new(*ssh.OpenChannelError))
	})

	// Try opening a channel that echoes input data back to output, run
	// it twice to make sure multiple channels can be opened.
	// testEchoChannel will also send channel requests.
	t.Run("echo channel 1", func(t *testing.T) {
		testEchoChannel(t, sshConn)
	})
	t.Run("echo channel 2", func(t *testing.T) {
		testEchoChannel(t, sshConn)
	})
}

func testGlobalRequests(t *testing.T, conn ssh.Conn) {
	// Send an echo request.
	msg := []byte("hello")
	reply, replyPayload, err := conn.SendRequest("echo", true, msg)
	assert.NoError(t, err)
	assert.True(t, reply)
	assert.Equal(t, msg, replyPayload)

	// Send an unexepected request type.
	reply, replyPayload, err = conn.SendRequest("unexpected", true, msg)
	assert.NoError(t, err)
	assert.False(t, reply)
	assert.Empty(t, replyPayload)
}

func testEchoChannel(t *testing.T, conn ssh.Conn) {
	ch, reqs, err := conn.OpenChannel("echo", nil)
	require.NoError(t, err)
	go ssh.DiscardRequests(reqs)
	defer ch.Close()

	// Try sending a message over the SSH channel and asserting that it is
	// echoed back.
	msg := []byte("hello")
	_, err = ch.Write(msg)
	require.NoError(t, err)
	var buf [16]byte
	n, err := ch.Read(buf[:])
	require.NoError(t, err)
	require.Equal(t, len(msg), n)
	require.Equal(t, msg, buf[:n])

	// Try sending a channel request that expects a reply.
	reply, err := ch.SendRequest("echo", true, nil)
	require.NoError(t, err)
	require.True(t, reply)

	// The test server replies false to channel requests with type other than
	// "echo".
	reply, err = ch.SendRequest("unknown", true, nil)
	require.NoError(t, err)
	require.False(t, reply)
}

type dialer interface {
	Dial() (net.Conn, error)
}

// runTestSSHProxy runs an SSH proxy server. The function under test
// [proxySSHConnection] requires an established client and server SSH connection
// and only handles proxying SSH requests and channels between them, this server
// is the glue that handles accepting connections from a listener, dialing the
// target server, completing SSH handshakes with each of them, and then finally
// calling [proxySSHConnection].
func runTestSSHProxy(
	ctx context.Context,
	lis net.Listener,
	serverCfg *ssh.ServerConfig,
	serverDialer dialer,
	clientCfg *ssh.ClientConfig,
) error {
	for {
		incomingConn, err := lis.Accept()
		if err != nil {
			if err.Error() == "closed" {
				return nil
			}
			return trace.Wrap(err)
		}
		outgoingConn, err := serverDialer.Dial()
		if err != nil {
			incomingConn.Close()
			if err.Error() == "closed" {
				return nil
			}
			return trace.Wrap(err)
		}
		if err := runTestSSHProxyInstance(
			ctx,
			incomingConn,
			serverCfg,
			outgoingConn,
			clientCfg,
		); err != nil {
			return trace.Wrap(err)
		}
	}
}

func runTestSSHProxyInstance(
	ctx context.Context,
	incomingConn net.Conn,
	serverCfg *ssh.ServerConfig,
	outgoingConn net.Conn,
	clientCfg *ssh.ClientConfig,
) error {
	defer incomingConn.Close()
	defer outgoingConn.Close()
	incomingSSHConn, incomingChans, incomingReqs, err := ssh.NewServerConn(incomingConn, serverCfg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer incomingSSHConn.Close()
	outgoingSSHConn, outgoingChans, outgoingReqs, err := ssh.NewClientConn(outgoingConn, "localhost", clientCfg)
	if err != nil {
		return trace.Wrap(err, "proxying SSH conn in test")
	}
	defer outgoingSSHConn.Close()
	proxySSHConnection(ctx, sshConn{
		conn:  incomingSSHConn,
		chans: incomingChans,
		reqs:  incomingReqs,
	}, sshConn{
		conn:  outgoingSSHConn,
		chans: outgoingChans,
		reqs:  outgoingReqs,
	})
	return trace.Wrap(err)
}

// runTestSSHServer runs a test SSH server that responds to new channel
// requests, global requests, and channel requests of type "echo". It handles
// each by replying with an "echo" of the input.
func runTestSSHServer(lis net.Listener, cfg *ssh.ServerConfig) error {
	for {
		tcpConn, err := lis.Accept()
		if err != nil {
			if err.Error() == "closed" {
				return nil
			}
			return trace.Wrap(err)
		}
		if err := runTestSSHServerInstance(tcpConn, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
}

func runTestSSHServerInstance(tcpConn net.Conn, cfg *ssh.ServerConfig) error {
	sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		handleEchoRequests(reqs)
		sshConn.Close()
	}()
	handleEchoChannels(chans)
	sshConn.Close()
	return nil
}

func handleEchoRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		switch req.Type {
		case "echo":
			req.Reply(true, req.Payload)
		default:
			req.Reply(false, nil)
		}
	}
}

func handleEchoChannels(chans <-chan ssh.NewChannel) {
	for newChan := range chans {
		switch newChan.ChannelType() {
		case "echo":
			go handleEchoChannel(newChan)
		default:
			newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
		}
	}
}

func handleEchoChannel(newChan ssh.NewChannel) {
	ch, reqs, err := newChan.Accept()
	if err != nil {
		return
	}
	go handleEchoRequests(reqs)
	io.Copy(ch, ch)
}

func sshServerConfig(t *testing.T) *ssh.ServerConfig {
	serverKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	hostSigner, err := ssh.NewSignerFromSigner(serverKey)
	require.NoError(t, err)
	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			// We're not testing SSH authentication here, just accept any user key.
			return nil, nil
		},
	}
	serverConfig.AddHostKey(hostSigner)
	return serverConfig
}

func sshClientConfig(t *testing.T) *ssh.ClientConfig {
	clientKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	clientSigner, err := ssh.NewSignerFromSigner(clientKey)
	require.NoError(t, err)
	return &ssh.ClientConfig{
		Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		// We're not testing SSH authentication here, just accept any host key.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
}
