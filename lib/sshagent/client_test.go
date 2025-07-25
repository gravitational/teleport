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
	"testing"

	"golang.org/x/crypto/ssh/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					assert.True(t, utils.IsUseOfClosedNetworkError(err))
					return
				}
				context.AfterFunc(ctx, func() {
					conn.Close()
				})

				go func() {
					defer conn.Close()
					agent.ServeAgent(keyring, conn)
				}()
			}
		}()

		return func() {
			l.Close()
			cancel()
		}
	}

	stopServer := startAgentServer()
	defer stopServer()

	// Initial connection should succeed.
	agentClient, err := sshagent.NewClient(func() (io.ReadWriteCloser, error) {
		return net.Dial("unix", agentPath)
	})
	require.NoError(t, err)

	// requests should succeed.
	_, err = agentClient.List()
	require.NoError(t, err)

	// Close the server, client should fail with an io.EOF.
	stopServer()
	_, err = agentClient.List()
	require.Error(t, err)
	require.ErrorIs(t, err, io.EOF)

	// Re-open the server, the client should automatically attempt to reconnect.
	stopServer = startAgentServer()
	defer stopServer()

	_, err = agentClient.List()
	require.NoError(t, err)

	// Close the client, it should return an error when receiving requests.
	err = agentClient.Close()
	require.Error(t, err)
}
