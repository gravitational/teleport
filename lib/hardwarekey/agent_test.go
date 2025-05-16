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

package hardwarekey_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	libhwk "github.com/gravitational/teleport/lib/hardwarekey"
)

func TestHardwareKeyAgent_Server(t *testing.T) {
	ctx := context.Background()
	agentDir := t.TempDir()

	mockService := hardwarekey.NewMockHardwareKeyService(nil /*prompt*/)

	// treat all keys as unknown (agent) keys.
	knownKeyFn := func(_ *hardwarekey.PrivateKeyRef, _ hardwarekey.ContextualKeyInfo) (bool, error) {
		return false, nil
	}

	// Prepare the agent server
	server, err := libhwk.NewAgentServer(ctx, mockService, agentDir, knownKeyFn)
	require.NoError(t, err)
	t.Cleanup(server.Stop)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(ctx)
	}()

	// Should fail to open a new server in the same directory.
	_, err = libhwk.NewAgentServer(ctx, mockService, agentDir, knownKeyFn)
	require.Error(t, err)

	// Existing server should be unaffected.
	clt, err := libhwk.NewAgentClient(ctx, agentDir)
	require.NoError(t, err)
	_, err = clt.Ping(ctx, &hardwarekeyagentv1.PingRequest{})
	require.NoError(t, err)

	// If the server stops gracefully, the directory should be cleaned up and a new server can be started.
	server.Stop()
	require.Eventually(t, func() bool {
		_, err := os.Stat(agentDir)
		return errors.Is(err, os.ErrNotExist)
	}, 5*time.Second, 100*time.Millisecond)
	server, err = libhwk.NewAgentServer(ctx, mockService, agentDir, knownKeyFn)
	require.NoError(t, err)
	t.Cleanup(server.Stop)

	// If the server is unresponsive, it should be replaced by a call to NewServer.
	// Use a timeoutCtx so that the failed Ping request fails quickly.
	timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	server, err = libhwk.NewAgentServer(timeoutCtx, mockService, agentDir, knownKeyFn)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
}
