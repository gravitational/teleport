/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type fakeConn struct {
	net.Conn
	closed atomic.Bool
}

func (f *fakeConn) Close() error {
	f.closed.CompareAndSwap(false, true)
	return nil
}

func (f *fakeConn) RemoteAddr() net.Addr {
	return &utils.NetAddr{
		Addr:        "127.0.0.1:6514",
		AddrNetwork: "tcp",
	}
}

func TestValidateClientVersion(t *testing.T) {
	cases := []struct {
		name          string
		middleware    *Middleware
		clientVersion string
		errAssertion  func(t *testing.T, err error)
	}{
		{
			name:       "rejection disabled",
			middleware: &Middleware{},
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:       "rejection enabled and client version not specified",
			middleware: &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:          "client rejected",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor - 2}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got %T, expected access denied error", err)
			},
		},
		{
			name:          "valid client v-1",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor - 1}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:          "valid client v-0",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:          "invalid client version",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: "abc123",
			errAssertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got %T, expected access denied error", err)
			},
		},
		{
			name:          "pre-release client allowed",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor - 1, PreRelease: "dev.abcd.123"}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:          "pre-release client rejected",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor - 2, PreRelease: "dev.abcd.123"}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got %T, expected access denied error", err)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.clientVersion != "" {
				ctx = metadata.NewIncomingContext(ctx, metadata.New(map[string]string{"version": tt.clientVersion}))
			}

			tt.errAssertion(t, tt.middleware.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: TestBuiltin(types.RoleNode).I}))
		})
	}
}

func TestRejectedClientClusterAlertContents(t *testing.T) {
	var alerts []types.ClusterAlert
	mw := Middleware{
		OldestSupportedVersion: teleport.MinClientSemVer(),
		AlertCreator: func(ctx context.Context, a types.ClusterAlert) error {
			alerts = append(alerts, a)
			return nil
		},
	}

	alertVersion := semver.Version{
		Major: mw.OldestSupportedVersion.Major,
		Minor: mw.OldestSupportedVersion.Minor,
		Patch: mw.OldestSupportedVersion.Patch,
	}.String()

	version := semver.Version{Major: api.VersionMajor - 5}.String()

	tests := []struct {
		name      string
		userAgent string
		identity  authz.IdentityGetter
		expected  string
	}{
		{
			name:     "invalid node",
			identity: TestServerID(types.RoleNode, "1-2-3-4").I,
			expected: fmt.Sprintf("Connection from Node 1-2-3-4 at 127.0.0.1:6514, running an unsupported version of v%s was rejected. Connections will be allowed after upgrading the agent to v%s or newer", version, alertVersion),
		},
		{
			name:      "invalid tsh",
			userAgent: "tsh/" + teleport.Version,
			identity:  TestUser("llama").I,
			expected:  fmt.Sprintf("Connection from tsh v%s by llama was rejected. Connections will be allowed after upgrading tsh to v%s or newer", version, alertVersion),
		},
		{
			name: "invalid remote node",
			identity: authz.RemoteBuiltinRole{
				Role:        types.RoleNode,
				Username:    string(types.RoleNode),
				ClusterName: "leaf",
				Identity: tlsca.Identity{
					Username: "1-2-3-4",
				},
			},
			expected: fmt.Sprintf("Connection from Node 1-2-3-4 at 127.0.0.1:6514 in cluster leaf, running an unsupported version of v%s was rejected. Connections will be allowed after upgrading the agent to v%s or newer", version, alertVersion),
		},

		{
			name:     "invalid tool",
			identity: TestUser("llama").I,
			expected: fmt.Sprintf("Connection from tsh, tctl, tbot, or a plugin running v%s by llama was rejected. Connections will be allowed after upgrading to v%s or newer", version, alertVersion),
		},
	}

	// Trigger alerts from a variety of identities and validate the content of emitted alerts.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"version":    version,
				"user-agent": test.userAgent,
			}))

			err := mw.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: test.identity})
			assert.Error(t, err)

			// Assert that only an alert was created and the content matches expectations.
			require.Len(t, alerts, 1)
			require.Equal(t, "rejected-unsupported-connection", alerts[0].GetName())
			require.Equal(t, test.expected, alerts[0].Spec.Message)

			// Reset the test alerts.
			alerts = nil

			// Reset the last alert time to a time beyond the rate limit, allowing the next
			// rejection to trigger another alert.
			mw.lastRejectedAlertTime.Store(time.Now().Add(-25 * time.Hour).UnixNano())
		})
	}
}

func TestRejectedClientClusterAlert(t *testing.T) {
	var alerts []types.ClusterAlert
	mw := Middleware{
		OldestSupportedVersion: teleport.MinClientSemVer(),
		AlertCreator: func(ctx context.Context, a types.ClusterAlert) error {
			alerts = append(alerts, a)
			return nil
		},
	}

	// Validate an unsupported client, which should trigger an alert
	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		"version": semver.Version{Major: api.VersionMajor - 20}.String(),
	}))
	err := mw.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: TestBuiltin(types.RoleNode).I})
	assert.Error(t, err)

	// Validate a client with an unknown version, which should trigger an alert, however,
	// due to rate limiting of 1 alert per 24h no alert should be created.
	ctx = metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		"version": "abcd",
	}))
	err = mw.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: TestBuiltin(types.RoleNode).I})
	assert.Error(t, err)

	// Assert that only a single alert was created based on the above rejections.
	require.Len(t, alerts, 1)
	require.Equal(t, "rejected-unsupported-connection", alerts[0].GetName())
	// Assert that the version in the message does not contain any prereleases
	require.NotContains(t, alerts[0].Spec.Message, "-aa")

	for _, tool := range []string{"tsh", "tctl", "tbot"} {
		t.Run(tool, func(t *testing.T) {
			// Reset the test alerts.
			alerts = nil

			// Reset the last alert time to a time beyond the rate limit, allowing the next
			// rejection to trigger another alert.
			mw.lastRejectedAlertTime.Store(time.Now().Add(-25 * time.Hour).UnixNano())

			// Create a new context with the user-agent set to a client tool. This should alter the
			// text in the alert to indicate the connection was from a client tool and not an agent.
			ctx = metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"version":    semver.Version{Major: api.VersionMajor - 20}.String(),
				"user-agent": tool + "/" + teleport.Version,
			}))

			// Validate two unsupported clients in parallel to verify that concurrent attempts
			// to create an alert are prevented.
			var wg sync.WaitGroup
			for range 2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := mw.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: TestBuiltin(types.RoleNode).I})
					assert.Error(t, err)
				}()
			}

			wg.Wait()

			// Assert that only a single additional alert was created and that
			// it was created for clients and not agents.
			require.Len(t, alerts, 1)
			assert.Equal(t, "rejected-unsupported-connection", alerts[0].GetName())
			require.Contains(t, alerts[0].Spec.Message, tool)
			// Assert that the version in the message does not contain any prereleases
			require.NotContains(t, alerts[0].Spec.Message, "-aa")
		})
	}
}
