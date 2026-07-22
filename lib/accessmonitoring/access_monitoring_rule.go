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

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
)

// EvaluateRules evalutes the rules againast the request environment and
// returns the list of rules that match the conditions.
func EvaluateRules(
	ctx context.Context,
	log *slog.Logger,
	env AccessRequestExpressionEnv,
	rules []*accessmonitoringrulesv1.AccessMonitoringRule,
) []*accessmonitoringrulesv1.AccessMonitoringRule {
	var matchingRules []*accessmonitoringrulesv1.AccessMonitoringRule
	for _, rule := range rules {
		log = log.With("rule", rule.GetMetadata().GetName())

		// Check if creation time is within rule schedules.
		schedule := rule.GetSpec().GetSchedules()
		isInSchedules, err := InSchedules(schedule, env.CreationTime)
		if err != nil {
			log.WarnContext(ctx, "Failed to evaluate access monitoring rule schedules", "error", err)
			continue
		}
		if len(schedule) != 0 && !isInSchedules {
			log.DebugContext(ctx, "Access request does not satisfy schedule condition")
			continue
		}

		// Check if environment matches rule conditions.
		conditionMatch, err := EvaluateCondition(rule.GetSpec().GetCondition(), env)
		if err != nil {
			log.WarnContext(ctx, "Failed to evaluate access monitoring rule condition", "error", err)
			continue
		}
		if conditionMatch {
			matchingRules = append(matchingRules, rule)
		}
	}
	return matchingRules
}
