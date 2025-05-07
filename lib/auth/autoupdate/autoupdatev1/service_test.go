// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package autoupdatev1

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestAutoUpdateConfigEvents(t *testing.T) {
	rwVerbs := []string{types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete}
	mockEmitter := &eventstest.MockRecorderEmitter{}
	service := newService(t, fakeChecker{allowedVerbs: rwVerbs}, mockEmitter)
	ctx := context.Background()

	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		Tools: &autoupdatev1pb.AutoUpdateConfigSpecTools{
			Mode: autoupdate.ToolsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)

	_, err = service.CreateAutoUpdateConfig(ctx, &autoupdatev1pb.CreateAutoUpdateConfigRequest{Config: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateConfigCreateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateConfigCreateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateConfig, mockEmitter.LastEvent().(*apievents.AutoUpdateConfigCreate).Name)
	mockEmitter.Reset()

	_, err = service.UpdateAutoUpdateConfig(ctx, &autoupdatev1pb.UpdateAutoUpdateConfigRequest{Config: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateConfigUpdateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateConfigUpdateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateConfig, mockEmitter.LastEvent().(*apievents.AutoUpdateConfigUpdate).Name)
	mockEmitter.Reset()

	_, err = service.UpsertAutoUpdateConfig(ctx, &autoupdatev1pb.UpsertAutoUpdateConfigRequest{Config: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateConfigUpdateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateConfigUpdateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateConfig, mockEmitter.LastEvent().(*apievents.AutoUpdateConfigUpdate).Name)
	mockEmitter.Reset()

	_, err = service.DeleteAutoUpdateConfig(ctx, &autoupdatev1pb.DeleteAutoUpdateConfigRequest{})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateConfigDeleteEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateConfigDeleteCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateConfig, mockEmitter.LastEvent().(*apievents.AutoUpdateConfigDelete).Name)
	mockEmitter.Reset()
}

func TestAutoUpdateVersionEvents(t *testing.T) {
	rwVerbs := []string{types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete}
	mockEmitter := &eventstest.MockRecorderEmitter{}
	service := newService(t, fakeChecker{allowedVerbs: rwVerbs}, mockEmitter)
	ctx := context.Background()

	config, err := autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{
		Tools: &autoupdatev1pb.AutoUpdateVersionSpecTools{
			TargetVersion: "1.2.3",
		},
	})
	require.NoError(t, err)

	_, err = service.CreateAutoUpdateVersion(ctx, &autoupdatev1pb.CreateAutoUpdateVersionRequest{Version: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateVersionCreateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateVersionCreateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateVersion, mockEmitter.LastEvent().(*apievents.AutoUpdateVersionCreate).Name)
	mockEmitter.Reset()

	_, err = service.UpdateAutoUpdateVersion(ctx, &autoupdatev1pb.UpdateAutoUpdateVersionRequest{Version: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateVersionUpdateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateVersionUpdateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateVersion, mockEmitter.LastEvent().(*apievents.AutoUpdateVersionUpdate).Name)
	mockEmitter.Reset()

	_, err = service.UpsertAutoUpdateVersion(ctx, &autoupdatev1pb.UpsertAutoUpdateVersionRequest{Version: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateVersionUpdateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateVersionUpdateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateVersion, mockEmitter.LastEvent().(*apievents.AutoUpdateVersionUpdate).Name)
	mockEmitter.Reset()

	_, err = service.DeleteAutoUpdateVersion(ctx, &autoupdatev1pb.DeleteAutoUpdateVersionRequest{})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateVersionDeleteEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateVersionDeleteCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateVersion, mockEmitter.LastEvent().(*apievents.AutoUpdateVersionDelete).Name)
	mockEmitter.Reset()
}

type fakeChecker struct {
	allowedVerbs []string
	builtinRole  *authz.BuiltinRole
	services.AccessChecker
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindAutoUpdateConfig || resource == types.KindAutoUpdateVersion || resource == types.KindAutoUpdateAgentRollout {
		for _, allowedVerb := range f.allowedVerbs {
			if allowedVerb == verb {
				return nil
			}
		}
	}

	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}

func (f fakeChecker) HasRole(name string) bool {
	if f.builtinRole == nil {
		return false
	}
	return name == f.builtinRole.Role.String()
}

func (f fakeChecker) identityGetter() authz.IdentityGetter {
	if f.builtinRole != nil {
		return *f.builtinRole
	}
	return nil
}

func newService(t *testing.T, checker fakeChecker, emitter apievents.Emitter) *Service {
	t.Helper()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	storage, err := local.NewAutoUpdateService(bk)
	require.NoError(t, err)

	return newServiceWithStorage(t, checker, storage, emitter)
}

func newServiceWithStorage(t *testing.T, checker fakeChecker, storage services.AutoUpdateService, emitter apievents.Emitter) *Service {
	t.Helper()

	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser("alice")
		if err != nil {
			return nil, err
		}

		return &authz.Context{
			User:     user,
			Checker:  checker,
			Identity: checker.identityGetter(),
		}, nil
	})

	service, err := NewService(ServiceConfig{
		Authorizer: authorizer,
		Backend:    storage,
		Cache:      storage,
		Emitter:    emitter,
	})
	require.NoError(t, err)
	return service
}

func TestComputeMinRolloutTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		groups        []*autoupdatev1pb.AgentAutoUpdateGroup
		expectedHours int
	}{
		{
			name:          "nil groups",
			groups:        nil,
			expectedHours: 0,
		},
		{
			name:          "empty groups",
			groups:        []*autoupdatev1pb.AgentAutoUpdateGroup{},
			expectedHours: 0,
		},
		{
			name: "single group",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name: "g1",
				},
			},
			expectedHours: 1,
		},
		{
			name: "two groups, same day, different start hour, no wait time",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 2,
				},
				{
					Name:      "g2",
					StartHour: 4,
				},
			},
			// g1 updates from 2:00 to 3:00, g2 updates from 4:00 to 5:00, rollout updates from 2:00 to 5:00.
			expectedHours: 3,
		},
		{
			name: "two groups, same day, same start hour, no wait time",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 2,
				},
				{
					Name:      "g2",
					StartHour: 2,
				},
			},
			// g1 and g2 can't update at the same time, the g1 updates from 2:00 to 3:00 days one,
			// and g2 updates from 2:00 to 3:00 the next day. Total update spans from 2:00 day 1, to 3:00 day 2
			expectedHours: 25,
		},
		{
			name: "two groups, cannot happen on the same day because of wait_hours",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 2,
				},
				{
					Name:      "g2",
					StartHour: 4,
					WaitHours: 6,
				},
			},
			// g1 updates from 2:00 to 3:00. At 4:00 g2 can't update yet, so we wait the next day.
			// On day 2, g2 updates from 4:00 to 5:00. Rollout spans from 2:00 day on to 7:00 day 2.
			expectedHours: 27,
		},
		{
			name: "two groups, wait hours is several days",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 2,
				},
				{
					Name:      "g2",
					StartHour: 4,
					WaitHours: 48,
				},
			},
			// g1 updates from 2:00 to 3:00. At 4:00 g2 can't update yet, so we wait 2 days.
			// On day 3, g2 updates from 4:00 to 5:00. Rollout spans from 2:00 day on to 7:00 day 3.
			expectedHours: 51,
		},
		{
			name: "two groups, one wait hour",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 2,
				},
				{
					Name:      "g2",
					StartHour: 3,
					WaitHours: 1,
				},
			},
			expectedHours: 2,
		},
		{
			name: "two groups different days",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 23,
				},
				{
					Name:      "g2",
					StartHour: 1,
				},
			},
			expectedHours: 3,
		},
		{
			name: "two groups different days, hour diff == wait hours == 1 day",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 12,
				},
				{
					Name:      "g2",
					StartHour: 12,
					WaitHours: 24,
				},
			},
			expectedHours: 25,
		},
		{
			name: "two groups different days, hour diff == wait hours",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 12,
				},
				{
					Name:      "g2",
					StartHour: 11,
					WaitHours: 23,
				},
			},
			expectedHours: 24,
		},
		{
			name: "everything at once",
			groups: []*autoupdatev1pb.AgentAutoUpdateGroup{
				{
					Name:      "g1",
					StartHour: 23,
				},
				{
					Name:      "g2",
					StartHour: 1,
					WaitHours: 4,
				},
				{
					Name:      "g3",
					StartHour: 1,
				},
				{
					Name:      "g4",
					StartHour: 10,
					WaitHours: 6,
				},
			},
			expectedHours: 60,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expectedHours, computeMinRolloutTime(tt.groups))
		})
	}
}

func generateGroups(n int, days []string) []*autoupdatev1pb.AgentAutoUpdateGroup {
	groups := make([]*autoupdatev1pb.AgentAutoUpdateGroup, n)
	for i := range groups {
		groups[i] = &autoupdatev1pb.AgentAutoUpdateGroup{
			Name:      strconv.Itoa(i),
			Days:      days,
			StartHour: int32(i % 24),
		}
	}
	return groups
}

func TestValidateServerSideAgentConfig(t *testing.T) {
	cloudModules := &modules.TestModules{
		TestFeatures: modules.Features{
			Cloud: true,
		},
	}
	selfHostedModules := &modules.TestModules{
		TestFeatures: modules.Features{
			Cloud: false,
		},
	}
	tests := []struct {
		name      string
		config    *autoupdatev1pb.AutoUpdateConfigSpecAgents
		modules   modules.Modules
		expectErr require.ErrorAssertionFunc
	}{
		{
			name:      "empty agent config",
			modules:   selfHostedModules,
			config:    nil,
			expectErr: require.NoError,
		},
		{
			name:    "over max groups time-based",
			modules: selfHostedModules,
			config: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
				Mode:                      autoupdate.AgentsUpdateModeEnabled,
				Strategy:                  autoupdate.AgentsStrategyTimeBased,
				MaintenanceWindowDuration: durationpb.New(time.Hour),
				Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
					Regular: generateGroups(maxGroupsTimeBasedStrategy+1, cloudGroupUpdateDays),
				},
			},
			expectErr: require.Error,
		},
		{
			name:    "over max groups halt-on-error",
			modules: selfHostedModules,
			config: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
				Mode:     autoupdate.AgentsUpdateModeEnabled,
				Strategy: autoupdate.AgentsStrategyHaltOnError,
				Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
					Regular: generateGroups(maxGroupsHaltOnErrorStrategy+1, cloudGroupUpdateDays),
				},
			},
			expectErr: require.Error,
		},
		{
			name:    "over max groups halt-on-error cloud",
			modules: cloudModules,
			config: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
				Mode:     autoupdate.AgentsUpdateModeEnabled,
				Strategy: autoupdate.AgentsStrategyHaltOnError,
				Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
					Regular: generateGroups(maxGroupsHaltOnErrorStrategyCloud+1, cloudGroupUpdateDays),
				},
			},
			expectErr: require.Error,
		},
		{
			name:    "cloud should reject custom weekdays",
			modules: cloudModules,
			config: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
				Mode:     autoupdate.AgentsUpdateModeEnabled,
				Strategy: autoupdate.AgentsStrategyHaltOnError,
				Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
					Regular: generateGroups(maxGroupsHaltOnErrorStrategyCloud, []string{"Mon"}),
				},
			},
			expectErr: require.Error,
		},
		{
			name:    "self-hosted should allow custom weekdays",
			modules: selfHostedModules,
			config: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
				Mode:     autoupdate.AgentsUpdateModeEnabled,
				Strategy: autoupdate.AgentsStrategyHaltOnError,
				Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
					Regular: generateGroups(maxGroupsHaltOnErrorStrategyCloud, []string{"Mon"}),
				},
			},
			expectErr: require.NoError,
		},
		{
			name:    "cloud should reject long rollouts",
			modules: cloudModules,
			config: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
				Mode:     autoupdate.AgentsUpdateModeEnabled,
				Strategy: autoupdate.AgentsStrategyHaltOnError,
				Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
					Regular: []*autoupdatev1pb.AgentAutoUpdateGroup{
						{Name: "g1", Days: cloudGroupUpdateDays},
						{Name: "g2", Days: cloudGroupUpdateDays, WaitHours: maxRolloutDurationCloudHours},
					},
				},
			},
			expectErr: require.Error,
		},
		{
			name:    "self-hosted should allow long rollouts",
			modules: selfHostedModules,
			config: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
				Mode:     autoupdate.AgentsUpdateModeEnabled,
				Strategy: autoupdate.AgentsStrategyHaltOnError,
				Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
					Regular: []*autoupdatev1pb.AgentAutoUpdateGroup{
						{Name: "g1", Days: cloudGroupUpdateDays},
						{Name: "g2", Days: cloudGroupUpdateDays, WaitHours: maxRolloutDurationCloudHours},
					},
				},
			},
			expectErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: crafing a config and setting modules
			config, err := autoupdate.NewAutoUpdateConfig(
				&autoupdatev1pb.AutoUpdateConfigSpec{
					Tools:  nil,
					Agents: tt.config,
				})
			require.NoError(t, err)
			modules.SetTestModules(t, tt.modules)

			// Test execution.
			tt.expectErr(t, validateServerSideAgentConfig(config))
		})
	}
}
