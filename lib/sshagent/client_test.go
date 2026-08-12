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

package sshagent_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/lib/sshagent"
	"github.com/gravitational/teleport/lib/utils"
)

func TestSSHAgentClient(t *testing.T) {
	keyring, ok := agent.NewKeyring().(agent.ExtendedAgent)
	require.True(t, ok)

	agentDir := t.TempDir()
	agentPath := filepath.Join(agentDir, "agent.sock")
	startAgentServer := func() (stop func()) {
		l, err := net.Listen("unix", agentPath)
		require.NoError(t, err)

		// create a context to close existing connections on server shutdown.
		serveCtx, serveCancel := context.WithCancel(t.Context())

		// Track open connections.
		var connWg sync.WaitGroup

		connWg.Add(1)
		go func() {
			defer connWg.Done()
			for {
				conn, err := l.Accept()
				if err != nil {
					assert.True(t, utils.IsUseOfClosedNetworkError(err))
					return
				}

				closeConn := func() {
					conn.Close()
					connWg.Done()
				}

				// Close the connection early if the server is stopped.
				connNotClosed := context.AfterFunc(serveCtx, closeConn)

				connWg.Add(1)
				go func() {
					defer func() {
						if connNotClosed() {
							closeConn()
						}
					}()
					agent.ServeAgent(keyring, conn)
				}()
			}
		}()

		// Close the listener, stop serving, and wait for all open client
		// connections to close.
		stopServer := func() {
			l.Close()
			serveCancel()
			connWg.Wait()
		}

		return stopServer
	}

	stopServer := startAgentServer()
	t.Cleanup(stopServer)

	clientConnect := func() (io.ReadWriteCloser, error) {
		return net.Dial("unix", agentPath)
	}
	clientGetter := func() (sshagent.Client, error) {
		return sshagent.NewClient(clientConnect)
	}

	// Get a new agent client and make successful requests.
	agentClient, err := clientGetter()
	require.NoError(t, err)
	_, err = agentClient.List()
	require.NoError(t, err)

	// Close the server and all client connections, client should fail.
	stopServer()
	_, err = agentClient.List()
	// TODO(Joerger): Ideally we would check for the error (io.EOF),
	// but the agent library isn't properly wrapping its errors.
	require.Error(t, err)

	// Getting a new client should fail.
	_, err = clientGetter()
	require.Error(t, err)

	// Re-open the server. Get a new agent client connection.
	stopServer = startAgentServer()
	t.Cleanup(stopServer)

	agentClient, err = clientGetter()
	require.NoError(t, err)
	_, err = agentClient.List()
	require.NoError(t, err)

	// Close the client, it should return an error when receiving requests.
	err = agentClient.Close()
	require.NoError(t, err)
	_, err = agentClient.List()
	require.Error(t, err)
}

// TestSingleRequestClient verifies that a single requeest agent client
// serves requests without keeping a connection to the agent open in between them.
func TestSingleRequestClient(t *testing.T) {
	keyring, ok := agent.NewKeyring().(agent.ExtendedAgent)
	require.True(t, ok)

	// Serve the keyring over a unix socket, keeping track of how many client
	// connections are currently open.
	var openConns atomic.Int32
	agentPath := filepath.Join(t.TempDir(), "agent.sock")
	l, err := net.Listen("unix", agentPath)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			openConns.Add(1)
			go func() {
				defer openConns.Add(-1)
				defer conn.Close()
				agent.ServeAgent(keyring, conn)
			}()
		}
	}()

	requireNoOpenConns := func(t *testing.T) {
		t.Helper()
		// The server notices a closed connection asynchronously.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Zero(t, openConns.Load())
		}, time.Second*5, time.Millisecond*10, "single-request agent client leaked a connection to the agent")
	}

	var dials atomic.Int32
	src := sshagent.NewSingleRequestClient(func() (sshagent.Client, error) {
		dials.Add(1)
		return sshagent.NewClient(func() (io.ReadWriteCloser, error) {
			return net.Dial("unix", agentPath)
		})
	})

	// Creating the client should not connect to the agent.
	require.Zero(t, dials.Load())
	requireNoOpenConns(t)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)

	require.NoError(t, src.Add(agent.AddedKey{PrivateKey: priv}))
	requireNoOpenConns(t)

	keys, err := src.List()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	requireNoOpenConns(t)

	// Signers returned by a single-request client must remain usable
	// after the request they were retrieved by has completed.
	signers, err := src.Signers()
	require.NoError(t, err)
	require.Len(t, signers, 1)
	requireNoOpenConns(t)
	require.Equal(t, sshPub.Marshal(), signers[0].PublicKey().Marshal())

	data := []byte("teleport")
	sig, err := signers[0].Sign(rand.Reader, data)
	require.NoError(t, err)
	require.NoError(t, sshPub.Verify(data, sig))
	requireNoOpenConns(t)

	// An algorithm signer must delegate to the agent's own signer, which knows
	// how to translate algorithm names into agent signature flags.
	algorithmSigner, ok := signers[0].(ssh.AlgorithmSigner)
	require.True(t, ok)
	sig, err = algorithmSigner.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoED25519)
	require.NoError(t, err)
	require.NoError(t, sshPub.Verify(data, sig))
	requireNoOpenConns(t)

	require.NoError(t, src.Remove(sshPub))
	keys, err = src.List()
	require.NoError(t, err)
	require.Empty(t, keys)
	requireNoOpenConns(t)

	// Verify that the requests above actually opened connections.
	require.Positive(t, dials.Load())

	// The signer should detect when the key is gone from the agent.
	_, err = algorithmSigner.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoED25519)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
	requireNoOpenConns(t)
}

func TestConcurrentServeChannelRequests(t *testing.T) {
	synctest.Test(t, synctestConcurrentServeChannelRequests)
}
func synctestConcurrentServeChannelRequests(t *testing.T) {
	cfg := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	_, k, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromSigner(k)
	require.NoError(t, err)
	cfg.AddHostKey(signer)

	c, s, err := bufconnPipe()
	require.NoError(t, err)
	defer c.Close()
	defer s.Close()

	const concurrentRequests = 5

	var waiting atomic.Int32
	barrier := make(chan struct{})
	clientReady := make(chan struct{})

	go func() {
		defer s.Close()
		conn, newChC, reqC, err := ssh.NewServerConn(s, cfg)
		if !assert.NoError(t, err) {
			return
		}
		go func() {
			for newCh := range newChC {
				_ = newCh.Reject(ssh.UnknownChannelType, ssh.UnknownChannelType.String())
			}
		}()
		go ssh.DiscardRequests(reqC)

		<-clientReady
		for range concurrentRequests {
			go func() {
				ch, reqC, err := conn.OpenChannel("auth-agent@openssh.com", nil)
				if !assert.Error(t, err) {
					defer ch.Close()
					go ssh.DiscardRequests(reqC)
				}
				if oc := *new(*ssh.OpenChannelError); assert.ErrorAs(t, err, &oc) {
					assert.Equal(t, ssh.ConnectionFailed, oc.Reason)
				}
			}()
		}
		_ = conn.Wait()
	}()

	go func() {
		defer c.Close()
		//nolint:forbidigo // this client is not speaking to a teleport server
		conn, newChC, reqC, err := ssh.NewClientConn(c, "localhost:22", &ssh.ClientConfig{
			User:            "u",
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
		if !assert.NoError(t, err) {
			return
		}
		//nolint:forbidigo // this client is not speaking to a teleport server
		clt := ssh.NewClient(conn, newChC, reqC)
		defer clt.Close()
		err = sshagent.ServeChannelRequests(t.Context(), clt, func() (sshagent.Client, error) {
			waiting.Add(1)
			<-barrier
			return nil, errors.New("nope")
		})
		if !assert.NoError(t, err) {
			return
		}
		_ = conn.Wait()
	}()

	synctest.Wait()
	close(clientReady)
	synctest.Wait()
	require.EqualValues(t, concurrentRequests, waiting.Load())
	close(barrier)
	synctest.Wait()
}

func bufconnPipe() (net.Conn, net.Conn, error) {
	l := bufconn.Listen(16384)
	defer l.Close()
	var eg errgroup.Group
	var c1, c2 net.Conn

	eg.Go(func() error {
		c, err := l.Dial()
		c1 = c
		return err
	})
	eg.Go(func() error {
		c, err := l.Accept()
		c2 = c
		return err
	})
	if err := eg.Wait(); err != nil {
		if c1 != nil {
			_ = c1.Close()
		}
		if c2 != nil {
			_ = c2.Close()
		}
		return nil, nil, err
	}
	return c1, c2, nil
}
