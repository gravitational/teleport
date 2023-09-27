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

package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"github.com/teambition/rrule-go"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	ownerUser = "owner-user"
	member1   = "member1"
	member2   = "member2"
	member3   = "member3"
	member4   = "member4"
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
				Frequency:     time.Hour,
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
				Frequency:     time.Hour,
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

func TestIsAccessListOwner(t *testing.T) {
	tests := []struct {
		name             string
		identity         tlsca.Identity
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "is owner",
			identity: tlsca.Identity{
				Username: ownerUser,
				Groups:   []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "is not an owner",
			identity: tlsca.Identity{
				Username: "not-owner",
				Groups:   []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "is owner with missing roles",
			identity: tlsca.Identity{
				Username: "not-owner",
				Groups:   []string{"orole1"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "is owner with missing traits",
			identity: tlsca.Identity{
				Username: "not-owner",
				Groups:   []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1"},
					"otrait2": {"ovalue3"},
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			accessList := newAccessList(t)
			test.errAssertionFunc(t, IsAccessListOwner(test.identity, accessList))
		})
	}
}

// testMembersGetter implements AccessListMembersGetter for testing.
type testMembersGetter struct {
	members map[string]map[string]*accesslist.AccessListMember
}

// ListAccessListMembers returns a paginated list of all access list members.
func (t *testMembersGetter) ListAccessListMembers(ctx context.Context, accessList string, _ int, _ string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	for _, member := range t.members[accessList] {
		members = append(members, member)
	}
	return members, "", nil
}

// GetAccessListMember returns the specified access list member resource.
func (t *testMembersGetter) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	members, ok := t.members[accessList]
	if !ok {
		return nil, trace.NotFound("not found")
	}

	member, ok := members[memberName]
	if !ok {
		return nil, trace.NotFound("not found")
	}

	return member, nil
}

func TestIsAccessListMember(t *testing.T) {
	tests := []struct {
		name             string
		identity         tlsca.Identity
		memberCtx        context.Context
		currentTime      time.Time
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "is member",
			identity: tlsca.Identity{
				Username: member1,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime:      time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: require.NoError,
		},
		{
			name: "is not a member",
			identity: tlsca.Identity{
				Username: member4,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "is expired member",
			identity: tlsca.Identity{
				Username: member2,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "member has no expiration",
			identity: tlsca.Identity{
				Username: member3,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime:      time.Date(2030, 7, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: require.NoError,
		},
		{
			name: "is member with missing roles",
			identity: tlsca.Identity{
				Username: member1,
				Groups:   []string{"mrole1"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "is member with missing traits",
			identity: tlsca.Identity{
				Username: member1,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1"},
					"mtrait2": {"mvalue3"},
				},
			},
			currentTime: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			accessList := newAccessList(t)
			members := newAccessListMembers(t)

			memberMap := map[string]map[string]*accesslist.AccessListMember{}
			for _, member := range members {
				accessListName := member.Spec.AccessList
				if _, ok := memberMap[accessListName]; !ok {
					memberMap[accessListName] = map[string]*accesslist.AccessListMember{}
				}
				memberMap[accessListName][member.Spec.Name] = member
			}
			getter := &testMembersGetter{members: memberMap}

			test.errAssertionFunc(t, IsAccessListMember(ctx, test.identity, clockwork.NewFakeClockAt(test.currentTime), accessList, getter))
		})
	}
}

func TestSelectNextAccessListReviewDate(t *testing.T) {
	t.Parallel()

	// Test over 20 years, which includes several leap years
	for year := 2023; year < 2053; year++ {
		year := year
		t.Run(fmt.Sprintf("select review date year %d", year), func(t *testing.T) {
			t.Parallel()

			// January dates, 1 month ahead
			februaryMaxDay := time.Date(year, time.March, 0, 0, 0, 0, 0, time.UTC).Day()
			for i := 1; i <= 31; i++ {
				startDate := time.Date(year, time.January, i, 0, 0, 0, 0, time.UTC)
				februaryDay := i
				if i >= februaryMaxDay {
					februaryDay = februaryMaxDay
				}
				recurrence := fmt.Sprintf("FREQ=MONTHLY;INTERVAL=1;BYMONTHDAY=%d;DTSTART=%s", februaryDay, startDate.Format(rrule.DateFormat))
				nextDate, err := SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, recurrence, startDate))
				require.NoError(t, err)
				expectedDate := time.Date(year, time.February, februaryDay, 0, 0, 0, 0, time.UTC)
				require.Equal(t, expectedDate, nextDate, "expected next review date of %s for starting date %s, got %s instead", expectedDate, startDate, nextDate)
			}

			// Recurrences starting from January.
			threeMonths := fmt.Sprintf("FREQ=MONTHLY;INTERVAL=3;DTSTART=%s", time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).Format(rrule.DateFormat))
			// 3 months from first days
			nextDate, err := SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.April, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.February, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.April, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.March, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.April, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.April, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.May, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.June, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.October, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.August, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.October, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.September, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year, time.October, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.October, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.November, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC), nextDate)
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, threeMonths, time.Date(year, time.December, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC), nextDate)

			// 200 years in the future
			years200 := fmt.Sprintf("FREQ=YEARLY;INTERVAL=200;DTSTART=%s", time.Date(year, time.July, 15, 0, 0, 0, 0, time.UTC).Format(rrule.DateFormat))
			nextDate, err = SelectNextAccessListReviewDate(newAccessListWithRecurrenceAndAuditDate(t, years200, time.Date(year, time.July, 15, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, err)
			require.Equal(t, time.Date(year+200, time.July, 15, 0, 0, 0, 0, time.UTC), nextDate)
		})
	}
}

// Verify defaults of the access list. We test these so that the API doesn't need the rrule dependency.
func TestAccessListRecurrenceDefaults(t *testing.T) {
	t.Parallel()

	// Frequency set, next audit date is zero, recurrence not set. This will trigger a frequency conversion.
	accessList := newAccessList(t)
	accessList.Spec.Audit.Frequency = time.Minute
	accessList.Spec.Audit.NextAuditDate = time.Time{}
	accessList.Spec.Audit.Recurrence = ""
	require.NoError(t, accessList.CheckAndSetDefaults())

	nextDate, err := SelectNextAccessListReviewDate(accessList)
	require.NoError(t, err)
	require.Equal(t, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), nextDate)

	// Frequency not set, next audit date is set, recurrence not set. This will trigger a default recurrence.
	accessList = newAccessList(t)
	accessList.Spec.Audit.NextAuditDate = time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC)
	accessList.Spec.Audit.Recurrence = ""
	require.NoError(t, accessList.CheckAndSetDefaults())

	nextDate, err = SelectNextAccessListReviewDate(accessList)
	require.NoError(t, err)
	require.Equal(t, time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC), nextDate)

	// Frequency not set, next audit date is set to a last day of the month, recurrence not set. This will trigger a default recurrence with
	// special handling for the last day of the month.
	accessList = newAccessList(t)
	accessList.Spec.Audit.NextAuditDate = time.Date(2023, 02, 28, 0, 0, 0, 0, time.UTC)
	accessList.Spec.Audit.Recurrence = ""
	require.NoError(t, accessList.CheckAndSetDefaults())

	nextDate, err = SelectNextAccessListReviewDate(accessList)
	require.NoError(t, err)
	require.Equal(t, time.Date(2023, 8, 31, 0, 0, 0, 0, time.UTC), nextDate)
}

func newAccessList(t *testing.T) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: "test",
		},
		accesslist.Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        ownerUser,
					Description: "owner user",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
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

	return accessList
}

func newAccessListWithRecurrenceAndAuditDate(t *testing.T, recurrence string, nextAuditDate time.Time) *accesslist.AccessList {
	t.Helper()

	accessList := newAccessList(t)
	accessList.Spec.Audit.Recurrence = recurrence
	accessList.Spec.Audit.NextAuditDate = nextAuditDate

	return accessList
}

func newAccessListMembers(t *testing.T) []*accesslist.AccessListMember {
	t.Helper()

	member1, err := accesslist.NewAccessListMember(header.Metadata{
		Name: member1,
	}, accesslist.AccessListMemberSpec{
		AccessList: "test",
		Name:       member1,
		Joined:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		Expires:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Reason:     "because",
		AddedBy:    ownerUser,
	})
	require.NoError(t, err)

	member2, err := accesslist.NewAccessListMember(header.Metadata{
		Name: member2,
	}, accesslist.AccessListMemberSpec{
		AccessList: "test",
		Name:       member2,
		Joined:     time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		Expires:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Reason:     "because again",
		AddedBy:    ownerUser,
	})
	require.NoError(t, err)

	member3, err := accesslist.NewAccessListMember(header.Metadata{
		Name: member3,
	}, accesslist.AccessListMemberSpec{
		AccessList: "test",
		Name:       member3,
		Joined:     time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		Reason:     "because for the third time",
		AddedBy:    ownerUser,
	})
	require.NoError(t, err)

	return []*accesslist.AccessListMember{member1, member2, member3}
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
