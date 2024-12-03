package rollout

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	update "github.com/gravitational/teleport/api/types/autoupdate"
)

type timeBasedStrategy struct {
	log   *slog.Logger
	clock clockwork.Clock
}

func (h *timeBasedStrategy) name() string {
	return update.AgentsStrategyTimeBased
}

func newTimeBasedStrategy(log *slog.Logger, clock clockwork.Clock) (rolloutStrategy, error) {
	if log == nil {
		return nil, trace.BadParameter("missing log")
	}
	if clock == nil {
		return nil, trace.BadParameter("missing clock")
	}
	return &timeBasedStrategy{
		log:   log.With("strategy", update.AgentsStrategyTimeBased),
		clock: clock,
	}, nil
}

func (h *timeBasedStrategy) progressRollout(ctx context.Context, groups []*autoupdate.AutoUpdateAgentRolloutStatusGroup) error {
	now := h.clock.Now()
	// We always process every group regardless of the order.
	var errors []error
	for _, group := range groups {
		switch group.State {
		case autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:
			// We start any group unstarted group in window.
			// Done groups can transition back to active if they enter their maintenance window again.
			// Some agents might have missed the previous windows and might expected to try again.
			shouldBeActive, err := inWindow(group, now)
			if err != nil {
				// In time-based rollouts, groups are not dependent.
				// Failing to transition a group should affect other groups.
				// We reflect that something went wrong in the status and go to the next group.
				setGroupState(group, group.State, updateReasonReconcilerError, now)
				errors = append(errors, err)
				continue
			}
			if shouldBeActive {
				setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE, updateReasonInWindow, now)
			} else {
				setGroupState(group, group.State, updateReasonOutsideWindow, now)
			}
		case autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK:
			// We don't touch any group that was manually rolled back.
			// Something happened and we should not try to update again.
		case autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:
			// The group is currently being updated. We check if the maintenance
			// is over and if we should transition it to the done state
			shouldBeActive, err := inWindow(group, now)
			if err != nil {
				// In time-based rollouts, groups are not dependent.
				// Failing to transition a group should affect other groups.
				// We reflect that something went wrong in the status and go to the next group.
				setGroupState(group, group.State, updateReasonReconcilerError, now)
				errors = append(errors, err)
				continue
			}

			if shouldBeActive {
				setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE, updateReasonInWindow, now)
			} else {
				setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE, updateReasonOutsideWindow, now)
			}
		default:
			return trace.BadParameter("unknown autoupdate group state: %v", group.State)
		}
	}
	return trace.NewAggregate(errors...)
}
