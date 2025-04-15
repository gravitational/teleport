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

package hardwarekeyagent_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	hardwarekeyagentv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/hardwarekeyagent/v1"
)

func TestHardwareKeyAgent_Server(t *testing.T) {
	ctx := context.Background()
	agentDir := t.TempDir()

	// Prepare the agent server
	mockService := hardwarekey.NewMockHardwareKeyService(nil /*prompt*/)
	s, err := hardwarekeyagent.NewServer(ctx, mockService, agentDir)
	require.NoError(t, err)
	t.Cleanup(s.Stop)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- s.Serve(ctx)
	}()

	// Should fail to open a new server in the same directory.
	_, err = hardwarekeyagent.NewServer(ctx, mockService, agentDir)
	require.Error(t, err)

	// Existing server should be unaffected.
	clt, err := hardwarekeyagent.NewClient(ctx, agentDir)
	require.NoError(t, err)
	_, err = clt.Ping(ctx, &hardwarekeyagentv1.PingRequest{})
	require.NoError(t, err)

	// If the server stops gracefully, the directory should be cleaned up and a new server can be started.
	s.Stop()
	time.Sleep(time.Second)
	_, err = os.Stat(agentDir)
	require.ErrorIs(t, err, os.ErrNotExist)
	s, err = hardwarekeyagent.NewServer(ctx, mockService, agentDir)
	require.NoError(t, err)
	t.Cleanup(s.Stop)

	// If the server is unresponsive, it should be replaced by a call to NewServer.
	// Use a timeoutCtx so that the failed Ping request fails quickly.
	timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	s, err = hardwarekeyagent.NewServer(timeoutCtx, mockService, agentDir)
	require.NoError(t, err)
	t.Cleanup(s.Stop)
}
