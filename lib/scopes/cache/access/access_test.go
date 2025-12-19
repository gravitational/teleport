/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package access

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// TestScopedAccessCacheReplication verifies basic replication behavior of the scoped access cache.
func TestScopedAccessCacheReplication(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	defer backend.Close()

	service := local.NewScopedAccessService(backend)

	events := local.NewEventsService(backend)

	// populate roles prior to starting the cache so that we can cover
	// loading of initial state.
	var expectedRoleNames []string
	expectedRoleRevisions := make(map[string]string)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("role-%d", i)
		crsp, err := service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
			Role: newScopedRole(name),
		})
		require.NoError(t, err)

		expectedRoleNames = append(expectedRoleNames, name)
		expectedRoleRevisions[name] = crsp.GetRole().GetMetadata().GetRevision()
	}

	// populate assignments prior to starting the cache so that we can cover
	// loading of initial state.
	var expectedAssignmentNames []string
	for i := 0; i < 10; i++ {
		assignment := newScopedRoleAssignment(expectedRoleNames[i])
		_, err := service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
			Assignment: assignment,
			RoleRevisions: map[string]string{
				expectedRoleNames[i]: expectedRoleRevisions[expectedRoleNames[i]],
			},
		})
		require.NoError(t, err)

		expectedAssignmentNames = append(expectedAssignmentNames, assignment.GetMetadata().GetName())
	}

	// populate access lists prior to starting the cache so that we can cover
	// loading of initial state.
	var expectedAccessListNames []string
	for i := range 10 {
		name := fmt.Sprintf("list-%d", i)
		list := newScopedAccessList(name)
		_, err := service.CreateScopedAccessList(ctx, &scopedaccessv1.CreateScopedAccessListRequest{
			List: list,
		})
		require.NoError(t, err)

		expectedAccessListNames = append(expectedAccessListNames, name)
	}

	// populate access list members prior to starting the cache so that we can cover
	// loading of initial state.
	var expectedAccessListMemberNames []string
	for i := range 10 {
		name := fmt.Sprintf("member-%d", i)
		member := newScopedAccessListMember(expectedAccessListNames[i], name)
		_, err := service.CreateScopedAccessListMember(ctx, &scopedaccessv1.CreateScopedAccessListMemberRequest{
			Member: member,
		})
		require.NoError(t, err)

		expectedAccessListMemberNames = append(expectedAccessListMemberNames, name)
	}

	// start the cache with the service and events.
	cache, err := NewCache(CacheConfig{
		Events:            events,
		Reader:            service,
		TTLCacheRetention: time.Hour, // ensures state-changes are from watcher events rather than ttl cache reloads
	})
	require.NoError(t, err)

	defer cache.Close()

	// verify that initial role states are immediately available
	var gotRoleNames []string
	for role, err := range scopedutils.RangeScopedRoles(ctx, cache, &scopedaccessv1.ListScopedRolesRequest{}) {
		require.NoError(t, err)

		gotRoleNames = append(gotRoleNames, role.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedRoleNames, gotRoleNames)

	// verify that initial assignment states are immediately available
	var gotAssignmentNames []string
	for assignment, err := range scopedutils.RangeScopedRoleAssignments(ctx, cache, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
		require.NoError(t, err)

		gotAssignmentNames = append(gotAssignmentNames, assignment.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedAssignmentNames, gotAssignmentNames)

	// verify that initial access list states states are immediately available
	var gotAccessListNames []string
	for list, err := range scopedutils.RangeScopedAccessLists(ctx, cache, &scopedaccessv1.ListScopedAccessListsRequest{}) {
		require.NoError(t, err)

		gotAccessListNames = append(gotAccessListNames, list.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedAccessListNames, gotAccessListNames)

	// verify that initial access list member states states are immediately available
	var gotAccessListMemberNames []string
	for member, err := range scopedutils.RangeScopedAccessListMembers(ctx, cache, &scopedaccessv1.ListScopedAccessListMembersRequest{}) {
		require.NoError(t, err)

		gotAccessListMemberNames = append(gotAccessListMemberNames, member.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedAccessListMemberNames, gotAccessListMemberNames)

	// perform additional role writes to cover event replication
	for i := 10; i < 20; i++ {
		name := fmt.Sprintf("role-%d", i)
		crsp, err := service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
			Role: newScopedRole(name),
		})
		require.NoError(t, err)

		expectedRoleNames = append(expectedRoleNames, name)
		expectedRoleRevisions[name] = crsp.GetRole().GetMetadata().GetRevision()
	}

	// perform additional assignment writes to cover event replication
	for i := 10; i < 20; i++ {
		assignment := newScopedRoleAssignment(expectedRoleNames[i])
		_, err := service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
			Assignment: assignment,
			RoleRevisions: map[string]string{
				expectedRoleNames[i]: expectedRoleRevisions[expectedRoleNames[i]],
			},
		})
		require.NoError(t, err)

		expectedAssignmentNames = append(expectedAssignmentNames, assignment.GetMetadata().GetName())
	}

	// perform additional access list writes to cover event replication
	for i := 10; i < 20; i++ {
		name := fmt.Sprintf("list-%d", i)
		list := newScopedAccessList(name)
		_, err := service.CreateScopedAccessList(ctx, &scopedaccessv1.CreateScopedAccessListRequest{
			List: list,
		})
		require.NoError(t, err)

		expectedAccessListNames = append(expectedAccessListNames, name)
	}

	// perform additional access list member writes to cover event replication
	for i := 10; i < 20; i++ {
		name := fmt.Sprintf("member-%d", i)
		member := newScopedAccessListMember(expectedAccessListNames[i], name)
		_, err := service.CreateScopedAccessListMember(ctx, &scopedaccessv1.CreateScopedAccessListMemberRequest{
			Member: member,
		})
		require.NoError(t, err)

		expectedAccessListMemberNames = append(expectedAccessListMemberNames, name)
	}

	// wait for the cache to replicate the new roles
	waitForRoleCondition(t, cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		t.Helper()
		return len(roles) >= len(expectedRoleNames)
	})

	gotRoleNames = nil
	for role, err := range scopedutils.RangeScopedRoles(ctx, cache, &scopedaccessv1.ListScopedRolesRequest{}) {
		require.NoError(t, err)
		gotRoleNames = append(gotRoleNames, role.GetMetadata().GetName())
	}
	require.ElementsMatch(t, expectedRoleNames, gotRoleNames)

	// wait for the cache to replicate the new assignments
	waitForAssignmentCondition(t, cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		t.Helper()
		return len(assignments) >= len(expectedAssignmentNames)
	})

	gotAssignmentNames = nil
	for assignment, err := range scopedutils.RangeScopedRoleAssignments(ctx, cache, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
		require.NoError(t, err)
		gotAssignmentNames = append(gotAssignmentNames, assignment.GetMetadata().GetName())
	}
	require.ElementsMatch(t, expectedAssignmentNames, gotAssignmentNames)

	// wait for the cache to replicate the new access lists
	waitForAccessListCondition(t, cache, func(lists []*scopedaccessv1.ScopedAccessList) bool {
		t.Helper()
		return len(lists) >= len(expectedAccessListNames)
	})

	gotAccessListNames = nil
	for list, err := range scopedutils.RangeScopedAccessLists(ctx, cache, &scopedaccessv1.ListScopedAccessListsRequest{}) {
		require.NoError(t, err)
		gotAccessListNames = append(gotAccessListNames, list.GetMetadata().GetName())
	}
	require.ElementsMatch(t, expectedAccessListNames, gotAccessListNames)

	// wait for the cache to replicate the new access list members
	waitForAccessListMemberCondition(t, cache, func(members []*scopedaccessv1.ScopedAccessListMember) bool {
		t.Helper()
		return len(members) >= len(expectedAccessListMemberNames)
	})

	gotAccessListMemberNames = nil
	for member, err := range scopedutils.RangeScopedAccessListMembers(ctx, cache, &scopedaccessv1.ListScopedAccessListMembersRequest{}) {
		require.NoError(t, err)
		gotAccessListMemberNames = append(gotAccessListMemberNames, member.GetMetadata().GetName())
	}
	require.ElementsMatch(t, expectedAccessListMemberNames, gotAccessListMemberNames)

	// test that cache can handle updates to existing roles (NOTE: no corellary for assignments)
	for role, err := range scopedutils.RangeScopedRoles(ctx, cache, &scopedaccessv1.ListScopedRolesRequest{}) {
		require.NoError(t, err)

		role.Metadata.Labels = map[string]string{"updated": "true"}

		crsp, err := service.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)

		expectedRoleRevisions[role.GetMetadata().GetName()] = crsp.GetRole().GetMetadata().GetRevision()
	}

	waitForRoleCondition(t, cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		t.Helper()
		for _, role := range roles {
			if role.GetMetadata().GetLabels()["updated"] != "true" {
				return false
			}
		}
		return true
	})

	// test that cache can handle partial deletes for assignments
	for _, name := range expectedAssignmentNames[0:10] {
		_, err := service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
			Name: name,
		})
		require.NoError(t, err)
	}

	waitForAssignmentCondition(t, cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		t.Helper()
		return len(assignments) <= len(expectedAssignmentNames)-10
	})

	gotAssignmentNames = nil
	for assignment, err := range scopedutils.RangeScopedRoleAssignments(ctx, cache, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
		require.NoError(t, err)
		gotAssignmentNames = append(gotAssignmentNames, assignment.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedAssignmentNames[10:], gotAssignmentNames)

	// test that cache can handle delete of all assignments
	for _, name := range expectedAssignmentNames[10:] {
		_, err := service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
			Name: name,
		})
		require.NoError(t, err)
	}

	waitForAssignmentCondition(t, cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		t.Helper()
		return len(assignments) == 0
	})

	// test that cache can handle partial deletes for roles
	for _, name := range expectedRoleNames[0:10] {
		_, err := service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
			Name: name,
		})
		require.NoError(t, err)
	}

	waitForRoleCondition(t, cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		t.Helper()
		return len(roles) <= len(expectedRoleNames)-10
	})

	gotRoleNames = nil
	for role, err := range scopedutils.RangeScopedRoles(ctx, cache, &scopedaccessv1.ListScopedRolesRequest{}) {
		require.NoError(t, err)
		gotRoleNames = append(gotRoleNames, role.GetMetadata().GetName())
	}
	require.ElementsMatch(t, expectedRoleNames[10:], gotRoleNames)

	// test that cache can handle delete of all roles
	for _, name := range expectedRoleNames[10:] {
		_, err := service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
			Name: name,
		})
		require.NoError(t, err)
	}

	waitForRoleCondition(t, cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		t.Helper()
		return len(roles) == 0
	})
}

// TestScopedAccessCacheFallback verified the fallback behavior of the scoped access cache
// when the upstream event system is unavailable.
func TestScopedAccessCacheFallback(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	defer backend.Close()

	service := local.NewScopedAccessService(backend)

	events := &neverEvents{} // use a fake events service that never initializes watchers

	// populate roles prior to starting the cache so that we can cover
	// loading of initial state.
	var expectedRoleNames []string
	expectedRoleRevisions := make(map[string]string)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("role-%d", i)
		crsp, err := service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
			Role: newScopedRole(name),
		})
		require.NoError(t, err)

		expectedRoleNames = append(expectedRoleNames, name)
		expectedRoleRevisions[name] = crsp.GetRole().GetMetadata().GetRevision()
	}

	// populate assignments prior to starting the cache so that we can cover
	// loading of initial state.
	var expectedAssignmentNames []string
	for i := 0; i < 10; i++ {
		assignment := newScopedRoleAssignment(expectedRoleNames[i])
		_, err := service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
			Assignment: assignment,
			RoleRevisions: map[string]string{
				expectedRoleNames[i]: expectedRoleRevisions[expectedRoleNames[i]],
			},
		})
		require.NoError(t, err)

		expectedAssignmentNames = append(expectedAssignmentNames, assignment.GetMetadata().GetName())
	}

	// start the cache with the service and never-events.
	cache, err := NewCache(CacheConfig{
		Events:            events,
		Reader:            service,
		TTLCacheRetention: time.Millisecond * 10, // ensure we don't spend more than 1 cycle waiting
	})
	require.NoError(t, err)

	defer cache.Close()

	// verify that initial role states are immediately available
	var gotRoleNames []string
	for role, err := range scopedutils.RangeScopedRoles(ctx, cache, &scopedaccessv1.ListScopedRolesRequest{}) {
		require.NoError(t, err)

		gotRoleNames = append(gotRoleNames, role.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedRoleNames, gotRoleNames)

	// verify that initial assignment states are immediately available
	var gotAssignmentNames []string
	for assignment, err := range scopedutils.RangeScopedRoleAssignments(ctx, cache, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
		require.NoError(t, err)

		gotAssignmentNames = append(gotAssignmentNames, assignment.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedAssignmentNames, gotAssignmentNames)

	// perform additional role writes to cover subsequent ttl-cache image loads
	for i := 10; i < 20; i++ {
		name := fmt.Sprintf("role-%d", i)
		crsp, err := service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
			Role: newScopedRole(name),
		})
		require.NoError(t, err)

		expectedRoleNames = append(expectedRoleNames, name)
		expectedRoleRevisions[name] = crsp.GetRole().GetMetadata().GetRevision()
	}

	// perform additional assignment writes to cover ttl-cache image loads
	for i := 10; i < 20; i++ {
		assignment := newScopedRoleAssignment(expectedRoleNames[i])
		_, err := service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
			Assignment: assignment,
			RoleRevisions: map[string]string{
				expectedRoleNames[i]: expectedRoleRevisions[expectedRoleNames[i]],
			},
		})
		require.NoError(t, err)

		expectedAssignmentNames = append(expectedAssignmentNames, assignment.GetMetadata().GetName())
	}

	// wait for the new roles to become visible
	waitForRoleCondition(t, cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		t.Helper()
		return len(roles) >= len(expectedRoleNames)
	})

	gotRoleNames = nil
	for role, err := range scopedutils.RangeScopedRoles(ctx, cache, &scopedaccessv1.ListScopedRolesRequest{}) {
		require.NoError(t, err)
		gotRoleNames = append(gotRoleNames, role.GetMetadata().GetName())
	}
	require.ElementsMatch(t, expectedRoleNames, gotRoleNames)

	// wait for the new assignments to become visible
	waitForAssignmentCondition(t, cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		t.Helper()
		return len(assignments) >= len(expectedAssignmentNames)
	})

	gotAssignmentNames = nil
	for assignment, err := range scopedutils.RangeScopedRoleAssignments(ctx, cache, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
		require.NoError(t, err)
		gotAssignmentNames = append(gotAssignmentNames, assignment.GetMetadata().GetName())
	}
	require.ElementsMatch(t, expectedAssignmentNames, gotAssignmentNames)
}

func TestAccessListMaterialization(t *testing.T) {
	t.Parallel()

	backend, err := memory.New(memory.Config{
		Context: t.Context(),
	})
	require.NoError(t, err)

	defer backend.Close()

	service := local.NewScopedAccessService(backend)

	events := local.NewEventsService(backend)

	cache, err := NewCache(CacheConfig{
		Events: events,
		Reader: service,
	})
	require.NoError(t, err)
	defer cache.Close()

	for roleName, roleSpec := range map[string]*scopedaccessv1.ScopedRoleSpec{
		"staging-east-access": {
			AssignableScopes: []string{"/staging/east"},
		},
		"staging-east-admin": {
			AssignableScopes: []string{"/staging/east"},
		},
		"staging-access": {
			AssignableScopes: []string{"/staging"},
		},
		"staging-admin": {
			AssignableScopes: []string{"/staging"},
		},
	} {
		role := newScopedRole(roleName)
		role.Spec = roleSpec
		_, err := service.CreateScopedRole(t.Context(), &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
	}

	for listName, grants := range map[string]*scopedaccessv1.ScopedAccessListGrants{
		"staging-east-users": {ScopedRoles: []*scopedaccessv1.ScopedRoleGrant{
			{
				Role:  "staging-east-access",
				Scope: "/staging/east",
			},
		}},
		"staging-east-admins": {ScopedRoles: []*scopedaccessv1.ScopedRoleGrant{
			{
				Role:  "staging-east-admin",
				Scope: "/staging/east",
			},
		}},
		"staging-users": {ScopedRoles: []*scopedaccessv1.ScopedRoleGrant{
			{
				Role:  "staging-access",
				Scope: "/staging",
			},
		}},
		"staging-admins": {ScopedRoles: []*scopedaccessv1.ScopedRoleGrant{
			{
				Role:  "staging-admin",
				Scope: "/staging",
			},
		}},
	} {
		list := newScopedAccessList(listName)
		list.Spec.Grants = grants
		_, err := service.CreateScopedAccessList(t.Context(), &scopedaccessv1.CreateScopedAccessListRequest{
			List: list,
		})
		require.NoError(t, err)
	}

	for _, memberSpec := range []*scopedaccessv1.ScopedAccessListMemberSpec{
		{
			// List staging-east-admins is a member of staging-east-users. This
			// should grant all members of staging-east-admins the roles
			// granted by staging-east-users.
			AccessList:     "staging-east-users",
			Name:           "staging-east-admins",
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_LIST,
		},
		{
			// List staging-admins is a member of staging-users. This should
			// grant all members of staging-admins the roles granted by
			// staging-users.
			AccessList:     "staging-users",
			Name:           "staging-admins",
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_LIST,
		},
		{
			// List staging-admins is a member of staging-east-admins. This
			// should grant all members of staging-admins the roles granted by
			// staging-east-admins.
			AccessList:     "staging-east-admins",
			Name:           "staging-admins",
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_LIST,
		},
		{
			// List staging-users is a member of staging-east-users. This
			// should grant all members of staging-users the roles granted by
			// staging-east-users.
			AccessList:     "staging-east-users",
			Name:           "staging-users",
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_LIST,
		},
		{
			AccessList:     "staging-east-users",
			Name:           "staging-east-agent",
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_USER,
		},
		{
			AccessList:     "staging-east-admins",
			Name:           "staging-east-owner",
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_USER,
		},
		{
			AccessList:     "staging-users",
			Name:           "staging-agent",
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_USER,
		},
		{
			AccessList:     "staging-admins",
			Name:           "staging-owner",
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_USER,
		},
	} {
		member := newScopedAccessListMember(memberSpec.AccessList, memberSpec.Name)
		member.Spec = memberSpec
		_, err := service.CreateScopedAccessListMember(t.Context(), &scopedaccessv1.CreateScopedAccessListMemberRequest{
			Member: member,
		})
		require.NoError(t, err)
	}

	type grant struct {
		role, scope string
	}
	type roleAssignment struct {
		scope  string
		user   string
		grants []grant
	}
	expectedAssignments := []roleAssignment{
		{
			user:  "staging-east-owner",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-east-admin",
					scope: "/staging/east",
				},
			},
		},
		{
			user:  "staging-east-owner",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-east-access",
					scope: "/staging/east",
				},
			},
		},
		{
			user:  "staging-east-agent",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-east-access",
					scope: "/staging/east",
				},
			},
		},
		{
			user:  "staging-owner",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-east-admin",
					scope: "/staging/east",
				},
			},
		},
		{
			user:  "staging-owner",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-east-access",
					scope: "/staging/east",
				},
			},
		},
		{
			user:  "staging-owner",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-admin",
					scope: "/staging",
				},
			},
		},
		{
			user:  "staging-owner",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-access",
					scope: "/staging",
				},
			},
		},
		{
			user:  "staging-agent",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-access",
					scope: "/staging",
				},
			},
		},
		{
			user:  "staging-agent",
			scope: "/",
			grants: []grant{
				{
					role:  "staging-east-access",
					scope: "/staging/east",
				},
			},
		},
	}

	waitForAssignmentCondition(t, cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		return len(assignments) >= len(expectedAssignments)
	})
	var gotAssignments []roleAssignment
	for assignment, err := range scopedutils.RangeScopedRoleAssignments(t.Context(), cache, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
		require.NoError(t, err)
		gotAssignment := roleAssignment{
			user:  assignment.GetSpec().GetUser(),
			scope: assignment.GetScope(),
		}
		for _, assign := range assignment.GetSpec().GetAssignments() {
			gotAssignment.grants = append(gotAssignment.grants, grant{
				role:  assign.GetRole(),
				scope: assign.GetScope(),
			})
		}
		gotAssignments = append(gotAssignments, gotAssignment)
	}

	assert.ElementsMatch(t, expectedAssignments, gotAssignments)
}

func newScopedRole(name string) *scopedaccessv1.ScopedRole {
	return &scopedaccessv1.ScopedRole{
		Kind: scopedaccess.KindScopedRole,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleSpec{
			AssignableScopes: []string{"/foo"},
		},
		Version: types.V1,
	}
}

func newScopedRoleAssignment(roleName string) *scopedaccessv1.ScopedRoleAssignment {
	return &scopedaccessv1.ScopedRoleAssignment{
		Kind: scopedaccess.KindScopedRoleAssignment,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "alice",
			Assignments: []*scopedaccessv1.Assignment{
				{
					Role:  roleName,
					Scope: "/foo",
				},
			},
		},
		Version: types.V1,
	}
}

func newScopedAccessList(name string) *scopedaccessv1.ScopedAccessList {
	return &scopedaccessv1.ScopedAccessList{
		Kind: scopedaccess.KindScopedAccessList,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedAccessListSpec{
			Title: name,
		},
		Version: types.V1,
	}
}

func newScopedAccessListMember(listName, memberName string) *scopedaccessv1.ScopedAccessListMember {
	return &scopedaccessv1.ScopedAccessListMember{
		Kind: scopedaccess.KindScopedAccessListMember,
		Metadata: &headerv1.Metadata{
			Name: memberName,
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedAccessListMemberSpec{
			AccessList:     listName,
			Name:           memberName,
			MembershipKind: scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_USER,
		},
		Version: types.V1,
	}
}

func waitForRoleCondition(t *testing.T, reader services.ScopedRoleReader, condition func([]*scopedaccessv1.ScopedRole) bool) {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		var roles []*scopedaccessv1.ScopedRole
		for role, err := range scopedutils.RangeScopedRoles(t.Context(), reader, &scopedaccessv1.ListScopedRolesRequest{}) {
			require.NoError(t, err)
			roles = append(roles, role)
		}

		if condition(roles) {
			return
		}

		select {
		case <-time.After(time.Millisecond * 120):
		case <-timeout:
			require.FailNow(t, "timeout waiting for role condition")
		}
	}
}

func waitForAssignmentCondition(t *testing.T, reader services.ScopedRoleAssignmentReader, condition func([]*scopedaccessv1.ScopedRoleAssignment) bool) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		var assignments []*scopedaccessv1.ScopedRoleAssignment
		for assignment, err := range scopedutils.RangeScopedRoleAssignments(t.Context(), reader, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
			require.NoError(t, err)
			assignments = append(assignments, assignment)
		}

		if condition(assignments) {
			return
		}

		select {
		case <-time.After(time.Millisecond * 200):
		case <-timeout:
			require.FailNow(t, "timeout waiting for assignment condition")
		}
	}
}

func waitForAccessListCondition(t *testing.T, reader services.ScopedAccessListReader, condition func([]*scopedaccessv1.ScopedAccessList) bool) {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		var lists []*scopedaccessv1.ScopedAccessList
		for list, err := range scopedutils.RangeScopedAccessLists(t.Context(), reader, &scopedaccessv1.ListScopedAccessListsRequest{}) {
			require.NoError(t, err)
			lists = append(lists, list)
		}

		if condition(lists) {
			return
		}

		select {
		case <-time.After(time.Millisecond * 120):
		case <-timeout:
			require.FailNow(t, "timeout waiting for access list condition")
		}
	}
}

func waitForAccessListMemberCondition(t *testing.T, reader services.ScopedAccessListMemberReader, condition func([]*scopedaccessv1.ScopedAccessListMember) bool) {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		var members []*scopedaccessv1.ScopedAccessListMember
		for member, err := range scopedutils.RangeScopedAccessListMembers(t.Context(), reader, &scopedaccessv1.ListScopedAccessListMembersRequest{}) {
			require.NoError(t, err)
			members = append(members, member)
		}

		if condition(members) {
			return
		}

		select {
		case <-time.After(time.Millisecond * 120):
		case <-timeout:
			require.FailNow(t, "timeout waiting for access list condition")
		}
	}
}

// neverEvents is a fake event service whose watchers never initialize.
type neverEvents struct{}

func (e *neverEvents) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return &neverWatcher{}, nil
}

// neverWatcher is a fake watcher that never initializes.
type neverWatcher struct{}

func (w *neverWatcher) Events() <-chan types.Event {
	return nil
}

func (w *neverWatcher) Done() <-chan struct{} {
	return nil
}

func (w *neverWatcher) Close() error {
	return nil
}

func (w *neverWatcher) Error() error {
	return nil
}
