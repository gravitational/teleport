package accesslist

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func Test_statusReconciler_MemberOfOwnerOf(t *testing.T) {
	t.Parallel()
	synctest.Test(t, test_statusReconciler_MemberOfOwnerOf)
}

func test_statusReconciler_MemberOfOwnerOf(t *testing.T) {
	ctx := t.Context()

	clock := clockwork.NewRealClock()
	bk, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	storage, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: bk,
		Modules: &modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{entitlements.Identity: {Enabled: true}},
			},
		},
		RunWhileLockedRetryInterval: -1 * time.Millisecond,
		ScopesFeatures: scopes.Features{
			Enabled: true,
		},
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

	// a8 has status.scoped_member_of and status.scoped_owner_of set to unparseable scope-qualified names
	a8Name := accesslists.NormalizedSQN{Name: "s8", Scope: "/test"}
	a8 := newScopedAccessList(t, a8Name, clock)
	// a9 is a member and an owner of a8, but doesn't have it set in status
	a9Name := accesslists.NormalizedSQN{Name: "s9", Scope: "/test"}
	a9 := newScopedAccessList(t, a9Name, clock)
	// a10 is not a member and an owner of a8, but does have it set in status
	a10Name := accesslists.NormalizedSQN{Name: "s10", Scope: "/test"}
	a10 := newScopedAccessList(t, a10Name, clock)

	a1m1 := newAccessListMember(t, a1.GetName(), a2.GetName(), accesslist.MembershipKindList, clock)
	a1.SetOwners([]accesslist.Owner{
		{
			Name:           a3.GetName(),
			MembershipKind: accesslist.MembershipKindList,
		},
	})
	a8m9 := newScopedAccessListMember(t, a8Name, a9Name, accesslist.MembershipKindScopedList, clock)
	a8.SetOwners([]accesslist.Owner{{
		Name:           a9Name.String(),
		MembershipKind: accesslist.MembershipKindScopedList,
	}})
	require.NoError(t, upsertAccessList(t.Context(), storage, []*accesslist.AccessList{a10, a9, a8, a7, a6, a5, a4, a3, a2, a1}, []*accesslist.AccessListMember{a8m9, a1m1}))

	const nonExistingList1, nonExistingList2 = "non_existing_list1", "non_existing_list2"
	const badSQN = "bad scope::bad/name"

	assertStatusMemberOf(t, ctx, storage, a2.GetName(), []string{a1.GetName()})
	assertStatusOwnerOf(t, ctx, storage, a3.GetName(), []string{a1.GetName()})

	resetAccessListStatus(t, bk, storage, a2)
	resetAccessListStatus(t, bk, storage, a3)
	setAccessListStatus(t, bk, storage, a4, accesslist.Status{MemberOf: []string{a1.GetName()}})
	setAccessListStatus(t, bk, storage, a5, accesslist.Status{OwnerOf: []string{a1.GetName()}})
	setAccessListStatus(t, bk, storage, a6, accesslist.Status{MemberOf: []string{nonExistingList1}})
	setAccessListStatus(t, bk, storage, a7, accesslist.Status{OwnerOf: []string{nonExistingList2}})
	setAccessListStatus(t, bk, storage, a8, accesslist.Status{ScopedMemberOf: []string{badSQN}, ScopedOwnerOf: []string{badSQN}})
	resetAccessListStatus(t, bk, storage, a9)
	setAccessListStatus(t, bk, storage, a10, accesslist.Status{ScopedMemberOf: []string{a8Name.String()}, ScopedOwnerOf: []string{a8Name.String()}})

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
	assertStatusScopedMemberOf(t, ctx, storage, a8Name, []string{badSQN})
	assertStatusScopedOwnerOf(t, ctx, storage, a8Name, []string{badSQN})
	assertStatusScopedMemberOf(t, ctx, storage, a9Name, []string{})
	assertStatusScopedOwnerOf(t, ctx, storage, a9Name, []string{})
	assertStatusScopedMemberOf(t, ctx, storage, a10Name, []string{a8Name.String()})
	assertStatusScopedOwnerOf(t, ctx, storage, a10Name, []string{a8Name.String()})

	go r.Run(ctx)

	synctest.Wait()
	time.Sleep(statusReconcilerStartupSeventhJitter)
	synctest.Wait()

	assertStatusMemberOf(t, ctx, storage, a2.GetName(), []string{a1.GetName()})
	assertStatusOwnerOf(t, ctx, storage, a3.GetName(), []string{a1.GetName()})
	assertStatusMemberOf(t, ctx, storage, a4.GetName(), []string{})
	assertStatusOwnerOf(t, ctx, storage, a5.GetName(), []string{})
	assertStatusMemberOf(t, ctx, storage, a6.GetName(), []string{})
	assertStatusOwnerOf(t, ctx, storage, a7.GetName(), []string{})
	assertStatusScopedMemberOf(t, ctx, storage, a8Name, []string{})
	assertStatusScopedOwnerOf(t, ctx, storage, a8Name, []string{})
	assertStatusScopedMemberOf(t, ctx, storage, a9Name, []string{a8Name.String()})
	assertStatusScopedOwnerOf(t, ctx, storage, a9Name, []string{a8Name.String()})
	assertStatusScopedMemberOf(t, ctx, storage, a10Name, []string{})
	assertStatusScopedOwnerOf(t, ctx, storage, a10Name, []string{})
}

// Test statusReconcilerConfig.reconcile can survive the situation where the owner or member
// access_list doesn't exist.
func Test_statusReconciler_reconcile_missingOwnerAndMemberLists(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	storage, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend:                     bk,
		Modules:                     modulestest.EnterpriseModules(),
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

func assertStatusMemberOf(t *testing.T, ctx context.Context, svc *local.AccessListService, name string, expected []string) {
	t.Helper()
	item, err := svc.GetAccessList(ctx, name)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.MemberOf)
}

func assertStatusOwnerOf(t *testing.T, ctx context.Context, svc *local.AccessListService, name string, expected []string) {
	t.Helper()
	item, err := svc.GetAccessList(ctx, name)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.OwnerOf)
}

func assertStatusScopedMemberOf(t *testing.T, ctx context.Context, svc *local.AccessListService, name accesslists.NormalizedSQN, expected []string) {
	t.Helper()
	item, err := svc.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
		Scope: name.Scope,
		Name:  name.Name,
	}.Build())
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.ScopedMemberOf)
}

func assertStatusScopedOwnerOf(t *testing.T, ctx context.Context, svc *local.AccessListService, name accesslists.NormalizedSQN, expected []string) {
	t.Helper()
	item, err := svc.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
		Scope: name.Scope,
		Name:  name.Name,
	}.Build())
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.ScopedOwnerOf)
}

func resetAccessListStatus(t *testing.T, bk *memory.Memory, storage *local.AccessListService, acl *accesslist.AccessList) {
	setAccessListStatus(t, bk, storage, acl, accesslist.Status{})
}

func setAccessListStatus(t *testing.T, bk *memory.Memory, storage *local.AccessListService, acl *accesslist.AccessList, newStatus accesslist.Status) {
	t.Helper()
	key := backend.NewKey("access_list", acl.GetName())
	if acl.Scope != "" {
		encodedScope, err := scopes.EncodeForKey(acl.Scope)
		require.NoError(t, err)
		key = backend.NewKey("scoped", "access_list", encodedScope, acl.GetName())
	}
	i, err := bk.Get(t.Context(), key)
	require.NoError(t, err)
	v, err := services.UnmarshalAccessList(i.Value)
	require.NoError(t, err)
	v.Status = newStatus
	buff, err := services.MarshalAccessList(v)
	require.NoError(t, err)
	i.Value = buff
	_, err = bk.Update(t.Context(), *i)
	require.NoError(t, err)
	v, err = storage.GetAccessListV2(t.Context(), accesslistv1.GetAccessListRequest_builder{
		Scope: acl.Scope,
		Name:  acl.GetName(),
	}.Build())
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
