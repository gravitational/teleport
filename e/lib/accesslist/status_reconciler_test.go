package accesslist

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func Test_statusReconciler_MemberOfOwnerOf(t *testing.T) {
	ctx := t.Context()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{entitlements.Identity: {Enabled: true}},
		},
	}
	modulestest.SetTestModules(t, testModules)

	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	storage, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend:                     bk,
		Modules:                     &testModules,
		RunWhileLockedRetryInterval: -1 * time.Millisecond,
	})
	require.NoError(t, err)

	// a1 not a owner nor a member of anything
	a1 := newAccessList(t, "1", clock)
	// a2 is a member of a1, but it doesn't have it set in status.member_of
	a2 := newAccessList(t, "2", clock)
	// a3 is an owner of a1, but it doesn't have it set in status.owner_of
	a3 := newAccessList(t, "3", clock)
	// a4 is not a member of a1 but it has a1 in status.member_of
	a4 := newAccessList(t, "4", clock)
	// a5 is not an owner of a1 but it has a1 in status.owner_of
	a5 := newAccessList(t, "5", clock)
	// a6 has status.member_of referencing a non-existing list
	a6 := newAccessList(t, "6", clock)
	// a7 has status.owner_of referencing a non-existing list
	a7 := newAccessList(t, "7", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), a2.GetName(), accesslist.MembershipKindList, clock)
	a1.SetOwners([]accesslist.Owner{
		{
			Name:           a3.GetName(),
			MembershipKind: accesslist.MembershipKindList,
		},
	})
	require.NoError(t, upsertAccessList(t.Context(), storage, []*accesslist.AccessList{a7, a6, a5, a4, a3, a2, a1}, []*accesslist.AccessListMember{a1m1}))

	const nonExistingList1, nonExistingList2 = "non_existing_list1", "non_existing_list2"

	assertStatusMemberOf(t, ctx, storage, a2.GetName(), []string{a1.GetName()})
	assertStatusOwnerOf(t, ctx, storage, a3.GetName(), []string{a1.GetName()})

	resetAccessListStatus(t, bk, storage, a2)
	resetAccessListStatus(t, bk, storage, a3)
	setAccessListStatus(t, bk, storage, a4, accesslist.Status{MemberOf: []string{a1.GetName()}})
	setAccessListStatus(t, bk, storage, a5, accesslist.Status{OwnerOf: []string{a1.GetName()}})
	setAccessListStatus(t, bk, storage, a6, accesslist.Status{MemberOf: []string{nonExistingList1}})
	setAccessListStatus(t, bk, storage, a7, accesslist.Status{OwnerOf: []string{nonExistingList2}})

	cfg := statusReconcilerConfig{
		Logger:      slog.Default(),
		Clock:       clock,
		AccessPoint: storage,
	}
	r, err := newStatusReconciler(cfg)
	require.NoError(t, err)

	assertStatusMemberOf(t, ctx, storage, a2.GetName(), []string{})
	assertStatusOwnerOf(t, ctx, storage, a3.GetName(), []string{})
	assertStatusMemberOf(t, ctx, storage, a4.GetName(), []string{a1.GetName()})
	assertStatusOwnerOf(t, ctx, storage, a5.GetName(), []string{a1.GetName()})
	assertStatusMemberOf(t, ctx, storage, a6.GetName(), []string{nonExistingList1})
	assertStatusOwnerOf(t, ctx, storage, a7.GetName(), []string{nonExistingList2})

	go r.Run(ctx)
	clock.Advance(statusReconcilerStartupSeventhJitter)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		clock.Advance(statusReconcilerStartupSeventhJitter)
		assertStatusMemberOf(c, ctx, storage, a2.GetName(), []string{a1.GetName()})
		assertStatusOwnerOf(c, ctx, storage, a3.GetName(), []string{a1.GetName()})
		assertStatusMemberOf(c, ctx, storage, a4.GetName(), []string{})
		assertStatusOwnerOf(c, ctx, storage, a5.GetName(), []string{})
		assertStatusMemberOf(c, ctx, storage, a6.GetName(), []string{})
		assertStatusOwnerOf(c, ctx, storage, a7.GetName(), []string{})
	}, 10*time.Second, 100*time.Millisecond)
}

// Test statusReconcilerConfig.reconcile can survive the situation where the owner or member
// access_list doesn't exist.
func Test_statusReconciler_reconcile_missingOwnerAndMemberLists(t *testing.T) {
	ctx := t.Context()

	testModules := modulestest.EnterpriseModules()
	modulestest.SetTestModules(t, *testModules)

	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	storage, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend:                     bk,
		Modules:                     testModules,
		RunWhileLockedRetryInterval: -1 * time.Millisecond,
	})
	require.NoError(t, err)

	cfg := statusReconcilerConfig{
		Logger:      slog.Default(),
		Clock:       clock,
		AccessPoint: storage,
	}
	r, err := newStatusReconciler(cfg)
	require.NoError(t, err)

	t.Run("for missing owner", func(t *testing.T) {
		err := storage.DeleteAllAccessLists(ctx)
		require.NoError(t, err)

		ownerThatDisappears := newAccessList(t, "owner_that_disappears", clock)

		a1 := newAccessList(t, "test_list_1", clock)
		a1.Spec.Owners = []accesslist.Owner{
			{
				Name:           ownerThatDisappears.GetName(),
				MembershipKind: accesslist.MembershipKindList,
			},
		}

		_, err = storage.UpsertAccessList(ctx, ownerThatDisappears)
		require.NoError(t, err)
		_, err = storage.UpsertAccessList(ctx, a1)
		require.NoError(t, err)

		// Use backend directly to bypass status checks.
		err = bk.Delete(ctx, backend.NewKey("access_list", ownerThatDisappears.GetName()))
		require.NoError(t, err)

		stats, err := r.reconcile(ctx)
		require.NoError(t, err)
		require.Equal(t, &statusReconcilerStats{
			processed: 1,
		}, stats)
	})

	t.Run("for missing member", func(t *testing.T) {
		err := storage.DeleteAllAccessLists(ctx)
		require.NoError(t, err)

		memberThatDisappears := newAccessList(t, "member_that_disappears", clock)

		a1 := newAccessList(t, "test_list_1", clock)

		_, err = storage.UpsertAccessList(ctx, memberThatDisappears)
		require.NoError(t, err)
		_, err = storage.UpsertAccessList(ctx, a1)
		require.NoError(t, err)

		_, err = storage.UpsertAccessListMember(ctx, newAccessListMember(t, a1.GetName(), memberThatDisappears.GetName(), accesslist.MembershipKindList, clock))
		require.NoError(t, err)

		// Use backend directly to bypass status checks.
		err = bk.Delete(ctx, backend.NewKey("access_list", memberThatDisappears.GetName()))
		require.NoError(t, err)

		stats, err := r.reconcile(ctx)
		require.NoError(t, err)
		require.Equal(t, &statusReconcilerStats{
			processed: 1,
		}, stats)
	})
}

func assertStatusMemberOf(t require.TestingT, ctx context.Context, svc *local.AccessListService, name string, expected []string) {
	if t, ok := t.(*testing.T); ok {
		t.Helper()
	}
	item, err := svc.GetAccessList(ctx, name)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.MemberOf)
}

func assertStatusOwnerOf(t require.TestingT, ctx context.Context, svc *local.AccessListService, name string, expected []string) {
	if t, ok := t.(*testing.T); ok {
		t.Helper()
	}
	item, err := svc.GetAccessList(ctx, name)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.OwnerOf)
}

func resetAccessListStatus(t *testing.T, bk *memory.Memory, storage *local.AccessListService, acl *accesslist.AccessList) {
	setAccessListStatus(t, bk, storage, acl, accesslist.Status{})
}

func setAccessListStatus(t *testing.T, bk *memory.Memory, storage *local.AccessListService, acl *accesslist.AccessList, newStatus accesslist.Status) {
	t.Helper()
	i, err := bk.Get(t.Context(), backend.NewKey("access_list", acl.GetName()))
	require.NoError(t, err)
	v, err := services.UnmarshalAccessList(i.Value)
	require.NoError(t, err)
	v.Status = newStatus
	buff, err := services.MarshalAccessList(v)
	require.NoError(t, err)
	i.Value = buff
	_, err = bk.Update(t.Context(), *i)
	require.NoError(t, err)
	v, err = storage.GetAccessList(t.Context(), acl.GetName())
	require.NoError(t, err)
	require.Equal(t, newStatus, v.Status)
}

func upsertAccessList(ctx context.Context, s *local.AccessListService, lists []*accesslist.AccessList, members []*accesslist.AccessListMember) error {
	for _, acl := range lists {
		_, err := s.UpsertAccessList(ctx, acl)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	for _, member := range members {
		_, err := s.UpsertAccessListMember(ctx, member)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
