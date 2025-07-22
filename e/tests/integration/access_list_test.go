package integration

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
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

	client := sut.Teleport.Process.GetAuthServer().AccessLists

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

	client := sut.Teleport.Process.GetAuthServer().AccessLists

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
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	})

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
	_, _, err = authServer.AccessLists.UpsertAccessListWithMembers(ctx, testAccessList, aclMember)
	require.NoError(t, err)

	aliceTC := sut.GetClusterClientForUser(t, "alice")
	aliceAccessListClient := aliceTC.AuthClient.AccessListClient()

	t.Run("owner should not be able to modify their membership properties", func(t *testing.T) {
		acl, members := mustGetAccessListAndMembers(t, aliceAccessListClient, testAccessList.GetName())

		require.True(t, isAccessListOwner(acl, "alice"))
		require.True(t, isAccessListMember(members, "alice"))

		idx := slices.IndexFunc(members, func(v *accesslist.AccessListMember) bool {
			return v.Spec.Name == "alice"
		})
		require.NotEqual(t, idx, -1)
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
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			acl, members = mustGetAccessListAndMembers(t, aliceAccessListClient, testAccessList.GetName())
			assert.True(collect, isAccessListOwner(acl, "alice"))
			assert.False(collect, isAccessListMember(members, "alice"))
		}, time.Second, time.Millisecond*100)

		members = append(members, mustCreateMember(t, testAccessList.GetName(), "alice"))
		_, _, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, acl, members)
		require.True(t, trace.IsAccessDenied(err))
	})
}
