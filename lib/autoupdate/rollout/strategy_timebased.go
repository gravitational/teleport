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
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	update "github.com/gravitational/teleport/api/types/autoupdate"
)

const (
	updateReasonInWindow      = "in_window"
	updateReasonOutsideWindow = "outside_window"
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

func (h *timeBasedStrategy) progressRollout(ctx context.Context, status *autoupdate.AutoUpdateAgentRolloutStatus) error {
	now := h.clock.Now()
	// We always process every group regardless of the order.
	var errs []error
	for _, group := range status.Groups {
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
				errs = append(errs, err)
				continue
			}

			// Check if the rollout got created after the theoretical group start time
			rolloutChangedDuringWindow, err := rolloutChangedInWindow(group, now, status.StartTime.AsTime())
			if err != nil {
				setGroupState(group, group.State, updateReasonReconcilerError, now)
				errs = append(errs, err)
				continue
			}

			switch {
			case !shouldBeActive:
				setGroupState(group, group.State, updateReasonOutsideWindow, now)
			case rolloutChangedDuringWindow:
				setGroupState(group, group.State, updateReasonRolloutChanged, now)
			default:
				setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE, updateReasonInWindow, now)
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
				errs = append(errs, err)
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
	return trace.NewAggregate(errs...)
}
