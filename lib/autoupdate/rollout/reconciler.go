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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	update "github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/api/utils"
)

const (
	reconciliationTimeout = 30 * time.Second
	defaultConfigMode     = update.AgentsUpdateModeEnabled
	defaultStrategy       = update.AgentsStrategyHaltOnError
	maxConflictRetry      = 3

	defaultGroupName = "default"
	defaultStartHour = 12
)

var (
	defaultUpdateDays = []string{"Mon", "Tue", "Wed", "Thu"}
)

// reconciler reconciles the AutoUpdateAgentRollout singleton based on the content of the AutoUpdateVersion and
// AutoUpdateConfig singletons. This reconciler is not based on the services.GenericReconciler because:
// - we reconcile 2 resources with one
// - both input and output are singletons, we don't need the multi resource logic nor stream/paginated APIs
type reconciler struct {
	clt   Client
	log   *slog.Logger
	clock clockwork.Clock

	rolloutStrategies []rolloutStrategy

	// mutex ensures we only run one reconciliation at a time
	mutex sync.Mutex
}

// reconcile the AutoUpdateAgentRollout singleton. The reconciliation can fail because of a conflict (multiple auths
// are racing), in this case we retry the reconciliation immediately.
func (r *reconciler) reconcile(ctx context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	ctx, cancel := context.WithTimeout(ctx, reconciliationTimeout)
	defer cancel()
	tries := 0
	var err error
	for tries < maxConflictRetry {
		tries++
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err = r.tryReconcile(ctx)
			switch {
			case err == nil:
				return nil
			case trace.IsCompareFailed(err), trace.IsNotFound(err):
				// The resource changed since we last saw it
				// We must have raced against another auth
				// Let's retry the reconciliation
				r.log.DebugContext(ctx, "retrying reconciliation", "error", err)
			default:
				// error is non-nil and non-retryable
				return trace.Wrap(err, "failed to reconcile rollout")
			}
		}
	}
	return trace.CompareFailed("compare failed, tried %d times, last error: %s", tries, err)
}

// tryReconcile tries to reconcile the AutoUpdateAgentRollout singleton.
// This function should be nilpotent if the AutoUpdateAgentRollout is already up-to-date.
// The creation/update/deletion can fail with a trace.CompareFailedError or trace.NotFoundError
// if the resource change while we were computing it.
// The caller must handle those error and retry the reconciliation.
func (r *reconciler) tryReconcile(ctx context.Context) error {
	// get autoupdate_config
	var config *autoupdate.AutoUpdateConfig
	if c, err := r.clt.GetAutoUpdateConfig(ctx); err == nil {
		config = c
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err, "getting autoupdate_config")
	}

	// get autoupdate_version
	var version *autoupdate.AutoUpdateVersion
	if v, err := r.clt.GetAutoUpdateVersion(ctx); err == nil {
		version = v
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err, "getting autoupdate version")
	}

	// get autoupdate_agent_rollout
	rolloutExists := true
	existingRollout, err := r.clt.GetAutoUpdateAgentRollout(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err, "getting autoupdate_agent_rollout")
	}
	if trace.IsNotFound(err) {
		// rollout doesn't exist yet, we'll need to call Create instead of Update.
		rolloutExists = false
	}

	// if autoupdate_version does not exist or does not contain spec.agents, we should not configure a rollout
	if version.GetSpec().GetAgents() == nil {
		if !rolloutExists {
			// the rollout doesn't exist, nothing to do
			return nil
		}
		// the rollout exists, we must delete it
		return r.clt.DeleteAutoUpdateAgentRollout(ctx)
	}

	// compute what the spec should look like
	newSpec, err := r.buildRolloutSpec(config.GetSpec().GetAgents(), version.GetSpec().GetAgents())
	if err != nil {
		return trace.Wrap(err, "mutating rollout")
	}
	newStatus, err := r.computeStatus(ctx, existingRollout, newSpec, config.GetSpec().GetAgents().GetSchedules())
	if err != nil {
		return trace.Wrap(err, "computing rollout status")
	}

	// there was an existing rollout, we must figure if something changed
	specChanged := !proto.Equal(existingRollout.GetSpec(), newSpec)
	statusChanged := !proto.Equal(existingRollout.GetStatus(), newStatus)
	rolloutChanged := specChanged || statusChanged

	// if nothing changed, no need to update the resource
	if !rolloutChanged {
		r.log.DebugContext(ctx, "rollout unchanged")
		return nil
	}

	// if there are no existing rollout, we create a new one and set the status
	if !rolloutExists {
		rollout, err := update.NewAutoUpdateAgentRollout(newSpec)
		rollout.Status = newStatus
		if err != nil {
			return trace.Wrap(err, "validating new rollout")
		}
		_, err = r.clt.CreateAutoUpdateAgentRollout(ctx, rollout)
		return trace.Wrap(err, "creating rollout")
	}

	// If there was a previous rollout, we update its spec and status and do an update.
	// We don't create a new resource to keep the metadata containing the revision ID.
	existingRollout.Spec = newSpec
	existingRollout.Status = newStatus
	err = update.ValidateAutoUpdateAgentRollout(existingRollout)
	if err != nil {
		return trace.Wrap(err, "validating mutated rollout")
	}
	_, err = r.clt.UpdateAutoUpdateAgentRollout(ctx, existingRollout)
	return trace.Wrap(err, "updating rollout")
}

func (r *reconciler) buildRolloutSpec(config *autoupdate.AutoUpdateConfigSpecAgents, version *autoupdate.AutoUpdateVersionSpecAgents) (*autoupdate.AutoUpdateAgentRolloutSpec, error) {
	// reconcile mode
	mode, err := getMode(config.GetMode(), version.GetMode())
	if err != nil {
		return nil, trace.Wrap(err, "computing agent update mode")
	}

	strategy := config.GetStrategy()
	if strategy == "" {
		strategy = defaultStrategy
	}

	return &autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   version.GetStartVersion(),
		TargetVersion:  version.GetTargetVersion(),
		Schedule:       version.GetSchedule(),
		AutoupdateMode: mode,
		Strategy:       strategy,
	}, nil

}

// agentModeCode maps agents mode to integers.
// When config and version modes don't match, the lowest integer takes precedence.
var (
	agentModeCode = map[string]int{
		update.AgentsUpdateModeDisabled:  0,
		update.AgentsUpdateModeSuspended: 1,
		update.AgentsUpdateModeEnabled:   2,
	}
	codeToAgentMode = map[int]string{
		0: update.AgentsUpdateModeDisabled,
		1: update.AgentsUpdateModeSuspended,
		2: update.AgentsUpdateModeEnabled,
	}
)

// getMode merges the agent modes coming from the version and config resources into a single mode.
// "disabled" takes precedence over "suspended", which takes precedence over "enabled".
func getMode(configMode, versionMode string) (string, error) {
	if configMode == "" {
		configMode = defaultConfigMode
	}
	if versionMode == "" {
		return "", trace.BadParameter("version mode empty")
	}

	configCode, ok := agentModeCode[configMode]
	if !ok {
		return "", trace.BadParameter("unsupported agent config mode: %v", configMode)
	}
	versionCode, ok := agentModeCode[versionMode]
	if !ok {
		return "", trace.BadParameter("unsupported agent version mode: %v", versionMode)
	}

	// The lowest code takes precedence
	if configCode <= versionCode {
		return codeToAgentMode[configCode], nil
	}
	return codeToAgentMode[versionCode], nil
}

// computeStatus computes the new rollout status based on the existing rollout,
// new rollout spec, and autoupdate_config. existingRollout might be nil if this
// is a new rollout.
// Even if the returned new status might be derived from the existing rollout
// status, it is a new deep-cloned structure.
func (r *reconciler) computeStatus(
	ctx context.Context,
	existingRollout *autoupdate.AutoUpdateAgentRollout,
	newSpec *autoupdate.AutoUpdateAgentRolloutSpec,
	configSchedules *autoupdate.AgentAutoUpdateSchedules,
) (*autoupdate.AutoUpdateAgentRolloutStatus, error) {

	var status *autoupdate.AutoUpdateAgentRolloutStatus

	// First, we check if a major spec change happened and we should reset the rollout status
	shouldResetRollout := existingRollout.GetSpec().GetStartVersion() != newSpec.GetStartVersion() ||
		existingRollout.GetSpec().GetTargetVersion() != newSpec.GetTargetVersion() ||
		existingRollout.GetSpec().GetSchedule() != newSpec.GetSchedule() ||
		existingRollout.GetSpec().GetStrategy() != newSpec.GetStrategy()

	// We create a new status if the rollout should be reset or the previous status was nil
	if shouldResetRollout || existingRollout.GetStatus() == nil {
		status = new(autoupdate.AutoUpdateAgentRolloutStatus)
	} else {
		status = utils.CloneProtoMsg(existingRollout.GetStatus())
	}

	// Then, we check if the selected schedule uses groups
	switch newSpec.GetSchedule() {
	case update.AgentsScheduleImmediate:
		// There are no groups with the immediate schedule, we must clean them
		status.Groups = nil
		return status, nil
	case update.AgentsScheduleRegular:
		// Regular schedule has groups, we will compute them after
	default:
		return nil, trace.BadParameter("unsupported agent schedule type %q", newSpec.GetSchedule())
	}

	// capture the current time to put it in the status update timestamps and to
	// compute the group state changes
	now := r.clock.Now()

	// If this is a new rollout or the rollout has been reset, we create groups from the config
	groups := status.GetGroups()
	var err error
	if len(groups) == 0 {
		groups, err = makeGroupsStatus(configSchedules, now)
		if err != nil {
			return nil, trace.Wrap(err, "creating groups status")
		}
	}

	err = r.progressRollout(ctx, newSpec.GetStrategy(), groups)
	// Failing to progress the update is not a hard failure.
	// We expected to update the status even if something went wrong to surface the failed reconciliation and potential errors to the user.
	if err != nil {
		r.log.ErrorContext(ctx, "Errors encountered during rollout progress. Some groups might not get updated properly.",
			"error", err)
	}

	status.Groups = groups
	return status, nil
}

// progressRollout picks the right rollout strategy and updates groups to progress the rollout.
// groups are updated in place.
// If an error is returned, the groups should still be upserted, depending on the strategy,
// failing to update a group might not be fatal (other groups can still progress independently).
func (r *reconciler) progressRollout(ctx context.Context, strategyName string, groups []*autoupdate.AutoUpdateAgentRolloutStatusGroup) error {
	for _, strategy := range r.rolloutStrategies {
		if strategy.name() == strategyName {
			return strategy.progressRollout(ctx, groups)
		}
	}
	return trace.NotImplemented("rollout strategy %q not implemented", strategyName)
}

// makeGroupStatus creates the autoupdate_agent_rollout.status.groups based on the autoupdate_config.
// This should be called if the status groups have not been initialized or must be reset.
func makeGroupsStatus(schedules *autoupdate.AgentAutoUpdateSchedules, now time.Time) ([]*autoupdate.AutoUpdateAgentRolloutStatusGroup, error) {
	configGroups := schedules.GetRegular()
	if len(configGroups) == 0 {
		defaultGroup, err := defaultConfigGroup()
		if err != nil {
			return nil, trace.Wrap(err, "retrieving default group")
		}
		configGroups = []*autoupdate.AgentAutoUpdateGroup{defaultGroup}
	}

	groups := make([]*autoupdate.AutoUpdateAgentRolloutStatusGroup, len(configGroups))
	for i, group := range configGroups {
		groups[i] = &autoupdate.AutoUpdateAgentRolloutStatusGroup{
			Name:             group.Name,
			StartTime:        nil,
			State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			LastUpdateTime:   timestamppb.New(now),
			LastUpdateReason: updateReasonCreated,
			ConfigDays:       group.Days,
			ConfigStartHour:  group.StartHour,
			ConfigWaitDays:   group.WaitDays,
		}
	}
	return groups, nil
}

// defaultConfigGroup returns the default group in case of missing autoupdate_config resource.
// This is a function and not a variable because we will need to add more logic there in the future
// lookup maintenance information from RFD 109's cluster_maintenance_config.
func defaultConfigGroup() (*autoupdate.AgentAutoUpdateGroup, error) {
	// TODO: get group from CMC if possible
	return &autoupdate.AgentAutoUpdateGroup{
		Name:      defaultGroupName,
		Days:      defaultUpdateDays,
		StartHour: defaultStartHour,
		WaitDays:  0,
	}, nil
}
