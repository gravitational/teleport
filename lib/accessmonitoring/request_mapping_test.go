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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		description string
		condition   string
		env         AccessRequestExpressionEnv
		match       bool
	}{
		{
			description: "does not match user 'level' trait",
			condition:   `contains_any(user.traits["level"], set("L1"))`,
			env: AccessRequestExpressionEnv{
				UserTraits: map[string][]string{
					"level": {"L2"},
				},
			},
			match: false,
		},
		{
			description: "matches one of user 'level' trait",
			condition:   `contains_any(user.traits["level"], set("L1", "L2"))`,
			env: AccessRequestExpressionEnv{
				UserTraits: map[string][]string{
					"level": {"L1"},
				},
			},
			match: true,
		},
		{
			description: "matches all user traits",
			condition: `
				contains_any(user.traits["level"], set("L1", "L2")) &&
				contains_any(user.traits["team"], set("Cloud")) &&
				contains_any(user.traits["location"], set("Seattle"))`,
			env: AccessRequestExpressionEnv{
				UserTraits: map[string][]string{
					"level":    {"L1"},
					"team":     {"Cloud"},
					"location": {"Seattle"},
				},
			},
			match: true,
		},
		{
			description: "matches some user traits",
			condition: `
				contains_any(user.traits["level"], set("L1", "L2")) &&
				contains_any(user.traits["team"], set("Cloud")) &&
				contains_any(user.traits["location"], set("Seattle"))`,
			env: AccessRequestExpressionEnv{
				UserTraits: map[string][]string{
					"level":    {"L1"},
					"team":     {"Tools"},
					"location": {"Seattle"},
				},
			},
			match: false,
		},
		{
			description: "match at least one role",
			condition: `
				contains_any(access_request.spec.roles, set("dev")) &&
				access_request.spec.roles.contains_any(set("dev"))`,
			env: AccessRequestExpressionEnv{
				Roles: []string{"dev", "stage", "prod"},
			},
			match: true,
		},
		{
			description: "does not match any role",
			condition: `
				contains_any(access_request.spec.roles, set("no-match")) ||
				access_request.spec.roles.contains_any(set("no-match"))`,
			env: AccessRequestExpressionEnv{
				Roles: []string{"dev", "stage", "prod"},
			},
			match: false,
		},
		{
			description: "matches all requested roles",
			condition: `
				contains_all(set("dev", "stage", "prod"), access_request.spec.roles) &&
				set("dev", "stage", "prod").contains_all(access_request.spec.roles)`,
			env: AccessRequestExpressionEnv{
				Roles: []string{"dev", "stage", "prod"},
			},
			match: true,
		},
		{
			description: "does not match all requested roles",
			condition: `
				contains_all(set("dev"), access_request.spec.roles) ||
				set("dev").contains_all(access_request.spec.roles)`,
			env: AccessRequestExpressionEnv{
				Roles: []string{"dev", "stage", "prod"},
			},
			match: false,
		},
		{
			description: "requested roles is empty",
			condition: `
				contains_all(set("dev"), access_request.spec.roles) ||
				set("dev").contains_all(access_request.spec.roles)`,
			env: AccessRequestExpressionEnv{
				Roles: []string{},
			},
			match: false,
		},
		{
			description: "both sets are empty",
			condition: `
				contains_all(set(), access_request.spec.roles) ||
				set().contains_all(access_request.spec.roles)`,
			env: AccessRequestExpressionEnv{
				Roles: []string{},
			},
			match: false,
		},
		{
			description: "(union) single resource has label",
			condition: `
				access_request.spec.resource_labels_union["env"].
					contains("test")`,
			env: AccessRequestExpressionEnv{
				RequestedResources: []types.ResourceWithLabels{
					&types.ServerV2{
						Metadata: types.Metadata{
							Labels: map[string]string{"env": "test"},
						},
					},
				},
			},
			match: true,
		},
		{
			description: "(union) multiple resources have label",
			condition: `
				access_request.spec.resource_labels_union["env"].
					contains_all(set("test", "dev"))`,
			env: AccessRequestExpressionEnv{
				RequestedResources: []types.ResourceWithLabels{
					&types.ServerV2{
						Metadata: types.Metadata{
							Labels: map[string]string{"env": "test"},
						},
					},
					&types.ServerV2{
						Metadata: types.Metadata{
							Labels: map[string]string{"env": "dev"},
						},
					},
				},
			},
			match: true,
		},
		{
			description: "(intersection) single resource has label",
			condition: `
				access_request.spec.resource_labels_intersection["env"].
					contains("test")`,
			env: AccessRequestExpressionEnv{
				RequestedResources: []types.ResourceWithLabels{
					&types.ServerV2{
						Metadata: types.Metadata{
							Labels: map[string]string{"env": "test"},
						},
					},
				},
			},
			match: true,
		},
		{
			description: "(intersection) multiple resources have label",
			condition: `
				access_request.spec.resource_labels_intersection["env"].
					contains("test")`,
			env: AccessRequestExpressionEnv{
				RequestedResources: []types.ResourceWithLabels{
					&types.ServerV2{
						Metadata: types.Metadata{
							Labels: map[string]string{"env": "test"},
						},
					},
					&types.ServerV2{
						Metadata: types.Metadata{
							Labels: map[string]string{"env": "test"},
						},
					},
				},
			},
			match: true,
		},
		{
			description: "(intersection) multiple resource labels do not intersect",
			condition: `
				access_request.spec.resource_labels_intersection["env"].
					contains("test")`,
			env: AccessRequestExpressionEnv{
				RequestedResources: []types.ResourceWithLabels{
					&types.ServerV2{
						Metadata: types.Metadata{
							Labels: map[string]string{"env": "test"},
						},
					},
					&types.ServerV2{
						Metadata: types.Metadata{
							Labels: map[string]string{"env": "dev"},
						},
					},
				},
			},
			match: false,
		},
		{
			description: "creation time is in schedule",
			condition:   `access_request.spec.creation_time.in_schedule(spec.schedules["test"])`,
			env: AccessRequestExpressionEnv{
				CreationTime: time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC), // Monday 14:30
				Schedules: ScheduleDict{
					"test": &accessmonitoringrulesv1.Schedule{
						Time: &accessmonitoringrulesv1.TimeSchedule{
							Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
								{
									Weekday: "Monday",
									Start:   "14:00",
									End:     "15:00",
								},
							},
						},
					},
				},
			},
			match: true,
		},
		{
			description: "shift interval is inclusive",
			condition:   `access_request.spec.creation_time.in_schedule(spec.schedules["test"])`,
			env: AccessRequestExpressionEnv{
				CreationTime: time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC), // Monday 14:30
				Schedules: ScheduleDict{
					"test": &accessmonitoringrulesv1.Schedule{
						Time: &accessmonitoringrulesv1.TimeSchedule{
							Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
								{
									Weekday: "Monday",
									Start:   "14:30",
									End:     "15:00",
								},
							},
						},
					},
				},
			},
			match: true,
		},
		{
			description: "schedule name not found",
			condition:   `access_request.spec.creation_time.in_schedule(spec.schedules["not-found"])`,
			env: AccessRequestExpressionEnv{
				CreationTime: time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC), // Monday 14:30
				Schedules: ScheduleDict{
					"test": &accessmonitoringrulesv1.Schedule{
						Time: &accessmonitoringrulesv1.TimeSchedule{
							Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
								{
									Weekday: "Monday",
									Start:   "14:00",
									End:     "15:00",
								},
							},
						},
					},
				},
			},
			match: false,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			match, err := EvaluateCondition(test.condition, test.env)
			require.NoError(t, err)
			require.Equal(t, test.match, match)
		})
	}
}
