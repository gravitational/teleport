package accesslist

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
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
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
	// ListAccessLists returns a paginated list of access lists.
	ListAccessLists(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error)
	// UpdateAccessList updates an access list resource.
	UpdateAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)

	// GetAccessListMember returns the specified access list member resource.
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	// ListAccessListMembers returns a paginated list of all access list members.
	ListAccessListMembers(ctx context.Context, accessList string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error)
	// UpdateAccessListMember conditionally updates an access list member resource.
	UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)

	// CleanupAccessListStatus removes invalid Status.OwnerOf and Status.MemberOf references.
	CleanupAccessListStatus(ctx context.Context, accessListName string) (*accesslist.AccessList, error)

	// EnsureNestedListStatuses goes over all nested owners and nested members of the named access
	// list and ensures the the nested lists' statuses owner_of/member_of contain the access list name.
	EnsureNestedAccessListStatuses(ctx context.Context, accessListName string) error
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
				if stats.fixed > 0 {
					stats.wrapLogger(r.Logger, took, waitTime).WarnContext(ctx,
						"Finished reconciling access_list statuses, but some weren't fixed due to conflict. Will retry on the next loop")
				} else {
					stats.wrapLogger(r.Logger, took, waitTime).InfoContext(ctx,
						"Finished reconciling access_list statuses")
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *statusReconciler) reconcile(ctx context.Context) (*statusReconcilerStats, error) {
	stats := statusReconcilerStats{}

	for accessList, err := range clientutils.Resources(ctx, r.AccessPoint.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		stats.processed++

		isStatusDirty, err := r.isAccessListStatusDirty(ctx, accessList)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if isStatusDirty {
			accessList, err = r.AccessPoint.CleanupAccessListStatus(ctx, accessList.GetName())
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
			if err := r.AccessPoint.EnsureNestedAccessListStatuses(ctx, accessList.GetName()); err != nil {
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

	for _, ownedListName := range accessList.Status.OwnerOf {
		ownedList, err := r.AccessPoint.GetAccessList(ctx, ownedListName)
		if err != nil {
			if trace.IsNotFound(err) {
				r.Logger.WarnContext(ctx, "Found access_list with status.owner_of reference to a list that does not exist. It will be cleared",
					"access_list", accessList.GetName(), "bad_owner_of_entry", ownedListName)
				dirty = true
				continue
			}
			return false, trace.Wrap(err)
		}
		isActualOwner := slices.ContainsFunc(ownedList.Spec.Owners, func(ownedListOwner accesslist.Owner) bool {
			return ownedListOwner.MembershipKind == accesslist.MembershipKindList && ownedListOwner.Name == accessList.GetName()
		})
		if !isActualOwner {
			r.Logger.WarnContext(ctx, "Found access_list with incorrect status.owner_of entry. It will be cleared",
				"access_list", accessList.GetName(), "bad_owner_of_entry", ownedListName)
			dirty = true
			continue
		}
	}

	for _, parentListName := range accessList.Status.MemberOf {
		if _, err := r.AccessPoint.GetAccessListMember(ctx, parentListName, accessList.GetName()); err != nil {
			if trace.IsNotFound(err) {
				r.Logger.WarnContext(ctx, "Found access_list with status.member_of reference to a list that does not exist. It will be cleared",
					"access_list", accessList.GetName(), "bad_member_of_entry", parentListName)
				dirty = true
				continue
			}
			return false, trace.Wrap(err)
		}
	}

	return dirty, nil
}

// getBadOwnerListCnt returns a total number of all the owner access lists (of the provided access
// list) which don't have the provided access list name in their status.owner_of.
func (r *statusReconciler) getBadOwnerListCnt(ctx context.Context, accessList *accesslist.AccessList) (int, error) {
	badOwnerListCnt := 0
	for _, owner := range accessList.Spec.Owners {
		if owner.MembershipKind != accesslist.MembershipKindList {
			continue
		}
		ownerList, err := r.AccessPoint.GetAccessList(ctx, owner.Name)
		if trace.IsNotFound(err) {
			r.Logger.WarnContext(ctx,
				"access_list has a non-existing owner. The access_list can have a non-existing owner set or the owner access_list could be deleted in the meantime.",
				"originated_access_list", accessList.GetName(), "owner_access_list", owner.Name)
			continue
		} else if err != nil {
			return 0, trace.Wrap(err)
		}
		if !slices.Contains(ownerList.Status.OwnerOf, accessList.GetName()) {
			r.Logger.WarnContext(ctx, "Found owner access_list with missing status.owner_of entry. It will be added",
				"access_list", ownerList.GetName(), "missing_owner_of_entry", accessList.GetName())
			badOwnerListCnt++
		}
	}
	return badOwnerListCnt, nil
}

// getBadMemberListCnt returns a total number of all the member access lists (of the provided
// access list) which don't have the provided access list name in their status.member_of.
func (r *statusReconciler) getBadMemberListCnt(ctx context.Context, accessList *accesslist.AccessList) (int, error) {
	badMemberListCnt := 0
	listMembersFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
		members, pageToken, err := r.AccessPoint.ListAccessListMembers(ctx, accessList.GetName(), pageSize, pageToken)
		return members, pageToken, trace.Wrap(err)
	}
	for member, err := range clientutils.Resources(ctx, listMembersFn) {
		if err != nil {
			return 0, trace.Wrap(err)
		}

		if member.Spec.MembershipKind != accesslist.MembershipKindList {
			continue
		}
		memberList, err := r.AccessPoint.GetAccessList(ctx, member.GetName())
		if trace.IsNotFound(err) {
			r.Logger.WarnContext(ctx,
				"access_list has a non-existing member. The originated access_list can have a non-existing member set or the member access_list could be deleted in the meantime.",
				"originated_access_list", accessList.GetName(), "member_access_list", member.GetName())
			continue
		} else if err != nil {
			return 0, trace.Wrap(err)
		}
		if !slices.Contains(memberList.Status.MemberOf, accessList.GetName()) {
			r.Logger.WarnContext(ctx, "Found member access_list with missing status.member_of entry. It will be added",
				"access_list", memberList.GetName(), "missing_member_of_entry", accessList.GetName())
			badMemberListCnt++
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

func (s *statusReconcilerStats) wrapLogger(l *slog.Logger, took, waitTime time.Duration) *slog.Logger {
	return l.With(
		slog.String("took", took.String()),
		slog.String("next_run_in", waitTime.String()),
		slog.Int("processed", s.processed),
		slog.Int("fixed", s.fixed),
	)
}
