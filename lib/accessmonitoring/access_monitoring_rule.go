/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package accessmonitoring

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
)

// EvaluateRules evaluates the rules against the request environment and returns
// the rules whose schedule and condition match. A rule that cannot be evaluated
// is dropped and logged.
func EvaluateRules(
	ctx context.Context,
	log *slog.Logger,
	env AccessRequestExpressionEnv,
	rules []*accessmonitoringrulesv1.AccessMonitoringRule,
) ([]*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	var matchingRules []*accessmonitoringrulesv1.AccessMonitoringRule
	var denyErr error
	for _, rule := range rules {
		log := log.With("rule", rule.GetMetadata().GetName())

		// Check if creation time is within rule schedules.
		schedule := rule.GetSpec().GetSchedules()
		isInSchedules, err := InSchedules(schedule, env.CreationTime)
		if err != nil {
			log.WarnContext(ctx, "Dropping rule with a schedule that failed evaluation", "error", err)
			if isDenyRule(rule) && denyErr == nil {
				denyErr = trace.Wrap(err, "evaluating schedules for access monitoring rule %q", rule.GetMetadata().GetName())
			}
			continue
		}
		if len(schedule) != 0 && !isInSchedules {
			log.DebugContext(ctx, "Access request does not satisfy schedule condition")
			continue
		}

		// Check if environment matches rule conditions.
		conditionMatch, err := EvaluateCondition(rule.GetSpec().GetCondition(), env)
		if err != nil {
			log.WarnContext(ctx, "Dropping rule with a condition that failed evaluation", "error", err)
			if isDenyRule(rule) && denyErr == nil {
				denyErr = trace.Wrap(err, "evaluating condition for access monitoring rule %q", rule.GetMetadata().GetName())
			}
			continue
		}
		if conditionMatch {
			matchingRules = append(matchingRules, rule)
		}
	}
	return matchingRules, denyErr
}

func isDenyRule(rule *accessmonitoringrulesv1.AccessMonitoringRule) bool {
	return rule.GetSpec().GetAutomaticReview().GetDecision() == types.RequestState_DENIED.String()
}
