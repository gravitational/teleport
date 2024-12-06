package rollout

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

var (
	// 2024-11-30 is a Saturday
	testSaturday = time.Date(2024, 11, 30, 15, 30, 0, 0, time.UTC)
	// 2024-12-01 is a Sunday
	testSunday            = time.Date(2024, 12, 1, 12, 30, 0, 0, time.UTC)
	matchingStartHour     = int32(12)
	nonMatchingStartHour  = int32(15)
	everyWeekday          = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	everyWeekdayButSunday = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
)

func Test_canUpdateToday(t *testing.T) {
	tests := []struct {
		name        string
		allowedDays []string
		now         time.Time
		want        bool
		wantErr     require.ErrorAssertionFunc
	}{
		{
			name:        "Empty list",
			allowedDays: []string{},
			now:         time.Now(),
			want:        false,
			wantErr:     require.NoError,
		},
		{
			name:        "Wildcard",
			allowedDays: []string{"*"},
			now:         time.Now(),
			want:        true,
			wantErr:     require.NoError,
		},
		{
			name:        "Matching day",
			allowedDays: everyWeekday,
			now:         testSunday,
			want:        true,
			wantErr:     require.NoError,
		},
		{
			name:        "No matching day",
			allowedDays: everyWeekdayButSunday,
			now:         testSunday,
			want:        false,
			wantErr:     require.NoError,
		},
		{
			name:        "Malformed day",
			allowedDays: []string{"Mon", "Tue", "HelloThereGeneralKenobi"},
			now:         testSunday,
			want:        false,
			wantErr:     require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := canUpdateToday(tt.allowedDays, tt.now)
			tt.wantErr(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_inWindow(t *testing.T) {
	tests := []struct {
		name    string
		group   *autoupdate.AutoUpdateAgentRolloutStatusGroup
		now     time.Time
		want    bool
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "out of window",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				ConfigDays:      everyWeekdayButSunday,
				ConfigStartHour: matchingStartHour,
			},
			now:     testSunday,
			want:    false,
			wantErr: require.NoError,
		},
		{
			name: "inside window, wrong hour",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				ConfigDays:      everyWeekday,
				ConfigStartHour: nonMatchingStartHour,
			},
			now:     testSunday,
			want:    false,
			wantErr: require.NoError,
		},
		{
			name: "inside window, correct hour",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				ConfigDays:      everyWeekday,
				ConfigStartHour: matchingStartHour,
			},
			now:     testSunday,
			want:    true,
			wantErr: require.NoError,
		},
		{
			name: "invalid weekdays",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				ConfigDays:      []string{"HelloThereGeneralKenobi"},
				ConfigStartHour: matchingStartHour,
			},
			now:     testSunday,
			want:    false,
			wantErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := inWindow(tt.group, tt.now)
			tt.wantErr(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_setGroupState(t *testing.T) {
	groupName := "test-group"

	clock := clockwork.NewFakeClock()
	// oldUpdateTime is 5 minutes in the past
	oldUpdateTime := clock.Now()
	clock.Advance(5 * time.Minute)

	tests := []struct {
		name     string
		group    *autoupdate.AutoUpdateAgentRolloutStatusGroup
		newState autoupdate.AutoUpdateAgentGroupState
		reason   string
		now      time.Time
		expected *autoupdate.AutoUpdateAgentRolloutStatusGroup
	}{
		{
			name: "same state, no change",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:             groupName,
				StartTime:        nil,
				State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				LastUpdateTime:   timestamppb.New(oldUpdateTime),
				LastUpdateReason: updateReasonCannotStart,
			},
			newState: autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			reason:   updateReasonCannotStart,
			now:      clock.Now(),
			expected: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:      groupName,
				StartTime: nil,
				State:     autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				// update time has not been bumped as nothing changed
				LastUpdateTime:   timestamppb.New(oldUpdateTime),
				LastUpdateReason: updateReasonCannotStart,
			},
		},
		{
			name: "same state, reason change",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:             groupName,
				StartTime:        nil,
				State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				LastUpdateTime:   timestamppb.New(oldUpdateTime),
				LastUpdateReason: updateReasonCannotStart,
			},
			newState: autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			reason:   updateReasonReconcilerError,
			now:      clock.Now(),
			expected: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:      groupName,
				StartTime: nil,
				State:     autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				// update time has been bumped because reason changed
				LastUpdateTime:   timestamppb.New(clock.Now()),
				LastUpdateReason: updateReasonReconcilerError,
			},
		},
		{
			name: "new state, no reason change",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:             groupName,
				StartTime:        nil,
				State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				LastUpdateTime:   timestamppb.New(oldUpdateTime),
				LastUpdateReason: updateReasonCannotStart,
			},
			newState: autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			reason:   updateReasonCannotStart,
			now:      clock.Now(),
			expected: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:      groupName,
				StartTime: nil,
				State:     autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
				// update time has been bumped because state changed
				LastUpdateTime:   timestamppb.New(clock.Now()),
				LastUpdateReason: updateReasonCannotStart,
			},
		},
		{
			name: "new state, reason change",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:             groupName,
				StartTime:        nil,
				State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				LastUpdateTime:   timestamppb.New(oldUpdateTime),
				LastUpdateReason: updateReasonCannotStart,
			},
			newState: autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			reason:   updateReasonReconcilerError,
			now:      clock.Now(),
			expected: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:      groupName,
				StartTime: nil,
				State:     autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
				// update time has been bumped because state and reason changed
				LastUpdateTime:   timestamppb.New(clock.Now()),
				LastUpdateReason: updateReasonReconcilerError,
			},
		},
		{
			name: "new state, transition to active",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:             groupName,
				StartTime:        nil,
				State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				LastUpdateTime:   timestamppb.New(oldUpdateTime),
				LastUpdateReason: updateReasonCannotStart,
			},
			newState: autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			reason:   updateReasonCanStart,
			now:      clock.Now(),
			expected: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name: groupName,
				// We set start time during the transition
				StartTime: timestamppb.New(clock.Now()),
				State:     autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
				// update time has been bumped because state and reason changed
				LastUpdateTime:   timestamppb.New(clock.Now()),
				LastUpdateReason: updateReasonCanStart,
			},
		},
		{
			name: "same state, transition from active to active",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:             groupName,
				StartTime:        timestamppb.New(oldUpdateTime),
				State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
				LastUpdateTime:   timestamppb.New(oldUpdateTime),
				LastUpdateReason: updateReasonCanStart,
			},
			newState: autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			reason:   updateReasonReconcilerError,
			now:      clock.Now(),
			expected: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name: groupName,
				// As the state was already active, the start time should not be refreshed
				StartTime: timestamppb.New(oldUpdateTime),
				State:     autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
				// update time has been bumped because reason changed
				LastUpdateTime:   timestamppb.New(clock.Now()),
				LastUpdateReason: updateReasonReconcilerError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setGroupState(tt.group, tt.newState, tt.reason, tt.now)
			require.Equal(t, tt.expected, tt.group)
		})
	}
}
