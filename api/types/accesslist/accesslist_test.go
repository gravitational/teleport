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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/testutils/structfill"
)

func TestParseReviewFrequency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected ReviewFrequency
	}{
		{input: "1 month", expected: OneMonth},
		{input: "1month", expected: OneMonth},
		{input: "1months", expected: OneMonth},
		{input: "1 m", expected: OneMonth},
		{input: "1m", expected: OneMonth},
		{input: "1", expected: OneMonth},

		{input: "3 month", expected: ThreeMonths},
		{input: "3month", expected: ThreeMonths},
		{input: "3months", expected: ThreeMonths},
		{input: "3 m", expected: ThreeMonths},
		{input: "3m", expected: ThreeMonths},
		{input: "3", expected: ThreeMonths},

		{input: "6 month", expected: SixMonths},
		{input: "6month", expected: SixMonths},
		{input: "6months", expected: SixMonths},
		{input: "6 m", expected: SixMonths},
		{input: "6m", expected: SixMonths},
		{input: "6", expected: SixMonths},

		{input: "12 month", expected: OneYear},
		{input: "12month", expected: OneYear},
		{input: "12months", expected: OneYear},
		{input: "12 m", expected: OneYear},
		{input: "12m", expected: OneYear},
		{input: "12", expected: OneYear},
		{input: "1 year", expected: OneYear},
		{input: "1year", expected: OneYear},
		{input: "1 y", expected: OneYear},
		{input: "1y", expected: OneYear},

		{input: "1 MoNtH", expected: OneMonth},
		{input: "unknown"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, parseReviewFrequency(test.input))
		})
	}
}

func TestParseReviewDayOfMonth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected ReviewDayOfMonth
	}{
		{input: "1", expected: FirstDayOfMonth},
		{input: "first", expected: FirstDayOfMonth},

		{input: "15", expected: FifteenthDayOfMonth},

		{input: "last", expected: LastDayOfMonth},

		{input: "FiRSt", expected: FirstDayOfMonth},
		{input: "unknown"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, parseReviewDayOfMonth(test.input))
		})
	}
}

func TestDeduplicateOwners(t *testing.T) {
	accessList, err := NewAccessList(
		header.Metadata{
			Name: "duplicate test",
		},
		Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
				{
					Name:        "test-user2",
					Description: "duplicate",
				},
			},
			Audit: Audit{
				NextAuditDate: time.Now(),
			},
			MembershipRequires: Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	)
	require.NoError(t, err)

	require.Len(t, accessList.Spec.Owners, 2)
	require.Equal(t, "test-user1", accessList.Spec.Owners[0].Name)
	require.Equal(t, "test user 1", accessList.Spec.Owners[0].Description)
	require.Equal(t, "test-user2", accessList.Spec.Owners[1].Name)
	require.Equal(t, "test user 2", accessList.Spec.Owners[1].Description)
}

func TestAuditMarshaling(t *testing.T) {
	audit := Audit{
		NextAuditDate: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
		Recurrence: Recurrence{
			Frequency:  SixMonths,
			DayOfMonth: LastDayOfMonth,
		},
		Notifications: Notifications{
			Start: 4 * time.Hour,
		},
	}

	data, err := json.Marshal(&audit)
	require.NoError(t, err)

	require.Equal(t, `{"next_audit_date":"2023-02-02T00:00:00Z","recurrence":{"frequency":"6 months","day_of_month":"last"},"notifications":{"start":"4h0m0s"}}`, string(data))
}

func TestAuditUnmarshaling(t *testing.T) {
	const twoWeeks = 14 * 24 * time.Hour

	tests := []struct {
		name                      string
		input                     map[string]interface{}
		expectedNextAudit         time.Time
		expectedRecurrence        Recurrence
		expectedNotificationStart time.Duration
	}{
		{
			name: "with next_audit_date",
			input: map[string]interface{}{
				"next_audit_date": "2023-02-02T00:00:00Z",
				"recurrence": map[string]interface{}{
					"frequency":    "3 months",
					"day_of_month": "1",
				},
				"notifications": map[string]interface{}{
					"start": twoWeeks.String(),
				},
			},
			expectedNextAudit: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
			expectedRecurrence: Recurrence{
				Frequency:  ThreeMonths,
				DayOfMonth: FirstDayOfMonth,
			},
			expectedNotificationStart: twoWeeks,
		},
		{
			name: "without next_audit_date",
			input: map[string]interface{}{
				"recurrence": map[string]interface{}{
					"frequency":    "3 months",
					"day_of_month": "1",
				},
				"notifications": map[string]interface{}{
					"start": twoWeeks.String(),
				},
			},
			expectedNextAudit: time.Time{},
			expectedRecurrence: Recurrence{
				Frequency:  ThreeMonths,
				DayOfMonth: FirstDayOfMonth,
			},
			expectedNotificationStart: twoWeeks,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(&tt.input)
			require.NoError(t, err)

			var audit Audit
			require.NoError(t, json.Unmarshal(data, &audit))

			require.Equal(t, tt.expectedNextAudit, audit.NextAuditDate)
			require.Equal(t, tt.expectedRecurrence, audit.Recurrence)
			require.Equal(t, tt.expectedNotificationStart, audit.Notifications.Start)
		})
	}
}

func TestSelectNextReviewDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		accessListTypes   []Type
		frequency         ReviewFrequency
		dayOfMonth        ReviewDayOfMonth
		currentReviewDate time.Time
		expected          time.Time
		expectedErr       bool
	}{
		{
			name:              "one month, first day",
			accessListTypes:   []Type{Default, DeprecatedDynamic},
			frequency:         OneMonth,
			dayOfMonth:        FirstDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			expectedErr:       false,
		},
		{
			name:              "one month, fifteenth day",
			accessListTypes:   []Type{Default, DeprecatedDynamic},
			frequency:         OneMonth,
			dayOfMonth:        FifteenthDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC),
			expectedErr:       false,
		},
		{
			name:              "one month, last day",
			accessListTypes:   []Type{Default, DeprecatedDynamic},
			frequency:         OneMonth,
			dayOfMonth:        LastDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 28, 0, 0, 0, 0, time.UTC),
			expectedErr:       false,
		},
		{
			name:              "six months, last day",
			accessListTypes:   []Type{Default, DeprecatedDynamic},
			frequency:         SixMonths,
			dayOfMonth:        LastDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 7, 31, 0, 0, 0, 0, time.UTC),
			expectedErr:       false,
		},
		{
			name:              "six months, last day",
			accessListTypes:   []Type{Static, SCIM, "__test_unknown__"},
			frequency:         SixMonths,
			dayOfMonth:        LastDayOfMonth,
			currentReviewDate: time.Time{},
			expected:          time.Time{},
			expectedErr:       true,
		},
		{
			name:              "six months, last day",
			accessListTypes:   []Type{Static, SCIM, "__test_unknown__"},
			frequency:         SixMonths,
			dayOfMonth:        LastDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Time{},
			expectedErr:       true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			for _, typ := range test.accessListTypes {
				t.Run(fmt.Sprintf("type=%q", typ), func(t *testing.T) {
					accessList := AccessList{}
					accessList.Spec.Type = typ
					accessList.Spec.Audit.NextAuditDate = test.currentReviewDate
					accessList.Spec.Audit.Recurrence = Recurrence{
						Frequency:  test.frequency,
						DayOfMonth: test.dayOfMonth,
					}
					nextReviewDate, err := accessList.SelectNextReviewDate()
					if test.expectedErr {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						require.Equal(t, test.expected, nextReviewDate)
					}
				})
			}
		})
	}
}

func TestAccessList_setInitialReviewDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		frequency         ReviewFrequency
		dayOfMonth        ReviewDayOfMonth
		currentReviewDate time.Time
		expected          time.Time
	}{
		{
			name:              "one month, first day",
			frequency:         OneMonth,
			dayOfMonth:        FirstDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:              "one month, fifteenth day",
			frequency:         OneMonth,
			dayOfMonth:        FifteenthDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:              "one month, last day",
			frequency:         OneMonth,
			dayOfMonth:        LastDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:              "six months, last day",
			frequency:         SixMonths,
			dayOfMonth:        LastDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 7, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			accessList := AccessList{}
			accessList.Spec.Audit.Recurrence = Recurrence{
				Frequency:  test.frequency,
				DayOfMonth: test.dayOfMonth,
			}
			accessList.setInitialAuditDate(clockwork.NewFakeClockAt(test.currentReviewDate))
			require.Equal(t, test.expected, accessList.Spec.Audit.NextAuditDate)
		})
	}
}

func TestAccessListClone(t *testing.T) {
	item := &AccessList{}
	err := structfill.Fill(item)
	require.NoError(t, err)
	cpy := item.Clone()
	require.Empty(t, cmp.Diff(item, cpy))
	require.NotSame(t, item, cpy)
}

func TestEqualIgnoreEphemeralFields(t *testing.T) {
	t.Parallel()

	baseTime := time.Now()
	createAccessList := func(name string) *AccessList {
		al, err := NewAccessList(
			header.Metadata{
				Name:     name,
				Revision: "rev1",
			},
			Spec{
				Title:       "Test Access List",
				Description: "Test description",
				Owners: []Owner{
					{
						Name:             "owner1",
						Description:      "First owner",
						IneligibleStatus: "ineligible-reason",
						MembershipKind:   MembershipKindUser,
					},
					{
						Name:             "owner2",
						Description:      "Second owner",
						IneligibleStatus: "another-reason",
						MembershipKind:   MembershipKindUser,
					},
				},
				Audit: Audit{
					NextAuditDate: baseTime,
					Recurrence: Recurrence{
						Frequency:  SixMonths,
						DayOfMonth: FirstDayOfMonth,
					},
					Notifications: Notifications{
						Start: 14 * 24 * time.Hour,
					},
				},
				MembershipRequires: Requires{
					Roles:  []string{"role1"},
					Traits: map[string][]string{"trait1": {"value1"}},
				},
				OwnershipRequires: Requires{
					Roles:  []string{"owner-role"},
					Traits: map[string][]string{"owner-trait": {"owner-value"}},
				},
				Grants: Grants{
					Roles:  []string{"granted-role"},
					Traits: map[string][]string{"granted-trait": {"granted-value"}},
				},
				OwnerGrants: Grants{
					Roles:  []string{"owner-granted-role"},
					Traits: map[string][]string{"owner-granted-trait": {"owner-granted-value"}},
				},
			},
		)
		require.NoError(t, err)

		al.Status = Status{
			OwnerOf:  []string{"list1", "list2"},
			MemberOf: []string{"list3", "list4"},
		}

		return al
	}

	t.Run("identical access lists are equal", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")

		result := EqualAccessLists(al1, al2, WithIgnoreEphemeralFields())
		require.True(t, result)
	})

	t.Run("default behavior does not mutate originals", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")

		originalRevision := al1.Metadata.Revision
		originalStatus := al1.Status
		originalIneligibleStatus := al1.Spec.Owners[0].IneligibleStatus

		EqualAccessLists(al1, al2, WithIgnoreEphemeralFields())

		// Verify original values are preserved
		require.Equal(t, originalRevision, al1.Metadata.Revision)
		require.Equal(t, originalStatus, al1.Status)
		require.Equal(t, originalIneligibleStatus, al1.Spec.Owners[0].IneligibleStatus)
	})

	t.Run("WithSkipClone mutates originals", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")

		originalRevision := al1.Metadata.Revision
		originalIneligibleStatus := al1.Spec.Owners[0].IneligibleStatus

		EqualAccessLists(al1, al2, WithSkipClone(), WithIgnoreEphemeralFields())

		// Verify values were mutated
		require.Empty(t, al1.Metadata.Revision)
		require.NotEqual(t, originalRevision, al1.Metadata.Revision)
		require.Empty(t, al1.Spec.Owners[0].IneligibleStatus)
		require.NotEqual(t, originalIneligibleStatus, al1.Spec.Owners[0].IneligibleStatus)
	})

	t.Run("different revisions are ignored", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Metadata.Revision = "different-revision"

		result := EqualAccessLists(al1, al2, WithIgnoreEphemeralFields())
		require.True(t, result)
	})

	t.Run("different status is ignored", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Status.OwnerOf = []string{"different", "lists"}
		al2.Status.MemberOf = []string{"other", "lists"}

		result := EqualAccessLists(al1, al2, WithIgnoreEphemeralFields())
		require.True(t, result)
	})

	t.Run("different owner ineligible status is ignored", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Spec.Owners[0].IneligibleStatus = "completely-different-reason"
		al2.Spec.Owners[1].IneligibleStatus = ""

		result := EqualAccessLists(al1, al2, WithIgnoreEphemeralFields())
		require.True(t, result)
	})

	t.Run("all ignored fields different at once", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")

		// Change all ignored fields
		al2.Metadata.Revision = "rev999"
		al2.Status.OwnerOf = []string{"completely", "different", "lists"}
		al2.Spec.Owners[0].IneligibleStatus = "new-reason-1"
		al2.Spec.Owners[1].IneligibleStatus = "new-reason-2"

		result := EqualAccessLists(al1, al2, WithIgnoreEphemeralFields())
		require.True(t, result)
	})

	t.Run("different name causes inequality", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test2")

		result := EqualAccessLists(al1, al2, WithIgnoreEphemeralFields())
		require.False(t, result)
	})

	t.Run("different title causes inequality", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Spec.Title = "Different Title"

		require.False(t, EqualAccessLists(al1, al2, WithIgnoreEphemeralFields()))
	})

	t.Run("different description causes inequality", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Spec.Description = "Different description"

		require.False(t, EqualAccessLists(al1, al2, WithIgnoreEphemeralFields()))
	})

	t.Run("different owner name causes inequality", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Spec.Owners[0].Name = "different-owner"

		require.False(t, EqualAccessLists(al1, al2, WithIgnoreEphemeralFields()))
	})

	t.Run("different owner description causes inequality", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Spec.Owners[0].Description = "Different owner description"

		require.False(t, EqualAccessLists(al1, al2, WithIgnoreEphemeralFields()))
	})

	t.Run("different audit settings cause inequality", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Spec.Audit.Recurrence.Frequency = OneMonth

		require.False(t, EqualAccessLists(al1, al2, WithIgnoreEphemeralFields()))
	})

	t.Run("different grants cause inequality", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Spec.Grants.Roles = []string{"different-role"}

		require.False(t, EqualAccessLists(al1, al2, WithIgnoreEphemeralFields()))
	})

	t.Run("different membership requires cause inequality", func(t *testing.T) {
		al1 := createAccessList("test1")
		al2 := createAccessList("test1")
		al2.Spec.MembershipRequires.Roles = []string{"different-role"}

		require.False(t, EqualAccessLists(al1, al2, WithIgnoreEphemeralFields()))
	})

	t.Run("one nil access list", func(t *testing.T) {
		al1 := createAccessList("test1")
		var al2 *AccessList

		require.False(t, EqualAccessLists(al1, al2, WithIgnoreEphemeralFields()))
		require.False(t, EqualAccessLists(al2, al1, WithIgnoreEphemeralFields()))
	})
}
