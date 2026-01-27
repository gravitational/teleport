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
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/backend/memory"
	cachepkg "github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/cache/assignments"
	"github.com/gravitational/teleport/lib/scopes/cache/roles"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

type grant struct {
	role  string
	scope string
}

type roleAssignment struct {
	scope  string
	user   string
	grants []grant
}

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
	accessListService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: backend,
		Modules: modules.GetModules(),
	})
	require.NoError(t, err)

	events := local.NewEventsService(backend)
	accessListCache, err := cachepkg.New(cachepkg.Config{
		Context: ctx,
		Events:  events,
		Watches: []types.WatchKind{
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
		},
		AccessLists: accessListService,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, accessListCache.Close()) })

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

	// start the cache with the service and events.
	cache, err := NewCache(CacheConfig{
		Events:            events,
		Reader:            service,
		AccessListReader:  accessListCache,
		AccessListEvents:  accessListCache,
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
	accessListService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: backend,
		Modules: modules.GetModules(),
	})
	require.NoError(t, err)

	events := &neverEvents{} // use a fake events service that never initializes watchers
	accessListEvents := local.NewEventsService(backend)
	accessListCache, err := cachepkg.New(cachepkg.Config{
		Context: ctx,
		Events:  accessListEvents,
		Watches: []types.WatchKind{
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
		},
		AccessLists: accessListService,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, accessListCache.Close()) })

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
		AccessListReader:  accessListCache,
		AccessListEvents:  accessListCache,
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
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true, Limit: 100},
			},
		},
	})

	backend, err := memory.New(memory.Config{
		Context: t.Context(),
	})
	require.NoError(t, err)

	defer backend.Close()

	service := local.NewScopedAccessService(backend)
	accessListService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: backend,
		Modules: modules.GetModules(),
	})
	require.NoError(t, err)

	events := local.NewEventsService(backend)
	accessListCache, err := cachepkg.New(cachepkg.Config{
		Context: t.Context(),
		Events:  events,
		Watches: []types.WatchKind{
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
		},
		AccessLists: accessListService,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, accessListCache.Close()) })

	cache, err := NewCache(CacheConfig{
		Events:           events,
		Reader:           service,
		AccessListReader: accessListCache,
		AccessListEvents: accessListCache,
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

	for listName, grants := range map[string]accesslist.Grants{
		"staging-east-users": {ScopedRoles: []accesslist.ScopedRoleGrant{
			{
				Role:  "staging-east-access",
				Scope: "/staging/east",
			},
		}},
		"staging-east-admins": {ScopedRoles: []accesslist.ScopedRoleGrant{
			{
				Role:  "staging-east-admin",
				Scope: "/staging/east",
			},
		}},
		"staging-users": {ScopedRoles: []accesslist.ScopedRoleGrant{
			{
				Role:  "staging-access",
				Scope: "/staging",
			},
		}},
		"staging-admins": {ScopedRoles: []accesslist.ScopedRoleGrant{
			{
				Role:  "staging-admin",
				Scope: "/staging",
			},
		}},
	} {
		list := newAccessList(t, listName)
		list.Spec.Grants = grants
		_, err := accessListService.UpsertAccessList(t.Context(), list)
		require.NoError(t, err)
	}

	for _, memberSpec := range []accesslist.AccessListMemberSpec{
		{
			// List staging-east-admins is a member of staging-east-users. This
			// should grant all members of staging-east-admins the roles
			// granted by staging-east-users.
			AccessList:     "staging-east-users",
			Name:           "staging-east-admins",
			MembershipKind: accesslist.MembershipKindList,
		},
		{
			// List staging-admins is a member of staging-users. This should
			// grant all members of staging-admins the roles granted by
			// staging-users.
			AccessList:     "staging-users",
			Name:           "staging-admins",
			MembershipKind: accesslist.MembershipKindList,
		},
		{
			// List staging-admins is a member of staging-east-admins. This
			// should grant all members of staging-admins the roles granted by
			// staging-east-admins.
			AccessList:     "staging-east-admins",
			Name:           "staging-admins",
			MembershipKind: accesslist.MembershipKindList,
		},
		{
			// List staging-users is a member of staging-east-users. This
			// should grant all members of staging-users the roles granted by
			// staging-east-users.
			AccessList:     "staging-east-users",
			Name:           "staging-users",
			MembershipKind: accesslist.MembershipKindList,
		},
		{
			AccessList:     "staging-east-users",
			Name:           "staging-east-agent",
			MembershipKind: accesslist.MembershipKindUser,
		},
		{
			AccessList:     "staging-east-admins",
			Name:           "staging-east-owner",
			MembershipKind: accesslist.MembershipKindUser,
		},
		{
			AccessList:     "staging-users",
			Name:           "staging-agent",
			MembershipKind: accesslist.MembershipKindUser,
		},
		{
			AccessList:     "staging-admins",
			Name:           "staging-owner",
			MembershipKind: accesslist.MembershipKindUser,
		},
	} {
		member := newAccessListMember(t, memberSpec.AccessList, memberSpec.Name, memberSpec.MembershipKind)
		_, err := accessListService.UpsertAccessListMember(t.Context(), member)
		require.NoError(t, err)
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

	waitForRoleAssignments(t, cache, expectedAssignments)

	t.Run("add_remove_scoped_roles", func(t *testing.T) {
		role := newScopedRole("staging-north-access")
		role.Spec = &scopedaccessv1.ScopedRoleSpec{AssignableScopes: []string{"/staging/north"}}
		_, err := service.CreateScopedRole(t.Context(), &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
		waitForRolePresence(t, cache, "staging-north-access", true)

		_, err = service.DeleteScopedRole(t.Context(), &scopedaccessv1.DeleteScopedRoleRequest{
			Name: "staging-north-access",
		})
		require.NoError(t, err)
		waitForRolePresence(t, cache, "staging-north-access", false)
	})

	t.Run("add_remove_members_after_init", func(t *testing.T) {
		member := newAccessListMember(t, "staging-users", "staging-temp", accesslist.MembershipKindUser)
		_, err := accessListService.UpsertAccessListMember(t.Context(), member)
		require.NoError(t, err)
		expectedAssignments = append(expectedAssignments, roleAssignment{
			user:  "staging-temp",
			scope: "/",
			grants: []grant{
				{role: "staging-access", scope: "/staging"},
			},
		}, roleAssignment{
			user:  "staging-temp",
			scope: "/",
			grants: []grant{
				{role: "staging-east-access", scope: "/staging/east"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)

		require.NoError(t, accessListService.DeleteAccessListMember(t.Context(), "staging-users", "staging-temp"))
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "staging-temp",
			scope: "/",
			grants: []grant{
				{role: "staging-access", scope: "/staging"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "staging-temp",
			scope: "/",
			grants: []grant{
				{role: "staging-east-access", scope: "/staging/east"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)
	})

	t.Run("add_remove_access_list_after_init", func(t *testing.T) {
		role := newScopedRole("staging-audit")
		role.Spec = &scopedaccessv1.ScopedRoleSpec{AssignableScopes: []string{"/staging"}}
		_, err := service.CreateScopedRole(t.Context(), &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
		waitForRolePresence(t, cache, "staging-audit", true)

		list := newAccessList(t, "staging-audit-users")
		list.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{
			{Role: "staging-audit", Scope: "/staging"},
		}
		_, err = accessListService.UpsertAccessList(t.Context(), list)
		require.NoError(t, err)

		member := newAccessListMember(t, "staging-audit-users", "staging-auditor", accesslist.MembershipKindUser)
		_, err = accessListService.UpsertAccessListMember(t.Context(), member)
		require.NoError(t, err)

		expectedAssignments = append(expectedAssignments, roleAssignment{
			user:  "staging-auditor",
			scope: "/",
			grants: []grant{
				{role: "staging-audit", scope: "/staging"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)

		require.NoError(t, accessListService.DeleteAccessList(t.Context(), "staging-audit-users"))
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "staging-auditor",
			scope: "/",
			grants: []grant{
				{role: "staging-audit", scope: "/staging"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)
	})

	t.Run("update_access_list_grants_after_init", func(t *testing.T) {
		role := newScopedRole("staging-readonly")
		role.Spec = &scopedaccessv1.ScopedRoleSpec{AssignableScopes: []string{"/staging"}}
		_, err := service.CreateScopedRole(t.Context(), &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
		waitForRolePresence(t, cache, "staging-readonly", true)

		list, err := accessListService.GetAccessList(t.Context(), "staging-users")
		require.NoError(t, err)
		list.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{
			{Role: "staging-readonly", Scope: "/staging"},
		}
		_, err = accessListService.UpsertAccessList(t.Context(), list)
		require.NoError(t, err)

		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "staging-agent",
			scope: "/",
			grants: []grant{
				{role: "staging-access", scope: "/staging"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "staging-owner",
			scope: "/",
			grants: []grant{
				{role: "staging-access", scope: "/staging"},
			},
		})
		expectedAssignments = append(expectedAssignments,
			roleAssignment{
				user:  "staging-agent",
				scope: "/",
				grants: []grant{
					{role: "staging-readonly", scope: "/staging"},
				},
			},
			roleAssignment{
				user:  "staging-owner",
				scope: "/",
				grants: []grant{
					{role: "staging-readonly", scope: "/staging"},
				},
			},
		)
		waitForRoleAssignments(t, cache, expectedAssignments)
	})

	t.Run("remove_members_after_init", func(t *testing.T) {
		require.NoError(t, accessListService.DeleteAccessListMember(t.Context(), "staging-east-admins", "staging-east-owner"))
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "staging-east-owner",
			scope: "/",
			grants: []grant{
				{role: "staging-east-admin", scope: "/staging/east"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "staging-east-owner",
			scope: "/",
			grants: []grant{
				{role: "staging-east-access", scope: "/staging/east"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)
	})
}

func TestAccessListMaterializationDeepForest(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true, Limit: 100},
			},
		},
	})

	backend, err := memory.New(memory.Config{
		Context: t.Context(),
	})
	require.NoError(t, err)
	defer backend.Close()

	service := local.NewScopedAccessService(backend)
	accessListService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: backend,
		Modules: modules.GetModules(),
	})
	require.NoError(t, err)

	events := local.NewEventsService(backend)
	accessListCache, err := cachepkg.New(cachepkg.Config{
		Context: t.Context(),
		Events:  events,
		Watches: []types.WatchKind{
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
		},
		AccessLists: accessListService,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, accessListCache.Close()) })

	cache, err := NewCache(CacheConfig{
		Events:           events,
		Reader:           service,
		AccessListReader: accessListCache,
		AccessListEvents: accessListCache,
	})
	require.NoError(t, err)
	defer cache.Close()

	for roleName, scope := range map[string]string{
		"role-a": "/aa",
		"role-b": "/bb",
		"role-c": "/cc",
		"role-d": "/dd",
		"role-e": "/ee",
		"role-x": "/xx",
		"role-y": "/yy",
	} {
		role := newScopedRole(roleName)
		role.Spec = &scopedaccessv1.ScopedRoleSpec{AssignableScopes: []string{scope}}
		_, err := service.CreateScopedRole(t.Context(), &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
	}

	for listName, grants := range map[string]accesslist.Grants{
		"list-a": {ScopedRoles: []accesslist.ScopedRoleGrant{{Role: "role-a", Scope: "/aa"}}},
		"list-b": {ScopedRoles: []accesslist.ScopedRoleGrant{{Role: "role-b", Scope: "/bb"}}},
		"list-c": {ScopedRoles: []accesslist.ScopedRoleGrant{{Role: "role-c", Scope: "/cc"}}},
		"list-d": {ScopedRoles: []accesslist.ScopedRoleGrant{{Role: "role-d", Scope: "/dd"}}},
		"list-e": {ScopedRoles: []accesslist.ScopedRoleGrant{{Role: "role-e", Scope: "/ee"}}},
		"list-x": {ScopedRoles: []accesslist.ScopedRoleGrant{{Role: "role-x", Scope: "/xx"}}},
		"list-y": {ScopedRoles: []accesslist.ScopedRoleGrant{{Role: "role-y", Scope: "/yy"}}},
	} {
		list := newAccessList(t, listName)
		list.Spec.Grants = grants
		_, err := accessListService.UpsertAccessList(t.Context(), list)
		require.NoError(t, err)
	}

	for _, memberSpec := range []accesslist.AccessListMemberSpec{
		{AccessList: "list-a", Name: "list-b", MembershipKind: accesslist.MembershipKindList},
		{AccessList: "list-b", Name: "list-c", MembershipKind: accesslist.MembershipKindList},
		{AccessList: "list-b", Name: "list-d", MembershipKind: accesslist.MembershipKindList},
		{AccessList: "list-d", Name: "list-e", MembershipKind: accesslist.MembershipKindList},
		{AccessList: "list-x", Name: "list-y", MembershipKind: accesslist.MembershipKindList},

		{AccessList: "list-a", Name: "user-a", MembershipKind: accesslist.MembershipKindUser},
		{AccessList: "list-b", Name: "user-b", MembershipKind: accesslist.MembershipKindUser},
		{AccessList: "list-c", Name: "user-c", MembershipKind: accesslist.MembershipKindUser},
		{AccessList: "list-e", Name: "user-e", MembershipKind: accesslist.MembershipKindUser},
		{AccessList: "list-y", Name: "user-y", MembershipKind: accesslist.MembershipKindUser},
	} {
		member := newAccessListMember(t, memberSpec.AccessList, memberSpec.Name, memberSpec.MembershipKind)
		_, err := accessListService.UpsertAccessListMember(t.Context(), member)
		require.NoError(t, err)
	}

	expectedAssignments := []roleAssignment{
		{user: "user-a", scope: "/", grants: []grant{{role: "role-a", scope: "/aa"}}},
		{user: "user-b", scope: "/", grants: []grant{{role: "role-b", scope: "/bb"}}},
		{user: "user-b", scope: "/", grants: []grant{{role: "role-a", scope: "/aa"}}},
		{user: "user-c", scope: "/", grants: []grant{{role: "role-c", scope: "/cc"}}},
		{user: "user-c", scope: "/", grants: []grant{{role: "role-b", scope: "/bb"}}},
		{user: "user-c", scope: "/", grants: []grant{{role: "role-a", scope: "/aa"}}},
		{user: "user-e", scope: "/", grants: []grant{{role: "role-e", scope: "/ee"}}},
		{user: "user-e", scope: "/", grants: []grant{{role: "role-d", scope: "/dd"}}},
		{user: "user-e", scope: "/", grants: []grant{{role: "role-b", scope: "/bb"}}},
		{user: "user-e", scope: "/", grants: []grant{{role: "role-a", scope: "/aa"}}},
		{user: "user-y", scope: "/", grants: []grant{{role: "role-y", scope: "/yy"}}},
		{user: "user-y", scope: "/", grants: []grant{{role: "role-x", scope: "/xx"}}},
	}
	waitForRoleAssignments(t, cache, expectedAssignments)

	t.Run("remove_middle_list_from_chain", func(t *testing.T) {
		require.NoError(t, accessListService.DeleteAccessListMember(t.Context(), "list-a", "list-b"))
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "user-b",
			scope: "/",
			grants: []grant{
				{role: "role-a", scope: "/aa"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "user-c",
			scope: "/",
			grants: []grant{
				{role: "role-a", scope: "/aa"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "user-e",
			scope: "/",
			grants: []grant{
				{role: "role-a", scope: "/aa"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)
	})

	t.Run("break_branch_membership", func(t *testing.T) {
		require.NoError(t, accessListService.DeleteAccessListMember(t.Context(), "list-b", "list-d"))
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "user-e",
			scope: "/",
			grants: []grant{
				{role: "role-b", scope: "/bb"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)
	})

	t.Run("delete_middle_list", func(t *testing.T) {
		require.NoError(t, accessListService.DeleteAccessList(t.Context(), "list-b"))
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "user-b",
			scope: "/",
			grants: []grant{
				{role: "role-b", scope: "/bb"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "user-c",
			scope: "/",
			grants: []grant{
				{role: "role-b", scope: "/bb"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)
	})
}

func TestAccessListMaterializationOwnerGrants(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true, Limit: 100},
			},
		},
	})

	backend, err := memory.New(memory.Config{
		Context: t.Context(),
	})
	require.NoError(t, err)
	defer backend.Close()

	service := local.NewScopedAccessService(backend)
	accessListService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: backend,
		Modules: modules.GetModules(),
	})
	require.NoError(t, err)

	events := local.NewEventsService(backend)
	accessListCache, err := cachepkg.New(cachepkg.Config{
		Context: t.Context(),
		Events:  events,
		Watches: []types.WatchKind{
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
		},
		AccessLists: accessListService,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, accessListCache.Close()) })

	cache, err := NewCache(CacheConfig{
		Events:           events,
		Reader:           service,
		AccessListReader: accessListCache,
		AccessListEvents: accessListCache,
	})
	require.NoError(t, err)
	defer cache.Close()

	for roleName, scope := range map[string]string{
		"primary-member":   "/primary",
		"primary-owner":    "/primary",
		"owners-member":    "/owners",
		"owners-owner":     "/owners",
		"subowners-member": "/subowners",
		"subowners-owner":  "/subowners",
	} {
		role := newScopedRole(roleName)
		role.Spec = &scopedaccessv1.ScopedRoleSpec{AssignableScopes: []string{scope}}
		_, err := service.CreateScopedRole(t.Context(), &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
	}

	ownersList := newAccessList(t, "owners-list")
	ownersList.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{
		{Role: "owners-member", Scope: "/owners"},
	}
	ownersList.Spec.OwnerGrants.ScopedRoles = []accesslist.ScopedRoleGrant{
		{Role: "owners-owner", Scope: "/owners"},
	}
	ownersList.Spec.Owners = append(ownersList.Spec.Owners,
		accesslist.Owner{Name: "zoe", MembershipKind: accesslist.MembershipKindUser},
	)
	_, err = accessListService.UpsertAccessList(t.Context(), ownersList)
	require.NoError(t, err)

	subowners := newAccessList(t, "subowners")
	subowners.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{
		{Role: "subowners-member", Scope: "/subowners"},
	}
	subowners.Spec.OwnerGrants.ScopedRoles = []accesslist.ScopedRoleGrant{
		{Role: "subowners-owner", Scope: "/subowners"},
	}
	_, err = accessListService.UpsertAccessList(t.Context(), subowners)
	require.NoError(t, err)

	primary := newAccessList(t, "primary")
	primary.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{
		{Role: "primary-member", Scope: "/primary"},
	}
	primary.Spec.OwnerGrants.ScopedRoles = []accesslist.ScopedRoleGrant{
		{Role: "primary-owner", Scope: "/primary"},
	}
	primary.Spec.Owners = append(primary.Spec.Owners,
		accesslist.Owner{Name: "alice", MembershipKind: accesslist.MembershipKindUser},
		accesslist.Owner{Name: "owners-list", MembershipKind: accesslist.MembershipKindList},
	)
	_, err = accessListService.UpsertAccessList(t.Context(), primary)
	require.NoError(t, err)

	for _, memberSpec := range []accesslist.AccessListMemberSpec{
		{AccessList: "primary", Name: "alice", MembershipKind: accesslist.MembershipKindUser},
		{AccessList: "primary", Name: "dave", MembershipKind: accesslist.MembershipKindUser},
		{AccessList: "owners-list", Name: "bob", MembershipKind: accesslist.MembershipKindUser},
		{AccessList: "owners-list", Name: "subowners", MembershipKind: accesslist.MembershipKindList},
		{AccessList: "subowners", Name: "carol", MembershipKind: accesslist.MembershipKindUser},
	} {
		member := newAccessListMember(t, memberSpec.AccessList, memberSpec.Name, memberSpec.MembershipKind)
		_, err := accessListService.UpsertAccessListMember(t.Context(), member)
		require.NoError(t, err)
	}

	expectedAssignments := []roleAssignment{
		{user: "alice", scope: "/", grants: []grant{{role: "primary-member", scope: "/primary"}, {role: "primary-owner", scope: "/primary"}}},
		{user: "dave", scope: "/", grants: []grant{{role: "primary-member", scope: "/primary"}}},
		{user: "bob", scope: "/", grants: []grant{{role: "primary-owner", scope: "/primary"}}},
		{user: "carol", scope: "/", grants: []grant{{role: "primary-owner", scope: "/primary"}}},
		{user: "owner", scope: "/", grants: []grant{{role: "primary-owner", scope: "/primary"}}},

		{user: "bob", scope: "/", grants: []grant{{role: "owners-member", scope: "/owners"}}},
		{user: "carol", scope: "/", grants: []grant{{role: "owners-member", scope: "/owners"}}},
		{user: "owner", scope: "/", grants: []grant{{role: "owners-owner", scope: "/owners"}}},
		{user: "zoe", scope: "/", grants: []grant{{role: "owners-owner", scope: "/owners"}}},

		{user: "carol", scope: "/", grants: []grant{{role: "subowners-member", scope: "/subowners"}}},
		{user: "owner", scope: "/", grants: []grant{{role: "subowners-owner", scope: "/subowners"}}},
	}
	waitForRoleAssignments(t, cache, expectedAssignments)

	t.Run("update_owner_grants", func(t *testing.T) {
		role := newScopedRole("primary-owner-v2")
		role.Spec = &scopedaccessv1.ScopedRoleSpec{AssignableScopes: []string{"/primary"}}
		_, err := service.CreateScopedRole(t.Context(), &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)

		list, err := accessListService.GetAccessList(t.Context(), "primary")
		require.NoError(t, err)
		list.Spec.OwnerGrants.ScopedRoles = []accesslist.ScopedRoleGrant{
			{Role: "primary-owner-v2", Scope: "/primary"},
		}
		_, err = accessListService.UpsertAccessList(t.Context(), list)
		require.NoError(t, err)

		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "alice",
			scope: "/",
			grants: []grant{
				{role: "primary-member", scope: "/primary"},
				{role: "primary-owner", scope: "/primary"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "bob",
			scope: "/",
			grants: []grant{
				{role: "primary-owner", scope: "/primary"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "carol",
			scope: "/",
			grants: []grant{
				{role: "primary-owner", scope: "/primary"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "owner",
			scope: "/",
			grants: []grant{
				{role: "primary-owner", scope: "/primary"},
			},
		})
		expectedAssignments = append(expectedAssignments,
			roleAssignment{
				user:  "alice",
				scope: "/",
				grants: []grant{
					{role: "primary-member", scope: "/primary"},
					{role: "primary-owner-v2", scope: "/primary"},
				},
			},
			roleAssignment{
				user:  "bob",
				scope: "/",
				grants: []grant{
					{role: "primary-owner-v2", scope: "/primary"},
				},
			},
			roleAssignment{
				user:  "carol",
				scope: "/",
				grants: []grant{
					{role: "primary-owner-v2", scope: "/primary"},
				},
			},
			roleAssignment{
				user:  "owner",
				scope: "/",
				grants: []grant{
					{role: "primary-owner-v2", scope: "/primary"},
				},
			},
		)
		waitForRoleAssignments(t, cache, expectedAssignments)
	})

	t.Run("remove_owner_list_member", func(t *testing.T) {
		require.NoError(t, accessListService.DeleteAccessListMember(t.Context(), "owners-list", "bob"))
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "bob",
			scope: "/",
			grants: []grant{
				{role: "owners-member", scope: "/owners"},
			},
		})
		expectedAssignments = removeRoleAssignment(expectedAssignments, roleAssignment{
			user:  "bob",
			scope: "/",
			grants: []grant{
				{role: "primary-owner-v2", scope: "/primary"},
			},
		})
		waitForRoleAssignments(t, cache, expectedAssignments)
	})
}

func BenchmarkAccessListMaterialization(b *testing.B) {
	modulestest.SetTestModules(b, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true, Limit: 100000},
			},
		},
	})

	cases := []struct {
		name         string
		lists        int
		users        int
		usersPerList int
		nestedFanout int
	}{
		{
			name:         "lists=1000/users=5000",
			lists:        1000,
			users:        5000,
			usersPerList: 10,
			nestedFanout: 2,
		},
		{
			name:         "lists=2000/users=10000",
			lists:        2000,
			users:        10000,
			usersPerList: 10,
			nestedFanout: 2,
		},
		{
			name:         "big",
			lists:        25000,
			users:        50000,
			usersPerList: 200,
			nestedFanout: 0,
		},
	}
	for _, tc := range cases {
		assignmentCount := 0
		b.Run(tc.name, func(b *testing.B) {
			ctx := context.Background()
			backend, err := memory.New(memory.Config{
				Context: ctx,
			})
			require.NoError(b, err)
			b.Cleanup(func() { require.NoError(b, backend.Close()) })

			accessListService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
				Backend: backend,
				Modules: modules.GetModules(),
			})
			require.NoError(b, err)

			seedAccessListMaterializationData(b, accessListService, materializationBenchConfig{
				lists:        tc.lists,
				users:        tc.users,
				usersPerList: tc.usersPerList,
				nestedFanout: tc.nestedFanout,
			})

			events := local.NewEventsService(backend)
			accessListCache, err := cachepkg.New(cachepkg.Config{
				Context: ctx,
				Events:  events,
				Watches: []types.WatchKind{
					{Kind: types.KindAccessList},
					{Kind: types.KindAccessListMember},
				},
				AccessLists: accessListService,
			})
			require.NoError(b, err)
			b.Cleanup(func() { require.NoError(b, accessListCache.Close()) })

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s := state{
					roles:       roles.NewRoleCache(),
					assignments: assignments.NewAssignmentCache(),
				}
				materializer := newAccessListMaterializer(accessListCache)
				require.NoError(b, materializer.init(ctx, s))
				assignmentCount = len(materializer.materializedAssignments)
			}
		})

		b.Run(tc.name+"_baseline", func(b *testing.B) {
			ctx := context.Background()
			backend, err := memory.New(memory.Config{
				Context: ctx,
			})
			require.NoError(b, err)
			b.Cleanup(func() { require.NoError(b, backend.Close()) })

			accessListService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
				Backend: backend,
				Modules: modules.GetModules(),
			})
			require.NoError(b, err)

			seedAccessListMaterializationData(b, accessListService, materializationBenchConfig{
				lists:        tc.lists,
				users:        tc.users,
				usersPerList: tc.usersPerList,
				nestedFanout: tc.nestedFanout,
			})

			events := local.NewEventsService(backend)
			accessListCache, err := cachepkg.New(cachepkg.Config{
				Context: ctx,
				Events:  events,
				Watches: []types.WatchKind{
					{Kind: types.KindAccessList},
					{Kind: types.KindAccessListMember},
				},
				AccessLists: accessListService,
			})
			require.NoError(b, err)
			b.Cleanup(func() { require.NoError(b, accessListCache.Close()) })

			b.ReportAllocs()
			for b.Loop() {
				assignmentCache := assignments.NewAssignmentCache()
				var allLists []string
				for list, err := range clientutils.Resources(ctx, accessListCache.ListAccessLists) {
					require.NoError(b, err)
					allLists = append(allLists, list.GetName())
				}
				for _, listName := range allLists {
					_, err := accessListCache.GetAccessList(ctx, listName)
					require.NoError(b, err)
				}
				var allMembers []string
				for member, err := range clientutils.Resources(ctx, accessListCache.ListAllAccessListMembers) {
					require.NoError(b, err)
					allMembers = append(allMembers, member.Spec.Name)
				}
				for range assignmentCount {
					assignment := &scopedaccessv1.ScopedRoleAssignment{
						Kind:    scopedaccess.KindScopedRoleAssignment,
						Version: types.V1,
						Metadata: &headerv1.Metadata{
							Name: uuid.NewString(),
						},
						Scope: "/",
						Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
							User:        "asdf",
							Assignments: make([]*scopedaccessv1.Assignment, 0, 1),
						},
					}
					assignment.Spec.Assignments = append(assignment.Spec.Assignments,
						&scopedaccessv1.Assignment{
							Role:  "asdf",
							Scope: "/a/s/d/f",
						},
					)
					err := assignmentCache.Put(assignment)
					require.NoError(b, err)
				}
			}
		})
	}
}

type materializationBenchConfig struct {
	lists        int
	users        int
	usersPerList int
	nestedFanout int
}

func seedAccessListMaterializationData(b testing.TB, service *local.AccessListService, cfg materializationBenchConfig) {
	b.Helper()

	ctx := context.Background()
	seedTime := time.Unix(1700000000, 0).UTC()
	const maxNestingDepth = 10

	collection := &accesslists.Collection{}

	for i := 0; i < cfg.lists; i++ {
		listName := fmt.Sprintf("list-%05d", i)
		list := newAccessList(b, listName)
		list.Spec.Grants = accesslist.Grants{
			ScopedRoles: []accesslist.ScopedRoleGrant{
				{
					Role:  fmt.Sprintf("scoped-role-%d", i%10),
					Scope: fmt.Sprintf("/scope/%d", i%10),
				},
			},
		}
		require.NoError(b, collection.AddAccessList(list, nil))
	}

	for i := 0; i < cfg.lists; i++ {
		listName := fmt.Sprintf("list-%05d", i)
		for j := 0; j < cfg.usersPerList; j++ {
			userIndex := (i*cfg.usersPerList + j) % cfg.users
			userName := fmt.Sprintf("user-%05d", userIndex)
			member := newAccessListMember(b, listName, userName, accesslist.MembershipKindUser)
			member.Spec.Joined = seedTime
			member.Spec.AddedBy = "seed"
			collection.MembersByAccessList[listName] = append(collection.MembersByAccessList[listName], member)
		}
	}

	levelSize := (cfg.lists + maxNestingDepth - 1) / maxNestingDepth
	for i := 0; i < cfg.lists; i++ {
		level := i / levelSize
		if level >= maxNestingDepth-1 {
			continue
		}
		parentList := fmt.Sprintf("list-%05d", i)
		childBase := (level + 1) * levelSize
		for j := 0; j < cfg.nestedFanout; j++ {
			childIndex := childBase + (i+j)%levelSize
			if childIndex >= cfg.lists {
				continue
			}
			childList := fmt.Sprintf("list-%05d", childIndex)
			member := newAccessListMember(b, parentList, childList, accesslist.MembershipKindList)
			member.Spec.Joined = seedTime
			member.Spec.AddedBy = "seed"
			collection.MembersByAccessList[parentList] = append(collection.MembersByAccessList[parentList], member)
		}
	}

	require.NoError(b, service.InsertAccessListCollection(ctx, collection))
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

func newAccessList(t testing.TB, name string) *accesslist.AccessList {
	t.Helper()

	list, err := accesslist.NewAccessList(header.Metadata{Name: name}, accesslist.Spec{
		Title: name,
		Owners: []accesslist.Owner{
			{
				Name:           "owner",
				MembershipKind: accesslist.MembershipKindUser,
			},
		},
	})
	require.NoError(t, err)

	return list
}

func newAccessListMember(t testing.TB, listName, memberName, membershipKind string) *accesslist.AccessListMember {
	t.Helper()

	if membershipKind == "" {
		membershipKind = accesslist.MembershipKindUser
	}
	member, err := accesslist.NewAccessListMember(header.Metadata{Name: memberName}, accesslist.AccessListMemberSpec{
		AccessList:     listName,
		Name:           memberName,
		Joined:         time.Now().UTC(),
		AddedBy:        "creator",
		MembershipKind: membershipKind,
	})
	require.NoError(t, err)

	return member
}

func collectRoleAssignments(t *testing.T, reader services.ScopedRoleAssignmentReader) []roleAssignment {
	t.Helper()

	var out []roleAssignment
	for assignment, err := range scopedutils.RangeScopedRoleAssignments(t.Context(), reader, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
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
		sort.Slice(gotAssignment.grants, func(i, j int) bool {
			if gotAssignment.grants[i].role == gotAssignment.grants[j].role {
				return gotAssignment.grants[i].scope < gotAssignment.grants[j].scope
			}
			return gotAssignment.grants[i].role < gotAssignment.grants[j].role
		})
		out = append(out, gotAssignment)
	}
	return out
}

func normalizeRoleAssignments(assignments []roleAssignment) []roleAssignment {
	for i := range assignments {
		sort.Slice(assignments[i].grants, func(a, b int) bool {
			if assignments[i].grants[a].role == assignments[i].grants[b].role {
				return assignments[i].grants[a].scope < assignments[i].grants[b].scope
			}
			return assignments[i].grants[a].role < assignments[i].grants[b].role
		})
	}
	sort.Slice(assignments, func(i, j int) bool {
		if assignments[i].user == assignments[j].user {
			if assignments[i].scope == assignments[j].scope {
				return grantsKey(assignments[i].grants) < grantsKey(assignments[j].grants)
			}
			return assignments[i].scope < assignments[j].scope
		}
		return assignments[i].user < assignments[j].user
	})
	return assignments
}

func waitForRoleAssignments(t *testing.T, reader services.ScopedRoleAssignmentReader, expected []roleAssignment) {
	t.Helper()

	expected = normalizeRoleAssignments(append([]roleAssignment(nil), expected...))
	timeout := time.After(5 * time.Second)
	for {
		got := normalizeRoleAssignments(collectRoleAssignments(t, reader))
		if assert.ObjectsAreEqual(expected, got) {
			return
		}

		select {
		case <-time.After(120 * time.Millisecond):
		case <-timeout:
			require.Equal(t, expected, got)
		}
	}
}

func grantsKey(grants []grant) string {
	parts := make([]string, 0, len(grants))
	for _, g := range grants {
		parts = append(parts, g.role+"@"+g.scope)
	}
	return strings.Join(parts, ",")
}

func removeRoleAssignment(assignments []roleAssignment, remove roleAssignment) []roleAssignment {
	assignments = normalizeRoleAssignments(append([]roleAssignment(nil), assignments...))
	remove = normalizeRoleAssignments([]roleAssignment{remove})[0]
	removeKey := assignmentKey(remove)

	removed := false
	out := make([]roleAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		if !removed && assignmentKey(assignment) == removeKey {
			removed = true
			continue
		}
		out = append(out, assignment)
	}
	return out
}

func assignmentKey(assignment roleAssignment) string {
	return assignment.user + "|" + assignment.scope + "|" + grantsKey(assignment.grants)
}

func waitForRolePresence(t *testing.T, reader services.ScopedRoleReader, roleName string, present bool) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		found := false
		for role, err := range scopedutils.RangeScopedRoles(t.Context(), reader, &scopedaccessv1.ListScopedRolesRequest{}) {
			require.NoError(t, err)
			if role.GetMetadata().GetName() == roleName {
				found = true
				break
			}
		}
		if found == present {
			return
		}
		select {
		case <-time.After(120 * time.Millisecond):
		case <-timeout:
			require.FailNow(t, "timeout waiting for scoped role presence", roleName)
		}
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
