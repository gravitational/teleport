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

	"github.com/gravitational/teleport/api/types/header"
	"github.com/stretchr/testify/require"
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
				Frequency: time.Hour,
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
		Frequency:     time.Hour,
		NextAuditDate: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(&audit)
	require.NoError(t, err)

	require.Equal(t, `{"frequency":"1h0m0s","next_audit_date":"2023-02-02T00:00:00Z"}`, string(data))

	raw := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(data, &raw))

	require.Equal(t, "1h0m0s", raw["frequency"])
	require.Equal(t, "2023-02-02T00:00:00Z", raw["next_audit_date"])
}

func TestAuditUnmarshaling(t *testing.T) {
	raw := map[string]interface{}{
		"frequency":       "1h",
		"next_audit_date": "2023-02-02T00:00:00Z",
	}

	data, err := json.Marshal(&raw)
	require.NoError(t, err)

	var audit Audit
	require.NoError(t, json.Unmarshal(data, &audit))

	require.Equal(t, time.Hour, audit.Frequency)
	require.Equal(t, time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC), audit.NextAuditDate)
}
