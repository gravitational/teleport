// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package debug

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/debug"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestSetLogLevel(t *testing.T) {
	ctx := context.Background()
	socketPath, closeFn := newSocketMockService(t, []byte{})
	defer closeFn()
	clt := NewClient(socketPath)

	// All supported log levels should be accepted here.
	for _, level := range logutils.SupportedLevelsText {
		t.Run("Set"+level, func(t *testing.T) {
			_, err := clt.SetLogLevel(ctx, level)
			require.NoError(t, err)
		})

		t.Run("SetLower"+level, func(t *testing.T) {
			_, err := clt.SetLogLevel(ctx, strings.ToLower(level))
			require.NoError(t, err)
		})
	}

	// Random or any other slog format should be rejected.
	for _, level := range []string{"RANDOM", "DEBUG-1", "INFO+1", "INVALID"} {
		t.Run("Set"+level, func(t *testing.T) {
			_, err := clt.SetLogLevel(ctx, level)
			require.NoError(t, err)
		})

		t.Run("SetLower"+level, func(t *testing.T) {
			_, err := clt.SetLogLevel(ctx, strings.ToLower(level))
			require.NoError(t, err)
		})
	}
}

func TestCollectProfile(t *testing.T) {
	ctx := context.Background()

	for _, test := range []struct {
		desc         string
		profile      string
		seconds      int
		expectedArgs string
	}{
		{
			desc:    "profile",
			profile: "goroutine",
		},
		{
			desc:         "profile with seconds flag",
			profile:      "block",
			seconds:      10,
			expectedArgs: "seconds=10",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			socketPath, closeFn := newSocketMockService(t, []byte("collected profile"))
			defer closeFn()
			clt := NewClient(socketPath)

			_, err := clt.CollectProfile(ctx, test.profile, test.seconds)
			require.NoError(t, err)

			requestedPaths := closeFn()
			require.Len(t, requestedPaths, 1)

			path, args, _ := strings.Cut(requestedPaths[0], "?")
			require.True(t, strings.HasPrefix(path, debug.PProfEndpointsPrefix), "expected %q request but got %q", debug.PProfEndpointsPrefix, path)
			require.Equal(t, test.expectedArgs, args)
			require.Equal(t, test.profile, strings.TrimPrefix(path, debug.PProfEndpointsPrefix))
		})
	}
}

// newSocketMockService creates a unix socket that access HTTP requests and
// always replies with success. Returns the path to the socket and `closeFn`,
// which when called closes the socket and returns the requested paths.
func newSocketMockService(t *testing.T, contents []byte) (string, func() []string) {
	t.Helper()

	// We cannot simply use the `t.TempDir()` due to the size limit of UDS.
	// Here, we place it inside the temporary directory, which will most likely
	// give a smaller path.
	// https://github.com/golang/go/issues/62614
	socketDir, err := os.MkdirTemp("", "*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(socketDir) })

	socketPath := filepath.Join(socketDir, debug.ServiceSocketName)
	require.Greater(t, 100, len(socketPath), "expected socket name to be smaller (less than 100 characters)"+
		" due to Unix domain socket size limitation but got %q (%d).", socketPath, len(socketPath))

	l, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	var requests []string
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.RequestURI())
			w.Write(contents)
		}),
	}

	go func() {
		err := srv.Serve(l)
		if err != nil && err != http.ErrServerClosed {
		}
	}()

	return socketPath, func() []string {
		srv.Shutdown(context.Background())
		return requests
	}
}
