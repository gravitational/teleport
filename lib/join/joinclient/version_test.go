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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestInvokeVersionCallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		minClientVersion string
		serverVersion    string
		findError        bool
		callback         func(t *testing.T, info VersionInfo) error
		assertErr        require.ErrorAssertionFunc
	}{
		{
			name:             "invokes callback with version info",
			minClientVersion: "2.0.0",
			serverVersion:    "2.0.0",
			callback: func(t *testing.T, info VersionInfo) error {
				require.Equal(t, VersionInfo{
					ServerVersion:    "2.0.0",
					MinClientVersion: "2.0.0",
				}, info)
				return nil
			},
			assertErr: require.NoError,
		},
		{
			name:             "callback error aborts",
			minClientVersion: "2.0.0",
			serverVersion:    "2.0.0",
			callback: func(t *testing.T, info VersionInfo) error {
				return trace.BadParameter("version rejected")
			},
			assertErr: require.Error,
		},
		{
			name:      "find error does not invoke callback",
			findError: true,
			callback: func(t *testing.T, info VersionInfo) error {
				t.Fatal("unexpected version callback invocation")
				return nil
			},
			assertErr: require.NoError,
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
				Insecure: true,
				OnVersionCallback: func(ctx context.Context, info VersionInfo) error {
					return tc.callback(t, info)
				},
				Log: logtest.NewLogger(),
			}
			addr := strings.TrimPrefix(srv.URL, "https://")
			tc.assertErr(t, invokeVersionCallback(t.Context(), params, addr))
		})
	}
}

// TestInvokeVersionCallbackNoHandler ensures version information is not fetched
// when no version callback is configured.
func TestInvokeVersionCallbackNoHandler(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to proxy web API: %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	params := JoinParams{
		Insecure: true,
		Log:      logtest.NewLogger(),
	}
	addr := strings.TrimPrefix(srv.URL, "https://")
	require.NoError(t, invokeVersionCallback(t.Context(), params, addr))
}
