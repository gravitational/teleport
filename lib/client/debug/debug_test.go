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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

func TestSetLogLevel(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		socketPath, _ := newSocketMockService(t, http.StatusOK, []byte{})
		clt := NewClient(socketPath)

		// The validation is done on the server side, here our server always
		// return success.
		_, err := clt.SetLogLevel(ctx, "INFO")
		require.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		socketPath, _ := newSocketMockService(t, http.StatusUnprocessableEntity, []byte{})
		clt := NewClient(socketPath)

		// The validation is done on the server side, here our server always
		// return failure.
		_, err := clt.SetLogLevel(ctx, "RANDOM")
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})
}

func TestGetReadiness(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		socketPath, _ := newSocketMockService(t, http.StatusOK, []byte(`{"status": "OK"}`))
		clt := NewClient(socketPath)

		ready, msg, err := clt.GetReadiness(ctx)
		require.NoError(t, err)
		require.Equal(t, "OK", msg)
		require.True(t, ready)
	})

	t.Run("Failure", func(t *testing.T) {
		socketPath, _ := newSocketMockService(t, http.StatusBadRequest, []byte(`{"status": "BAD"}`))
		clt := NewClient(socketPath)

		ready, msg, err := clt.GetReadiness(ctx)
		require.NoError(t, err)
		require.Equal(t, "BAD", msg)
		require.False(t, ready)
	})

	t.Run("Not found", func(t *testing.T) {
		socketPath, _ := newSocketMockService(t, http.StatusNotFound, []byte(`404`))
		clt := NewClient(socketPath)

		ready, msg, err := clt.GetReadiness(ctx)
		require.True(t, trace.IsNotFound(err))
		require.Equal(t, "not found", msg)
		require.False(t, ready)
	})
}

func TestCollectProfile(t *testing.T) {
	ctx := context.Background()

	for _, test := range []struct {
		desc         string
		profile      string
		seconds      int
		expectErr    bool
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
		{
			desc:      "invalid profile",
			profile:   "RANDOM",
			expectErr: true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			socketPath, closeFn := newSocketMockService(t, http.StatusOK, []byte("collected profile"))
			defer closeFn()
			clt := NewClient(socketPath)

			_, err := clt.CollectProfile(ctx, test.profile, test.seconds)
			if test.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			requestedPaths := closeFn()
			require.Len(t, requestedPaths, 1)

			path, args, _ := strings.Cut(requestedPaths[0], "?")
			require.True(t, strings.HasPrefix(path, "/debug/pprof/"), "expected PProf request but got %q", path)
			require.Equal(t, test.expectedArgs, args)
			require.Equal(t, test.profile, strings.TrimPrefix(path, "/debug/pprof/"))
		})
	}
}

// newSocketMockService creates a unix socket that access HTTP requests and
// always replies with success. Returns the path to the socket and `closeFn`,
// which when called closes the socket and returns the requested paths.
func newSocketMockService(t *testing.T, status int, contents []byte) (string, func() []string) {
	t.Helper()

	// We cannot simply use the `t.TempDir()` due to the size limit of UDS.
	// Here, we place it inside the temporary directory, which will most likely
	// give a smaller path.
	// https://github.com/golang/go/issues/62614
	socketDir, err := os.MkdirTemp("", "*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(socketDir) })

	socketPath := filepath.Join(socketDir, teleport.DebugServiceSocketName)
	require.Greater(t, 100, len(socketPath), "expected socket name to be smaller (less than 100 characters)"+
		" due to Unix domain socket size limitation but got %q (%d).", socketPath, len(socketPath))

	l, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	var requests []string
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.RequestURI())
			w.WriteHeader(status)
			w.Write(contents)
		}),
	}

	go func() {
		err := srv.Serve(l)
		if err != nil && err != http.ErrServerClosed {
			t.Log("Failed to start server", err)
		}
	}()

	t.Cleanup(func() { srv.Shutdown(context.Background()) })
	return socketPath, func() []string {
		return requests
	}
}
