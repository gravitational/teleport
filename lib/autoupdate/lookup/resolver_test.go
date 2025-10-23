/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package lookup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const (
	testVersionHigh = "2.3.4"
	testVersionLow  = "2.0.4"
)

// fakeRolloutAccessPoint allows us to mock the ProxyAccessPoint in autoupdate
// tests.
type fakeRolloutAccessPoint struct {
	authclient.ProxyAccessPoint

	rollout *autoupdatepb.AutoUpdateAgentRollout
	err     error
}

func (ap *fakeRolloutAccessPoint) GetAutoUpdateAgentRollout(_ context.Context) (*autoupdatepb.AutoUpdateAgentRollout, error) {
	return ap.rollout, ap.err
}

// fakeRolloutAccessPoint allows us to mock the proxy's auth client in autoupdate
// tests.
type fakeCMCAuthClient struct {
	authclient.ClientI

	cmc types.ClusterMaintenanceConfig
	err error
}

func (c *fakeCMCAuthClient) GetClusterMaintenanceConfig(_ context.Context) (types.ClusterMaintenanceConfig, error) {
	return c.cmc, c.err
}

func TestAutoUpdateAgentVersion(t *testing.T) {
	t.Parallel()
	groupName := "test-group"
	ctx := context.Background()

	// brokenChannelUpstream is a buggy upstream version server.
	// This allows us to craft version channels returning errors.
	brokenChannelUpstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
	t.Cleanup(brokenChannelUpstream.Close)

	tests := []struct {
		name            string
		rollout         *autoupdatepb.AutoUpdateAgentRollout
		rolloutErr      error
		channel         *automaticupgrades.Channel
		expectedVersion *semver.Version
		expectError     require.ErrorAssertionFunc
	}{
		{
			name: "version is looked up from rollout if it is here",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					TargetVersion:  testVersionHigh,
					Schedule:       autoupdate.AgentsScheduleImmediate,
				},
			},
			channel:         &automaticupgrades.Channel{StaticVersion: testVersionLow},
			expectError:     require.NoError,
			expectedVersion: semver.Must(version.EnsureSemver(testVersionHigh)),
		},
		{
			name:            "version is looked up from channel if rollout is not here",
			rolloutErr:      trace.NotFound("rollout is not here"),
			channel:         &automaticupgrades.Channel{StaticVersion: testVersionLow},
			expectError:     require.NoError,
			expectedVersion: semver.Must(version.EnsureSemver(testVersionLow)),
		},
		{
			name:       "hard error getting rollout should not fallback to version channels",
			rolloutErr: trace.AccessDenied("something is very broken"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
			},
			expectError: require.Error,
		},
		{
			name:        "no rollout, error checking channel",
			rolloutErr:  trace.NotFound("rollout is not here"),
			channel:     &automaticupgrades.Channel{ForwardURL: brokenChannelUpstream.URL},
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: building the channel, mock client, and handler with test config.
			require.NoError(t, tt.channel.CheckAndSetDefaults())
			h, err := NewResolver(
				Config{
					RolloutGetter: &fakeRolloutAccessPoint{
						rollout: tt.rollout,
						err:     tt.rolloutErr,
					},
					CMCGetter: &fakeCMCAuthClient{
						cmc: nil,
						err: trace.NotImplemented("cmc should not be called in this test"),
					},
					Channels: map[string]*automaticupgrades.Channel{
						groupName: tt.channel,
					},
					Log: logtest.NewLogger(),
				})
			require.NoError(t, err)

			// Test execution
			result, err := h.GetVersion(ctx, groupName, "")
			tt.expectError(t, err)
			require.Equal(t, tt.expectedVersion, result)
		})
	}
}

// TestAutoUpdateAgentShouldUpdate also accidentally tests getTriggerFromWindowThenChannel.
func TestAutoUpdateAgentShouldUpdate(t *testing.T) {
	t.Parallel()

	groupName := "test-group"
	ctx := context.Background()

	// brokenChannelUpstream is a buggy upstream version server.
	// This allows us to craft version channels returning errors.
	brokenChannelUpstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
	t.Cleanup(brokenChannelUpstream.Close)

	cacheClock := clockwork.NewFakeClock()
	cmcCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         cmcCacheTTL,
		Clock:       cacheClock,
		Context:     ctx,
		ReloadOnErr: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cmcCache.Shutdown(ctx)
	})

	// We don't use the cache clock because we are advancing it to invalidate the cmc cache and
	// this would interfere with the test logic
	clock := clockwork.NewFakeClock()
	activeUpgradeWindow := types.AgentUpgradeWindow{UTCStartHour: uint32(clock.Now().Hour())}
	inactiveUpgradeWindow := types.AgentUpgradeWindow{UTCStartHour: uint32(clock.Now().Add(2 * time.Hour).Hour())}
	tests := []struct {
		name            string
		rollout         *autoupdatepb.AutoUpdateAgentRollout
		rolloutErr      error
		channel         *automaticupgrades.Channel
		upgradeWindow   types.AgentUpgradeWindow
		cmcErr          error
		windowLookup    bool
		expectedTrigger bool
		expectError     require.ErrorAssertionFunc
	}{
		{
			name: "trigger is looked up from rollout if it is here, trigger firing",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					TargetVersion:  testVersionHigh,
					Schedule:       autoupdate.AgentsScheduleImmediate,
				},
			},
			channel:         &automaticupgrades.Channel{StaticVersion: testVersionLow},
			expectError:     require.NoError,
			expectedTrigger: true,
		},
		{
			name: "trigger is looked up from rollout if it is here, trigger not firing",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
					AutoupdateMode: autoupdate.AgentsUpdateModeDisabled,
					TargetVersion:  testVersionHigh,
					Schedule:       autoupdate.AgentsScheduleImmediate,
				},
			},
			channel:         &automaticupgrades.Channel{StaticVersion: testVersionLow},
			expectError:     require.NoError,
			expectedTrigger: false,
		},
		{
			name:       "trigger is looked up from channel if rollout is not here and window lookup is disabled, trigger not firing",
			rolloutErr: trace.NotFound("rollout is not here"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
				Critical:      false,
			},
			expectError:     require.NoError,
			expectedTrigger: false,
		},
		{
			name:       "trigger is looked up from channel if rollout is not here and window lookup is disabled, trigger firing",
			rolloutErr: trace.NotFound("rollout is not here"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
				Critical:      true,
			},
			expectError:     require.NoError,
			expectedTrigger: true,
		},
		{
			name:       "trigger is looked up from cmc, then channel if rollout is not here and window lookup is enabled, cmc firing",
			rolloutErr: trace.NotFound("rollout is not here"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
				Critical:      false,
			},
			upgradeWindow:   activeUpgradeWindow,
			windowLookup:    true,
			expectError:     require.NoError,
			expectedTrigger: true,
		},
		{
			name:       "trigger is looked up from cmc, then channel if rollout is not here and window lookup is enabled, cmc not firing",
			rolloutErr: trace.NotFound("rollout is not here"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
				Critical:      false,
			},
			upgradeWindow:   inactiveUpgradeWindow,
			windowLookup:    true,
			expectError:     require.NoError,
			expectedTrigger: false,
		},
		{
			name:       "trigger is looked up from cmc, then channel if rollout is not here and window lookup is enabled, cmc not firing but channel firing",
			rolloutErr: trace.NotFound("rollout is not here"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
				Critical:      true,
			},
			upgradeWindow:   inactiveUpgradeWindow,
			windowLookup:    true,
			expectError:     require.NoError,
			expectedTrigger: true,
		},
		{
			name:       "trigger is looked up from cmc, then channel if rollout is not here and window lookup is enabled, no cmc and channel not firing",
			rolloutErr: trace.NotFound("rollout is not here"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
				Critical:      false,
			},
			cmcErr:          trace.NotFound("no cmc for this cluster"),
			windowLookup:    true,
			expectError:     require.NoError,
			expectedTrigger: false,
		},
		{
			name:       "trigger is looked up from cmc, then channel if rollout is not here and window lookup is enabled, no cmc and channel firing",
			rolloutErr: trace.NotFound("rollout is not here"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
				Critical:      true,
			},
			cmcErr:          trace.NotFound("no cmc for this cluster"),
			windowLookup:    true,
			expectError:     require.NoError,
			expectedTrigger: true,
		},
		{
			name:       "hard error getting rollout should not fallback to RFD109 trigger",
			rolloutErr: trace.AccessDenied("something is very broken"),
			channel: &automaticupgrades.Channel{
				StaticVersion: testVersionLow,
			},
			expectError: require.Error,
		},
		{
			name:       "no rollout, error checking channel",
			rolloutErr: trace.NotFound("rollout is not here"),
			channel: &automaticupgrades.Channel{
				ForwardURL: brokenChannelUpstream.URL,
			},
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: building the channel, mock clients, and handler with test config.
			cmc := types.NewClusterMaintenanceConfig()
			cmc.SetAgentUpgradeWindow(tt.upgradeWindow)
			require.NoError(t, tt.channel.CheckAndSetDefaults())
			// Advance cache clock to expire cached cmc
			cacheClock.Advance(2 * cmcCacheTTL)
			h, err := NewResolver(Config{
				RolloutGetter: &fakeRolloutAccessPoint{
					rollout: tt.rollout,
					err:     tt.rolloutErr,
				},
				CMCGetter: &fakeCMCAuthClient{
					cmc: cmc,
					err: tt.cmcErr,
				},
				Channels: map[string]*automaticupgrades.Channel{
					groupName: tt.channel,
				},
				Clock: clock,
				Log:   logtest.NewLogger(),
			})
			require.NoError(t, err)

			// Test execution
			result, err := h.ShouldUpdate(ctx, groupName, "", tt.windowLookup)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedTrigger, result)
		})
	}
}
