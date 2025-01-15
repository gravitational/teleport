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

package rollout

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	update "github.com/gravitational/teleport/api/types/autoupdate"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

// rolloutEquals returns a require.ValueAssertionFunc that checks the rollout is identical.
// The comparison does not take into account the proto internal state.
func rolloutEquals(expected *autoupdate.AutoUpdateAgentRollout) require.ValueAssertionFunc {
	return func(t require.TestingT, i interface{}, _ ...interface{}) {
		require.IsType(t, &autoupdate.AutoUpdateAgentRollout{}, i, "resource should be an autoupdate_agent_rollout")
		actual := i.(*autoupdate.AutoUpdateAgentRollout)
		require.Empty(t, cmp.Diff(expected, actual, protocmp.Transform()))
	}
}

// cancelContext wraps a require.ValueAssertionFunc so that the given context is canceled before checking the assertion.
// This is used to test how the reconciler behaves when its context is canceled.
func cancelContext(assertionFunc require.ValueAssertionFunc, cancel func()) require.ValueAssertionFunc {
	return func(t require.TestingT, i interface{}, i2 ...interface{}) {
		cancel()
		assertionFunc(t, i, i2...)
	}
}

// withRevisionID creates a deep copy of an agent rollout and sets the revisionID in its metadata.
// This is used to test the conditional update retry logic.
func withRevisionID(original *autoupdate.AutoUpdateAgentRollout, revision string) *autoupdate.AutoUpdateAgentRollout {
	revisioned := apiutils.CloneProtoMsg(original)
	revisioned.Metadata.Revision = revision
	return revisioned
}

func TestGetMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		configMode  string
		versionMode string
		expected    string
		checkErr    require.ErrorAssertionFunc
	}{
		{
			name:        "config and version equal",
			configMode:  update.AgentsUpdateModeEnabled,
			versionMode: update.AgentsUpdateModeEnabled,
			expected:    update.AgentsUpdateModeEnabled,
			checkErr:    require.NoError,
		},
		{
			name:        "config suspends, version enables",
			configMode:  update.AgentsUpdateModeSuspended,
			versionMode: update.AgentsUpdateModeEnabled,
			expected:    update.AgentsUpdateModeSuspended,
			checkErr:    require.NoError,
		},
		{
			name:        "config enables, version suspends",
			configMode:  update.AgentsUpdateModeEnabled,
			versionMode: update.AgentsUpdateModeSuspended,
			expected:    update.AgentsUpdateModeSuspended,
			checkErr:    require.NoError,
		},
		{
			name:        "config suspends, version disables",
			configMode:  update.AgentsUpdateModeSuspended,
			versionMode: update.AgentsUpdateModeDisabled,
			expected:    update.AgentsUpdateModeDisabled,
			checkErr:    require.NoError,
		},
		{
			name:        "version enables, no config",
			configMode:  "",
			versionMode: update.AgentsUpdateModeEnabled,
			expected:    update.AgentsUpdateModeEnabled,
			checkErr:    require.NoError,
		},
		{
			name:        "config enables, no version",
			configMode:  update.AgentsUpdateModeEnabled,
			versionMode: "",
			expected:    "",
			checkErr:    require.Error,
		},
		{
			name:        "unknown mode",
			configMode:  "this in not a mode",
			versionMode: update.AgentsUpdateModeEnabled,
			expected:    "",
			checkErr:    require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getMode(tt.configMode, tt.versionMode)
			tt.checkErr(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestTryReconcile(t *testing.T) {
	t.Parallel()
	log := utils.NewSlogLoggerForTests()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	// Test setup: creating fixtures
	configOK, err := update.NewAutoUpdateConfig(&autoupdate.AutoUpdateConfigSpec{
		Tools: &autoupdate.AutoUpdateConfigSpecTools{
			Mode: update.ToolsUpdateModeEnabled,
		},
		Agents: &autoupdate.AutoUpdateConfigSpecAgents{
			Mode:     update.AgentsUpdateModeEnabled,
			Strategy: update.AgentsStrategyHaltOnError,
		},
	})
	require.NoError(t, err)

	configNoAgent, err := update.NewAutoUpdateConfig(&autoupdate.AutoUpdateConfigSpec{
		Tools: &autoupdate.AutoUpdateConfigSpecTools{
			Mode: update.ToolsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)

	versionOK, err := update.NewAutoUpdateVersion(&autoupdate.AutoUpdateVersionSpec{
		Tools: &autoupdate.AutoUpdateVersionSpecTools{
			TargetVersion: "1.2.3",
		},
		Agents: &autoupdate.AutoUpdateVersionSpecAgents{
			StartVersion:  "1.2.3",
			TargetVersion: "1.2.4",
			Schedule:      update.AgentsScheduleImmediate,
			Mode:          update.AgentsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)

	versionNoAgent, err := update.NewAutoUpdateVersion(&autoupdate.AutoUpdateVersionSpec{
		Tools: &autoupdate.AutoUpdateVersionSpecTools{
			TargetVersion: "1.2.3",
		},
	})
	require.NoError(t, err)

	upToDateRollout, err := update.NewAutoUpdateAgentRollout(&autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "1.2.4",
		Schedule:       update.AgentsScheduleImmediate,
		AutoupdateMode: update.AgentsUpdateModeEnabled,
		Strategy:       update.AgentsStrategyHaltOnError,
	})
	require.NoError(t, err)
	upToDateRollout.Status = &autoupdate.AutoUpdateAgentRolloutStatus{StartTime: timestamppb.New(clock.Now())}

	outOfDateRollout, err := update.NewAutoUpdateAgentRollout(&autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.2",
		TargetVersion:  "1.2.3",
		Schedule:       update.AgentsScheduleImmediate,
		AutoupdateMode: update.AgentsUpdateModeEnabled,
		Strategy:       update.AgentsStrategyHaltOnError,
	})
	require.NoError(t, err)
	outOfDateRollout.Status = &autoupdate.AutoUpdateAgentRolloutStatus{}

	tests := []struct {
		name            string
		config          *autoupdate.AutoUpdateConfig
		version         *autoupdate.AutoUpdateVersion
		existingRollout *autoupdate.AutoUpdateAgentRollout
		createExpect    *autoupdate.AutoUpdateAgentRollout
		updateExpect    *autoupdate.AutoUpdateAgentRollout
		deleteExpect    bool
	}{
		{
			name: "config and version exist, no existing rollout",
			// rollout should be created
			config:       configOK,
			version:      versionOK,
			createExpect: upToDateRollout,
		},
		{
			name: "version exist, no existing rollout nor config",
			// rollout should be created
			version:      versionOK,
			createExpect: upToDateRollout,
		},
		{
			name: "version exist, no existing rollout, config exist but doesn't contain agent section",
			// rollout should be created
			config:       configNoAgent,
			version:      versionOK,
			createExpect: upToDateRollout,
		},
		{
			name: "config exist, no existing rollout nor version",
			// rollout should not be created as there is no version
			config: configOK,
		},
		{
			name: "config exist, no existing rollout, version exist but doesn't contain agent section",
			// rollout should not be created as there is no version
			config:  configOK,
			version: versionNoAgent,
		},
		{
			name: "no existing rollout, config, nor version",
			// rollout should not be created as there is no version
		},
		{
			name: "existing out-of-date rollout, config and version exist",
			// rollout should be updated
			config:          configOK,
			version:         versionOK,
			existingRollout: outOfDateRollout,
			updateExpect:    upToDateRollout,
		},
		{
			name: "existing up-to-date rollout, config and version exist",
			// rollout should not be updated as its spec is already good
			config:          configOK,
			version:         versionOK,
			existingRollout: upToDateRollout,
		},
		{
			name: "existing rollout and config but no version",
			// rollout should be deleted as there is no version
			config:          configOK,
			existingRollout: upToDateRollout,
			deleteExpect:    true,
		},
		{
			name: "existing rollout but no config nor version",
			// rollout should be deleted as there is no version
			existingRollout: upToDateRollout,
			deleteExpect:    true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Test setup: creating a fake client answering fixtures
			var stubs mockClientStubs

			if tt.config != nil {
				stubs.configAnswers = []callAnswer[*autoupdate.AutoUpdateConfig]{{tt.config, nil}}
			} else {
				stubs.configAnswers = []callAnswer[*autoupdate.AutoUpdateConfig]{{nil, trace.NotFound("no config")}}
			}

			if tt.version != nil {
				stubs.versionAnswers = []callAnswer[*autoupdate.AutoUpdateVersion]{{tt.version, nil}}
			} else {
				stubs.versionAnswers = []callAnswer[*autoupdate.AutoUpdateVersion]{{nil, trace.NotFound("no version")}}
			}

			if tt.existingRollout != nil {
				stubs.rolloutAnswers = []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{tt.existingRollout, nil}}
			} else {
				stubs.rolloutAnswers = []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{nil, trace.NotFound("no rollout")}}
			}

			if tt.createExpect != nil {
				stubs.createRolloutAnswers = []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{tt.createExpect, nil}}
				stubs.createRolloutExpects = []require.ValueAssertionFunc{rolloutEquals(tt.createExpect)}
			}

			if tt.updateExpect != nil {
				stubs.updateRolloutAnswers = []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{tt.updateExpect, nil}}
				stubs.updateRolloutExpects = []require.ValueAssertionFunc{rolloutEquals(tt.updateExpect)}
			}

			if tt.deleteExpect {
				stubs.deleteRolloutAnswers = []error{nil}
			}

			client := newMockClient(t, stubs)

			// Test execution: Running the reconciliation

			reconciler := &reconciler{
				clt:   client,
				log:   log,
				clock: clock,
			}

			require.NoError(t, reconciler.tryReconcile(ctx))
			// Test validation: Checking that the mock client is now empty

			client.checkIfEmpty(t)
		})
	}
}

func TestReconciler_Reconcile(t *testing.T) {
	log := utils.NewSlogLoggerForTests()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	// Test setup: creating fixtures
	config, err := update.NewAutoUpdateConfig(&autoupdate.AutoUpdateConfigSpec{
		Tools: &autoupdate.AutoUpdateConfigSpecTools{
			Mode: update.ToolsUpdateModeEnabled,
		},
		Agents: &autoupdate.AutoUpdateConfigSpecAgents{
			Mode:     update.AgentsUpdateModeEnabled,
			Strategy: update.AgentsStrategyHaltOnError,
		},
	})
	require.NoError(t, err)
	version, err := update.NewAutoUpdateVersion(&autoupdate.AutoUpdateVersionSpec{
		Tools: &autoupdate.AutoUpdateVersionSpecTools{
			TargetVersion: "1.2.3",
		},
		Agents: &autoupdate.AutoUpdateVersionSpecAgents{
			StartVersion:  "1.2.3",
			TargetVersion: "1.2.4",
			Schedule:      update.AgentsScheduleImmediate,
			Mode:          update.AgentsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)
	upToDateRollout, err := update.NewAutoUpdateAgentRollout(&autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "1.2.4",
		Schedule:       update.AgentsScheduleImmediate,
		AutoupdateMode: update.AgentsUpdateModeEnabled,
		Strategy:       update.AgentsStrategyHaltOnError,
	})
	require.NoError(t, err)
	upToDateRollout.Status = &autoupdate.AutoUpdateAgentRolloutStatus{StartTime: timestamppb.New(clock.Now())}

	outOfDateRollout, err := update.NewAutoUpdateAgentRollout(&autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.2",
		TargetVersion:  "1.2.3",
		Schedule:       update.AgentsScheduleImmediate,
		AutoupdateMode: update.AgentsUpdateModeEnabled,
		Strategy:       update.AgentsStrategyHaltOnError,
	})
	require.NoError(t, err)
	outOfDateRollout.Status = &autoupdate.AutoUpdateAgentRolloutStatus{}

	// Those tests are not written in table format because the fixture setup it too complex and this would harm
	// readability.
	t.Run("reconciliation has nothing to do, should exit", func(t *testing.T) {
		// Test setup: build mock client
		stubs := mockClientStubs{
			configAnswers:  []callAnswer[*autoupdate.AutoUpdateConfig]{{config, nil}},
			versionAnswers: []callAnswer[*autoupdate.AutoUpdateVersion]{{version, nil}},
			rolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{upToDateRollout, nil}},
		}

		client := newMockClient(t, stubs)
		reconciler := &reconciler{
			clt: client,
			log: log,
		}

		// Test execution: run the reconciliation loop
		require.NoError(t, reconciler.reconcile(ctx))

		// Test validation: check that all the expected calls were received
		client.checkIfEmpty(t)
	})

	t.Run("reconciliation succeeds on first try, should exit", func(t *testing.T) {
		stubs := mockClientStubs{
			configAnswers:        []callAnswer[*autoupdate.AutoUpdateConfig]{{config, nil}},
			versionAnswers:       []callAnswer[*autoupdate.AutoUpdateVersion]{{version, nil}},
			rolloutAnswers:       []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{outOfDateRollout, nil}},
			updateRolloutExpects: []require.ValueAssertionFunc{rolloutEquals(upToDateRollout)},
			updateRolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{upToDateRollout, nil}},
		}

		client := newMockClient(t, stubs)
		reconciler := &reconciler{
			clt:   client,
			log:   log,
			clock: clock,
		}

		// Test execution: run the reconciliation loop
		require.NoError(t, reconciler.reconcile(ctx))

		// Test validation: check that all the expected calls were received
		client.checkIfEmpty(t)
	})

	t.Run("reconciliation faces conflict on first try, should retry and see that there's nothing left to do", func(t *testing.T) {
		stubs := mockClientStubs{
			// because of the retry, we expect 2 GETs on every resource
			configAnswers:  []callAnswer[*autoupdate.AutoUpdateConfig]{{config, nil}, {config, nil}},
			versionAnswers: []callAnswer[*autoupdate.AutoUpdateVersion]{{version, nil}, {version, nil}},
			rolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{outOfDateRollout, nil}, {upToDateRollout, nil}},
			// Single update expected, because there's nothing to do after the retry
			updateRolloutExpects: []require.ValueAssertionFunc{rolloutEquals(upToDateRollout)},
			updateRolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{nil, trace.Wrap(backend.ErrIncorrectRevision)}},
		}

		client := newMockClient(t, stubs)
		reconciler := &reconciler{
			clt:   client,
			log:   log,
			clock: clock,
		}

		// Test execution: run the reconciliation loop
		require.NoError(t, reconciler.reconcile(ctx))

		// Test validation: check that all the expected calls were received
		client.checkIfEmpty(t)
	})

	t.Run("reconciliation faces conflict on first try, should retry and update a second time", func(t *testing.T) {
		rev1, err := uuid.NewUUID()
		require.NoError(t, err)
		rev2, err := uuid.NewUUID()
		require.NoError(t, err)
		rev3, err := uuid.NewUUID()
		require.NoError(t, err)

		stubs := mockClientStubs{
			// because of the retry, we expect 2 GETs on every resource
			configAnswers:  []callAnswer[*autoupdate.AutoUpdateConfig]{{config, nil}, {config, nil}},
			versionAnswers: []callAnswer[*autoupdate.AutoUpdateVersion]{{version, nil}, {version, nil}},
			rolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{
				{withRevisionID(outOfDateRollout, rev1.String()), nil},
				{withRevisionID(outOfDateRollout, rev2.String()), nil}},
			// Two updates expected, one with the old revision, then a second one with the new
			updateRolloutExpects: []require.ValueAssertionFunc{
				rolloutEquals(withRevisionID(upToDateRollout, rev1.String())),
				rolloutEquals(withRevisionID(upToDateRollout, rev2.String())),
			},
			// We mimic a race and reject the first update because of the outdated revision
			updateRolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{
				{nil, trace.Wrap(backend.ErrIncorrectRevision)},
				{withRevisionID(upToDateRollout, rev3.String()), nil},
			},
		}

		client := newMockClient(t, stubs)
		reconciler := &reconciler{
			clt:   client,
			log:   log,
			clock: clock,
		}

		// Test execution: run the reconciliation loop
		require.NoError(t, reconciler.reconcile(ctx))

		// Test validation: check that all the expected calls were received
		client.checkIfEmpty(t)
	})

	t.Run("reconciliation faces missing rollout on first try, should retry and create the rollout", func(t *testing.T) {
		stubs := mockClientStubs{
			// because of the retry, we expect 2 GETs on every resource
			configAnswers:  []callAnswer[*autoupdate.AutoUpdateConfig]{{config, nil}, {config, nil}},
			versionAnswers: []callAnswer[*autoupdate.AutoUpdateVersion]{{version, nil}, {version, nil}},
			rolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{
				{outOfDateRollout, nil},
				{nil, trace.NotFound("no rollout")}},
			// One update expected on the first try, the second try should create
			updateRolloutExpects: []require.ValueAssertionFunc{
				rolloutEquals(upToDateRollout),
			},
			// We mimic the fact the rollout got deleted in the meantime
			updateRolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{
				{nil, trace.NotFound("no rollout")},
			},
			// One create expected on the second try
			createRolloutExpects: []require.ValueAssertionFunc{
				rolloutEquals(upToDateRollout),
			},
			createRolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{
				{upToDateRollout, nil},
			},
		}

		client := newMockClient(t, stubs)
		reconciler := &reconciler{
			clt:   client,
			log:   log,
			clock: clock,
		}

		// Test execution: run the reconciliation loop
		require.NoError(t, reconciler.reconcile(ctx))

		// Test validation: check that all the expected calls were received
		client.checkIfEmpty(t)
	})

	t.Run("reconciliation meets a hard unexpected failure on first try, should exit in error", func(t *testing.T) {
		stubs := mockClientStubs{
			configAnswers:        []callAnswer[*autoupdate.AutoUpdateConfig]{{config, nil}},
			versionAnswers:       []callAnswer[*autoupdate.AutoUpdateVersion]{{version, nil}},
			rolloutAnswers:       []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{outOfDateRollout, nil}},
			updateRolloutExpects: []require.ValueAssertionFunc{rolloutEquals(upToDateRollout)},
			updateRolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{
				{nil, trace.ConnectionProblem(trace.Errorf("io/timeout"), "the DB fell on the floor")},
			},
		}

		client := newMockClient(t, stubs)
		reconciler := &reconciler{
			clt:   client,
			log:   log,
			clock: clock,
		}

		// Test execution: run the reconciliation loop
		require.ErrorContains(t, reconciler.reconcile(ctx), "the DB fell on the floor")

		// Test validation: check that all the expected calls were received
		client.checkIfEmpty(t)
	})

	t.Run("reconciliation faces conflict on first try, should retry but context is expired so it bails out", func(t *testing.T) {
		cancelableCtx, cancel := context.WithCancel(ctx)
		// just in case
		t.Cleanup(cancel)

		stubs := mockClientStubs{
			// we expect a single GET because the context expires before the second retry
			configAnswers:  []callAnswer[*autoupdate.AutoUpdateConfig]{{config, nil}},
			versionAnswers: []callAnswer[*autoupdate.AutoUpdateVersion]{{version, nil}},
			rolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{outOfDateRollout, nil}},
			// Single update expected, because there's nothing to do after the retry.
			// We wrap the update validation function into a context canceler, so the context is done after the first update
			updateRolloutExpects: []require.ValueAssertionFunc{cancelContext(rolloutEquals(upToDateRollout), cancel)},
			// return a retryable error
			updateRolloutAnswers: []callAnswer[*autoupdate.AutoUpdateAgentRollout]{{nil, trace.Wrap(backend.ErrIncorrectRevision)}},
		}

		client := newMockClient(t, stubs)
		reconciler := &reconciler{
			clt:   client,
			log:   log,
			clock: clock,
		}

		// Test execution: run the reconciliation loop
		require.ErrorIs(t, reconciler.reconcile(cancelableCtx), context.Canceled)

		// Test validation: check that all the expected calls were received
		client.checkIfEmpty(t)
	})
}

func Test_makeGroupsStatus(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	tests := []struct {
		name      string
		schedules *autoupdate.AgentAutoUpdateSchedules
		expected  []*autoupdate.AutoUpdateAgentRolloutStatusGroup
	}{
		{
			name:      "nil schedules",
			schedules: nil,
			expected: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             defaultGroupName,
					StartTime:        nil,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(now),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       defaultUpdateDays,
					ConfigStartHour:  defaultStartHour,
				},
			},
		},
		{
			name:      "no groups in schedule",
			schedules: &autoupdate.AgentAutoUpdateSchedules{Regular: make([]*autoupdate.AgentAutoUpdateGroup, 0)},
			expected: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             defaultGroupName,
					StartTime:        nil,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(now),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       defaultUpdateDays,
					ConfigStartHour:  defaultStartHour,
				},
			},
		},
		{
			name: "one group in schedule",
			schedules: &autoupdate.AgentAutoUpdateSchedules{
				Regular: []*autoupdate.AgentAutoUpdateGroup{
					{
						Name:      "group1",
						Days:      everyWeekday,
						StartHour: matchingStartHour,
					},
				},
			},
			expected: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             "group1",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(now),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       everyWeekday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "multiple groups in schedule",
			schedules: &autoupdate.AgentAutoUpdateSchedules{
				Regular: []*autoupdate.AgentAutoUpdateGroup{
					{
						Name:      "group1",
						Days:      everyWeekday,
						StartHour: matchingStartHour,
					},
					{
						Name:      "group2",
						Days:      everyWeekdayButSunday,
						StartHour: nonMatchingStartHour,
						WaitHours: 1,
					},
				},
			},
			expected: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             "group1",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(now),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       everyWeekday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:             "group2",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(now),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       everyWeekdayButSunday,
					ConfigStartHour:  nonMatchingStartHour,
					ConfigWaitHours:  1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We craft a mock client always answering there's no cmc.
			// It's not the point of this test to check the cmc client usage so we don't count the number of calls here.
			// CMC-specific tests happen in TestDefaultConfigGroup().
			clt := newMockClient(t, mockClientStubs{cmcAnswers: []callAnswer[*types.ClusterMaintenanceConfigV1]{{
				result: nil,
				err:    trace.NotFound("no cmc"),
			}}})
			r := reconciler{clt: clt}
			result, err := r.makeGroupsStatus(ctx, tt.schedules, now)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

const fakeRolloutStrategyName = "fake"

type fakeRolloutStrategy struct {
	strategyName string
	// calls counts how many times the fake rollout strategy was called.
	// This is not thread safe.
	calls int
}

func (f *fakeRolloutStrategy) name() string {
	return f.strategyName
}

func (f *fakeRolloutStrategy) progressRollout(ctx context.Context, spec *autoupdate.AutoUpdateAgentRolloutSpec, status *autoupdate.AutoUpdateAgentRolloutStatus, now time.Time) error {
	f.calls++
	return nil
}

func Test_reconciler_computeStatus(t *testing.T) {
	log := utils.NewSlogLoggerForTests()
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	oldStatus := &autoupdate.AutoUpdateAgentRolloutStatus{
		Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
			{
				Name:  "old group",
				State: autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			},
		},
		State: autoupdate.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_UNSTARTED,
	}
	oldSpec := &autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "1.2.4",
		Schedule:       update.AgentsScheduleRegular,
		AutoupdateMode: update.AgentsUpdateModeEnabled,
		Strategy:       fakeRolloutStrategyName,
	}
	schedules := &autoupdate.AgentAutoUpdateSchedules{
		Regular: []*autoupdate.AgentAutoUpdateGroup{
			{
				Name: "new group",
				Days: everyWeekday,
			},
		},
	}
	r := reconciler{}
	newGroups, err := r.makeGroupsStatus(ctx, schedules, clock.Now())
	require.NoError(t, err)
	newStatus := &autoupdate.AutoUpdateAgentRolloutStatus{
		Groups:    newGroups,
		State:     autoupdate.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_UNSTARTED,
		StartTime: timestamppb.New(clock.Now()),
	}

	tests := []struct {
		name                  string
		existingRollout       *autoupdate.AutoUpdateAgentRollout
		newSpec               *autoupdate.AutoUpdateAgentRolloutSpec
		expectedStatus        *autoupdate.AutoUpdateAgentRolloutStatus
		expectedStrategyCalls int
	}{
		{
			name: "status is reset if start version changes",
			existingRollout: &autoupdate.AutoUpdateAgentRollout{
				Spec:   oldSpec,
				Status: oldStatus,
			},
			newSpec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.2",
				TargetVersion:  "1.2.4",
				Schedule:       update.AgentsScheduleRegular,
				AutoupdateMode: update.AgentsUpdateModeEnabled,
				Strategy:       fakeRolloutStrategyName,
			},
			// status should have been reset and is now the new status
			expectedStatus:        newStatus,
			expectedStrategyCalls: 1,
		},
		{
			name: "status is reset if target version changes",
			existingRollout: &autoupdate.AutoUpdateAgentRollout{
				Spec:   oldSpec,
				Status: oldStatus,
			},
			newSpec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "1.2.5",
				Schedule:       update.AgentsScheduleRegular,
				AutoupdateMode: update.AgentsUpdateModeEnabled,
				Strategy:       fakeRolloutStrategyName,
			},
			// status should have been reset and is now the new status
			expectedStatus:        newStatus,
			expectedStrategyCalls: 1,
		},
		{
			name: "status is reset if strategy changes",
			existingRollout: &autoupdate.AutoUpdateAgentRollout{
				Spec:   oldSpec,
				Status: oldStatus,
			},
			newSpec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "1.2.4",
				Schedule:       update.AgentsScheduleRegular,
				AutoupdateMode: update.AgentsUpdateModeEnabled,
				Strategy:       fakeRolloutStrategyName + "2",
			},
			// status should have been reset and is now the new status
			expectedStatus:        newStatus,
			expectedStrategyCalls: 1,
		},
		{
			name: "status is not reset if mode changes",
			existingRollout: &autoupdate.AutoUpdateAgentRollout{
				Spec:   oldSpec,
				Status: oldStatus,
			},
			newSpec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "1.2.4",
				Schedule:       update.AgentsScheduleRegular,
				AutoupdateMode: update.AgentsUpdateModeSuspended,
				Strategy:       fakeRolloutStrategyName,
			},
			// status should NOT have been reset and still contain the old groups
			expectedStatus:        oldStatus,
			expectedStrategyCalls: 1,
		},
		{
			name: "groups are unset if schedule is immediate",
			existingRollout: &autoupdate.AutoUpdateAgentRollout{
				Spec:   oldSpec,
				Status: oldStatus,
			},
			newSpec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "1.2.4",
				Schedule:       update.AgentsScheduleImmediate,
				AutoupdateMode: update.AgentsUpdateModeEnabled,
				Strategy:       fakeRolloutStrategyName,
			},
			// groups should be unset
			expectedStatus: &autoupdate.AutoUpdateAgentRolloutStatus{
				StartTime: timestamppb.New(clock.Now()),
			},
			expectedStrategyCalls: 0,
		},
		{
			name: "new groups are populated if previous ones were empty",
			existingRollout: &autoupdate.AutoUpdateAgentRollout{
				Spec: oldSpec,
				// old groups were empty
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					StartTime: timestamppb.New(clock.Now()),
				},
			},
			// no spec change
			newSpec: oldSpec,
			// still, we have the new groups set
			expectedStatus:        newStatus,
			expectedStrategyCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &fakeRolloutStrategy{strategyName: tt.newSpec.Strategy}
			r := &reconciler{
				log:               log,
				clock:             clock,
				rolloutStrategies: []rolloutStrategy{strategy},
				mutex:             sync.Mutex{},
			}
			result, err := r.computeStatus(ctx, tt.existingRollout, tt.newSpec, schedules)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.expectedStatus, result, protocmp.Transform()))
			require.Equal(t, tt.expectedStrategyCalls, strategy.calls)
		})
	}
}

func TestDefaultConfigGroup(t *testing.T) {
	ctx := context.Background()
	testStartHour := 16

	tests := []struct {
		name           string
		cmcAnswer      callAnswer[*types.ClusterMaintenanceConfigV1]
		expectedResult *autoupdate.AgentAutoUpdateGroup
		expectError    require.ErrorAssertionFunc
	}{
		{
			name: "no CMC",
			cmcAnswer: callAnswer[*types.ClusterMaintenanceConfigV1]{
				nil, trace.NotFound("no cmc"),
			},
			expectedResult: defaultGroup(),
			expectError:    require.NoError,
		},
		{
			name: "CMC with no upgrade window",
			cmcAnswer: callAnswer[*types.ClusterMaintenanceConfigV1]{
				&types.ClusterMaintenanceConfigV1{
					Spec: types.ClusterMaintenanceConfigSpecV1{
						AgentUpgrades: nil,
					},
				}, nil,
			},
			expectedResult: defaultGroup(),
			expectError:    require.NoError,
		},
		{
			name: "CMC with no weekdays",
			cmcAnswer: callAnswer[*types.ClusterMaintenanceConfigV1]{
				&types.ClusterMaintenanceConfigV1{
					Spec: types.ClusterMaintenanceConfigSpecV1{
						AgentUpgrades: &types.AgentUpgradeWindow{
							UTCStartHour: uint32(testStartHour),
							Weekdays:     nil,
						},
					},
				}, nil,
			},
			expectedResult: &autoupdate.AgentAutoUpdateGroup{
				Name:      defaultCMCGroupName,
				Days:      []string{"*"},
				StartHour: int32(testStartHour),
				WaitHours: 0,
			},
			expectError: require.NoError,
		},
		{
			name: "CMC with weekdays",
			cmcAnswer: callAnswer[*types.ClusterMaintenanceConfigV1]{
				&types.ClusterMaintenanceConfigV1{
					Spec: types.ClusterMaintenanceConfigSpecV1{
						AgentUpgrades: &types.AgentUpgradeWindow{
							UTCStartHour: uint32(testStartHour),
							Weekdays:     everyWeekdayButSunday,
						},
					},
				}, nil,
			},
			expectedResult: &autoupdate.AgentAutoUpdateGroup{
				Name:      defaultCMCGroupName,
				Days:      everyWeekdayButSunday,
				StartHour: int32(testStartHour),
				WaitHours: 0,
			},
			expectError: require.NoError,
		},
		{
			name: "unexpected error getting CMC",
			cmcAnswer: callAnswer[*types.ClusterMaintenanceConfigV1]{
				nil, trace.ConnectionProblem(trace.Errorf("oh no"), "connection failed"),
			},
			expectedResult: nil,
			expectError:    require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: loading fixtures.
			clt := newMockClient(t, mockClientStubs{cmcAnswers: []callAnswer[*types.ClusterMaintenanceConfigV1]{tt.cmcAnswer}})
			r := &reconciler{clt: clt}
			// Test execution.
			result, err := r.defaultConfigGroup(ctx)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedResult, result)
			// Test validation: the mock client should be empty.
			clt.checkIfEmpty(t)
		})
	}
}
