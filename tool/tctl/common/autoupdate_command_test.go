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

package common

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/breaker"
	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestClientToolsAutoUpdateCommands verifies all commands related to client auto updates, by
// enabling/disabling auto update, setting the target version and retrieve it.
func TestClientToolsAutoUpdateCommands(t *testing.T) {
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	process := testenv.MakeTestServer(t, testenv.WithLogger(log))
	authClient := testenv.MakeDefaultAuthClient(t, process)

	// Check that AutoUpdateConfig and AutoUpdateVersion are not created.
	_, err := authClient.GetAutoUpdateConfig(ctx)
	require.True(t, trace.IsNotFound(err))
	_, err = authClient.GetAutoUpdateVersion(ctx)
	require.True(t, trace.IsNotFound(err))

	// Enable client tools auto updates to check that AutoUpdateConfig resource is modified.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "enable"})
	require.NoError(t, err)

	config, err := authClient.GetAutoUpdateConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "enabled", config.Spec.Tools.Mode)

	// Disable client tools auto updates to check that AutoUpdateConfig resource is modified.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "disable"})
	require.NoError(t, err)

	config, err = authClient.GetAutoUpdateConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "disabled", config.Spec.Tools.Mode)

	// Set target version for client tools auto updates.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "target", "1.2.3"})
	require.NoError(t, err)

	version, err := authClient.GetAutoUpdateVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", version.Spec.Tools.TargetVersion)

	getBuf, err := runAutoUpdateCommand(t, authClient, []string{"client-tools", "status", "--format=json"})
	require.NoError(t, err)
	response := mustDecodeJSON[getResponse](t, getBuf)
	assert.Equal(t, "1.2.3", response.TargetVersion)
	assert.Equal(t, "disabled", response.Mode)

	// Make same request with proxy flag to read command expecting the same
	// response from `webapi/find` endpoint.
	proxy, err := process.ProxyWebAddr()
	require.NoError(t, err)
	getProxyBuf, err := runAutoUpdateCommand(t, authClient, []string{"client-tools", "status", "--proxy=" + proxy.Addr, "--format=json"})
	require.NoError(t, err)
	response = mustDecodeJSON[getResponse](t, getProxyBuf)
	assert.Equal(t, "1.2.3", response.TargetVersion)
	assert.Equal(t, "disabled", response.Mode)

	// Set clear flag for the target version update to check that it is going to be reset.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "target", "--clear"})
	require.NoError(t, err)
	version, err = authClient.GetAutoUpdateVersion(ctx)
	require.NoError(t, err)
	assert.Nil(t, version.Spec.Tools)
}

func runAutoUpdateCommand(t *testing.T, client *authclient.Client, args []string) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &AutoUpdateCommand{
		stdout: &stdoutBuff,
	}

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	app := utils.InitCLIParser("tctl", GlobalHelpString)
	command.Initialize(app, &tctlcfg.GlobalCLIFlags{Insecure: true}, cfg)

	selectedCmd, err := app.Parse(append([]string{"autoupdate"}, args...))
	require.NoError(t, err)

	_, err = command.TryRun(context.Background(), selectedCmd, func(ctx context.Context) (*authclient.Client, func(context.Context), error) {
		return client, func(context.Context) {}, nil
	})
	return &stdoutBuff, err
}

type mockRolloutClient struct {
	authclient.Client
	mock.Mock
}

func (m *mockRolloutClient) GetAutoUpdateAgentRollout(_ context.Context) (*autoupdatepb.AutoUpdateAgentRollout, error) {
	args := m.Called()
	return args.Get(0).(*autoupdatepb.AutoUpdateAgentRollout), args.Error(1)
}

func TestAutoUpdateAgentStatusCommand(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		fixture        *autoupdatepb.AutoUpdateAgentRollout
		fixtureErr     error
		expectedOutput string
	}{
		{
			name:           "no rollout",
			fixture:        nil,
			fixtureErr:     trace.NotFound("no rollout found"),
			expectedOutput: "No active agent rollout (autoupdate_agent_rollout).\n",
		},
		{
			name: "rollout immediate schedule",
			fixture: &autoupdatepb.AutoUpdateAgentRollout{
				Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleImmediate,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
				},
			},
			expectedOutput: `Agent autoupdate mode: enabled
Start version: 1.2.3
Target version: 1.2.4
Schedule is immediate. Every group immediately updates to the target version.
`,
		},
		{
			name: "rollout regular schedule time-based",
			fixture: &autoupdatepb.AutoUpdateAgentRollout{
				Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
					StartVersion:              "1.2.3",
					TargetVersion:             "1.2.4",
					Schedule:                  autoupdate.AgentsScheduleRegular,
					AutoupdateMode:            autoupdate.AgentsUpdateModeEnabled,
					Strategy:                  autoupdate.AgentsStrategyTimeBased,
					MaintenanceWindowDuration: durationpb.New(1 * time.Hour),
				},
				Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						{
							Name:             "dev",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 12, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
							LastUpdateTime:   nil,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  8,
						},
						{
							Name:             "stage",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 14, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
							LastUpdateReason: "in_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  14,
						},
						{
							Name:             "prod",
							StartTime:        nil,
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  18,
						},
					},
					State:        autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
					StartTime:    timestamppb.New(time.Date(2025, 1, 15, 2, 0, 0, 0, time.UTC)),
					TimeOverride: nil,
				},
			},
			expectedOutput: `Agent autoupdate mode: enabled
Rollout creation date: 2025-01-15 02:00:00
Start version: 1.2.3
Target version: 1.2.4
Rollout state: Active
Strategy: time-based

Group Name State     Start Time          State Reason   
---------- --------- ------------------- -------------- 
dev        Done      2025-01-15 12:00:00 outside_window 
stage      Active    2025-01-15 14:00:00 in_window      
prod       Unstarted                     outside_window 
`,
		},
		{
			name: "rollout regular schedule halt-on-error",
			fixture: &autoupdatepb.AutoUpdateAgentRollout{
				Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyHaltOnError,
				},
				Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						{
							Name:             "dev",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 12, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
							LastUpdateTime:   nil,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  8,
						},
						{
							Name:             "stage",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 14, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
							LastUpdateReason: "in_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  14,
						},
						{
							Name:             "prod",
							StartTime:        nil,
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  18,
						},
					},
					State:        autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
					StartTime:    timestamppb.New(time.Date(2025, 1, 15, 2, 0, 0, 0, time.UTC)),
					TimeOverride: nil,
				},
			},
			expectedOutput: `Agent autoupdate mode: enabled
Rollout creation date: 2025-01-15 02:00:00
Start version: 1.2.3
Target version: 1.2.4
Rollout state: Active
Strategy: halt-on-error

Group Name State     Start Time          State Reason   
---------- --------- ------------------- -------------- 
dev        Done      2025-01-15 12:00:00 outside_window 
stage      Active    2025-01-15 14:00:00 in_window      
prod       Unstarted                     outside_window 
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: create mock client and load fixtures.
			clt := &mockRolloutClient{}
			clt.On("GetAutoUpdateAgentRollout", mock.Anything).Return(tt.fixture, tt.fixtureErr).Once()

			// Test execution: run command.
			output := &bytes.Buffer{}
			cmd := AutoUpdateCommand{stdout: output}
			err := cmd.agentsStatusCommand(ctx, clt)
			require.NoError(t, err)

			// Test validation: check the command output.
			require.Equal(t, tt.expectedOutput, output.String())

			// Test validation: check that the mock received the expected calls.
			clt.AssertExpectations(t)
		})
	}

}
