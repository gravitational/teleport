package accesslist

import (
	"context"
	"log/slog"
	"sync/atomic"
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

	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{entitlements.Identity: {Enabled: true}},
		},
	})

	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	storage, err := local.NewAccessListService(bk, clock, local.WithRunWhileLockedRetryInterval(-1*time.Millisecond))
	require.NoError(t, err)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)
	a1m1 := newAccessListMember(t, a1.GetName(), a2.GetName(), accesslist.MembershipKindList, clock)
	a1.SetOwners([]accesslist.Owner{
		{
			Name:           a3.GetName(),
			MembershipKind: accesslist.MembershipKindList,
		},
	})
	require.NoError(t, upsertAccessList(t.Context(), storage, []*accesslist.AccessList{a3, a2, a1}, []*accesslist.AccessListMember{a1m1}))

	assertMemberOf(t, ctx, storage, a2.GetName(), []string{a1.GetName()})
	assertOwnerOf(t, ctx, storage, a3.GetName(), []string{a1.GetName()})

	resetAccessListStatus(t, bk, storage, a2)
	resetAccessListStatus(t, bk, storage, a3)

	assertMemberOf(t, ctx, storage, a2.GetName(), []string{})
	assertOwnerOf(t, ctx, storage, a3.GetName(), []string{})

	cfg := statusReconcilerConfig{
		Logger:      slog.Default(),
		Clock:       clock,
		AccessPoint: storage,
	}
	r, err := newStatusReconciler(cfg)
	require.NoError(t, err)

	go r.Run(ctx)
	clock.Advance(statusReconcilerStartupSeventhJitter)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		clock.Advance(statusReconcilerStartupSeventhJitter)
		assertMemberOf(c, ctx, storage, a2.GetName(), []string{a1.GetName()})
		assertOwnerOf(c, ctx, storage, a3.GetName(), []string{a1.GetName()})
	}, 10*time.Second, 100*time.Millisecond)
}

// Test statusReconcilerConfig.reconcile can survive the situation where the owner or member
// access_list doesn't exist.
func Test_statusReconciler_reconcile_missingOwnerAndMemberLists(t *testing.T) {
	ctx := t.Context()

	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{entitlements.Identity: {Enabled: true}},
		},
	})

	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	storage, err := local.NewAccessListService(bk, clock, local.WithRunWhileLockedRetryInterval(-1*time.Millisecond))
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

func Test_statusReconciler_reconcile_retriesOnConflict(t *testing.T) {
	ctx := t.Context()

	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{entitlements.Identity: {Enabled: true}},
		},
	})

	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	storage, err := local.NewAccessListService(bk, clock, local.WithRunWhileLockedRetryInterval(-1*time.Millisecond))
	require.NoError(t, err)

	a1 := newAccessList(t, "test_list_1", clock)
	badMember1 := newAccessList(t, "test_bad_member_1", clock)
	badOwner1 := newAccessList(t, "test_bad_owner_1", clock)
	a1m1 := newAccessListMember(t, a1.GetName(), badMember1.GetName(), accesslist.MembershipKindList, clock)
	a1.SetOwners([]accesslist.Owner{
		{
			Name:           badOwner1.GetName(),
			MembershipKind: accesslist.MembershipKindList,
		},
	})
	require.NoError(t, upsertAccessList(t.Context(), storage, []*accesslist.AccessList{badOwner1, badMember1, a1}, []*accesslist.AccessListMember{a1m1}))

	assertMemberOf(t, ctx, storage, badMember1.GetName(), []string{a1.GetName()})
	assertOwnerOf(t, ctx, storage, badOwner1.GetName(), []string{a1.GetName()})

	cfg := statusReconcilerConfig{
		Logger:      slog.Default(),
		Clock:       clock,
		AccessPoint: storage,
	}
	r, err := newStatusReconciler(cfg)
	require.NoError(t, err)

	// Allow retries.
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				clock.Advance(statusReconcilerRetryHalfJitter)
			}
		}
	}(ctx)

	t.Run("conflicts below the retry limits", func(t *testing.T) {
		resetAccessListStatus(t, bk, storage, badOwner1)
		resetAccessListStatus(t, bk, storage, badMember1)
		assertMemberOf(t, ctx, storage, badMember1.GetName(), []string{})
		assertOwnerOf(t, ctx, storage, badOwner1.GetName(), []string{})

		r.AccessPoint = &testStatusReconcilerAccessPoint{
			statusReconcilerAccessPoint:     storage,
			updateConflictForTheFirstNTimes: 2,
		}

		stats, err := r.reconcile(ctx)
		require.NoError(t, err)
		require.Equal(t, &statusReconcilerStats{
			processed:                      3,
			fixedOwnerLists:                1,
			fixedMemberLists:               1,
			hadConflicts:                   1,
			ownerListsNotFixedDueConflict:  0,
			memberListsNotFixedDueConflict: 0,
			attemptedRetries:               4, // 2 conflicts 1 owner and 1 member each
		}, stats)

		assertMemberOf(t, ctx, storage, badMember1.GetName(), []string{a1.GetName()})
		assertOwnerOf(t, ctx, storage, badOwner1.GetName(), []string{a1.GetName()})
	})

	t.Run("conflicts above the retry limits", func(t *testing.T) {
		resetAccessListStatus(t, bk, storage, badOwner1)
		resetAccessListStatus(t, bk, storage, badMember1)
		assertMemberOf(t, ctx, storage, badMember1.GetName(), []string{})
		assertOwnerOf(t, ctx, storage, badOwner1.GetName(), []string{})

		r.AccessPoint = &testStatusReconcilerAccessPoint{
			statusReconcilerAccessPoint:     storage,
			updateConflictForTheFirstNTimes: 1000,
		}

		stats, err := r.reconcile(ctx)
		require.NoError(t, err)
		require.Equal(t, &statusReconcilerStats{
			processed:                      3,
			fixedOwnerLists:                0,
			fixedMemberLists:               0,
			hadConflicts:                   1,
			ownerListsNotFixedDueConflict:  1,
			memberListsNotFixedDueConflict: 1,
			attemptedRetries:               statusReconcilerMaxRetries * 2, // x2 because it's for the bad owner and member
		}, stats)

		// Still bad because of conflicts.
		assertMemberOf(t, ctx, storage, badMember1.GetName(), []string{})
		assertOwnerOf(t, ctx, storage, badOwner1.GetName(), []string{})
	})
}

func assertMemberOf(t require.TestingT, ctx context.Context, svc *local.AccessListService, name string, expected []string) {
	if t, ok := t.(*testing.T); ok {
		t.Helper()
	}
	item, err := svc.GetAccessList(ctx, name)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.MemberOf)
}

func assertOwnerOf(t require.TestingT, ctx context.Context, svc *local.AccessListService, name string, expected []string) {
	if t, ok := t.(*testing.T); ok {
		t.Helper()
	}
	item, err := svc.GetAccessList(ctx, name)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.OwnerOf)
}

func resetAccessListStatus(t *testing.T, bk *memory.Memory, storage *local.AccessListService, acl *accesslist.AccessList) {
	t.Helper()
	i, err := bk.Get(t.Context(), backend.NewKey("access_list", acl.GetName()))
	require.NoError(t, err)
	v, err := services.UnmarshalAccessList(i.Value)
	require.NoError(t, err)
	v.Status = accesslist.Status{}
	buff, err := services.MarshalAccessList(v)
	require.NoError(t, err)
	i.Value = buff
	_, err = bk.Update(t.Context(), *i)
	require.NoError(t, err)
	v, err = storage.GetAccessList(t.Context(), acl.GetName())
	require.NoError(t, err)
	require.Equal(t, accesslist.Status{}, v.Status)
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

type testStatusReconcilerAccessPoint struct {
	statusReconcilerAccessPoint

	updateConflictForTheFirstNTimes uint64

	state struct {
		UpdateAccessListCnt       uint64
		UpdateAccessListMemberCnt uint64
	}
}

func (ap *testStatusReconcilerAccessPoint) UpdateAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	cnt := atomic.AddUint64(&ap.state.UpdateAccessListCnt, 1)
	if cnt <= ap.updateConflictForTheFirstNTimes {
		return nil, trace.CompareFailed("%d call to %T.UpdateAccessList", cnt, ap)
	}
	return ap.statusReconcilerAccessPoint.UpdateAccessList(ctx, accessList)
}
func (ap *testStatusReconcilerAccessPoint) UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	cnt := atomic.AddUint64(&ap.state.UpdateAccessListMemberCnt, 1)
	if cnt <= ap.updateConflictForTheFirstNTimes {
		return nil, trace.CompareFailed("%d call to %T.UpdateAccessListMember", cnt, ap)
	}
	return ap.statusReconcilerAccessPoint.UpdateAccessListMember(ctx, member)
}
