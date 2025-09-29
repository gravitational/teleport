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

	// statusReconcilerMaxRetries in case of conflicts.
	statusReconcilerMaxRetries = 3

	// statusReconcilerRetryHalfJitter between retries in case of conflicts.
	statusReconcilerRetryHalfJitter = 2 * time.Second
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
				if stats.ownerListsNotFixedDueConflict > 0 {
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

		badOwnerLists, err := r.findBadOwnerLists(ctx, accessList)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		badMemberLists, err := r.findBadMemberLists(ctx, accessList)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		hadConflicts := false
		if len(badOwnerLists) > 0 {
			fixed, attempts, err := r.tryFixOwnersStatusesFor(ctx, accessList)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if fixed {
				stats.fixedOwnerLists += len(badOwnerLists)
			} else {
				stats.ownerListsNotFixedDueConflict += len(badOwnerLists)
			}
			hadConflicts = hadConflicts || attempts > 1
			stats.attemptedRetries += attempts - 1
		}

		for _, badMember := range badMemberLists {
			fixed, attempts, err := r.tryFixMemberStatus(ctx, accessList.GetName(), badMember)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if fixed {
				stats.fixedMemberLists++
			} else {
				stats.memberListsNotFixedDueConflict++
			}
			hadConflicts = hadConflicts || attempts > 1
			stats.attemptedRetries += attempts - 1
		}
		if hadConflicts {
			stats.hadConflicts++
		}
	}

	return &stats, nil
}

// findBadOwnerLists returns names of all the owner access lists (of the provided access list)
// which don't have the provided access list name in their status.owner_of.
func (r *statusReconciler) findBadOwnerLists(ctx context.Context, accessList *accesslist.AccessList) ([]string, error) {
	var badOwnerLists []string
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
			return nil, trace.Wrap(err)
		}
		if !slices.Contains(ownerList.Status.OwnerOf, accessList.GetName()) {
			badOwnerLists = append(badOwnerLists, ownerList.GetName())
		}
	}
	return badOwnerLists, nil
}

// findBadMemberLists returns names of all the member access lists (of the provided access list)
// which don't have the provided access list name in their status.member_of.
func (r *statusReconciler) findBadMemberLists(ctx context.Context, accessList *accesslist.AccessList) ([]string, error) {
	var badMemberLists []string
	listMembersFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
		members, pageToken, err := r.AccessPoint.ListAccessListMembers(ctx, accessList.GetName(), pageSize, pageToken)
		return members, pageToken, trace.Wrap(err)
	}
	for member, err := range clientutils.Resources(ctx, listMembersFn) {
		if err != nil {
			return nil, trace.Wrap(err)
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
			return nil, trace.Wrap(err)
		}
		if !slices.Contains(memberList.Status.MemberOf, accessList.GetName()) {
			badMemberLists = append(badMemberLists, memberList.GetName())
		}
	}
	return badMemberLists, nil
}

// tryFixOwnersStatusesFor makes sure status.owner_of of all the owner lists of the provided access
// list contains the provided access list's name.
func (r *statusReconciler) tryFixOwnersStatusesFor(ctx context.Context, accessList *accesslist.AccessList) (fixed bool, attempts int, err error) {
	accessListName := accessList.GetName()
	refreshFn := func(ctx context.Context, isRetry bool) (*accesslist.AccessList, error) {
		attempts++
		if !isRetry {
			// If not a retry attempt, do not refresh and return the original list from
			// pagination.
			return accessList, nil
		}
		refreshed, err := r.AccessPoint.GetAccessList(ctx, accessListName)
		return refreshed, trace.Wrap(err)
	}
	fixFn := func(ctx context.Context, accessList *accesslist.AccessList) error {
		// UpdateAccessList makes sure accessList name is in all status.owner_of of all the owner lists.
		_, err := r.AccessPoint.UpdateAccessList(ctx, accessList)
		return trace.Wrap(err)
	}
	err = retryutils.UpdateWithRetry(ctx, r.Clock, refreshFn, fixFn)
	switch {
	case trace.IsNotFound(err):
		r.Logger.DebugContext(ctx, "access_list not found, probably deleted in the meantime",
			"access_list", accessListName, "error", err.Error())
		return false, attempts, nil
	case trace.IsCompareFailed(err):
		r.Logger.WarnContext(ctx, "access_list owners could not be fixed due to conflicts. Will not retry till the next reconciliation loop",
			"access_list", accessListName, "error", err.Error())
		return false, attempts, nil
	case err != nil:
		return false, 0, trace.Wrap(err)
	default:
		return true, attempts, nil
	}
}

// tryFixMemberStatus makes sure status.member_of of the access list nested member contains the
// access list's name.
func (r *statusReconciler) tryFixMemberStatus(ctx context.Context, accessList, member string) (fixed bool, attempts int, err error) {
	refreshFn := func(ctx context.Context, _ bool) (*accesslist.AccessListMember, error) {
		attempts++
		refreshed, err := r.AccessPoint.GetAccessListMember(ctx, accessList, member)
		return refreshed, trace.Wrap(err)
	}
	fixFn := func(ctx context.Context, member *accesslist.AccessListMember) error {
		// UpdateAccessListMember for nested access list member ensures that member's list
		// status.member_of contains the parent access list name.
		_, err := r.AccessPoint.UpdateAccessListMember(ctx, member)
		return trace.Wrap(err)
	}
	err = retryutils.UpdateWithRetry(ctx, r.Clock, refreshFn, fixFn)
	switch {
	case trace.IsNotFound(err):
		r.Logger.DebugContext(ctx, "access_list_member not found, probably deleted in the meantime",
			"access_list_member", accessList+"/"+member, "error", err.Error())
		return false, attempts, nil
	case trace.IsCompareFailed(err):
		r.Logger.WarnContext(ctx, "access_list_member could not be fixed due to conflicts. Will not retry till the next reconciliation loop",
			"access_list_member", accessList+"/"+member, "error", err.Error())
		return false, attempts, nil
	case err != nil:
		return false, 0, trace.Wrap(err)
	default:
		return true, attempts, nil
	}
}

// statusReconcilerStats are the statistics for a single reconciliation loop.
type statusReconcilerStats struct {
	// processed is the number of access lists processed.
	processed int
	// fixedOwnerLists is the number of owner access lists fixed.
	fixedOwnerLists int
	// fixedMemberLists is the number of member access lists fixed.
	fixedMemberLists int
	// hadConflicts is the number of access lists which had not up-to-date owner or member
	// lists and some or all of them were not fixed due to conflicts.
	hadConflicts int
	// ownerListsNotFixedDueConflict is the number of owner access lists not fixed due to conflicts.
	ownerListsNotFixedDueConflict int
	// memberListsNotFixedDueConflict is the number of member access lists not fixed due to conflicts.
	memberListsNotFixedDueConflict int
	// attemptedRetries is the number of retries happened due to conflicts. This number bigger
	// than 0 doesn't mean all lists are not fixed.
	attemptedRetries int
}

func (s *statusReconcilerStats) wrapLogger(l *slog.Logger, took, waitTime time.Duration) *slog.Logger {
	return l.With(
		slog.String("took", took.String()),
		slog.String("next_run_in", waitTime.String()),
		slog.Int("processed", s.processed),
		slog.Int("fixed_owner_lists", s.fixedOwnerLists),
		slog.Int("fixed_member_lists", s.fixedMemberLists),
		slog.Int("had_conflicts", s.hadConflicts),
		slog.Int("owner_lists_not_fixed_due_conflict", s.ownerListsNotFixedDueConflict),
		slog.Int("member_lists_not_fixed_due_conflict", s.memberListsNotFixedDueConflict),
		slog.Int("attempted_retries", s.attemptedRetries),
	)
}
