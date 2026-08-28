/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package git

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

func TestKeyManager_verify_github(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	caSigner, err := apisshutils.MakeTestSSHCA()
	require.NoError(t, err)
	gitService, err := local.NewGitServerService(bk)
	require.NoError(t, err)
	githubServer := makeGitServer(t, "org")
	_, err = gitService.CreateGitServer(ctx, githubServer)
	require.NoError(t, err)

	// Prep mock servers and point things to them.
	// If TELEPORT_GIT_TEST_REAL_GITHUB=true, use local SSH agent to connect
	// against "github.com:22".
	var clientAuth []ssh.AuthMethod
	var targetAddress string
	githubServerKeys := newGitHubKeyDownloader()
	switch os.Getenv("TELEPORT_GIT_TEST_REAL_GITHUB") {
	case "true", "1":
		targetAddress = "github.com:22"
		t.Log("Verifying against real", targetAddress)

		sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		require.NoError(t, err)
		defer sock.Close()
		agentClient := agent.NewClient(sock)
		clientAuth = append(clientAuth, ssh.PublicKeysCallback(agentClient.Signers))
	default:
		mockGitHubSSHServer := newMockGitHostingService(t, caSigner)
		targetAddress = mockGitHubSSHServer.Addr()
		githubServerKeys.apiEndpoint = newMockGitHubMetaAPIServer(t, mockGitHubSSHServer.hostKey).URL
	}

	m, err := NewKeyManager(&KeyManagerConfig{
		ParentContext: ctx,
		AuthClient: mockAuthClient{
			events: local.NewEventsService(bk),
		},
		AccessPoint: &mockAccessPoint{
			GitServers: gitService,
		},
		githubServerKeys: githubServerKeys,
	})
	require.NoError(t, err)

	t.Run("connect and verify", func(t *testing.T) {
		hostKeyCallback, err := m.HostKeyCallback(githubServer)
		require.NoError(t, err)
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			conn, err := ssh.Dial("tcp", targetAddress, &ssh.ClientConfig{
				User:            "git",
				Auth:            clientAuth,
				HostKeyCallback: hostKeyCallback,
			})
			assert.NoError(collect, err)
			if conn != nil {
				conn.Close()
			}
		}, time.Second*5, time.Millisecond*200, "failed to connect and verify GitHub")
	})

	t.Run("unknown key", func(t *testing.T) {
		hostKeyCallback, err := m.HostKeyCallback(githubServer)
		require.NoError(t, err)
		unknownHostKey, err := apisshutils.MakeRealHostCert(caSigner)
		require.NoError(t, err)
		require.Error(t, hostKeyCallback("github.com", utils.MustParseAddr(targetAddress), unknownHostKey.PublicKey()))
	})

	t.Run("unknown Git server type", func(t *testing.T) {
		unsupported := githubServer.DeepCopy()
		unsupported.SetSubKind("unsupported")
		_, err := m.HostKeyCallback(unsupported)
		require.Error(t, err)
	})
}
