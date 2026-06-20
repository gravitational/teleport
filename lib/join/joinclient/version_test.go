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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func requireClientTooOld(t require.TestingT, err error, _ ...any) {
	require.ErrorAs(t, err, &ClientTooOldError{})
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

func TestCheckClientVersionSupported(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		clientVersion    string
		minClientVersion string
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
			name:          "find error fails open",
			clientVersion: "1.0.0",
			findError:     true,
			assertErr:     require.NoError,
		},
		{
			// An empty version must resolve to api.Version, not "". A minimum
			// above any real release proves the default kicks in: "" would pass
			// (an empty version "meets" any minimum), while api.Version is
			// correctly rejected as too old.
			name:             "empty version defaults to api.Version",
			clientVersion:    "",
			minClientVersion: "9999.0.0",
			assertErr:        requireClientTooOld,
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
				}))
			}))
			t.Cleanup(srv.Close)

			params := JoinParams{
				Insecure:         true,
				SkipVersionCheck: tc.skipVersionCheck,
				Log:              logtest.NewLogger(),
				Testing:          JoinTestingParams{TeleportVersion: tc.clientVersion},
			}
			addr := strings.TrimPrefix(srv.URL, "https://")
			tc.assertErr(t, checkClientVersionSupported(t.Context(), params, addr))
		})
	}
}

// TestJoinFailsFastWhenClientTooOld ensures a confirmed too-old client gets
// a [ClientTooOldError] back from [Join] itself. If the error were ever classified as
// a connection error anywhere in the chain, [Join] would instead fall back to
// the legacy join service, which has no version check and discards the
// original error.
func TestJoinFailsFastWhenClientTooOld(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webapi/find" {
			http.NotFound(w, r)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(webclient.PingResponse{
			MinClientVersion: teleport.MinClientSemVer().String(),
		}))
	}))
	t.Cleanup(srv.Close)

	tooOldVersion := semver.Version{Major: teleport.MinClientSemVer().Major - 1}

	_, err := Join(t.Context(), JoinParams{
		Token:       "token",
		ID:          state.IdentityID{Role: types.RoleInstance},
		ProxyServer: utils.NetAddr{AddrNetwork: "tcp", Addr: strings.TrimPrefix(srv.URL, "https://")},
		JoinMethod:  types.JoinMethodToken,
		Insecure:    true,
		Log:         logtest.NewLogger(),
		Testing:     JoinTestingParams{TeleportVersion: tooOldVersion.String()},
		// GetHostCredentials is only reached if the version check error is
		// misclassified and the legacy fallback engages. This stub makes that
		// regression fail legibly instead of panicking.
		GetHostCredentials: func(context.Context, string, bool, types.RegisterUsingTokenRequest) (*proto.Certs, error) {
			return nil, errors.New("host credentials unavailable")
		},
	})
	requireClientTooOld(t, err)
}
