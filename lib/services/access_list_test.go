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
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
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
			errAssertionFunc: requireAccessDenied,
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
			errAssertionFunc: requireAccessDenied,
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
			errAssertionFunc: requireAccessDenied,
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

// testMembersAndLockGetter implements AccessListMembersGetter and LockGetter for testing.
type testMembersAndLockGetter struct {
	members map[string]map[string]*accesslist.AccessListMember
	locks   map[string]types.Lock
}

// ListAccessListMembers returns a paginated list of all access list members.
func (t *testMembersAndLockGetter) ListAccessListMembers(ctx context.Context, accessList string, _ int, _ string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	for _, member := range t.members[accessList] {
		members = append(members, member)
	}
	return members, "", nil
}

// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
func (t *testMembersAndLockGetter) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	var allMembers []*accesslist.AccessListMember
	for _, members := range t.members {
		for _, member := range members {
			allMembers = append(allMembers, member)
		}
	}

	return allMembers, "", nil
}

// GetAccessListMember returns the specified access list member resource.
func (t *testMembersAndLockGetter) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
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

// GetLock gets a lock by name.
func (t *testMembersAndLockGetter) GetLock(_ context.Context, name string) (types.Lock, error) {
	if t.locks == nil {
		return nil, trace.NotFound("not found")
	}

	lock, ok := t.locks[name]
	if !ok {
		return nil, trace.NotFound("not found")
	}

	return lock, nil
}

// GetLocks gets all/in-force locks that match at least one of the targets when specified.
func (t *testMembersAndLockGetter) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	locks := make([]types.Lock, 0, len(t.locks))
	for _, lock := range t.locks {
		locks = append(locks, lock)
	}
	return locks, nil
}

func requireAccessDenied(t require.TestingT, err error, i ...interface{}) {
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %T: %s", err, err.Error())
}

func TestIsAccessListMemberChecker(t *testing.T) {
	tests := []struct {
		name             string
		identity         tlsca.Identity
		memberCtx        context.Context
		currentTime      time.Time
		locks            map[string]types.Lock
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
			name: "is locked member",
			identity: tlsca.Identity{
				Username: member1,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			locks: map[string]types.Lock{
				"test-lock": newUserLock(t, "test-lock", member1),
			},
			currentTime: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.AccessDenied("user %s is currently locked", member1))
			},
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
			currentTime:      time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: requireAccessDenied,
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
			currentTime:      time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: requireAccessDenied,
		},
		{
			name: "is member with no expiration and missing roles",
			identity: tlsca.Identity{
				Username: member3,
				Groups:   []string{"mrole1"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime:      time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: requireAccessDenied,
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
			currentTime:      time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: requireAccessDenied,
		},
		{
			name: "is member with no expiration and missing traits",
			identity: tlsca.Identity{
				Username: member3,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1"},
					"mtrait2": {"mvalue3"},
				},
			},
			currentTime:      time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: requireAccessDenied,
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
			getter := &testMembersAndLockGetter{members: memberMap, locks: test.locks}

			checker := NewAccessListMembershipChecker(clockwork.NewFakeClockAt(test.currentTime), getter, getter)
			test.errAssertionFunc(t, checker.IsAccessListMember(ctx, test.identity, accessList))
		})
	}
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
				Recurrence: accesslist.Recurrence{
					Frequency:  accesslist.ThreeMonths,
					DayOfMonth: accesslist.FifteenthDayOfMonth,
				},
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

func newUserLock(t *testing.T, name, user string) types.Lock {
	t.Helper()

	lock, err := types.NewLock(name, types.LockSpecV2{
		Target: types.LockTarget{
			User: user,
		},
	})
	require.NoError(t, err)

	return lock
}
