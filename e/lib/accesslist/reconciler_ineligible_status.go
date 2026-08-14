package accesslist

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/utils/batcher"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// IneligibleStatusReconcilerConfig is the configuration for the IneligibleStatusReconciler.
type IneligibleStatusReconcilerConfig struct {
	// Cache is the cache of resources.
	Cache Cache
	// Service is the Access Lists service in the auth server.
	Service AccessListUpdater
	// Logger emits log messages.
	Logger *slog.Logger
	// Clock is the clock.
	Clock clockwork.Clock
}

// AccessListUpdater is the interface for updating access list members and access lists.
type AccessListUpdater interface {
	UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	UpdateAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error)
}

// Cache is a cache of resources.
type Cache interface {
	GetAccessListMemberV2(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslist.AccessListMember, error)
	GetAccessListV2(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error)
	GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error)
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	ListAllAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAllAccessListMembersRequest) (members []*accesslist.AccessListMember, nextToken string, err error)
	ListAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) (members []*accesslist.AccessListMember, nextToken string, err error)
	ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error)
	// ListAccessListsV2 lists access lists with filtering and sorting options.
	ListAccessListsV2(ctx context.Context, req *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error)
	CountAccessListMembersV2(ctx context.Context, req *accesslistv1.CountAccessListMembersRequest) (uint32, uint32, error)
}

// NewIneligibleStatusReconciler creates a new IneligibleStatusReconciler.
func NewIneligibleStatusReconciler(ctx context.Context, cfg IneligibleStatusReconcilerConfig) (*IneligibleStatusReconciler, error) {
	watcher, err := cfg.Cache.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindUser},
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := waitForInitOp(ctx, watcher); err != nil {
		return nil, trace.Wrap(err)
	}

	return &IneligibleStatusReconciler{
		cache:   cfg.Cache,
		service: cfg.Service,
		watcher: watcher,
		logger:  cfg.Logger,
		clock:   cfg.Clock,
	}, nil
}

func waitForInitOp(ctx context.Context, watcher types.Watcher) error {
	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case p := <-watcher.Events():
			if p.Type == types.OpInit {
				return nil
			}
		case <-watcher.Done():
			return trace.Wrap(watcher.Error())
		}
	}
}

// IneligibleStatusReconciler is a reconciler that updates the ineligible status of access list members and access list owners.
type IneligibleStatusReconciler struct {
	cache   Cache
	service AccessListUpdater
	clock   clockwork.Clock
	watcher types.Watcher
	logger  *slog.Logger
}

const (
	// 100 years is a good enough approximation for "never"
	neverDuration = time.Hour * 24 * 365 * 100
	// forceReconcileDuration is the duration after which we will force a reconciliation to happen.
	forceReconcileDuration = 30 * time.Minute
)

// Run runs the reconciliation loop.
func (r *IneligibleStatusReconciler) Run(ctx context.Context) error {
	// reconcileC is used to trigger a reconciliation
	// or queue one if a reconciliation is already in progress
	// to make sure that reconciler will react to the latest events
	// without skipping any action.
	reconcileC := make(chan struct{}, 1)
	// add initial one minute  then set the expiration based on the next member expiration
	// or forceReconcileDuration, whichever is sooner.
	wakeTimer := interval.New(interval.Config{
		FirstDuration: time.Minute,
		Duration:      forceReconcileDuration,
		Clock:         r.clock,
	})
	defer wakeTimer.Stop()

	queueReconcileFn := func() {
		select {
		case <-ctx.Done():
			return
		case reconcileC <- struct{}{}:
			// Trigger or Queue a reconciliation.
		default:
			// Reconciliation already running with one queued, no need to queue more.
		}
	}

	sm, err := batcher.NewStateMonitor(batcher.StateConfig{
		Threshold:             maxBatchQueueSize,
		OverloadedIdleTimeout: 2 * batchWindow,
		Clock:                 r.clock,
		OnExitOverloaded: func() {
			r.logger.InfoContext(ctx, "IneligibleStatusReconciler has exited overloaded mode. Resuming event-based reconciliation.")
			queueReconcileFn()
		},
		OnEnterOverloaded: func() {
			r.logger.WarnContext(ctx,
				"IneligibleStatusReconciler has entered overloaded mode due to high event volume. "+
					"Event-driven reconciliations will be delayed, but periodic reconciliations will continue to ensure eventual consistency.",
			)
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer sm.Stop()

	go r.runTriggerLoop(ctx, queueReconcileFn, sm)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.watcher.Done():
			return trace.Wrap(r.watcher.Error())
		case <-wakeTimer.Next():
		case <-reconcileC:
		}

		nextExpiration, err := r.reconciliationLoop(ctx)
		if trace.IsCompareFailed(err) {
			// retry on compare failed errors
			// this means that the membership was updated while we were processing
			// the list, so we need to reprocess it slilghtly.
			// we sleep for a bit to avoid spinning too much
			// when we get a lot of these errors because of a lot of updates.
			wakeTimer.ResetTo(15 * time.Second)
			continue
		} else if err != nil {
			r.logger.WarnContext(ctx, "Failed to reconcile memberships", "error", err)
		}
		// The reconciliationLoop returns the earliest expiration time across all access list members
		wakeTimer.ResetTo(min(nextExpiration, forceReconcileDuration))
	}
}

func (r *IneligibleStatusReconciler) runTriggerLoop(ctx context.Context, queueReconcile func(), sm *batcher.StateMonitor) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	batchedEventsC := r.startBatchingEvents(ctx, r.watcher, sm)

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.watcher.Done():
			return
		case batch, ok := <-batchedEventsC:
			if !ok {
				return
			}
			if batch.State == batcher.StateOverloaded {
				// In overloaded state we skip reconciliation till we exit overloaded state.
				// This means we will reconcile when receiving less than 5 events per 20 seconds.
				// This is done because IneligibleStatus reconciliation is not time-critical, and
				// we want to avoid excessive load on CPU during actions like EntraID group imports
				// where many access list members can be updated is a short period of time.
				continue
			}
			queueReconcile()
		}
	}
}

const (
	maxBatchQueueSize = 10
	batchWindow       = 20 * time.Second
)

type batchWithState struct {
	events []types.Event
	batcher.State
}

func (r *IneligibleStatusReconciler) startBatchingEvents(ctx context.Context, w types.Watcher, sm *batcher.StateMonitor) chan batchWithState {
	batchedEventsC := make(chan batchWithState)
	go func() {
		defer close(batchedEventsC)
		if err := batcher.RunWithState(ctx, w.Events(), func(batch []types.Event, state batcher.State) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case batchedEventsC <- batchWithState{events: batch, State: state}:
				return nil
			}
		}, sm, batcher.WithWindow(batchWindow), batcher.WithThreshold(maxBatchQueueSize), batcher.WithClock(r.clock)); err != nil {
			r.logger.ErrorContext(ctx, "Event batching failed", "error", err)
		}
	}()
	return batchedEventsC
}

// Close closes the reconciler.
func (r *IneligibleStatusReconciler) Close() error {
	return r.watcher.Close()
}

func (r *IneligibleStatusReconciler) reconciliationLoop(ctx context.Context) (nextExpirationTime time.Duration, err error) {
	select {
	case <-ctx.Done():
		return 0, trace.Wrap(ctx.Err())
	default:
	}

	r.logger.DebugContext(ctx, "Reconciling memberships")
	defer func() {
		log := r.logger.With("next_expiration_time", nextExpirationTime)
		if trace.IsCompareFailed(err) {
			log.DebugContext(ctx, "AccessList reconciliation failed due CAS failure, retrying")
			return
		}
		if err != nil {
			r.logger.DebugContext(ctx, "AccessList reconciliation failed", "error", err)
			return
		}
		r.logger.DebugContext(ctx, "AccessList reconciliation complete")
	}()
	// get all users
	users, err := getAllUsers(ctx, r.cache)
	if err != nil {
		return 0, trace.Wrap(err, "unable to get users")
	}
	usersMap := sliceToMap(users)

	accessLists, err := getAllAccessLists(ctx, r.cache)
	if err != nil {
		return 0, trace.Wrap(err, "unable to get access lists")
	}
	// reconcile access list ownership
	if err := r.reconcileAccessListOwnership(ctx, accessLists, usersMap); err != nil {
		return 0, trace.Wrap(err, "unable to reconcile access list ownership")
	}

	nextExpirationTime, err = r.reconcileMemberships(ctx, r.clock.Now(), accessLists, usersMap)
	return nextExpirationTime, trace.Wrap(err, "unable to reconcile memberships")
}

func getAllAccessLists(ctx context.Context, cache Cache) ([]*accesslist.AccessList, error) {
	out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
		return cache.ListAccessListsV2(ctx, accesslistv1.ListAccessListsV2Request_builder{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
			ScopeFilter: scopesv1.Filter_builder{
				Mode: scopesv1.Mode_MODE_ALL,
			}.Build(),
		}.Build())
	}))
	return out, trace.Wrap(err)
}

func (r *IneligibleStatusReconciler) reconcileAccessListOwnership(ctx context.Context, accessLists []*accesslist.AccessList, usersMap map[string]types.User) error {
	var updateAccessLists []*accesslist.AccessList
	for _, accessList := range accessLists {
		var toUpdate bool
		for i, owner := range accessList.Spec.Owners {
			var ineligibleStatus accesslistv1.IneligibleStatus
			if owner.IsMembershipKindList() {
				// ownership requires are not checked for lists
				// they are always eligible
				ineligibleStatus = accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE
			} else {
				ineligibleStatus = checkUserIsStillEligible(StillEligibleFields{
					userLookup: usersMap,
					username:   owner.Name,
					expires:    time.Time{}, // owners don't have expiry's
					clock:      r.clock,
					requires:   accessList.GetOwnershipRequires(),
				})
			}
			oldIneligibleStatus := owner.IneligibleStatus
			owner.IneligibleStatus = accesslistv1.IneligibleStatus_name[int32(ineligibleStatus)]
			if oldIneligibleStatus != owner.IneligibleStatus {
				r.logger.DebugContext(ctx, "Updating access list owner ineligibility status",
					"access_list", accessList.GetName(),
					"owner_name", owner.Name,
					"membership_kind", owner.MembershipKind,
					"old_status", oldIneligibleStatus,
					"new_status", owner.IneligibleStatus,
				)
				toUpdate = true
			}
			accessList.Spec.Owners[i] = owner
		}
		if toUpdate {
			updateAccessLists = append(updateAccessLists, accessList)
		}
	}
	for _, member := range updateAccessLists {
		if err := r.updateAccessList(ctx, member); err != nil {
			return trace.Wrap(err, "unable to update access list member")
		}
	}

	return nil
}

func (r *IneligibleStatusReconciler) reconcileMemberships(ctx context.Context, now time.Time, accessLists []*accesslist.AccessList, usersMap map[string]types.User) (time.Duration, error) {
	accessListsMap := make(map[accesslists.NormalizedSQN]*accesslist.AccessList, len(accessLists))
	for _, acl := range accessLists {
		accessListsMap[accesslists.ScopeQualifiedName(acl)] = acl
	}

	pageFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
		return r.cache.ListAllAccessListMembersV2(ctx, accesslistv1.ListAllAccessListMembersRequest_builder{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
			ScopeFilter: scopesv1.Filter_builder{
				Mode: scopesv1.Mode_MODE_ALL,
			}.Build(),
		}.Build())
	}
	nextExpiration := neverDuration
	for member, err := range clientutils.Resources(ctx, pageFn) {
		if err != nil {
			return 0, trace.Wrap(err, "unable to get access list members")
		}

		parentListName, err := accesslists.ParentListOf(member)
		if err != nil {
			r.logger.WarnContext(ctx, "Failed to parse member.Spec.AccessList",
				"error", err,
				"access_list", member.Spec.AccessList,
			)
			continue
		}
		accessList, ok := accessListsMap[parentListName]
		if !ok {
			r.logger.WarnContext(ctx, "Access list not found", "access_list", member.Spec.AccessList)
			continue
		}

		var ineligibleStatus accesslistv1.IneligibleStatus
		if member.IsList() {
			// membership requires are not checked for lists
			// they are always eligible
			ineligibleStatus = accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE
		} else {
			if _, ok := usersMap[member.Spec.Name]; ok {
				ineligibleStatus = checkUserIsStillEligible(StillEligibleFields{
					userLookup: usersMap,
					username:   member.Spec.Name,
					expires:    member.Spec.Expires,
					clock:      r.clock,
					requires:   accessList.GetMembershipRequires(),
				})
			} else if member.Origin() == common.OriginAWSIdentityCenter {
				// Identity Center originated members who do not have an account in Teleport
				// should always be eligible.
				ineligibleStatus = accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE
			} else {
				ineligibleStatus = accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST
			}
		}

		oldIneligibleStatus := member.Spec.IneligibleStatus
		member.Spec.IneligibleStatus = ineligibleStatus.String()

		nextExpiration = min(nextExpiration, nextExpirationTime(now, member))
		if oldIneligibleStatus == member.Spec.IneligibleStatus {
			continue
		}
		r.logger.DebugContext(ctx, "Updating access list member ineligibility status",
			"access_list", member.Spec.AccessList,
			"member_name", member.Spec.Name,
			"membership_kind", member.Spec.MembershipKind,
			"old_status", oldIneligibleStatus,
			"new_status", member.Spec.IneligibleStatus,
		)
		if err := r.updateMember(ctx, member); err != nil {
			return 0, trace.Wrap(err, "unable to update access list member")
		}
	}
	return nextExpiration, nil
}

func (r *IneligibleStatusReconciler) updateMember(ctx context.Context, member *accesslist.AccessListMember) error {
	_, err := r.service.UpdateAccessListMember(ctx, member)
	if trace.IsNotFound(err) {
		// if the access list member was deleted, we can ignore the error
		return nil
	}
	return trace.Wrap(err)
}

func (r *IneligibleStatusReconciler) updateAccessList(ctx context.Context, a *accesslist.AccessList) error {
	_, err := r.service.UpdateAccessList(ctx, a)
	if trace.IsNotFound(err) {
		// if the access list was deleted, we can ignore the error
		return nil
	}
	return trace.Wrap(err)
}

func sliceToMap[T interface{ GetName() string }](slice []T) map[string]T {
	m := make(map[string]T, len(slice))
	for _, item := range slice {
		m[item.GetName()] = item
	}
	return m
}

// nextExpirationTime returns the time when the next access list member expires across all access lists.
// If no access list members expire, it returns a duration of 100 years.
func nextExpirationTime(now time.Time, member *accesslist.AccessListMember) time.Duration {
	expires := now.Add(neverDuration)
	if !member.Spec.Expires.IsZero() &&
		member.Spec.Expires.After(now) &&
		member.Spec.Expires.Before(expires) {
		expires = member.Spec.Expires
	}
	return expires.Sub(now)
}
