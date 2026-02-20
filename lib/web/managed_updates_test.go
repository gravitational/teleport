/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/web/ui"
)

// TestGetManagedUpdateDetails tests fetching managed updates details.
func TestGetManagedUpdatesDetails(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Create a role with permissions to read autoupdate resources
	role, err := types.NewRole("testrole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindAutoUpdateConfig, []string{types.VerbRead}),
				types.NewRule(types.KindAutoUpdateVersion, []string{types.VerbRead}),
				types.NewRule(types.KindAutoUpdateAgentRollout, []string{types.VerbRead}),
				types.NewRule(types.KindAutoUpdateAgentReport, []string{types.VerbRead, types.VerbList}),
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	pack := proxy.authPack(t, "testuser", []types.Role{role})

	// Create an auto update config
	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatepb.AutoUpdateConfigSpec{
		Tools: &autoupdatepb.AutoUpdateConfigSpecTools{
			Mode: autoupdate.ToolsUpdateModeEnabled,
		},
		Agents: &autoupdatepb.AutoUpdateConfigSpecAgents{
			Mode:                      autoupdate.AgentsUpdateModeEnabled,
			Strategy:                  autoupdate.AgentsStrategyTimeBased,
			MaintenanceWindowDuration: durationpb.New(time.Hour),
			Schedules: &autoupdatepb.AgentAutoUpdateSchedules{
				Regular: []*autoupdatepb.AgentAutoUpdateGroup{
					{Name: "all", Days: []string{"Mon", "Tue", "Wed"}, StartHour: 14},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	// Create AutoUpdateVersion
	version, err := autoupdate.NewAutoUpdateVersion(&autoupdatepb.AutoUpdateVersionSpec{
		Tools: &autoupdatepb.AutoUpdateVersionSpecTools{
			TargetVersion: "18.2.0",
		},
		Agents: &autoupdatepb.AutoUpdateVersionSpecAgents{
			StartVersion:  "18.1.0",
			TargetVersion: "18.2.0",
			Schedule:      autoupdate.AgentsScheduleRegular,
			Mode:          autoupdate.AgentsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	// Create AutoUpdateAgentRollout
	rollout := &autoupdatepb.AutoUpdateAgentRollout{
		Kind:    types.KindAutoUpdateAgentRollout,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateAgentRollout,
		},
		Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
			StartVersion:   "18.1.0",
			TargetVersion:  "18.2.0",
			Schedule:       autoupdate.AgentsScheduleRegular,
			AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
			Strategy:       autoupdate.AgentsStrategyTimeBased,
		},
		Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
			State: autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
			Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:            "all",
					State:           autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					PresentCount:    100,
					UpToDateCount:   75,
					ConfigDays:      []string{"Mon", "Tue", "Wed"},
					ConfigStartHour: 14,
				},
			},
		},
	}
	_, err = env.server.Auth().UpsertAutoUpdateAgentRollout(ctx, rollout)
	require.NoError(t, err)

	// Make the request
	resp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "managedupdates"), url.Values{})
	require.NoError(t, err)

	var result ui.ManagedUpdatesDetails
	require.NoError(t, json.Unmarshal(resp.Bytes(), &result))

	// Verify tools info
	require.NotNil(t, result.Tools)
	require.Equal(t, autoupdate.ToolsUpdateModeEnabled, result.Tools.Mode)
	require.Equal(t, "18.2.0", result.Tools.TargetVersion)

	// Verify rollout info
	require.NotNil(t, result.Rollout)
	require.Equal(t, "18.1.0", result.Rollout.StartVersion)
	require.Equal(t, "18.2.0", result.Rollout.TargetVersion)
	require.Equal(t, autoupdate.AgentsStrategyTimeBased, result.Rollout.Strategy)
	require.Equal(t, autoupdate.AgentsScheduleRegular, result.Rollout.Schedule)
	require.Equal(t, "active", result.Rollout.State)
	require.Equal(t, autoupdate.AgentsUpdateModeEnabled, result.Rollout.Mode)

	// Verify groups info
	require.Len(t, result.Groups, 1)
	require.Equal(t, "all", result.Groups[0].Name)
	require.Equal(t, "active", result.Groups[0].State)
	require.Equal(t, uint64(100), result.Groups[0].PresentCount)
	require.Equal(t, uint64(75), result.Groups[0].UpToDateCount)

	// Verify cluster maintenance info is not set (not a cloud cluster)
	require.Nil(t, result.ClusterMaintenance)
}

// TestGetOrphanedAgentCounts tests that orphaned agents (agents in a group that doesn't exist) are correctly included in the response.
func TestGetOrphanedAgentCounts(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Create a role with permissions to read autoupdate resources
	role, err := types.NewRole("testrole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindAutoUpdateConfig, []string{types.VerbRead}),
				types.NewRule(types.KindAutoUpdateVersion, []string{types.VerbRead}),
				types.NewRule(types.KindAutoUpdateAgentRollout, []string{types.VerbRead}),
				types.NewRule(types.KindAutoUpdateAgentReport, []string{types.VerbRead, types.VerbList}),
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	pack := proxy.authPack(t, "testuser", []types.Role{role})

	// Create AutoUpdateConfig with "prod" as the only group
	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatepb.AutoUpdateConfigSpec{
		Agents: &autoupdatepb.AutoUpdateConfigSpecAgents{
			Mode:                      autoupdate.AgentsUpdateModeEnabled,
			Strategy:                  autoupdate.AgentsStrategyTimeBased,
			MaintenanceWindowDuration: durationpb.New(time.Hour),
			Schedules: &autoupdatepb.AgentAutoUpdateSchedules{
				Regular: []*autoupdatepb.AgentAutoUpdateGroup{
					{Name: "prod", Days: []string{"*"}, StartHour: 10},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	// Create AutoUpdateVersion
	version, err := autoupdate.NewAutoUpdateVersion(&autoupdatepb.AutoUpdateVersionSpec{
		Agents: &autoupdatepb.AutoUpdateVersionSpecAgents{
			StartVersion:  "18.1.0",
			TargetVersion: "18.2.0",
			Schedule:      autoupdate.AgentsScheduleRegular,
			Mode:          autoupdate.AgentsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	// Create the AutoUpdateAgentRollout
	rollout := &autoupdatepb.AutoUpdateAgentRollout{
		Kind:    types.KindAutoUpdateAgentRollout,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateAgentRollout,
		},
		Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
			StartVersion:   "18.1.0",
			TargetVersion:  "18.2.0",
			Schedule:       autoupdate.AgentsScheduleRegular,
			AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
			Strategy:       autoupdate.AgentsStrategyTimeBased,
		},
		Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
			State: autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
			Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:            "prod",
					State:           autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					PresentCount:    50,
					UpToDateCount:   25,
					ConfigDays:      []string{"*"},
					ConfigStartHour: 10,
				},
			},
		},
	}
	_, err = env.server.Auth().UpsertAutoUpdateAgentRollout(ctx, rollout)
	require.NoError(t, err)

	// Create AutoUpdateAgentReport with some agents in "prod" (valid) and some in "invalidgroup" (orphaned)
	report := &autoupdatepb.AutoUpdateAgentReport{
		Kind:    types.KindAutoUpdateAgentReport,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "test-auth-server",
		},
		Spec: &autoupdatepb.AutoUpdateAgentReportSpec{
			Timestamp: timestamppb.Now(),
			Groups: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroup{
				"prod": {
					Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
						"18.1.0": {Count: 30},
						"18.2.0": {Count: 20},
					},
				},
				"invalidgroup": {
					Versions: map[string]*autoupdatepb.AutoUpdateAgentReportSpecGroupVersion{
						"18.1.0": {Count: 5},
						"18.2.0": {Count: 3},
					},
				},
			},
		},
	}
	_, err = env.server.Auth().UpsertAutoUpdateAgentReport(ctx, report)
	require.NoError(t, err)

	// Make the request
	resp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "managedupdates"), url.Values{})
	require.NoError(t, err)

	var result ui.ManagedUpdatesDetails
	require.NoError(t, json.Unmarshal(resp.Bytes(), &result))

	// Verify that the orphaned agent version counts are present and correct
	require.NotNil(t, result.OrphanedAgentVersionCounts)
	require.Equal(t, 5, result.OrphanedAgentVersionCounts["18.1.0"])
	require.Equal(t, 3, result.OrphanedAgentVersionCounts["18.2.0"])

	// Verify that the prod group agent version counts are present and correct
	require.Len(t, result.Groups, 1)
	require.Equal(t, "prod", result.Groups[0].Name)
	require.Equal(t, 30, result.Groups[0].AgentVersionCounts["18.1.0"])
	require.Equal(t, 20, result.Groups[0].AgentVersionCounts["18.2.0"])
}

// TestStartGroupUpdate tests starting an update for a group.
func TestStartGroupUpdate(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	role, err := types.NewRole("testrole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindAutoUpdateConfig, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
				types.NewRule(types.KindAutoUpdateVersion, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
				types.NewRule(types.KindAutoUpdateAgentRollout, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	pack := proxy.authPack(t, "testuser", []types.Role{role})

	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatepb.AutoUpdateConfigSpec{
		Agents: &autoupdatepb.AutoUpdateConfigSpecAgents{
			Mode:     autoupdate.AgentsUpdateModeEnabled,
			Strategy: autoupdate.AgentsStrategyHaltOnError,
			Schedules: &autoupdatepb.AgentAutoUpdateSchedules{
				Regular: []*autoupdatepb.AgentAutoUpdateGroup{
					{Name: "dev", Days: []string{"*"}, StartHour: 10, WaitHours: 0},
					{Name: "staging", Days: []string{"*"}, StartHour: 10, WaitHours: 24},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	version, err := autoupdate.NewAutoUpdateVersion(&autoupdatepb.AutoUpdateVersionSpec{
		Agents: &autoupdatepb.AutoUpdateVersionSpecAgents{
			StartVersion:  "18.0.0",
			TargetVersion: "18.1.0",
			Schedule:      autoupdate.AgentsScheduleRegular,
			Mode:          autoupdate.AgentsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	rollout := &autoupdatepb.AutoUpdateAgentRollout{
		Kind:    types.KindAutoUpdateAgentRollout,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateAgentRollout,
		},
		Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
			StartVersion:   "18.0.0",
			TargetVersion:  "18.1.0",
			Schedule:       autoupdate.AgentsScheduleRegular,
			AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
			Strategy:       autoupdate.AgentsStrategyHaltOnError,
		},
		Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
			State: autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
			Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:            "dev",
					State:           autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					ConfigDays:      []string{"*"},
					ConfigStartHour: 10,
					ConfigWaitHours: 0,
					CanaryCount:     1,
				},
				{
					Name:            "staging",
					State:           autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					ConfigDays:      []string{"*"},
					ConfigStartHour: 10,
					ConfigWaitHours: 24,
				},
			},
		},
	}
	_, err = env.server.Auth().UpsertAutoUpdateAgentRollout(ctx, rollout)
	require.NoError(t, err)

	// Start the update
	resp, err := pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "dev", "start"), nil)
	require.NoError(t, err)

	// Verify the group goes to canary state
	var result ui.GroupActionResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &result))
	require.NotNil(t, result.Group)
	require.Equal(t, "canary", result.Group.State)

	// Start the update with the force flag set
	resp, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "staging", "start"), ui.StartGroupUpdateRequest{
		Force: true,
	})
	require.NoError(t, err)

	// Verify that the group goes straight to active
	require.NoError(t, json.Unmarshal(resp.Bytes(), &result))
	require.NotNil(t, result.Group)
	require.Equal(t, "staging", result.Group.Name)
	require.Equal(t, "active", result.Group.State)

	// Trying to start a nonexistent group returns an error
	_, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "nonexistent", "start"), nil)
	require.Error(t, err)
}

// TestMarkGroupDone tests marking a managed update group as done.
func TestMarkGroupDone(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	role, err := types.NewRole("testrole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindAutoUpdateConfig, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
				types.NewRule(types.KindAutoUpdateVersion, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
				types.NewRule(types.KindAutoUpdateAgentRollout, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	pack := proxy.authPack(t, "testuser", []types.Role{role})

	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatepb.AutoUpdateConfigSpec{
		Agents: &autoupdatepb.AutoUpdateConfigSpecAgents{
			Mode:     autoupdate.AgentsUpdateModeEnabled,
			Strategy: autoupdate.AgentsStrategyHaltOnError,
			Schedules: &autoupdatepb.AgentAutoUpdateSchedules{
				Regular: []*autoupdatepb.AgentAutoUpdateGroup{
					{Name: "dev", Days: []string{"*"}, StartHour: 10, WaitHours: 0},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	version, err := autoupdate.NewAutoUpdateVersion(&autoupdatepb.AutoUpdateVersionSpec{
		Agents: &autoupdatepb.AutoUpdateVersionSpecAgents{
			StartVersion:  "18.0.0",
			TargetVersion: "18.1.0",
			Schedule:      autoupdate.AgentsScheduleRegular,
			Mode:          autoupdate.AgentsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	rollout := &autoupdatepb.AutoUpdateAgentRollout{
		Kind:    types.KindAutoUpdateAgentRollout,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateAgentRollout,
		},
		Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
			StartVersion:   "18.0.0",
			TargetVersion:  "18.1.0",
			Schedule:       autoupdate.AgentsScheduleRegular,
			AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
			Strategy:       autoupdate.AgentsStrategyHaltOnError,
		},
		Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
			State: autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
			Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:            "dev",
					State:           autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					ConfigDays:      []string{"*"},
					ConfigStartHour: 10,
					ConfigWaitHours: 0,
				},
			},
		},
	}
	_, err = env.server.Auth().UpsertAutoUpdateAgentRollout(ctx, rollout)
	require.NoError(t, err)

	// Force-start the group
	resp, err := pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "dev", "start"), ui.StartGroupUpdateRequest{
		Force: true,
	})
	require.NoError(t, err)

	// Verify that the state is now active
	var result ui.GroupActionResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &result))
	require.NotNil(t, result.Group)
	require.Equal(t, "active", result.Group.State)

	// Mark the group as done
	resp, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "dev", "done"), nil)
	require.NoError(t, err)

	// Verify that the state is now done
	require.NoError(t, json.Unmarshal(resp.Bytes(), &result))
	require.NotNil(t, result.Group)
	require.Equal(t, "dev", result.Group.Name)
	require.Equal(t, "done", result.Group.State)

	// Trying to mark a nonexistent group as done returns an error
	_, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "nonexistent", "done"), nil)
	require.Error(t, err)
}

// TestRollbackGroup tests rolling back a managed update group.
func TestRollbackGroup(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	role, err := types.NewRole("testrole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindAutoUpdateConfig, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
				types.NewRule(types.KindAutoUpdateVersion, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
				types.NewRule(types.KindAutoUpdateAgentRollout, []string{types.VerbRead, types.VerbCreate, types.VerbUpdate}),
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	pack := proxy.authPack(t, "testuser", []types.Role{role})

	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatepb.AutoUpdateConfigSpec{
		Agents: &autoupdatepb.AutoUpdateConfigSpecAgents{
			Mode:     autoupdate.AgentsUpdateModeEnabled,
			Strategy: autoupdate.AgentsStrategyHaltOnError,
			Schedules: &autoupdatepb.AgentAutoUpdateSchedules{
				Regular: []*autoupdatepb.AgentAutoUpdateGroup{
					{Name: "prod", Days: []string{"*"}, StartHour: 10, WaitHours: 0},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	version, err := autoupdate.NewAutoUpdateVersion(&autoupdatepb.AutoUpdateVersionSpec{
		Agents: &autoupdatepb.AutoUpdateVersionSpecAgents{
			StartVersion:  "18.0.0",
			TargetVersion: "18.1.0",
			Schedule:      autoupdate.AgentsScheduleRegular,
			Mode:          autoupdate.AgentsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	rollout := &autoupdatepb.AutoUpdateAgentRollout{
		Kind:    types.KindAutoUpdateAgentRollout,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateAgentRollout,
		},
		Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
			StartVersion:   "18.0.0",
			TargetVersion:  "18.1.0",
			Schedule:       autoupdate.AgentsScheduleRegular,
			AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
			Strategy:       autoupdate.AgentsStrategyHaltOnError,
		},
		Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
			State: autoupdatepb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE,
			Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:            "prod",
					State:           autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					ConfigDays:      []string{"*"},
					ConfigStartHour: 10,
					ConfigWaitHours: 0,
				},
			},
		},
	}
	_, err = env.server.Auth().UpsertAutoUpdateAgentRollout(ctx, rollout)
	require.NoError(t, err)

	// Force-start the group
	resp, err := pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "prod", "start"), ui.StartGroupUpdateRequest{
		Force: true,
	})
	require.NoError(t, err)

	// Verify that the state is now active
	var result ui.GroupActionResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &result))
	require.NotNil(t, result.Group)
	require.Equal(t, "active", result.Group.State)

	// Rollback the group
	resp, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "prod", "rollback"), nil)
	require.NoError(t, err)

	// Verify that the state is now rolledback
	require.NoError(t, json.Unmarshal(resp.Bytes(), &result))
	require.NotNil(t, result.Group)
	require.Equal(t, "prod", result.Group.Name)
	require.Equal(t, "rolledback", result.Group.State)

	// Trying to rollback a nonexistent group returns an error
	_, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "managedupdates", "groups", "nonexistent", "rollback"), nil)
	require.Error(t, err)
}
