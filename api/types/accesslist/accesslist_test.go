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

	"github.com/gravitational/teleport/api/types/header"
)

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

func TestRecurrenceConfiguration(t *testing.T) {
	t.Parallel()

	// This is a barebones access list that we'll use to modify and test lter.
	sourceAccessList := AccessList{
		ResourceHeader: header.ResourceHeaderFromMetadata(
			header.Metadata{
				Name: "recurrence-test",
			},
		),
		Spec: Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
			},
			MembershipRequires: Requires{},
			OwnershipRequires:  Requires{},
			Grants: Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	}

	// Recurrence is set.
	accessListDoesNotNeedConversion := sourceAccessList
	accessListDoesNotNeedConversion.Spec.Audit.NextAuditDate = time.Date(2023, 1, 12, 0, 0, 0, 0, time.UTC)
	require.NoError(t, accessListDoesNotNeedConversion.CheckAndSetDefaults())
	require.Zero(t, accessListDoesNotNeedConversion.Spec.Audit.Frequency)
	require.Equal(t, "FREQ=MONTHLY;INTERVAL=6;BYMONTHDAY=12;DTSTART=20230112", accessListDoesNotNeedConversion.Spec.Audit.Recurrence)

	// Frequency of 1 minute is set.
	accessList1MinuteConversion := sourceAccessList
	accessList1MinuteConversion.Spec.Audit.Frequency = time.Minute
	accessList1MinuteConversion.Spec.Audit.NextAuditDate = time.Date(2022, 2, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, accessList1MinuteConversion.CheckAndSetDefaults())
	require.Zero(t, accessList1MinuteConversion.Spec.Audit.Frequency)
	require.Equal(t, "FREQ=MONTHLY;INTERVAL=1;BYMONTHDAY=1;DTSTART=20220201", accessList1MinuteConversion.Spec.Audit.Recurrence)

	// Frequency of 1 month is set.
	accessList1MonthConversion := sourceAccessList
	accessList1MonthConversion.Spec.Audit.Frequency = averageHoursInAMonth * 1
	accessList1MonthConversion.Spec.Audit.NextAuditDate = time.Date(2022, 2, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, accessList1MonthConversion.CheckAndSetDefaults())
	require.Zero(t, accessList1MonthConversion.Spec.Audit.Frequency)
	require.Equal(t, "FREQ=MONTHLY;INTERVAL=1;BYMONTHDAY=1;DTSTART=20220201", accessList1MonthConversion.Spec.Audit.Recurrence)

	// Frequency of 12 months is set.
	accessList12MonthConversion := sourceAccessList
	accessList12MonthConversion.Spec.Audit.Frequency = averageHoursInAMonth * 12
	accessList12MonthConversion.Spec.Audit.NextAuditDate = time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, accessList12MonthConversion.CheckAndSetDefaults())
	require.Zero(t, accessList12MonthConversion.Spec.Audit.Frequency)
	require.Equal(t, "FREQ=MONTHLY;INTERVAL=12;BYMONTHDAY=15;DTSTART=20230115", accessList12MonthConversion.Spec.Audit.Recurrence)

	// Frequency of greater than 12 months is set. This should max out at 12 months.
	accessListGT12MonthConversion := sourceAccessList
	accessListGT12MonthConversion.Spec.Audit.Frequency = averageHoursInAMonth * 24
	accessListGT12MonthConversion.Spec.Audit.NextAuditDate = time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, accessListGT12MonthConversion.CheckAndSetDefaults())
	require.Zero(t, accessListGT12MonthConversion.Spec.Audit.Frequency)
	require.Equal(t, "FREQ=MONTHLY;INTERVAL=12;BYMONTHDAY=15;DTSTART=20230115", accessListGT12MonthConversion.Spec.Audit.Recurrence)
}

func TestAuditMarshaling(t *testing.T) {
	audit := Audit{
		Frequency:     time.Hour,
		NextAuditDate: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
		Recurrence:    "recurrence string",
	}

	data, err := json.Marshal(&audit)
	require.NoError(t, err)

	// Frequency is no longer being used, so it won't show up in the marshaled string.
	require.Equal(t, `{"next_audit_date":"2023-02-02T00:00:00Z","recurrence":"recurrence string"}`, string(data))

	raw := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(data, &raw))

	require.Equal(t, "2023-02-02T00:00:00Z", raw["next_audit_date"])
}

func TestAuditUnmarshaling(t *testing.T) {
	t.Parallel()

	raw := map[string]interface{}{
		"next_audit_date": "2023-02-02T00:00:00Z",
	}

	data, err := json.Marshal(&raw)
	require.NoError(t, err)

	var audit Audit
	require.NoError(t, json.Unmarshal(data, &audit))

	require.Equal(t, time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC), audit.NextAuditDate)

	raw = map[string]interface{}{
		"frequency":       "1h",
		"next_audit_date": "2023-02-02T00:00:00Z",
	}

	data, err = json.Marshal(&raw)
	require.NoError(t, err)

	audit = Audit{}
	require.NoError(t, json.Unmarshal(data, &audit))

	require.Equal(t, time.Hour, audit.Frequency)
	require.Equal(t, time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC), audit.NextAuditDate)
}
