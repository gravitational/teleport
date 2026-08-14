package accesslist

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/accesslists"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// statusReconcilerStartupSeventhJitter is there to delay startup to avoid the initial
	// server load and try to wait for caches to load.
	statusReconcilerStartupSeventhJitter = 2*time.Minute + 30*time.Second

	// statusReconcilerTimeBetweenRuns is the time between reconciliation loops.
	statusReconcilerTimeBetweenRuns = 10 * time.Minute
)

type statusReconcilerAccessPoint interface {
	// GetAccessListV2 returns the specified access list resource.
	GetAccessListV2(context.Context, *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error)
	// ListAccessListsV2 returns a paginated list of access lists.
	ListAccessListsV2(context.Context, *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error)
	// UpdateAccessList updates an access list resource.
	UpdateAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)

	// GetAccessListMemberV2 returns the specified access list member resource.
	GetAccessListMemberV2(context.Context, *accesslistv1.GetAccessListMemberRequest) (*accesslist.AccessListMember, error)
	// ListAccessListMembersV2 returns a paginated list of all access list members.
	ListAccessListMembersV2(context.Context, *accesslistv1.ListAccessListMembersRequest) (members []*accesslist.AccessListMember, nextToken string, err error)
	// UpdateAccessListMember conditionally updates an access list member resource.
	UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)

	// CleanupAccessListStatusV2 removes invalid Status.OwnerOf and Status.MemberOf references.
	CleanupAccessListStatusV2(ctx context.Context, accessListName accesslists.NormalizedSQN) (*accesslist.AccessList, error)

	// EnsureNestedListStatusesV2 goes over all nested owners and nested members of the named access
	// list and ensures the the nested lists' statuses owner_of/member_of contain the access list name.
	EnsureNestedAccessListStatusesV2(ctx context.Context, accessListName accesslists.NormalizedSQN) error
}

// statusReconcilerConfig configuration.
type statusReconcilerConfig struct {
	Logger      *slog.Logger
	Clock       clockwork.Clock
	AccessPoint statusReconcilerAccessPoint
}

func (c *statusReconcilerConfig) CheckAndSetDefaults() error {
	if c.Logger == nil {
		return trace.BadParameter("statusReconciler logger missing")
	}
	c.Logger = c.Logger.With(teleport.ComponentKey, eteleport.ComponentAccessListStatusReconciler)
	if c.Clock == nil {
		return trace.BadParameter("statusReconciler clock missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("statusReconciler access point missing")
	}
	return nil
}

// statusReconciler makes sure status field of AccessList is correct. In particular it reconciles
// member_of/owner_of fields which may be incorrect due a bug fixed in
// https://github.com/gravitational/teleport/pull/56760.
//
// It seems to be a good candidate for a migration step instead
// (https://github.com/gravitational/teleport/blob/0888abce3dc62d6947c49d501d45cd02e32aaffd/lib/auth/init.go#L640),
// but there are a few reasons to go with the reconciler approach instead:
//
//   - in HA mode there is still possibility that and old backend with a bug will be used while the
//     migration is done on the new version instance (rolling instance may take even 8h in some
//     cases)
//   - migration works at the startup of the process without cache, which on 50k lists x 200 users
//     clusters may take a minute or more (vs a few seconds with cache)
//   - we can't ever get rid of some edge cases, like SIGKILL between cleaning status and deleting
//     access_list_member resource
//   - there still may be some uncovered bugs with correctly setting the status
//
// As for the performance. Single reconciliation run on a healthy loadtest cluster (50k lists x 200
// users per list) takes <4s to run every 10 min. It also runs on a single instance in HA mode. So
// it doesn't seem to be a bigger load than refreshing access lists page in the web UI without
// pagination.
type statusReconciler struct {
	statusReconcilerConfig
}

func newStatusReconciler(config statusReconcilerConfig) (*statusReconciler, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &statusReconciler{
		statusReconcilerConfig: config,
	}, nil
}

// Run the reconciliation loop. This methods blocks till the context is canceled.
func (r *statusReconciler) Run(ctx context.Context) error {
	startupDelay := retryutils.SeventhJitter(statusReconcilerStartupSeventhJitter)
	r.Logger.DebugContext(ctx, "Delaying access_list statuses reconciler startup", "delay", logutils.StringerAttr(startupDelay))

	timer := r.Clock.NewTimer(startupDelay)
	defer timer.Stop()

	for {
		select {
		case <-timer.Chan():
			r.Logger.InfoContext(ctx, "Starting reconciling access_list statuses")

			start := r.Clock.Now()
			stats, err := r.reconcile(ctx)
			took := r.Clock.Since(start)
			waitTime := statusReconcilerTimeBetweenRuns
			timer.Reset(waitTime)
			if err != nil {
				r.Logger.ErrorContext(ctx, "Error reconciling access_list statuses", "error", trace.Wrap(err))
			} else {
				r.Logger.InfoContext(ctx, "Finished reconciling access_list statuses",
					"took", took.String(),
					"next_run_in", waitTime.String(),
					"processed", stats.processed,
					"fixed", stats.fixed,
				)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *statusReconciler) reconcile(ctx context.Context) (*statusReconcilerStats, error) {
	stats := statusReconcilerStats{}

	pageFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
		return r.AccessPoint.ListAccessListsV2(ctx, accesslistv1.ListAccessListsV2Request_builder{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
			ScopeFilter: scopesv1.Filter_builder{
				Mode: scopesv1.Mode_MODE_ALL,
			}.Build(),
		}.Build())
	}
	for accessList, err := range clientutils.Resources(ctx, pageFn) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		stats.processed++

		isStatusDirty, err := r.isAccessListStatusDirty(ctx, accessList)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if isStatusDirty {
			accessList, err = r.AccessPoint.CleanupAccessListStatusV2(ctx, accesslists.ScopeQualifiedName(accessList))
			if err != nil {
				return nil, trace.Wrap(err)
			}
			stats.fixed++
		}

		badOwnerListCnt, err := r.getBadOwnerListCnt(ctx, accessList)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		badMemberListCnt, err := r.getBadMemberListCnt(ctx, accessList)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if badOwnerListCnt+badMemberListCnt > 0 {
			if err := r.AccessPoint.EnsureNestedAccessListStatusesV2(ctx, accesslists.ScopeQualifiedName(accessList)); err != nil {
				return nil, trace.Wrap(err)
			}
			stats.fixed += badOwnerListCnt + badMemberListCnt
		}
	}

	return &stats, nil
}

// isAccessListStatusDirty returns true if the access_list.status.{owner_of,member_of} contains
// invalid entries. I.e. entries referencing lists that are not owned by this access list, or are
// not parents of this access list.
func (r *statusReconciler) isAccessListStatusDirty(ctx context.Context, accessList *accesslist.AccessList) (bool, error) {
	dirty := false

	accessListName := accesslists.ScopeQualifiedName(accessList)

	for _, ownedListName := range accessList.Status.OwnerOf {
		ownedList, err := r.AccessPoint.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
			Name: ownedListName,
		}.Build())
		if err != nil {
			if trace.IsNotFound(err) {
				r.Logger.WarnContext(ctx, "Found access_list with status.owner_of reference to a list that does not exist. It will be cleared",
					"access_list", accessListName.String(), "bad_owner_of_entry", ownedListName)
				dirty = true
				continue
			}
			return false, trace.Wrap(err)
		}
		isActualOwner := slices.ContainsFunc(ownedList.Spec.Owners, func(ownedListOwner accesslist.Owner) bool {
			return ownedListOwner.IsMembershipKindList() && ownedListOwner.Name == accessListName.String()
		})
		if !isActualOwner {
			r.Logger.WarnContext(ctx, "Found access_list with incorrect status.owner_of entry. It will be cleared",
				"access_list", accessListName.String(), "bad_owner_of_entry", ownedListName)
			dirty = true
			continue
		}
	}
	for _, ownedListName := range accessList.Status.ScopedOwnerOf {
		ownedListSQN, err := accesslists.ParseScopeQualifiedName(ownedListName)
		if err != nil {
			r.Logger.WarnContext(ctx, "Found access_list with status.scoped_owner_of reference that does not parse. It will be cleared",
				"access_list", accessListName.String(), "bad_scoped_owner_of_entry", ownedListName)
			dirty = true
			continue
		}
		ownedList, err := r.AccessPoint.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
			Scope: ownedListSQN.Scope,
			Name:  ownedListSQN.Name,
		}.Build())
		if err != nil {
			if trace.IsNotFound(err) {
				r.Logger.WarnContext(ctx, "Found access_list with status.scoped_owner_of reference to a list that does not exist. It will be cleared",
					"access_list", accessListName.String(), "bad_scoped_owner_of_entry", ownedListName)
				dirty = true
				continue
			}
			return false, trace.Wrap(err)
		}
		isActualOwner := slices.ContainsFunc(ownedList.Spec.Owners, func(ownedListOwner accesslist.Owner) bool {
			return ownedListOwner.IsMembershipKindList() && ownedListOwner.Name == accessListName.String()
		})
		if !isActualOwner {
			r.Logger.WarnContext(ctx, "Found access_list with incorrect status.scoped_owner_of entry. It will be cleared",
				"access_list", accessListName.String(), "bad_owner_of_entry", ownedListName)
			dirty = true
			continue
		}
	}

	for _, parentListName := range accessList.Status.MemberOf {
		member, err := r.AccessPoint.GetAccessListMemberV2(ctx, accesslistv1.GetAccessListMemberRequest_builder{
			AccessList:  parentListName,
			MemberScope: accessListName.Scope,
			MemberName:  accessListName.Name,
		}.Build())
		if trace.IsNotFound(err) || err == nil && !member.IsList() {
			r.Logger.WarnContext(ctx, "Found access_list with status.member_of reference to a list it's not a member of. It will be cleared",
				"access_list", accessListName.String(), "bad_member_of_entry", parentListName)
			dirty = true
			continue
		}
		if err != nil {
			return false, trace.Wrap(err)
		}
	}
	for _, parentListName := range accessList.Status.ScopedMemberOf {
		parentListSQN, err := accesslists.ParseScopeQualifiedName(parentListName)
		if err != nil {
			r.Logger.WarnContext(ctx, "Found access_list with status.scoped_member_of reference that does not parse. It will be cleared",
				"access_list", accessListName.String(), "bad_scoped_member_of_entry", parentListName)
			dirty = true
			continue
		}
		member, err := r.AccessPoint.GetAccessListMemberV2(ctx, accesslistv1.GetAccessListMemberRequest_builder{
			AccessListScope: parentListSQN.Scope,
			AccessList:      parentListSQN.Name,
			MemberScope:     accessListName.Scope,
			MemberName:      accessListName.Name,
		}.Build())
		if trace.IsNotFound(err) || err == nil && !member.IsList() {
			r.Logger.WarnContext(ctx, "Found access_list with status.scoped_member_of reference to a list to a list it's not a member of. It will be cleared",
				"access_list", accessListName.String(), "bad_member_of_entry", parentListName)
			dirty = true
			continue
		}
		if err != nil {
			return false, trace.Wrap(err)
		}
	}

	return dirty, nil
}

// getBadOwnerListCnt returns a total number of all the owner access lists (of the provided access
// list) which don't have the provided access list name in their status.(scoped_)owner_of.
func (r *statusReconciler) getBadOwnerListCnt(ctx context.Context, accessList *accesslist.AccessList) (int, error) {
	accessListName := accesslists.ScopeQualifiedName(accessList)
	badOwnerListCnt := 0
	for _, owner := range accessList.Spec.Owners {
		if !owner.IsMembershipKindList() {
			continue
		}
		ownerName, err := accesslists.OwnerScopeQualifiedName(owner)
		if err != nil {
			r.Logger.WarnContext(ctx, "access_list has an owner with an unparseable name",
				"originated_access_list", accessListName.String(), "owner_access_list", owner.Name)
			continue
		}
		ownerList, err := r.AccessPoint.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
			Scope: ownerName.Scope,
			Name:  ownerName.Name,
		}.Build())
		if trace.IsNotFound(err) {
			r.Logger.WarnContext(ctx,
				"access_list has a non-existing owner. The access_list can have a non-existing owner set or the owner access_list could be deleted in the meantime.",
				"originated_access_list", accessListName.String(), "owner_access_list", owner.Name)
			continue
		} else if err != nil {
			return 0, trace.Wrap(err)
		}
		if accessList.GetScope() == "" {
			if !slices.Contains(ownerList.Status.OwnerOf, accessListName.String()) {
				r.Logger.WarnContext(ctx, "Found owner access_list with missing status.owner_of entry. It will be added",
					"access_list", ownerName.String(), "missing_owner_of_entry", accessListName.String())
				badOwnerListCnt++
			}
		} else {
			if !slices.Contains(ownerList.Status.ScopedOwnerOf, accessListName.String()) {
				r.Logger.WarnContext(ctx, "Found owner access_list with missing status.scoped_owner_of entry. It will be added",
					"access_list", ownerName.String(), "missing_scoped_owner_of_entry", accessListName.String())
				badOwnerListCnt++
			}
		}
	}
	return badOwnerListCnt, nil
}

// getBadMemberListCnt returns a total number of all the member access lists (of the provided
// access list) which don't have the provided access list name in their status.member_of.
func (r *statusReconciler) getBadMemberListCnt(ctx context.Context, accessList *accesslist.AccessList) (int, error) {
	accessListName := accesslists.ScopeQualifiedName(accessList)
	badMemberListCnt := 0
	listMembersFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
		members, pageToken, err := r.AccessPoint.ListAccessListMembersV2(ctx, accesslistv1.ListAccessListMembersRequest_builder{
			AccessListScope: accessList.GetScope(),
			AccessList:      accessList.GetName(),
			PageSize:        int32(pageSize),
			PageToken:       pageToken,
		}.Build())
		return members, pageToken, trace.Wrap(err)
	}
	for member, err := range clientutils.Resources(ctx, listMembersFn) {
		if err != nil {
			return 0, trace.Wrap(err)
		}

		if !member.IsList() {
			continue
		}
		memberName, err := accesslists.MemberScopeQualifiedName(member)
		if err != nil {
			r.Logger.WarnContext(ctx, "access_list has a member with an unparseable name",
				"originated_access_list", accessListName.String(), "member_access_list", member.GetName())
			continue
		}
		memberList, err := r.AccessPoint.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
			Scope: memberName.Scope,
			Name:  memberName.Name,
		}.Build())
		if trace.IsNotFound(err) {
			r.Logger.WarnContext(ctx,
				"access_list has a non-existing member. The originated access_list can have a non-existing member set or the member access_list could be deleted in the meantime.",
				"originated_access_list", accessListName.String(), "member_access_list", memberName.String())
			continue
		} else if err != nil {
			return 0, trace.Wrap(err)
		}
		if accessList.GetScope() == "" {
			if !slices.Contains(memberList.Status.MemberOf, accessListName.String()) {
				r.Logger.WarnContext(ctx, "Found member access_list with missing status.member_of entry. It will be added",
					"access_list", memberName.String(), "missing_scoped_member_of_entry", accessListName.String())
				badMemberListCnt++
			}
		} else {
			if !slices.Contains(memberList.Status.ScopedMemberOf, accessListName.String()) {
				r.Logger.WarnContext(ctx, "Found member access_list with missing status.scoped_member_of entry. It will be added",
					"access_list", memberName.String(), "missing_scoped_member_of_entry", accessListName.String())
				badMemberListCnt++
			}
		}
	}
	return badMemberListCnt, nil
}

// statusReconcilerStats are the statistics for a single reconciliation loop.
type statusReconcilerStats struct {
	// processed is the number of access lists processed.
	processed int
	// fixed is the number of access lists that required a fix and were fixed.
	fixed int
}
