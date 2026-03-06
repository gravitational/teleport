package integration

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/tests/common"
)

// TestNestedAccessListCycleValidation tests the concurrent safety of nested Access List cycle validation.
// It creates two Access Lists, A and B, and attempts to add A as a member of B and B as a member of A.
func TestNestedAccessListCycleValidation(t *testing.T) {
	ctx := context.Background()

	const (
		listA = "list-a"
		listB = "list-b"
		user  = "alice"
	)

	sut := common.InitSUT(t, common.WithUser(t, user, "editor"))

	client := sut.Teleport.Process.GetAuthServer().AccessListsInternal

	// delete the lists if they already exist
	for _, list := range []string{listA, listB} {
		if err := client.DeleteAccessList(ctx, list); err != nil && !trace.IsNotFound(err) {
			t.Fatal(err)
		}
	}

	// create the lists
	for _, list := range []string{listA, listB} {
		accessList, err := accesslist.NewAccessList(header.Metadata{
			Name: list,
		}, accesslist.Spec{
			Title: "test list",
			Grants: accesslist.Grants{
				Roles: []string{"access"},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Now().AddDate(1, 0, 0),
			},
			Owners: []accesslist.Owner{
				{
					Name:           user,
					MembershipKind: accesslist.MembershipKindUser,
				},
			},
		})
		require.NoError(t, err)
		_, err = client.UpsertAccessList(ctx, accessList)
		require.NoError(t, err)
	}

	t.Log("created access lists, waiting for cache replication...")

	for {
		time.Sleep(time.Millisecond * 100)
		for _, list := range []string{listA, listB} {
			_, err := client.GetAccessList(ctx, list)
			if err != nil {
				if !trace.IsNotFound(err) {
					t.Fatal(err)
				}
				continue
			}
		}
		break
	}

	var eg errgroup.Group
	startC := make(chan struct{})
	for _, ll := range [][]string{{listA, listB}, {listB, listA}} {
		eg.Go(func() error {
			member, err := accesslist.NewAccessListMember(
				header.Metadata{
					Name: ll[1], /* name of member */
				},
				accesslist.AccessListMemberSpec{
					AccessList:     ll[0], /* name of list */
					Name:           ll[1], /* name of member */
					Joined:         time.Now().UTC(),
					Expires:        time.Now().UTC().Add(24 * time.Hour),
					Reason:         "test",
					AddedBy:        user,
					MembershipKind: accesslist.MembershipKindList,
				},
			)
			if err != nil {
				return trace.Wrap(err)
			}
			<-startC
			_, err = client.UpsertAccessListMember(ctx, member)
			return trace.Wrap(err)
		})
	}

	time.Sleep(time.Millisecond * 100)
	close(startC)

	err := eg.Wait()
	require.Error(t, err)

	// Depending on the order of execution, the order of list names in the error message may vary.
	require.ErrorContains(t, err, "can't be added as a Member of")
}

// TestAccessListMaxDepthValidation tests that nested Access Lists cannot exceed the maximum allowed depth.
// It creates two hierarchies of nested lists, each 5 deep, then creates a central list.
// It then concurrently adds the central list as a member of the last node of h1, and adds the first node of h2
// as a member of the central list.
func TestAccessListMaxDepthValidation(t *testing.T) {
	ctx := context.Background()

	const (
		user     = "alice"
		maxDepth = accesslist.MaxAllowedDepth
		central  = "central-list"
	)

	sut := common.InitSUT(t, common.WithUser(t, user, "editor"))

	client := sut.Teleport.Process.GetAuthServer().AccessListsInternal

	// Helper function to create a hierarchy of nested access lists.
	createHierarchy := func(prefix string) []string {
		var lists []string
		for i := 1; i <= maxDepth; i++ {
			name := fmt.Sprintf("%s-%d", prefix, i)
			lists = append(lists, name)
		}

		// Create the lists.
		for idx, list := range lists {
			accessList, err := accesslist.NewAccessList(header.Metadata{
				Name: list,
			}, accesslist.Spec{
				Title: fmt.Sprintf("test list %d", idx),
				Grants: accesslist.Grants{
					Roles: []string{"access"},
				},
				Audit: accesslist.Audit{
					NextAuditDate: time.Now().AddDate(1, 0, 0),
				},
				Owners: []accesslist.Owner{
					{
						Name:           user,
						MembershipKind: accesslist.MembershipKindUser,
					},
				},
			})
			require.NoError(t, err)
			_, err = client.UpsertAccessList(ctx, accessList)
			require.NoError(t, err)
		}

		// Wait for cache replication.
		t.Logf("Created access lists for %s, waiting for cache replication...", prefix)
		for {
			time.Sleep(time.Millisecond * 100)
			allReady := true
			for _, list := range lists {
				_, err := client.GetAccessList(ctx, list)
				if err != nil {
					if !trace.IsNotFound(err) {
						t.Fatal(err)
					}
					allReady = false
					break
				}
			}
			if allReady {
				break
			}
		}

		// Nest the lists.
		for i := range len(lists) - 1 {
			parent := lists[i]
			child := lists[i+1]

			member, err := accesslist.NewAccessListMember(
				header.Metadata{
					Name: child,
				},
				accesslist.AccessListMemberSpec{
					AccessList:     parent,
					Name:           child,
					Joined:         time.Now().UTC(),
					Expires:        time.Now().UTC().Add(24 * time.Hour),
					Reason:         "test nesting",
					AddedBy:        user,
					MembershipKind: accesslist.MembershipKindList,
				},
			)
			require.NoError(t, err)
			_, err = client.UpsertAccessListMember(ctx, member)
			require.NoError(t, err)
		}

		return lists
	}

	h1 := createHierarchy("h1")
	h2 := createHierarchy("h2")

	centralList, err := accesslist.NewAccessList(header.Metadata{
		Name: central,
	}, accesslist.Spec{
		Title: "central list",
		Grants: accesslist.Grants{
			Roles: []string{"access"},
		},
		Audit: accesslist.Audit{
			NextAuditDate: time.Now().AddDate(1, 0, 0),
		},
		Owners: []accesslist.Owner{
			{
				Name:           user,
				MembershipKind: accesslist.MembershipKindUser,
			},
		},
	})
	require.NoError(t, err)
	_, err = client.UpsertAccessList(ctx, centralList)
	require.NoError(t, err)

	// Wait for central list to be ready.
	t.Log("Created central list, waiting for cache replication...")
	for {
		time.Sleep(time.Millisecond * 100)
		_, err := client.GetAccessList(ctx, central)
		if err != nil {
			if !trace.IsNotFound(err) {
				t.Fatal(err)
			}
			continue
		}
		break
	}

	var eg errgroup.Group
	startC := make(chan struct{})

	// Add central list to last node of h1.
	eg.Go(func() error {
		member, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: central,
			},
			accesslist.AccessListMemberSpec{
				AccessList:     h1[len(h1)-1],
				Name:           central,
				Joined:         time.Now().UTC(),
				Expires:        time.Now().UTC().Add(24 * time.Hour),
				Reason:         "test nesting",
				AddedBy:        user,
				MembershipKind: accesslist.MembershipKindList,
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
		<-startC
		_, err = client.UpsertAccessListMember(ctx, member)
		return trace.Wrap(err)
	})

	// Add first node of h2 to central list.
	eg.Go(func() error {
		member, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: h2[0],
			},
			accesslist.AccessListMemberSpec{
				AccessList:     central,
				Name:           h2[0],
				Joined:         time.Now().UTC(),
				Expires:        time.Now().UTC().Add(24 * time.Hour),
				Reason:         "test nesting",
				AddedBy:        user,
				MembershipKind: accesslist.MembershipKindList,
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
		<-startC
		_, err = client.UpsertAccessListMember(ctx, member)
		return trace.Wrap(err)
	})

	time.Sleep(time.Millisecond * 100)
	close(startC)
	err = eg.Wait()
	require.Error(t, err)

	// Depending on the order of execution, the order of list names in the error message may vary.
	require.ErrorContains(t, err, fmt.Sprintf("because it would exceed the maximum nesting depth of %d", maxDepth))
}

// TestAccessListMemberRBAC tests the permissions of an access list owner.
// Access List Owner without role RBAC:
// - should not be able to modify their membership properties
// - should not be able to add themselves as a member
// - If an owner is also a member, they should be able to add a new member
func TestAccessListOwnerPermissions(t *testing.T) {
	sut := common.InitSUT(t, common.WithUser(t, "alice", "requester"))

	testAccessList, err := accesslist.NewAccessList(header.Metadata{
		Name: "test-access-list",
	}, accesslist.Spec{
		Title: "access list 1",
		Owners: []accesslist.Owner{
			{Name: "alice", IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String()},
		},
		Grants: accesslist.Grants{Roles: []string{"access"}},
		Audit:  accesslist.Audit{NextAuditDate: sut.Clock.Now()},
	})
	require.NoError(t, err)
	ctx := context.Background()

	aclMember := []*accesslist.AccessListMember{
		mustCreateMember(t, testAccessList.GetName(), "alice"),
	}

	authServer := sut.Teleport.Process.GetAuthServer()
	_, _, err = authServer.AccessListsInternal.UpsertAccessListWithMembers(ctx, testAccessList, aclMember)
	require.NoError(t, err)

	aliceTC := sut.GetClusterClientForUser(t, "alice")
	aliceAccessListClient := aliceTC.AuthClient.AccessListClient()

	// Wait for the cache to propagate.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_ = mustGetAccessList(t, aliceAccessListClient, testAccessList.GetName())
		_ = mustGetAccessListMember(t, aliceAccessListClient, testAccessList.GetName(), "alice")
	}, time.Minute, 100*time.Millisecond)

	t.Run("owner should not be able to modify their membership properties", func(t *testing.T) {
		acl, members := mustGetAccessListAndMembers(t, aliceAccessListClient, testAccessList.GetName())

		require.True(t, isAccessListOwner(acl, "alice"))
		require.True(t, isAccessListMember(members, "alice"))

		idx := slices.IndexFunc(members, func(v *accesslist.AccessListMember) bool {
			return v.Spec.Name == "alice"
		})
		require.NotEqual(t, -1, idx)
		members[idx].Spec.Expires = sut.Clock.Now().Add(time.Hour * 24 * 10)

		_, _, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, acl, members)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("owner that is also a member should be able to add a new member", func(t *testing.T) {
		acl, members := mustGetAccessListAndMembers(t, aliceAccessListClient, testAccessList.GetName())

		require.True(t, isAccessListOwner(acl, "alice"))
		require.True(t, isAccessListMember(members, "alice"))

		members = append(members, mustCreateMember(t, acl.GetName(), "bob"))
		_, _, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, acl, members)
		require.NoError(t, err)
	})

	t.Run("owner should not be able to add themselves as member", func(t *testing.T) {
		err = aliceAccessListClient.DeleteAccessListMember(ctx, testAccessList.GetName(), "alice")
		require.NoError(t, err)

		var acl *accesslist.AccessList
		var members []*accesslist.AccessListMember
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			acl, members = mustGetAccessListAndMembers(t, aliceAccessListClient, testAccessList.GetName())
			require.True(t, isAccessListOwner(acl, "alice"))
			require.False(t, isAccessListMember(members, "alice"))
		}, time.Minute, time.Millisecond*100)

		members = append(members, mustCreateMember(t, testAccessList.GetName(), "alice"))
		_, _, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, acl, members)
		require.True(t, trace.IsAccessDenied(err))
	})
}

func TestAccessListNestedGrants(t *testing.T) {
	ctx := context.Background()

	sut := common.InitSUT(t)

	var (
		noneRole                          = common.CreateRole(t, sut, "none", common.RoleAllowDesc{}, common.RoleDenyDesc{})
		rootACLRole                       = common.CreateRole(t, sut, "root_acl", common.RoleAllowDesc{}, common.RoleDenyDesc{})
		rootACLRoleForUpdate              = common.CreateRole(t, sut, "root_acl_for_update", common.RoleAllowDesc{}, common.RoleDenyDesc{})
		rootACLRoleForUpsert              = common.CreateRole(t, sut, "root_acl_for_upsert", common.RoleAllowDesc{}, common.RoleDenyDesc{})
		rootACLRoleForUpsertWithMembers   = common.CreateRole(t, sut, "root_acl_for_upsert_with_members", common.RoleAllowDesc{}, common.RoleDenyDesc{})
		nestedACLRole                     = common.CreateRole(t, sut, "nested_acl", common.RoleAllowDesc{}, common.RoleDenyDesc{})
		nestedACLRoleForUpdate            = common.CreateRole(t, sut, "nested_acl_for_update", common.RoleAllowDesc{}, common.RoleDenyDesc{})
		nestedACLRoleForUpsert            = common.CreateRole(t, sut, "nested_acl_for_upsert", common.RoleAllowDesc{}, common.RoleDenyDesc{})
		nestedACLRoleForUpsertWithMembers = common.CreateRole(t, sut, "nested_acl_for_upsert_with_members", common.RoleAllowDesc{}, common.RoleDenyDesc{})
	)

	alice := common.MustCreateUser(t, sut, "alice", noneRole.GetName())
	admin := common.MustCreateUser(t, sut, "admin", "editor")

	adminAuthClt := sut.GetClusterClientForUser(t, admin.GetName()).AuthClient

	rootAccessList := common.CreateAccessList(t, sut,
		common.WithName("test_root_acl"),
		common.WithOwners(admin.GetName()),
		common.WithGrants(accesslist.Grants{
			Roles: []string{rootACLRole.GetName()},
		}),
	)
	nestedAccessList := common.CreateAccessList(t, sut,
		common.WithName("test_nested_acl"),
		common.WithOwners(admin.GetName()),
		common.WithGrants(accesslist.Grants{
			Roles: []string{nestedACLRole.GetName()},
		}),
	)

	eventuallyHasRoles(t, sut, alice, []types.Role{
		noneRole,
	})

	nestedACLMember := common.CreateAccessListMember(t, sut,
		rootAccessList.GetName(),
		nestedAccessList.GetName(),
		accesslist.MembershipKindList,
	)
	aliceMember := common.CreateAccessListMember(t, sut,
		nestedAccessList.GetName(),
		alice.GetName(),
		accesslist.MembershipKindUser,
	)

	eventuallyHasRoles(t, sut, alice, []types.Role{
		noneRole,
		rootACLRole,
		nestedACLRole,
	})

	var err error

	rootAccessList.Spec.Grants.Roles = append(rootAccessList.Spec.Grants.Roles, rootACLRoleForUpdate.GetName())
	nestedAccessList.Spec.Grants.Roles = append(nestedAccessList.Spec.Grants.Roles, nestedACLRoleForUpdate.GetName())
	_, err = adminAuthClt.AccessListClient().UpdateAccessList(ctx, rootAccessList)
	require.NoError(t, err)
	_, err = adminAuthClt.AccessListClient().UpdateAccessList(ctx, nestedAccessList)
	require.NoError(t, err)

	eventuallyHasRoles(t, sut, alice, []types.Role{
		noneRole,
		rootACLRole, rootACLRoleForUpdate,
		nestedACLRole, nestedACLRoleForUpdate,
	})

	rootAccessList.Spec.Grants.Roles = append(rootAccessList.Spec.Grants.Roles, rootACLRoleForUpsert.GetName())
	nestedAccessList.Spec.Grants.Roles = append(nestedAccessList.Spec.Grants.Roles, nestedACLRoleForUpsert.GetName())
	_, err = adminAuthClt.AccessListClient().UpsertAccessList(ctx, rootAccessList)
	require.NoError(t, err)
	_, err = adminAuthClt.AccessListClient().UpsertAccessList(ctx, nestedAccessList)
	require.NoError(t, err)

	eventuallyHasRoles(t, sut, alice, []types.Role{
		noneRole,
		rootACLRole, rootACLRoleForUpdate, rootACLRoleForUpsert,
		nestedACLRole, nestedACLRoleForUpdate, nestedACLRoleForUpsert,
	})

	rootAccessList.Spec.Grants.Roles = append(rootAccessList.Spec.Grants.Roles, rootACLRoleForUpsertWithMembers.GetName())
	nestedAccessList.Spec.Grants.Roles = append(nestedAccessList.Spec.Grants.Roles, nestedACLRoleForUpsertWithMembers.GetName())
	_, _, err = adminAuthClt.AccessListClient().UpsertAccessListWithMembers(ctx, rootAccessList, []*accesslist.AccessListMember{nestedACLMember})
	require.NoError(t, err)
	_, _, err = adminAuthClt.AccessListClient().UpsertAccessListWithMembers(ctx, nestedAccessList, []*accesslist.AccessListMember{aliceMember})
	require.NoError(t, err)

	eventuallyHasRoles(t, sut, alice, []types.Role{
		noneRole,
		rootACLRole, rootACLRoleForUpdate, rootACLRoleForUpsert, rootACLRoleForUpsertWithMembers,
		nestedACLRole, nestedACLRoleForUpdate, nestedACLRoleForUpsert, nestedACLRoleForUpsertWithMembers,
	})

	membershipRequiresRole := common.CreateRole(t, sut, "membership_requires_1", common.RoleAllowDesc{}, common.RoleDenyDesc{})
	rootAccessList.Spec.MembershipRequires = accesslist.Requires{Roles: []string{membershipRequiresRole.GetName()}}
	_, _, err = adminAuthClt.AccessListClient().UpsertAccessListWithMembers(ctx, rootAccessList, []*accesslist.AccessListMember{nestedACLMember})
	require.NoError(t, err)

	eventuallyHasRoles(t, sut, alice, []types.Role{
		noneRole,
		nestedACLRole, nestedACLRoleForUpdate, nestedACLRoleForUpsert, nestedACLRoleForUpsertWithMembers,
	})

	rootAccessList.Spec.MembershipRequires = accesslist.Requires{}
	_, _, err = adminAuthClt.AccessListClient().UpsertAccessListWithMembers(ctx, rootAccessList, []*accesslist.AccessListMember{nestedACLMember})
	require.NoError(t, err)

	eventuallyHasRoles(t, sut, alice, []types.Role{
		noneRole,
		rootACLRole, rootACLRoleForUpdate, rootACLRoleForUpsert, rootACLRoleForUpsertWithMembers,
		nestedACLRole, nestedACLRoleForUpdate, nestedACLRoleForUpsert, nestedACLRoleForUpsertWithMembers,
	})
}

// TestAccessListGrantsPropagation verifies that access list role grants are properly
// applied through the web access path during user authentication. This test ensures that
// user state is correctly updated with the appropriate roles when access list membership
// changes between login attempts. The primary purpose is to validate that user login state
// is not cached anywhere and that web login hooks do not cache the user or user state object,
// ensuring fresh role grants are applied on each authentication.
func TestAccessListGrantsPropagation(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithLicense("../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice", "requester"),
	)
	authServer := sut.Teleport.Process.GetAuthServer()
	ctx := t.Context()

	testAccessList, err := accesslist.NewAccessList(header.Metadata{
		Name: "test-access-list",
	}, accesslist.Spec{
		Title: "access list 1",
		Owners: []accesslist.Owner{
			{Name: "alice"},
		},
		Grants: accesslist.Grants{Roles: []string{"editor"}},
		Audit:  accesslist.Audit{NextAuditDate: sut.Clock.Now()},
	})
	require.NoError(t, err)

	aclMember := []*accesslist.AccessListMember{
		mustCreateMember(t, testAccessList.GetName(), "alice"),
	}

	// checkRoleListAccess checks if Alice can access the role list endpoint with the single login flow
	// where webhooks are called and user login state is created.
	checkRoleListAccess := func(wantStatus int) {
		webClient := sut.CreateWebClientForUser(t, "alice")
		endpoint := webClient.Endpoint("webapi", "roles")
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		require.NoError(t, err)
		resp, err := webClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, wantStatus, resp.StatusCode)
	}

	// Initially, Alice has only requester role and should not be able to list roles.
	checkRoleListAccess(http.StatusForbidden)

	// Alice is added as a member to access list with editor role grant.
	_, _, err = authServer.AccessListsInternal.UpsertAccessListWithMembers(ctx, testAccessList, aclMember)
	require.NoError(t, err)
	// Membership 'editor' grant should allow Alice to list roles.
	checkRoleListAccess(http.StatusOK)

	// After alice is removed from the access list,
	err = authServer.AccessListsInternal.DeleteAccessListMember(ctx, testAccessList.GetName(), "alice")
	require.NoError(t, err)

	// She should no longer be able to list roles.
	checkRoleListAccess(http.StatusForbidden)
}

func eventuallyHasRoles(t *testing.T, sut *common.SUT, user types.User, roles []types.Role) {
	t.Helper()
	ctx := context.Background()

	// trigger UserLoginState refresh.
	_ = sut.CreateWebClientForUser(t, user.GetName())

	var expectedRoles []string
	for _, r := range roles {
		expectedRoles = append(expectedRoles, r.GetName())
	}
	slices.Sort(expectedRoles)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		uls, err := sut.Teleport.Process.GetAuthServer().GetUserLoginState(ctx, user.GetName())
		require.NoError(t, err)

		actualRoles := uls.GetRoles()
		slices.Sort(actualRoles)
		require.Equal(t, expectedRoles, actualRoles)
	}, time.Second*10, time.Second*1)
}
