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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestClientToolsAutoUpdateCommands verifies all commands related to client auto updates, by
// enabling/disabling auto update, setting the target version and retrieve it.
func TestClientToolsAutoUpdateCommands(t *testing.T) {
	ctx := context.Background()
	log := logtest.NewLogger()
	process, err := testenv.NewTeleportProcess(t.TempDir(), testenv.WithLogger(log))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})
	authClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authClient.Close() })

	// Check that AutoUpdateConfig and AutoUpdateVersion are not created.
	_, err = authClient.GetAutoUpdateConfig(ctx)
	require.True(t, trace.IsNotFound(err))
	_, err = authClient.GetAutoUpdateVersion(ctx)
	require.True(t, trace.IsNotFound(err))

	// Enable client tools auto updates to check that AutoUpdateConfig resource is modified.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "enable"})
	require.NoError(t, err)

	config, err := authClient.GetAutoUpdateConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "enabled", config.GetSpec().GetTools().GetMode())

	// Disable client tools auto updates to check that AutoUpdateConfig resource is modified.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "disable"})
	require.NoError(t, err)

	config, err = authClient.GetAutoUpdateConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "disabled", config.GetSpec().GetTools().GetMode())

	// Set target version for client tools auto updates.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "target", "1.2.3"})
	require.NoError(t, err)

	version, err := authClient.GetAutoUpdateVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", version.GetSpec().GetTools().GetTargetVersion())

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
	assert.Nil(t, version.GetSpec().GetTools())
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

type mockAutoUpdateClient struct {
	authclient.Client
	mock.Mock
}

func (m *mockAutoUpdateClient) GetAutoUpdateAgentRollout(_ context.Context) (*autoupdatepb.AutoUpdateAgentRollout, error) {
	args := m.Called()
	return args.Get(0).(*autoupdatepb.AutoUpdateAgentRollout), args.Error(1)
}

func (m *mockAutoUpdateClient) ListAutoUpdateAgentReports(_ context.Context, pageSize int, nextKey string) ([]*autoupdatepb.AutoUpdateAgentReport, string, error) {
	args := m.Called()
	return args.Get(0).([]*autoupdatepb.AutoUpdateAgentReport), "", args.Error(1)
}

func (m *mockAutoUpdateClient) TriggerAutoUpdateAgentGroup(_ context.Context, groups []string, state autoupdatepb.AutoUpdateAgentGroupState) (*autoupdatepb.AutoUpdateAgentRollout, error) {
	args := m.Called(groups, state)
	return args.Get(0).(*autoupdatepb.AutoUpdateAgentRollout), args.Error(1)
}

func (m *mockAutoUpdateClient) ForceAutoUpdateAgentGroup(_ context.Context, groups []string) (*autoupdatepb.AutoUpdateAgentRollout, error) {
	args := m.Called(groups)
	return args.Get(0).(*autoupdatepb.AutoUpdateAgentRollout), args.Error(1)
}

func (m *mockAutoUpdateClient) RollbackAutoUpdateAgentGroup(_ context.Context, groups []string, allStartedGroups bool) (*autoupdatepb.AutoUpdateAgentRollout, error) {
	args := m.Called(groups, allStartedGroups)
	return args.Get(0).(*autoupdatepb.AutoUpdateAgentRollout), args.Error(1)
}

func TestAutoUpdateAgentStatusCommand(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

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
			expectedOutput: "No active agent rollout (autoupdate_agent_rollout).\n\n",
		},
		{
			name: "rollout immediate schedule",
			fixture: autoupdatepb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatepb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleImmediate,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
				}.Build(),
			}.Build(),
			expectedOutput: `Agent autoupdate mode: enabled
Start version: 1.2.3
Target version: 1.2.4
Schedule is immediate. Every group immediately updates to the target version.

`,
		},
		{
			name: "rollout regular schedule time-based",
			fixture: autoupdatepb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatepb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:              "1.2.3",
					TargetVersion:             "1.2.4",
					Schedule:                  autoupdate.AgentsScheduleRegular,
					AutoupdateMode:            autoupdate.AgentsUpdateModeEnabled,
					Strategy:                  autoupdate.AgentsStrategyTimeBased,
					MaintenanceWindowDuration: durationpb.New(1 * time.Hour),
				}.Build(),
				Status: autoupdatepb.AutoUpdateAgentRolloutStatus_builder{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "dev",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 12, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
							LastUpdateTime:   nil,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  8,
						}.Build(),
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "stage",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 14, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
							LastUpdateReason: "in_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  14,
						}.Build(),
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "prod",
							StartTime:        nil,
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  18,
						}.Build(),
					},
					State:        autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
					StartTime:    timestamppb.New(time.Date(2025, 1, 15, 2, 0, 0, 0, time.UTC)),
					TimeOverride: nil,
				}.Build(),
			}.Build(),
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
			fixture: autoupdatepb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatepb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyHaltOnError,
				}.Build(),
				Status: autoupdatepb.AutoUpdateAgentRolloutStatus_builder{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "dev",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 12, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
							LastUpdateTime:   nil,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  8,
						}.Build(),
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "stage",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 14, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
							LastUpdateReason: "in_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  14,
						}.Build(),
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "prod",
							StartTime:        nil,
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  18,
						}.Build(),
					},
					State:        autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
					StartTime:    timestamppb.New(time.Date(2025, 1, 15, 2, 0, 0, 0, time.UTC)),
					TimeOverride: nil,
				}.Build(),
			}.Build(),
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
		{
			name: "rollout regular schedule halt-on-error with progress",
			fixture: autoupdatepb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatepb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyHaltOnError,
				}.Build(),
				Status: autoupdatepb.AutoUpdateAgentRolloutStatus_builder{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "dev",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 12, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
							LastUpdateTime:   nil,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  8,
							PresentCount:     1023,
							UpToDateCount:    567,
							InitialCount:     1012,
						}.Build(),
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "stage",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 14, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
							LastUpdateReason: "in_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  14,
						}.Build(),
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "prod",
							StartTime:        nil,
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  18,
							PresentCount:     789,
						}.Build(),
					},
					State:        autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
					StartTime:    timestamppb.New(time.Date(2025, 1, 15, 2, 0, 0, 0, time.UTC)),
					TimeOverride: nil,
				}.Build(),
			}.Build(),
			expectedOutput: `Agent autoupdate mode: enabled
Rollout creation date: 2025-01-15 02:00:00
Start version: 1.2.3
Target version: 1.2.4
Rollout state: Active
Strategy: halt-on-error

Group Name       State     Start Time          State Reason   Agent Count Up-to-date 
---------------- --------- ------------------- -------------- ----------- ---------- 
dev              Done      2025-01-15 12:00:00 outside_window 1023        567        
stage            Active    2025-01-15 14:00:00 in_window      0           0          
prod (catch-all) Unstarted                     outside_window 789         0          
`,
		},
		{
			name: "rollout regular schedule halt-on-error with progress, with canary",
			fixture: autoupdatepb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatepb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyHaltOnError,
				}.Build(),
				Status: autoupdatepb.AutoUpdateAgentRolloutStatus_builder{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "dev",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 12, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
							LastUpdateTime:   nil,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  8,
							PresentCount:     1023,
							UpToDateCount:    567,
							InitialCount:     1012,
						}.Build(),
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "stage",
							StartTime:        timestamppb.New(time.Date(2025, 1, 15, 14, 00, 0, 0, time.UTC)),
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
							LastUpdateReason: "in_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  14,
							CanaryCount:      5,
							Canaries: []*autoupdatepb.Canary{
								autoupdatepb.Canary_builder{
									UpdaterId: uuid.NewString(),
									HostId:    uuid.NewString(),
									Hostname:  "host-1",
									Success:   true,
								}.Build(),
								autoupdatepb.Canary_builder{
									UpdaterId: uuid.NewString(),
									HostId:    uuid.NewString(),
									Hostname:  "host-2",
									Success:   false,
								}.Build(),
								autoupdatepb.Canary_builder{
									UpdaterId: uuid.NewString(),
									HostId:    uuid.NewString(),
									Hostname:  "host-3",
									Success:   true,
								}.Build(),
							},
						}.Build(),
						autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
							Name:             "prod",
							StartTime:        nil,
							State:            autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
							LastUpdateReason: "outside_window",
							ConfigDays:       []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
							ConfigStartHour:  18,
							PresentCount:     789,
						}.Build(),
					},
					State:        autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
					StartTime:    timestamppb.New(time.Date(2025, 1, 15, 2, 0, 0, 0, time.UTC)),
					TimeOverride: nil,
				}.Build(),
			}.Build(),
			expectedOutput: `Agent autoupdate mode: enabled
Rollout creation date: 2025-01-15 02:00:00
Start version: 1.2.3
Target version: 1.2.4
Rollout state: Active
Strategy: halt-on-error

Group Name       State        Start Time          State Reason   Agent Count Up-to-date 
---------------- ------------ ------------------- -------------- ----------- ---------- 
dev              Done         2025-01-15 12:00:00 outside_window 1023        567        
stage            Canary (2/5) 2025-01-15 14:00:00 in_window      0           0          
prod (catch-all) Unstarted                        outside_window 789         0          
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: create mock client and load fixtures.
			clt := &mockAutoUpdateClient{}
			clt.On("GetAutoUpdateAgentRollout", mock.Anything).Return(tt.fixture, tt.fixtureErr).Once()

			// Test execution: run command.
			output := &bytes.Buffer{}
			cmd := AutoUpdateCommand{stdout: output, now: func() time.Time { return now }, format: teleport.Text}
			err := cmd.agentsStatusCommand(ctx, clt)
			require.NoError(t, err)

			// Test validation: check the command output.
			require.Equal(t, tt.expectedOutput, output.String())

			// Test validation: check that the mock received the expected calls.
			clt.AssertExpectations(t)
		})
	}

}

func TestAutoUpdateAgentReportCommand(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	fewSecondsAgo := now.Add(-5 * time.Second)
	fewMinutesAgo := now.Add(-5 * time.Minute)

	tests := []struct {
		name           string
		fixtures       []*autoupdatepb.AutoUpdateAgentReport
		fixturesErr    error
		expectedOutput string
		expectErr      require.ErrorAssertionFunc
	}{
		{
			name:           "no agent report",
			fixtures:       nil,
			fixturesErr:    trace.NotFound("no agent report"),
			expectedOutput: "No autoupdate_agent_report found.\n",
			expectErr:      require.Error,
		},
		{
			name: "only expired agent reports",
			fixtures: []*autoupdatepb.AutoUpdateAgentReport{
				autoupdatepb.AutoUpdateAgentReport_builder{
					Metadata: headerv1.Metadata_builder{Name: "auth1"}.Build(),
					Spec: autoupdatepb.AutoUpdateAgentReportSpec_builder{
						Timestamp: timestamppb.New(fewMinutesAgo),
						Groups: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroup{
							"dev": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
									"1.2.4": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 234}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
				autoupdatepb.AutoUpdateAgentReport_builder{
					Metadata: headerv1.Metadata_builder{Name: "auth2"}.Build(),
					Spec: autoupdatepb.AutoUpdateAgentReportSpec_builder{
						Timestamp: timestamppb.New(fewMinutesAgo),
						Groups: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroup{
							"dev": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 456}.Build(),
									"1.2.4": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 567}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			expectedOutput: "Read 2 reports, but they are expired. If you just (re)deployed the Auth service, you might want to retry after 60 seconds.\n",
			expectErr:      require.Error,
		},
		{
			name: "valid reports",
			fixtures: []*autoupdatepb.AutoUpdateAgentReport{
				autoupdatepb.AutoUpdateAgentReport_builder{
					Metadata: headerv1.Metadata_builder{Name: "auth1"}.Build(),
					Spec: autoupdatepb.AutoUpdateAgentReportSpec_builder{
						Timestamp: timestamppb.New(fewSecondsAgo),
						Groups: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroup{
							"dev": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
									"1.2.4": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 234}.Build(),
								},
							}.Build(),
							"stage": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
				autoupdatepb.AutoUpdateAgentReport_builder{
					Metadata: headerv1.Metadata_builder{Name: "auth2"}.Build(),
					Spec: autoupdatepb.AutoUpdateAgentReportSpec_builder{
						Timestamp: timestamppb.New(fewSecondsAgo),
						Groups: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroup{
							"dev": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 456}.Build(),
									"1.2.4": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 567}.Build(),
								},
							}.Build(),
							"prod": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 789}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			expectErr: require.NoError,
			expectedOutput: `2 autoupdate agent reports aggregated

Agent Version dev  prod stage 
------------- ---  ---- ----- 
1.2.3         579  789  123   
1.2.4         801  0    0     
`,
		},
		{
			name: "valid reports with omissions",
			fixtures: []*autoupdatepb.AutoUpdateAgentReport{
				autoupdatepb.AutoUpdateAgentReport_builder{
					Metadata: headerv1.Metadata_builder{Name: "auth1"}.Build(),
					Spec: autoupdatepb.AutoUpdateAgentReportSpec_builder{
						Timestamp: timestamppb.New(fewSecondsAgo),
						Groups: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroup{
							"dev": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
									"1.2.4": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 234}.Build(),
								},
							}.Build(),
							"stage": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
								},
							}.Build(),
						},
						Omitted: []*autoupdatepb.AutoUpdateAgentReportSpecOmitted{
							autoupdatepb.AutoUpdateAgentReportSpecOmitted_builder{Reason: "agent is too old", Count: 2}.Build(),
						},
					}.Build(),
				}.Build(),
				autoupdatepb.AutoUpdateAgentReport_builder{
					Metadata: headerv1.Metadata_builder{Name: "auth2"}.Build(),
					Spec: autoupdatepb.AutoUpdateAgentReportSpec_builder{
						Timestamp: timestamppb.New(fewSecondsAgo),
						Groups: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroup{
							"dev": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 456}.Build(),
									"1.2.4": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 567}.Build(),
								},
							}.Build(),
							"prod": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
									"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 789}.Build(),
								},
							}.Build(),
						},
						Omitted: []*autoupdatepb.AutoUpdateAgentReportSpecOmitted{
							autoupdatepb.AutoUpdateAgentReportSpecOmitted_builder{Reason: "updater is disabled", Count: 5}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			expectErr: require.NoError,
			expectedOutput: `2 autoupdate agent reports aggregated

Agent Version dev  prod stage 
------------- ---  ---- ----- 
1.2.3         579  789  123   
1.2.4         801  0    0     

7 agents were omitted from the reports:
- 2 omitted because: agent is too old
- 5 omitted because: updater is disabled
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: create mock client and load fixtures.
			clt := &mockAutoUpdateClient{}
			clt.On("ListAutoUpdateAgentReports", mock.Anything, mock.Anything, mock.Anything).Return(tt.fixtures, tt.fixturesErr).Once()

			// Test execution: run command.
			output := &bytes.Buffer{}
			cmd := AutoUpdateCommand{stdout: output, now: func() time.Time { return now }, format: teleport.Text}
			err := cmd.agentsReportCommand(ctx, clt)
			tt.expectErr(t, err)

			// Test validation: check the command output.
			require.Equal(t, tt.expectedOutput, output.String())

			// Test validation: check that the mock received the expected calls.
			clt.AssertExpectations(t)
		})
	}

}

func TestAutoUpdateAgentStatusStructuredOutput(t *testing.T) {
	ctx := t.Context()
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	rollout := autoupdatepb.AutoUpdateAgentRollout_builder{
		Spec: autoupdatepb.AutoUpdateAgentRolloutSpec_builder{
			AutoupdateMode: "enabled",
			StartVersion:   "1.2.3",
			TargetVersion:  "1.2.4",
			Schedule:       "regular",
			Strategy:       "time-based",
		}.Build(),
		Status: autoupdatepb.AutoUpdateAgentRolloutStatus_builder{
			StartTime: timestamppb.New(now),
			State:     autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
			Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:          "dev",
					State:         autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					PresentCount:  3,
					UpToDateCount: 2,
				}.Build(),
			},
		}.Build(),
	}.Build()

	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			clt := &mockAutoUpdateClient{}
			clt.On("GetAutoUpdateAgentRollout", mock.Anything).Return(rollout, nil).Once()

			output := &bytes.Buffer{}
			cmd := AutoUpdateCommand{stdout: output, format: format}
			require.NoError(t, cmd.agentsStatusCommand(ctx, clt))

			var got agentStatusOutput
			if format == "yaml" {
				got = mustDecodeJSON[agentStatusOutput](t, bytes.NewReader(mustTranscodeYAMLToJSON(t, output)))
			} else {
				got = mustDecodeJSON[agentStatusOutput](t, output)
			}
			require.Equal(t, newAgentStatusOutput(rollout), got)
			clt.AssertExpectations(t)
		})
	}
}

func TestAutoUpdateAgentReportStructuredOutput(t *testing.T) {
	ctx := t.Context()
	now := time.Now()
	reports := []*autoupdatepb.AutoUpdateAgentReport{
		autoupdatepb.AutoUpdateAgentReport_builder{
			Metadata: headerv1.Metadata_builder{Name: "auth1"}.Build(),
			Spec: autoupdatepb.AutoUpdateAgentReportSpec_builder{
				Timestamp: timestamppb.New(now),
				Groups: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroup{
					"dev": autoupdatepb.AutoUpdateAgentReportSpecGroup_builder{
						Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.3": autoupdatepb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 2}.Build(),
						},
					}.Build(),
				},
				Omitted: []*autoupdatepb.AutoUpdateAgentReportSpecOmitted{
					autoupdatepb.AutoUpdateAgentReportSpecOmitted_builder{Reason: "agent is too old", Count: 1}.Build(),
				},
			}.Build(),
		}.Build(),
	}

	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			clt := &mockAutoUpdateClient{}
			clt.On("ListAutoUpdateAgentReports", mock.Anything, mock.Anything, mock.Anything).Return(reports, nil).Once()

			output := &bytes.Buffer{}
			cmd := AutoUpdateCommand{stdout: output, now: func() time.Time { return now }, format: format}
			require.NoError(t, cmd.agentsReportCommand(ctx, clt))

			var got agentReportSummary
			if format == "yaml" {
				got = mustDecodeJSON[agentReportSummary](t, bytes.NewReader(mustTranscodeYAMLToJSON(t, output)))
			} else {
				got = mustDecodeJSON[agentReportSummary](t, output)
			}
			require.Equal(t, newAgentReportSummary(reports), got)
			clt.AssertExpectations(t)
		})
	}
}

// rolloutFixture is the rollout returned by the mocked rollout mutation
// methods, used to verify structured output of start-update/mark-done/rollback.
func rolloutFixture() *autoupdatepb.AutoUpdateAgentRollout {
	return autoupdatepb.AutoUpdateAgentRollout_builder{
		Spec: autoupdatepb.AutoUpdateAgentRolloutSpec_builder{
			AutoupdateMode: "enabled",
			StartVersion:   "1.2.3",
			TargetVersion:  "1.2.4",
			Schedule:       "regular",
			Strategy:       "time-based",
		}.Build(),
		Status: autoupdatepb.AutoUpdateAgentRolloutStatus_builder{
			State: autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
			Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				autoupdatepb.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:          "dev",
					State:         autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					PresentCount:  3,
					UpToDateCount: 2,
				}.Build(),
			},
		}.Build(),
	}.Build()
}

func TestAutoUpdateAgentRolloutMutationStructuredOutput(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	rollout := rolloutFixture()

	// Each rollout mutation command shares the same structured output shape, so
	// we exercise all three against json and yaml, plus an invalid format.
	tests := []struct {
		name   string
		groups []string
		setup  func(clt *mockAutoUpdateClient)
		run    func(cmd *AutoUpdateCommand, clt autoupdateClient) error
	}{
		{
			name:   "start-update",
			groups: []string{"dev"},
			setup: func(clt *mockAutoUpdateClient) {
				clt.On("TriggerAutoUpdateAgentGroup", mock.Anything, mock.Anything).Return(rollout, nil).Once()
			},
			run: func(cmd *AutoUpdateCommand, clt autoupdateClient) error {
				return cmd.agentsStartUpdateCommand(ctx, clt)
			},
		},
		{
			name: "mark-done",
			setup: func(clt *mockAutoUpdateClient) {
				clt.On("ForceAutoUpdateAgentGroup", mock.Anything).Return(rollout, nil).Once()
			},
			run: func(cmd *AutoUpdateCommand, clt autoupdateClient) error {
				return cmd.agentsMarkDoneCommand(ctx, clt)
			},
		},
		{
			name: "rollback",
			setup: func(clt *mockAutoUpdateClient) {
				clt.On("RollbackAutoUpdateAgentGroup", mock.Anything, mock.Anything).Return(rollout, nil).Once()
			},
			run: func(cmd *AutoUpdateCommand, clt autoupdateClient) error {
				return cmd.agentsRollbackCommand(ctx, clt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, format := range []string{teleport.JSON, teleport.YAML} {
				t.Run(format, func(t *testing.T) {
					clt := &mockAutoUpdateClient{}
					tt.setup(clt)

					output := &bytes.Buffer{}
					cmd := AutoUpdateCommand{stdout: output, format: format, groups: tt.groups}
					require.NoError(t, tt.run(&cmd, clt))

					var got agentStatusOutput
					if format == teleport.YAML {
						got = mustDecodeJSON[agentStatusOutput](t, bytes.NewReader(mustTranscodeYAMLToJSON(t, output)))
					} else {
						got = mustDecodeJSON[agentStatusOutput](t, output)
					}
					require.Equal(t, newAgentStatusOutput(rollout), got)
					clt.AssertExpectations(t)
				})
			}

			t.Run("invalid format", func(t *testing.T) {
				clt := &mockAutoUpdateClient{}
				tt.setup(clt)

				cmd := AutoUpdateCommand{stdout: &bytes.Buffer{}, format: "bogus", groups: tt.groups}
				err := tt.run(&cmd, clt)
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
			})
		})
	}
}
