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

package join

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
)

// clockAuthService is a minimal AuthService that only supplies a clock; the
// alert rate-limiter is the only caller of GetClock reached by these tests.
type clockAuthService struct {
	AuthService
	clock clockwork.Clock
}

func (s clockAuthService) GetClock() clockwork.Clock { return s.clock }

func requireAccessDenied(t require.TestingT, err error, _ ...any) {
	require.True(t, trace.IsAccessDenied(err), "got %v, expected access denied error", err)
}

func TestValidateClientVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                   string
		oldestSupportedVersion *semver.Version
		version                string
		assertErr              require.ErrorAssertionFunc
	}{
		{
			name:                   "check disabled allows old client",
			oldestSupportedVersion: nil,
			version:                "1.0.0",
			assertErr:              require.NoError,
		},
		{
			name:                   "client too old",
			oldestSupportedVersion: &semver.Version{Major: 2},
			version:                "1.0.0",
			assertErr:              requireAccessDenied,
		},
		{
			name:                   "client meets minimum",
			oldestSupportedVersion: &semver.Version{Major: 2},
			version:                "2.0.0",
			assertErr:              require.NoError,
		},
		{
			name:                   "client newer than minimum",
			oldestSupportedVersion: &semver.Version{Major: 2},
			version:                "3.0.0",
			assertErr:              require.NoError,
		},
		{
			// An unreported version fails open so clients that don't send a
			// version are not blocked from joining.
			name:                   "empty version fails open",
			oldestSupportedVersion: &semver.Version{Major: 2},
			version:                "",
			assertErr:              require.NoError,
		},
		{
			// A present-but-unparseable version is rejected rather than
			// failing open, unlike an absent version.
			name:                   "malformed version rejected",
			oldestSupportedVersion: &semver.Version{Major: 2},
			version:                "not-a-version",
			assertErr:              requireAccessDenied,
		},
		{
			// MinClientSemVer carries an "aa" prerelease so that alpha, beta,
			// rc, and dev builds of the minimum major sort after it and are
			// permitted.
			name:                   "prerelease at minimum major allowed",
			oldestSupportedVersion: &semver.Version{Major: 17, PreRelease: "aa"},
			version:                "17.0.0-dev.1",
			assertErr:              require.NoError,
		},
		{
			name:                   "stable release at minimum major allowed",
			oldestSupportedVersion: &semver.Version{Major: 17, PreRelease: "aa"},
			version:                "17.0.0",
			assertErr:              require.NoError,
		},
		{
			name:                   "prior major rejected against prerelease minimum",
			oldestSupportedVersion: &semver.Version{Major: 17, PreRelease: "aa"},
			version:                "16.9.9",
			assertErr:              requireAccessDenied,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &Server{
				cfg:                    &ServerConfig{Logger: slog.Default()},
				oldestSupportedVersion: tc.oldestSupportedVersion,
			}
			tc.assertErr(t, s.validateClientVersion(t.Context(), diagnostic.Info{ClientVersion: tc.version}))
		})
	}
}

func TestValidateClientVersionRaisesAlert(t *testing.T) {
	t.Parallel()

	var alerts []types.ClusterAlert
	clock := clockwork.NewFakeClock()
	s := &Server{
		cfg: &ServerConfig{
			Logger:      slog.Default(),
			AuthService: clockAuthService{clock: clock},
			AlertCreator: func(_ context.Context, a types.ClusterAlert) error {
				alerts = append(alerts, a)
				return nil
			},
		},
		oldestSupportedVersion: &semver.Version{Major: 2, PreRelease: "aa"},
	}

	reject := func() error {
		return s.validateClientVersion(t.Context(), diagnostic.Info{
			ClientVersion: "1.0.0",
			RemoteAddr:    "10.1.2.3:4567",
			Role:          "Instance",
		})
	}

	// An allowed client raises no alert.
	require.NoError(t, s.validateClientVersion(t.Context(), diagnostic.Info{ClientVersion: "2.0.0"}))
	require.Empty(t, alerts)

	// A rejected client raises one alert naming the client version, the minimum
	// version, and the peer info (role and remote address) from the diagnostic.
	require.True(t, trace.IsAccessDenied(reject()))
	require.Len(t, alerts, 1)
	require.Equal(t, "rejected-unsupported-connection", alerts[0].Metadata.Name)
	require.Equal(t, alerts[0].Spec.Message, "One or more agents or bots were rejected from joining due to running unsupported versions. Check the audit log for more details.")

	// A second rejection within the 24h window is suppressed.
	clock.Advance(23 * time.Hour)
	require.True(t, trace.IsAccessDenied(reject()))
	require.Len(t, alerts, 1)

	// Once the window elapses, a rejection raises a fresh alert.
	clock.Advance(2 * time.Hour)
	require.True(t, trace.IsAccessDenied(reject()))
	require.Len(t, alerts, 2)
}

// TestValidateClientVersionAlertRetriesAfterWriteFailure asserts that a failed
// alert write rolls back the rate-limit timestamp so the next rejection retries
// rather than staying suppressed for a full day.
func TestValidateClientVersionAlertRetriesAfterWriteFailure(t *testing.T) {
	t.Parallel()

	var writes int
	clock := clockwork.NewFakeClock()
	s := &Server{
		cfg: &ServerConfig{
			Logger:      slog.Default(),
			AuthService: clockAuthService{clock: clock},
			AlertCreator: func(_ context.Context, _ types.ClusterAlert) error {
				writes++
				if writes == 1 {
					return trace.ConnectionProblem(nil, "backend unavailable")
				}
				return nil
			},
		},
		oldestSupportedVersion: &semver.Version{Major: 2, PreRelease: "aa"},
	}

	reject := func() error {
		return s.validateClientVersion(t.Context(), diagnostic.Info{ClientVersion: "1.0.0", Role: "Instance"})
	}

	// First rejection attempts the write, which fails and rolls back the timestamp.
	require.True(t, trace.IsAccessDenied(reject()))
	require.Equal(t, 1, writes)

	// The next rejection retries immediately (still within 24h) and succeeds.
	require.True(t, trace.IsAccessDenied(reject()))
	require.Equal(t, 2, writes)
}

func TestNewServerOldestSupportedVersion(t *testing.T) {
	t.Run("enforces the minimum client version by default", func(t *testing.T) {
		t.Setenv("TELEPORT_UNSTABLE_ALLOW_OLD_CLIENTS", "")
		s := NewServer(&ServerConfig{})
		require.Equal(t, teleport.MinClientSemVer(), s.oldestSupportedVersion)
	})

	t.Run("override disables the version check", func(t *testing.T) {
		t.Setenv("TELEPORT_UNSTABLE_ALLOW_OLD_CLIENTS", "yes")
		s := NewServer(&ServerConfig{})
		require.Nil(t, s.oldestSupportedVersion)
	})
}
