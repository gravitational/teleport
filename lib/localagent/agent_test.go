// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package localagent_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/gravitational/teleport/lib/localagent"
)

func TestLocalAgent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	healthCheck := func(ctx context.Context, conn *grpc.ClientConn) error {
		_, err := healthpb.NewHealthClient(conn).
			Check(ctx, &healthpb.HealthCheckRequest{})
		return err
	}

	server, err := localagent.NewServer(dir, localagent.WithActiveConnectionCheck(healthCheck))
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop(ctx) })
	healthpb.RegisterHealthServer(server, health.NewServer())

	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(ctx) }()

	clientConn, err := localagent.NewClient(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, clientConn.Close())
	})
	require.NoError(t, healthCheck(ctx, clientConn))

	// check starting a new agent fails because the old one is still active.
	_, err = localagent.NewServer(dir, localagent.WithActiveConnectionCheck(healthCheck))
	require.True(t, trace.IsAlreadyExists(err), "got %v", err)

	// Check socket is cleaned up.
	server.Stop(ctx)
	require.NoError(t, <-serverErr)
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, localagent.SocketFileName))
		return errors.Is(err, os.ErrNotExist)
	}, 5*time.Second, 100*time.Millisecond)

	// Check new agent can start successfully.
	server, err = localagent.NewServer(dir, localagent.WithActiveConnectionCheck(healthCheck))
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop(context.Background()) })
}
