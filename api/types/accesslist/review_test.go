/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package accesslist

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/trait"
)

func TestReviewSpecMarshaling(t *testing.T) {
	reviewSpec := ReviewSpec{
		AccessList: "access-list",
		Reviewers: []string{
			"user1",
			"user2",
		},
		ReviewDate: time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC),
		Notes:      "Some notes",
		Changes: ReviewChanges{
			MembershipRequirementsChanged: &Requires{
				Roles: []string{
					"member1",
					"member2",
				},
				Traits: trait.Traits{
					"trait1": []string{
						"value1",
						"value2",
					},
					"trait2": []string{
						"value1",
						"value2",
					},
				},
			},
			RemovedMembers: []string{
				"member1",
				"member2",
			},
			ReviewFrequencyChanged:  SixMonths,
			ReviewDayOfMonthChanged: FirstDayOfMonth,
		},
	}

	data, err := json.Marshal(&reviewSpec)
	require.NoError(t, err)

	require.Equal(t, `{"review_date":"2023-01-01T00:00:00Z","access_list":"access-list","reviewers":["user1","user2"],`+
		`"notes":"Some notes","changes":{"review_frequency_changed":"6 months","review_day_of_month_changed":"1",`+
		`"membership_requirements_changed":{"roles":["member1","member2"],"traits":{"trait1":["value1","value2"],"trait2":["value1","value2"]}},`+
		`"removed_members":["member1","member2"]}}`, string(data))

	raw := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(data, &raw))

	require.Equal(t, "2023-01-01T00:00:00Z", raw["review_date"])
	require.Equal(t, SixMonths.String(), raw["changes"].(map[string]interface{})["review_frequency_changed"])
	require.Equal(t, FirstDayOfMonth.String(), raw["changes"].(map[string]interface{})["review_day_of_month_changed"])
}

func TestReviewSpecUnmarshaling(t *testing.T) {
	raw := map[string]interface{}{
		"access_list": "access-list",
		"reviewers": []string{
			"user1",
			"user2",
		},
		"review_date": "2023-01-01T00:00:00Z",
		"notes":       "Some notes",
		"changes": map[string]interface{}{
			"membership_requirements_changed": map[string]interface{}{
				"roles": []string{
					"member1",
					"member2",
				},
				"traits": map[string]interface{}{
					"trait1": []string{
						"value1",
						"value2",
					},
					"trait2": []string{
						"value1",
						"value2",
					},
				},
			},
			"removed_members": []string{
				"member1",
				"member2",
			},
			"review_frequency_changed":    "1 month",
			"review_day_of_month_changed": "1",
		},
	}

	data, err := json.Marshal(&raw)
	require.NoError(t, err)

	var reviewSpec ReviewSpec
	require.NoError(t, json.Unmarshal(data, &reviewSpec))

	require.Equal(t, time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC), reviewSpec.ReviewDate)
	require.Equal(t, OneMonth, reviewSpec.Changes.ReviewFrequencyChanged)
	require.Equal(t, FirstDayOfMonth, reviewSpec.Changes.ReviewDayOfMonthChanged)
}
