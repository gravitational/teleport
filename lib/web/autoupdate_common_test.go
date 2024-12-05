/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
	"github.com/gravitational/teleport/lib/utils"
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
		expectedVersion string
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
			expectedVersion: testVersionHigh,
		},
		{
			name:            "version is looked up from channel if rollout is not here",
			rolloutErr:      trace.NotFound("rollout is not here"),
			channel:         &automaticupgrades.Channel{StaticVersion: testVersionLow},
			expectError:     require.NoError,
			expectedVersion: testVersionLow,
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
			h := &Handler{
				cfg: Config{
					AccessPoint: &fakeRolloutAccessPoint{
						rollout: tt.rollout,
						err:     tt.rolloutErr,
					},
					AutomaticUpgradesChannels: map[string]*automaticupgrades.Channel{
						groupName: tt.channel,
					},
				},
			}

			// Test execution
			result, err := h.autoUpdateAgentVersion(ctx, groupName, "")
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

	clock := clockwork.NewFakeClock()
	cmcCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         findEndpointCacheTTL,
		Clock:       clock,
		Context:     ctx,
		ReloadOnErr: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cmcCache.Shutdown(ctx)
	})

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
			// Advance clock to invalidate cache
			clock.Advance(2 * findEndpointCacheTTL)
			h := &Handler{
				cfg: Config{
					AccessPoint: &fakeRolloutAccessPoint{
						rollout: tt.rollout,
						err:     tt.rolloutErr,
					},
					ProxyClient: &fakeCMCAuthClient{
						cmc: cmc,
						err: tt.cmcErr,
					},
					AutomaticUpgradesChannels: map[string]*automaticupgrades.Channel{
						groupName: tt.channel,
					},
				},
				clock:                         clock,
				clusterMaintenanceConfigCache: cmcCache,
			}

			// Test execution
			result, err := h.autoUpdateAgentShouldUpdate(ctx, groupName, "", tt.windowLookup)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedTrigger, result)
		})
	}
}

func TestGetVersionFromRollout(t *testing.T) {
	t.Parallel()
	groupName := "test-group"

	// This test matrix is written based on:
	// https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md#rollout-status-disabled
	latestAllTheTime := map[autoupdatepb.AutoUpdateAgentGroupState]string{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: testVersionHigh,
	}

	activeDoneOnly := map[autoupdatepb.AutoUpdateAgentGroupState]string{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  testVersionLow,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: testVersionLow,
	}

	tests := map[string]map[string]map[autoupdatepb.AutoUpdateAgentGroupState]string{
		autoupdate.AgentsUpdateModeDisabled: {
			autoupdate.AgentsScheduleImmediate: latestAllTheTime,
			autoupdate.AgentsScheduleRegular:   latestAllTheTime,
		},
		autoupdate.AgentsUpdateModeSuspended: {
			autoupdate.AgentsScheduleImmediate: latestAllTheTime,
			autoupdate.AgentsScheduleRegular:   activeDoneOnly,
		},
		autoupdate.AgentsUpdateModeEnabled: {
			autoupdate.AgentsScheduleImmediate: latestAllTheTime,
			autoupdate.AgentsScheduleRegular:   activeDoneOnly,
		},
	}
	for mode, scheduleCases := range tests {
		for schedule, stateCases := range scheduleCases {
			for state, expectedVersion := range stateCases {
				t.Run(fmt.Sprintf("%s/%s/%s", mode, schedule, state), func(t *testing.T) {
					rollout := &autoupdatepb.AutoUpdateAgentRollout{
						Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
							StartVersion:   testVersionLow,
							TargetVersion:  testVersionHigh,
							Schedule:       schedule,
							AutoupdateMode: mode,
							// Strategy does not affect which version are served
							Strategy: autoupdate.AgentsStrategyTimeBased,
						},
						Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
							Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
								{
									Name:  groupName,
									State: state,
								},
							},
						},
					}
					version, err := getVersionFromRollout(rollout, groupName, "")
					require.NoError(t, err)
					require.Equal(t, expectedVersion, version)
				})
			}
		}
	}
}

func TestGetTriggerFromRollout(t *testing.T) {
	t.Parallel()
	groupName := "test-group"

	// This test matrix is written based on:
	// https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md#rollout-status-disabled
	neverUpdate := map[autoupdatepb.AutoUpdateAgentGroupState]bool{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       false,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     false,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: false,
	}
	alwaysUpdate := map[autoupdatepb.AutoUpdateAgentGroupState]bool{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  true,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       true,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
	}

	tests := map[string]map[string]map[string]map[autoupdatepb.AutoUpdateAgentGroupState]bool{
		autoupdate.AgentsUpdateModeDisabled: {
			autoupdate.AgentsStrategyTimeBased: {
				autoupdate.AgentsScheduleImmediate: neverUpdate,
				autoupdate.AgentsScheduleRegular:   neverUpdate,
			},
			autoupdate.AgentsStrategyHaltOnError: {
				autoupdate.AgentsScheduleImmediate: neverUpdate,
				autoupdate.AgentsScheduleRegular:   neverUpdate,
			},
		},
		autoupdate.AgentsUpdateModeSuspended: {
			autoupdate.AgentsStrategyTimeBased: {
				autoupdate.AgentsScheduleImmediate: neverUpdate,
				autoupdate.AgentsScheduleRegular:   neverUpdate,
			},
			autoupdate.AgentsStrategyHaltOnError: {
				autoupdate.AgentsScheduleImmediate: neverUpdate,
				autoupdate.AgentsScheduleRegular:   neverUpdate,
			},
		},
		autoupdate.AgentsUpdateModeEnabled: {
			autoupdate.AgentsStrategyTimeBased: {
				autoupdate.AgentsScheduleImmediate: alwaysUpdate,
				autoupdate.AgentsScheduleRegular: {
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       false,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
				},
			},
			autoupdate.AgentsStrategyHaltOnError: {
				autoupdate.AgentsScheduleImmediate: alwaysUpdate,
				autoupdate.AgentsScheduleRegular: {
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       true,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
				},
			},
		},
	}
	for mode, strategyCases := range tests {
		for strategy, scheduleCases := range strategyCases {
			for schedule, stateCases := range scheduleCases {
				for state, expectedTrigger := range stateCases {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", mode, strategy, schedule, state), func(t *testing.T) {
						rollout := &autoupdatepb.AutoUpdateAgentRollout{
							Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
								StartVersion:   testVersionLow,
								TargetVersion:  testVersionHigh,
								Schedule:       schedule,
								AutoupdateMode: mode,
								Strategy:       strategy,
							},
							Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
								Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
									{
										Name:  groupName,
										State: state,
									},
								},
							},
						}
						shouldUpdate, err := getTriggerFromRollout(rollout, groupName, "")
						require.NoError(t, err)
						require.Equal(t, expectedTrigger, shouldUpdate)
					})
				}
			}
		}
	}
}

func TestGetGroup(t *testing.T) {
	groupName := "test-group"
	t.Parallel()
	tests := []struct {
		name           string
		rollout        *autoupdatepb.AutoUpdateAgentRollout
		expectedResult *autoupdatepb.AutoUpdateAgentRolloutStatusGroup
		expectError    require.ErrorAssertionFunc
	}{
		{
			name:        "nil",
			expectError: require.Error,
		},
		{
			name:        "nil status",
			rollout:     &autoupdatepb.AutoUpdateAgentRollout{},
			expectError: require.Error,
		},
		{
			name:        "nil status groups",
			rollout:     &autoupdatepb.AutoUpdateAgentRollout{Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{}},
			expectError: require.Error,
		},
		{
			name: "empty status groups",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{},
				},
			},
			expectError: require.Error,
		},
		{
			name: "group matching name",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						{Name: "foo", State: 1},
						{Name: "bar", State: 1},
						{Name: groupName, State: 2},
						{Name: "baz", State: 1},
					},
				},
			},
			expectedResult: &autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				Name:  groupName,
				State: 2,
			},
			expectError: require.NoError,
		},
		{
			name: "no group matching name, should fallback to default",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						{Name: "foo", State: 1},
						{Name: "bar", State: 1},
						{Name: "baz", State: 1},
					},
				},
			},
			expectedResult: &autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				Name:  "baz",
				State: 1,
			},
			expectError: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getGroup(tt.rollout, groupName)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

type mockRFD109VersionServer struct {
	t        *testing.T
	channels map[string]channelStub
}

type channelStub struct {
	// with our without the leading "v"
	version  string
	critical bool
	fail     bool
}

func (m *mockRFD109VersionServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var path string
	var writeResp func(w http.ResponseWriter, stub channelStub) error

	switch {
	case strings.HasSuffix(r.URL.Path, constants.VersionPath):
		path = strings.Trim(strings.TrimSuffix(r.URL.Path, constants.VersionPath), "/")
		writeResp = func(w http.ResponseWriter, stub channelStub) error {
			_, err := w.Write([]byte(stub.version))
			return err
		}
	case strings.HasSuffix(r.URL.Path, constants.MaintenancePath):
		path = strings.Trim(strings.TrimSuffix(r.URL.Path, constants.MaintenancePath), "/")
		writeResp = func(w http.ResponseWriter, stub channelStub) error {
			response := "no"
			if stub.critical {
				response = "yes"
			}
			_, err := w.Write([]byte(response))
			return err
		}
	default:
		assert.Fail(m.t, "unsupported path %q", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	channel, ok := m.channels[path]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		assert.Fail(m.t, "channel %q not found", path)
		return
	}
	if channel.fail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	assert.NoError(m.t, writeResp(w, channel), "failed to write response")
}

func TestGetVersionFromChannel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	channelName := "test-channel"

	mock := mockRFD109VersionServer{
		t: t,
		channels: map[string]channelStub{
			"broken":            {fail: true},
			"with-leading-v":    {version: "v" + testVersionHigh},
			"without-leading-v": {version: testVersionHigh},
			"low":               {version: testVersionLow},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(mock.ServeHTTP))
	t.Cleanup(srv.Close)

	tests := []struct {
		name           string
		channels       automaticupgrades.Channels
		expectedResult string
		expectError    require.ErrorAssertionFunc
	}{
		{
			name: "channel with leading v",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/with-leading-v"},
				"default":   {ForwardURL: srv.URL + "/low"},
			},
			expectedResult: testVersionHigh,
			expectError:    require.NoError,
		},
		{
			name: "channel without leading v",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/without-leading-v"},
				"default":   {ForwardURL: srv.URL + "/low"},
			},
			expectedResult: testVersionHigh,
			expectError:    require.NoError,
		},
		{
			name: "fallback to default with leading v",
			channels: automaticupgrades.Channels{
				"default": {ForwardURL: srv.URL + "/with-leading-v"},
			},
			expectedResult: testVersionHigh,
			expectError:    require.NoError,
		},
		{
			name: "fallback to default without leading v",
			channels: automaticupgrades.Channels{
				"default": {ForwardURL: srv.URL + "/without-leading-v"},
			},
			expectedResult: testVersionHigh,
			expectError:    require.NoError,
		},
		{
			name: "broken channel",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/broken"},
				"default":   {ForwardURL: srv.URL + "/without-leading-v"},
			},
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup
			require.NoError(t, tt.channels.CheckAndSetDefaults())

			// Test execution
			result, err := getVersionFromChannel(ctx, tt.channels, channelName)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetTriggerFromChannel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	channelName := "test-channel"

	mock := mockRFD109VersionServer{
		t: t,
		channels: map[string]channelStub{
			"broken":       {fail: true},
			"critical":     {critical: true},
			"non-critical": {critical: false},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(mock.ServeHTTP))
	t.Cleanup(srv.Close)

	tests := []struct {
		name           string
		channels       automaticupgrades.Channels
		expectedResult bool
		expectError    require.ErrorAssertionFunc
	}{
		{
			name: "critical channel",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/critical"},
				"default":   {ForwardURL: srv.URL + "/non-critical"},
			},
			expectedResult: true,
			expectError:    require.NoError,
		},
		{
			name: "non-critical channel",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/non-critical"},
				"default":   {ForwardURL: srv.URL + "/critical"},
			},
			expectedResult: false,
			expectError:    require.NoError,
		},
		{
			name: "fallback to default which is critical",
			channels: automaticupgrades.Channels{
				"default": {ForwardURL: srv.URL + "/critical"},
			},
			expectedResult: true,
			expectError:    require.NoError,
		},
		{
			name: "fallback to default which is non-critical",
			channels: automaticupgrades.Channels{
				"default": {ForwardURL: srv.URL + "/non-critical"},
			},
			expectedResult: false,
			expectError:    require.NoError,
		},
		{
			name: "broken channel",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/broken"},
				"default":   {ForwardURL: srv.URL + "/critical"},
			},
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup
			require.NoError(t, tt.channels.CheckAndSetDefaults())

			// Test execution
			result, err := getTriggerFromChannel(ctx, tt.channels, channelName)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}
