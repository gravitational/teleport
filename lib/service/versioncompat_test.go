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

package service

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func requireClientTooOld(t require.TestingT, err error, _ ...any) {
	var target *clientTooOldError
	require.ErrorAs(t, err, &target)
}

func requireClientTooNew(t require.TestingT, err error, _ ...any) {
	var target *clientTooNewError
	require.ErrorAs(t, err, &target)
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
				require.Contains(t, err.Error(), "minimum version of v2.0.0.")
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
				var target *clientTooOldError
				require.NotErrorAs(t, err, &target)
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
				require.Contains(t, err.Error(), "supports instances on v2 or v1")
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

func TestEnforceVersionPolicy(t *testing.T) {
	t.Parallel()

	// The skew is induced from the server side: advertise requirements the
	// running build can't satisfy, so the check trips against the real
	// teleport.Version without overriding the local version.
	tooOldMinVersion := semver.Version{Major: teleport.SemVer().Major + 1}.String()
	tooNewServerVersion := semver.Version{Major: teleport.SemVer().Major - 1}.String()

	cases := []struct {
		name             string
		minClientVersion string
		serverVersion    string
		skipVersionCheck bool
		assertErr        require.ErrorAssertionFunc
	}{
		{
			name:             "compatible",
			minClientVersion: teleport.MinClientSemVer().String(),
			serverVersion:    teleport.Version,
			assertErr:        require.NoError,
		},
		{
			name:             "client too old",
			minClientVersion: tooOldMinVersion,
			assertErr:        requireClientTooOld,
		},
		{
			name:          "client too new",
			serverVersion: tooNewServerVersion,
			assertErr:     requireClientTooNew,
		},
		{
			name:             "client too old but skip version check",
			minClientVersion: tooOldMinVersion,
			skipVersionCheck: true,
			assertErr:        require.NoError,
		},
		{
			name:             "client too new but skip version check",
			serverVersion:    tooNewServerVersion,
			skipVersionCheck: true,
			assertErr:        require.NoError,
		},
		{
			// The min parse error must fail open without short-circuiting the
			// loop before the independent server check produces its verdict.
			name:             "malformed minimum does not mask client too new",
			minClientVersion: "not-a-version",
			serverVersion:    tooNewServerVersion,
			assertErr:        requireClientTooNew,
		},
		{
			// The bad server version fails open and must not suppress the
			// client-too-old verdict from the min check.
			name:             "client too old with malformed server version",
			minClientVersion: tooOldMinVersion,
			serverVersion:    "not-a-version",
			assertErr:        requireClientTooOld,
		},
		{
			name:             "both versions malformed fail open",
			minClientVersion: "not-a-version",
			serverVersion:    "not-a-version",
			assertErr:        require.NoError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			process := &TeleportProcess{
				Config: &servicecfg.Config{SkipVersionCheck: tc.skipVersionCheck},
				logger: logtest.NewLogger(),
			}
			err := process.enforceVersionPolicy(t.Context(), joinclient.VersionInfo{
				MinClientVersion: tc.minClientVersion,
				ServerVersion:    tc.serverVersion,
			})
			tc.assertErr(t, err)
		})
	}
}
