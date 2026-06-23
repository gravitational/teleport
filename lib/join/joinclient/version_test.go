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

package joinclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/trace"
)

func requireClientTooOld(t require.TestingT, err error, _ ...any) {
	require.ErrorAs(t, err, &ClientTooOldError{})
}

func requireClientTooNew(t require.TestingT, err error, _ ...any) {
	require.ErrorAs(t, err, &ClientTooNewError{})
}

func TestCheckClientMeetsMinVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		clientVersion string
		minVersion    string
		assertErr     require.ErrorAssertionFunc
	}{
		{
			name:          "client too old",
			clientVersion: "1.0.0",
			minVersion:    "2.0.0",
			assertErr:     requireClientTooOld,
		},
		{
			// The pre-release suffix is stripped from the minimum reported in
			// the error so the user-facing message shows a clean version.
			name:          "client too old with pre-release minimum",
			clientVersion: "1.0.0",
			minVersion:    "2.0.0-aa",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				requireClientTooOld(t, err)
				require.Contains(t, err.Error(), "minimum v2.0.0)")
			},
		},
		{
			name:          "pre-release client too old for release minimum",
			clientVersion: "2.0.0-dev",
			minVersion:    "2.0.0",
			assertErr:     requireClientTooOld,
		},
		{
			// The proxy advertises "<major>.0.0-aa" so that pre-release builds
			// of the minimum version are permitted. This case fails if the
			// comparison ever switches to the stripped minimum (2.0.0-dev is
			// below 2.0.0 but above 2.0.0-aa).
			name:          "pre-release client meets pre-release minimum",
			clientVersion: "2.0.0-dev",
			minVersion:    "2.0.0-aa",
			assertErr:     require.NoError,
		},
		{
			name:          "client meets minimum",
			clientVersion: "2.0.0",
			minVersion:    "1.0.0",
			assertErr:     require.NoError,
		},
		{
			name:          "client exactly meets minimum",
			clientVersion: "1.0.0",
			minVersion:    "1.0.0",
			assertErr:     require.NoError,
		},
		{
			name:          "no minimum advertised",
			clientVersion: "1.0.0",
			minVersion:    "",
			assertErr:     require.NoError,
		},
		{
			name:          "malformed minimum returns parse error",
			clientVersion: "1.0.0",
			minVersion:    "not-a-version",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
				require.NotErrorAs(t, err, &ClientTooOldError{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.assertErr(t, checkClientMeetsMinVersion(tc.clientVersion, tc.minVersion))
		})
	}
}

func TestCheckClientMeetsMaxVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		clientVersion string
		serverVersion string
		assertErr     require.ErrorAssertionFunc
	}{
		{
			name:          "client is newer than server",
			clientVersion: "3.0.0",
			serverVersion: "2.0.0",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				requireClientTooNew(t, err)
				require.Contains(t, err.Error(), "supports clients on v2 or v1")
			},
		},
		{
			name:          "client is older than server",
			clientVersion: "1.0.0",
			serverVersion: "2.0.0",
			assertErr:     require.NoError,
		},
		{
			name:          "client and server are the same version",
			clientVersion: "1.0.0",
			serverVersion: "1.0.0",
			assertErr:     require.NoError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.assertErr(t, checkClientMeetsMaxVersion(tc.clientVersion, tc.serverVersion))
		})
	}
}

func TestCheckClientVersionSupported(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		clientVersion    string
		minClientVersion string
		serverVersion    string
		findError        bool
		skipVersionCheck bool
		assertErr        require.ErrorAssertionFunc
	}{
		{
			name:             "too old",
			clientVersion:    "1.0.0",
			minClientVersion: "2.0.0",
			assertErr:        requireClientTooOld,
		},
		{
			name:             "too old but skip version check",
			clientVersion:    "1.0.0",
			minClientVersion: "2.0.0",
			skipVersionCheck: true,
			assertErr:        require.NoError,
		},
		{
			name:             "meets minimum",
			clientVersion:    "2.0.0",
			minClientVersion: "1.0.0",
			assertErr:        require.NoError,
		},
		{
			name:             "malformed minimum fails open",
			clientVersion:    "1.0.0",
			minClientVersion: "not-a-version",
			assertErr:        require.NoError,
		},
		{
			name:          "client too new",
			clientVersion: "3.0.0",
			serverVersion: "2.0.0",
			assertErr:     requireClientTooNew,
		},
		{
			name:             "client too new but skip version check",
			clientVersion:    "3.0.0",
			serverVersion:    "2.0.0",
			skipVersionCheck: true,
			assertErr:        require.NoError,
		},
		{
			name:          "server same major",
			clientVersion: "2.5.0",
			serverVersion: "2.0.0",
			assertErr:     require.NoError,
		},
		{
			name:          "malformed server version fails open",
			clientVersion: "3.0.0",
			serverVersion: "not-a-version",
			assertErr:     require.NoError,
		},
		{
			// A malformed minimum must not mask a real server-too-old verdict.
			// The min parse error fails open on its own, but the independent
			// server check must still be honored.
			name:             "malformed minimum does not mask client too new",
			clientVersion:    "3.0.0",
			minClientVersion: "not-a-version",
			serverVersion:    "2.0.0",
			assertErr:        requireClientTooNew,
		},
		{
			// A client-too-old verdict stands even when the server version is
			// unparseable. The bad server version fails open and must not
			// suppress the client check.
			name:             "client too old with malformed server version",
			clientVersion:    "1.0.0",
			minClientVersion: "2.0.0",
			serverVersion:    "not-a-version",
			assertErr:        requireClientTooOld,
		},
		{
			name:          "find error fails open",
			clientVersion: "1.0.0",
			findError:     true,
			assertErr:     require.NoError,
		},
		{
			name:             "malformed client version",
			clientVersion:    "not-a-version",
			minClientVersion: "2.0.0",
			serverVersion:    "3.0.0",
			assertErr:        require.NoError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.findError {
					http.Error(w, "unavailable", http.StatusServiceUnavailable)
					return
				}
				assert.Equal(t, "/webapi/find", r.URL.Path)
				assert.NoError(t, json.NewEncoder(w).Encode(webclient.PingResponse{
					MinClientVersion: tc.minClientVersion,
					ServerVersion:    tc.serverVersion,
				}))
			}))
			t.Cleanup(srv.Close)

			params := JoinParams{
				Insecure:         true,
				SkipVersionCheck: tc.skipVersionCheck,
				Log:              logtest.NewLogger(),
			}
			addr := strings.TrimPrefix(srv.URL, "https://")
			tc.assertErr(t, checkClientVersionSupported(t.Context(), tc.clientVersion, params, addr))
		})
	}
}

// TestClientVersionErrorsAreFatal ensures a confirmed too-old or too-new client
// error is not misclassified as a connection or not-implemented error. If it
// were, Join would fall back to the legacy join service (join.go), which has no
// version check and would discard the original error.
func TestClientVersionErrorsAreFatal(t *testing.T) {
	t.Parallel()

	for _, err := range []error{ClientTooOldError{}, ClientTooNewError{}} {
		assert.False(t, isConnectionError(err), "%T must not be a connection error", err)
		assert.False(t, trace.IsNotImplemented(err), "%T must not be not-implemented", err)
	}
}
