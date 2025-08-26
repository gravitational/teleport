/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/utils"
)

// TestAccessListUnmarshal verifies an access list resource can be unmarshaled.
func TestAccessListUnmarshal(t *testing.T) {
	expected, err := accesslist.NewAccessList(
		header.Metadata{
			Name: "test-access-list",
		},
		accesslist.Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	)
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(accessListYAML))
	require.NoError(t, err)
	actual, err := UnmarshalAccessList(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestAccessListMarshal verifies a marshaled access list resource can be unmarshaled back.
func TestAccessListMarshal(t *testing.T) {
	expected, err := accesslist.NewAccessList(
		header.Metadata{
			Name: "test-access-list",
		},
		accesslist.Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	)

	require.NoError(t, err)
	data, err := MarshalAccessList(expected)
	require.NoError(t, err)

	actual, err := UnmarshalAccessList(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestAccessListMemberUnmarshal verifies an access list member resource can be unmarshaled.
func TestAccessListMemberUnmarshal(t *testing.T) {
	expected, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: "test-access-list-member",
		},
		accesslist.AccessListMemberSpec{
			AccessList: "access-list",
			Name:       "member1",
			Joined:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Expires:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Reason:     "because",
			AddedBy:    "test-user1",
		},
	)
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(accessListMemberYAML))
	require.NoError(t, err)
	actual, err := UnmarshalAccessListMember(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestAccessListMemberMarshal verifies a marshaled access list member resource can be unmarshaled back.
func TestAccessListMemberMarshal(t *testing.T) {
	expected, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: "test-access-list-member",
		},
		accesslist.AccessListMemberSpec{
			AccessList: "access-list",
			Name:       "member1",
			Joined:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Expires:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Reason:     "because",
			AddedBy:    "test-user1",
		},
	)
	require.NoError(t, err)
	data, err := MarshalAccessListMember(expected)
	require.NoError(t, err)
	actual, err := UnmarshalAccessListMember(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func requireAccessDenied(t require.TestingT, err error, i ...any) {
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %T: %s", err, err.Error())
}

// TestAccessListReviewUnmarshal verifies an access list review resource can be unmarshaled.
func TestAccessListReviewUnmarshal(t *testing.T) {
	expected, err := accesslist.NewReview(
		header.Metadata{
			Name: "test-access-list-review",
		},
		accesslist.ReviewSpec{
			AccessList: "access-list",
			Reviewers: []string{
				"user1",
				"user2",
			},
			ReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Notes:      "Some notes",
			Changes: accesslist.ReviewChanges{
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{
						"role1",
						"role2",
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
				ReviewFrequencyChanged:  accesslist.ThreeMonths,
				ReviewDayOfMonthChanged: accesslist.FifteenthDayOfMonth,
			},
		},
	)
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(accessListReviewYAML))
	require.NoError(t, err)
	actual, err := UnmarshalAccessListReview(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestMatchAccessList(t *testing.T) {
	al := &accesslist.AccessList{
		Spec: accesslist.Spec{
			Title:       "Production Database Access",
			Description: "Access to production MySQL and PostgreSQL databases",
			Owners: []accesslist.Owner{
				{Name: "john.doe"},
				{Name: "jane.smith"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"db-admin", "db-readonly", "backup-operator"},
			},
		},
	}
	al.SetName("prod-db-access")

	tests := []struct {
		name     string
		search   string
		expected bool
	}{
		{
			name:     "empty search matches all",
			search:   "",
			expected: true,
		},
		{
			name:     "whitespace only search matches all",
			search:   "   ",
			expected: true,
		},
		{
			name:     "exact title match",
			search:   "Production Database Access",
			expected: true,
		},
		{
			name:     "partial title match",
			search:   "Production",
			expected: true,
		},
		{
			name:     "case insensitive title match",
			search:   "database",
			expected: true,
		},
		{
			name:     "multiple words in title",
			search:   "Production Access",
			expected: true,
		},
		{
			name:     "exact name match",
			search:   "prod-db-access",
			expected: true,
		},
		{
			name:     "partial name match",
			search:   "prod-db",
			expected: true,
		},
		{
			name:     "case insensitive name match",
			search:   "PROD",
			expected: true,
		},
		{
			name:     "first owner match",
			search:   "john.doe",
			expected: true,
		},
		{
			name:     "second owner match",
			search:   "jane.smith",
			expected: true,
		},
		{
			name:     "partial owner match",
			search:   "john",
			expected: true,
		},
		{
			name:     "case insensitive owner match",
			search:   "JANE",
			expected: true,
		},
		{
			name:     "description match",
			search:   "MySQL",
			expected: true,
		},
		{
			name:     "case insensitive description match",
			search:   "postgresql",
			expected: true,
		},
		{
			name:     "multiple words in description",
			search:   "production databases",
			expected: true,
		},
		{
			name:     "exact role match",
			search:   "db-admin",
			expected: true,
		},
		{
			name:     "partial role match",
			search:   "readonly",
			expected: true,
		},
		{
			name:     "case insensitive role match",
			search:   "BACKUP",
			expected: true,
		},
		{
			name:     "multiple terms all found",
			search:   "Production db",
			expected: true,
		},
		{
			name:     "multiple terms across different fields",
			search:   "john database",
			expected: true,
		},
		{
			name:     "no match found",
			search:   "nonexistent",
			expected: false,
		},
		{
			name:     "partial match but not all terms",
			search:   "Production nonexistent",
			expected: false,
		},
		{
			name:     "case sensitive mismatch with special characters",
			search:   "prod_db_access",
			expected: false,
		},

		{
			name:     "single character match",
			search:   "p",
			expected: true,
		},
		{
			name:     "special characters in search",
			search:   "prod-db",
			expected: true,
		},
		{
			name:     "search with extra spaces",
			search:   "  Production   Database  ",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchAccessList(al, tt.search)
			if result != tt.expected {
				t.Errorf("MatchAccessList(%q) = %v, want %v", tt.search, result, tt.expected)
			}
		})
	}
}

// TestAccessListReviewMarshal verifies a marshaled access list review resource can be unmarshaled back.
func TestAccessListReviewMarshal(t *testing.T) {
	expected, err := accesslist.NewAccessList(
		header.Metadata{
			Name: "test-access-list-review",
		},
		accesslist.Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	)
	require.NoError(t, err)
	data, err := MarshalAccessList(expected)
	require.NoError(t, err)
	actual, err := UnmarshalAccessList(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var accessListYAML = `---
kind: access_list
version: v1
metadata:
  name: test-access-list
spec:
  title: "title"
  description: "test access list"  
  owners:
  - name: test-user1
    description: "test user 1"
  - name: test-user2
    description: "test user 2"
  audit:
    frequency: "1h"
    next_audit_date: "2023-02-02T00:00:00Z"
  membership_requires:
    roles:
    - mrole1
    - mrole2
    traits:
      mtrait1:
      - mvalue1
      - mvalue2
      mtrait2:
      - mvalue3
      - mvalue4
  ownership_requires:
    roles:
    - orole1
    - orole2
    traits:
      otrait1:
      - ovalue1
      - ovalue2
      otrait2:
      - ovalue3
      - ovalue4
  grants:
    roles:
    - grole1
    - grole2
    traits:
      gtrait1:
      - gvalue1
      - gvalue2
      gtrait2:
      - gvalue3
      - gvalue4
`

var accessListMemberYAML = `---
kind: access_list_member
version: v1
metadata:
  name: test-access-list-member
spec:
  access_list: access-list
  name: member1
  joined: 2023-01-01T00:00:00Z
  expires: 2024-01-01T00:00:00Z
  reason: "because"
  added_by: "test-user1"
`

var accessListReviewYAML = `---
kind: access_list_review
version: v1
metadata:
  name: test-access-list-review
spec:
  access_list: access-list
  reviewers:
  - user1
  - user2
  review_date: 2023-01-01T00:00:00Z
  notes: "Some notes"
  changes:
    membership_requirements_changed:
      roles:
      - role1
      - role2
      traits:
        trait1:
        - value1
        - value2
        trait2:
        - value1
        - value2
    removed_members:
    - member1
    - member2
    review_frequency_changed: 3 months
    review_day_of_month_changed: "15"
`
