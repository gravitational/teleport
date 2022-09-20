// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package teleterm_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/teleterm"

	"github.com/stretchr/testify/require"
)

func TestStart(t *testing.T) {
	homeDir := t.TempDir()
	sockPath := filepath.Join(homeDir, "teleterm.sock")
	cfg := teleterm.Config{
		Addr:    fmt.Sprintf("unix://%v", sockPath),
		HomeDir: fmt.Sprintf("%v/", homeDir),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveErr := make(chan error)
	go func() {
		err := teleterm.Serve(ctx, cfg)
		serveErr <- err
	}()

	blockUntilServerAcceptsConnections(t, sockPath)

	// Stop the server.
	cancel()
	require.NoError(t, <-serveErr)
}

func blockUntilServerAcceptsConnections(t *testing.T, sockPath string) {
	// Wait for the socket to be created.
	require.Eventually(t, func() bool {
		_, err := os.Stat(sockPath)
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		require.NoError(t, err)
		return true
	}, time.Millisecond*500, time.Millisecond*50)

	conn, err := net.DialTimeout("unix", sockPath, time.Second*1)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	err = conn.SetReadDeadline(time.Now().Add(time.Second))
	require.NoError(t, err)

	out := make([]byte, 1024)
	_, err = conn.Read(out)
	require.NoError(t, err)

	err = conn.Close()
	require.NoError(t, err)
}
