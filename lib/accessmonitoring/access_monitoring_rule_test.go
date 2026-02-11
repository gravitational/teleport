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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestEvaluateRules verifies proper rules validation.
func TestEvaluateRules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	tests := []struct {
		description string
		env         AccessRequestExpressionEnv
		rules       []*accessmonitoringrulesv1.AccessMonitoringRule
		expected    []string
	}{
		{
			description: "condition and schedule match 1 rule",
			env: AccessRequestExpressionEnv{
				CreationTime: time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC),
			},
			rules: []*accessmonitoringrulesv1.AccessMonitoringRule{
				makeRule("test-rule", "true", time.Monday, "14:00", "15:00"),
			},
			expected: []string{"test-rule"},
		},
		{
			description: "condition and schedule match multiple rules",
			env: AccessRequestExpressionEnv{
				CreationTime: time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC),
			},
			rules: []*accessmonitoringrulesv1.AccessMonitoringRule{
				makeRule("test-rule-1", "true", time.Monday, "14:00", "15:00"),
				makeRule("test-rule-2", "true", time.Monday, "14:00", "15:00"),
			},
			expected: []string{"test-rule-1", "test-rule-2"},
		},
		{
			description: "condition does not match",
			env: AccessRequestExpressionEnv{
				CreationTime: time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC),
			},
			rules: []*accessmonitoringrulesv1.AccessMonitoringRule{
				makeRule("test-rule-1", "false", time.Monday, "14:00", "15:00"),
			},
			expected: []string{},
		},
		{
			description: "schedule does not match",
			env: AccessRequestExpressionEnv{
				CreationTime: time.Date(2025, time.August, 11, 15, 30, 0, 0, time.UTC),
			},
			rules: []*accessmonitoringrulesv1.AccessMonitoringRule{
				makeRule("test-rule-1", "true", time.Monday, "14:00", "15:00"),
			},
			expected: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			rules := EvaluateRules(ctx, logtest.NewLogger(), test.env, test.rules)
			require.Len(t, rules, len(test.expected))
			for _, rule := range rules {
				require.Contains(t, test.expected, rule.Metadata.GetName())
			}
		})
	}
}

func makeRule(
	name, condition string,
	weekday time.Weekday,
	start, end string,
) *accessmonitoringrulesv1.AccessMonitoringRule {
	return &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:  []string{types.KindAccessRequest},
			Condition: condition,
			Schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{{
							Weekday: weekday.String(),
							Start:   start,
							End:     end,
						}},
					},
				},
			},
		},
	}
}
