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
	"io"
	"net"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"

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
