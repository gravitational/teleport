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

package rolloutcontroller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	update "github.com/gravitational/teleport/api/types/autoupdate"
)

const (
	reconciliationTimeout = 30 * time.Second
	defaultConfigMode     = update.AgentsUpdateModeEnabled
	defaultStrategy       = update.AgentsStrategyHaltOnError
	maxConflictRetry      = 3
)

// Reconciler reconciles the AutoUpdateAgentRollout singleton based on the content of the AutoUpdateVersion and
// AutoUpdateConfig singletons. This reconciler is not based on the services.GenericReconciler because:
// - we reconcile 2 resources with one
// - both input and output are singletons, we don't need the multi resource logic nor stream/paginated APIs
type Reconciler struct {
	clt Client
	log *slog.Logger

	// mutex ensures we only run one reconciliation at a time
	mutex sync.Mutex
}

// Reconcile the AutoUpdateAgentRollout singleton. The reconciliation can fail because of a conflict (multiple auths
// are racing), in this case we retry the reconciliation immediately.
func (r *Reconciler) Reconcile(ctx context.Context) error {
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
func (r *Reconciler) tryReconcile(ctx context.Context) error {
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

	// if there are no existing rollout, we create a new one
	if !rolloutExists {
		rollout, err := update.NewAutoUpdateAgentRollout(newSpec)
		if err != nil {
			return trace.Wrap(err, "validating new rollout")
		}
		_, err = r.clt.CreateAutoUpdateAgentRollout(ctx, rollout)
		return trace.Wrap(err, "creating rollout")
	}

	// there was an existing rollout, we must figure if something changed
	specChanged := existingRollout.GetSpec().GetStartVersion() != newSpec.GetStartVersion() ||
		existingRollout.GetSpec().GetTargetVersion() != newSpec.GetTargetVersion() ||
		existingRollout.GetSpec().GetAutoupdateMode() != newSpec.GetAutoupdateMode() ||
		existingRollout.GetSpec().GetStrategy() != newSpec.GetStrategy() ||
		existingRollout.GetSpec().GetSchedule() != newSpec.GetSchedule()

	// TODO: reconcile the status here when we'll add group support.
	// Even if the spec does not change, we might still have to update the status:
	// - sync groups with the ones from the user config
	// - progress the rollout across groups

	// if nothing changed, no need to update the resource
	if !specChanged {
		r.log.DebugContext(ctx, "rollout unchanged")
		return nil
	}

	// something changed, we replace the old spec with the new one, validate and update the resource
	// we don't create a new resource to keep the revision ID and
	existingRollout.Spec = newSpec
	err = update.ValidateAutoUpdateAgentRollout(existingRollout)
	if err != nil {
		return trace.Wrap(err, "validating mutated rollout")
	}
	_, err = r.clt.UpdateAutoUpdateAgentRollout(ctx, existingRollout)
	return trace.Wrap(err, "updating rollout")
}

func (r *Reconciler) buildRolloutSpec(config *autoupdate.AutoUpdateConfigSpecAgents, version *autoupdate.AutoUpdateVersionSpecAgents) (*autoupdate.AutoUpdateAgentRolloutSpec, error) {
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
