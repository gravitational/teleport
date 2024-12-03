package rollout

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
)

// rolloutStrategy is responsible for rolling out the update across groups.
// This interface allows us to inject dummy strategies for simpler testing.
type rolloutStrategy interface {
	name() string
	progressRollout(context.Context, []*autoupdate.AutoUpdateAgentRolloutStatusGroup) error
}

func inWindow(group *autoupdate.AutoUpdateAgentRolloutStatusGroup, now time.Time) (bool, error) {
	dayOK, err := canUpdateToday(group.ConfigDays, now)
	if err != nil {
		return false, trace.Wrap(err, "checking the day of the week")
	}
	if !dayOK {
		return false, nil
	}
	return int(group.ConfigStartHour) == now.Hour(), nil
}

func canUpdateToday(allowedDays []string, now time.Time) (bool, error) {
	for _, allowedDay := range allowedDays {
		if allowedDay == types.Wildcard {
			return true, nil
		}
		weekday, ok := types.ParseWeekday(allowedDay)
		if !ok {
			return false, trace.BadParameter("failed to parse weekday %q", allowedDay)
		}
		if weekday == now.Weekday() {
			return true, nil
		}
	}
	return false, nil
}

func setGroupState(group *autoupdate.AutoUpdateAgentRolloutStatusGroup, newState autoupdate.AutoUpdateAgentGroupState, reason string, now time.Time) {
	changed := false
	previousState := group.State

	// Check if there is a state transition
	if previousState != newState {
		group.State = newState
		changed = true
		// If we just started the group, also update the start time
		if newState == autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE {
			group.StartTime = timestamppb.New(now)
		}
	}

	// Check if there is a reason change. Even if the state did not change, we
	// might expected to explain why.
	if group.LastUpdateReason != reason {
		group.LastUpdateReason = reason
		changed = true
	}

	if changed {
		group.LastUpdateTime = timestamppb.New(now)
	}
}
