package accesslist

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
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
	GetAccessListMember(ctx context.Context, accessList string, name string) (*accesslist.AccessListMember, error)
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
	GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error)
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	ListAccessListMembers(ctx context.Context, accessList string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error)
	CountAccessListMembers(ctx context.Context, accessList string) (uint32, uint32, error)
}

// NewIneligibleStatusReconciler creates a new IneligibleStatusReconciler.
func NewIneligibleStatusReconciler(ctx context.Context, cfg IneligibleStatusReconcilerConfig) (*IneligibleStatusReconciler, error) {
	watcher, err := cfg.Cache.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{
				Kind: types.KindUser,
			},
			{
				Kind: types.KindAccessList,
			},
			{
				Kind: types.KindAccessListMember,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := waitForInitOp(watcher); err != nil {
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

func waitForInitOp(watcher types.Watcher) error {
	for {
		select {
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
	drainAndResetTimer := func(t clockwork.Timer, timeout time.Duration) {
		if !t.Stop() {
			// drain the channel if it's not empty
			// draining an already fired timer results in a deadlock
			// so we need to wrap it in a select
			select {
			case <-t.Chan():
			default:
			}
		}
		t.Reset(timeout)
	}

	t := r.clock.NewTimer(neverDuration)
	// forceReconcile is a timer that will force a reconciliation to happen
	// after a certain amount of time has passed.
	forceReconcile := r.clock.NewTimer(forceReconcileDuration)
	// reconcile is a blocking channel that will be used to signal that
	// a reconciliation must happen.
	// It must be unbuffered to ensure that we don't re-reconcile if we
	// receive multiple signals to reconcile during the same reconciliation.
	// The reconciliation loop will wait for a signal to reconcile, and then
	// reconcile, and then wait for the next signal.
	reconcile := make(chan struct{})
	go r.informReconciliationMustHappen(reconcile)
	for {
		now := r.clock.Now()
		nextExpirationTime, err := r.reconciliationLoop(ctx, now)
		if trace.IsCompareFailed(err) {
			// retry on compare failed errors
			// this means that the membership was updated while we were processing
			// the list, so we need to reprocess it slilghtly.
			// we sleep for a bit to avoid spinning too much
			// when we get a lot of these errors because of a lot of updates.
			const waitTime = 15 * time.Second
			select {
			case <-r.clock.After(waitTime):
				continue
			case <-r.watcher.Done():
				return nil
			}
		} else if err != nil {
			r.logger.WarnContext(ctx, "Failed to reconcile memberships", "error", err)
		}

		if nextExpirationTime <= 0 {
			nextExpirationTime = 1 * time.Second
		}
		drainAndResetTimer(t, nextExpirationTime)
		drainAndResetTimer(forceReconcile, forceReconcileDuration)
		select {
		case <-ctx.Done():
			return nil
		case <-r.watcher.Done():
			return trace.Wrap(r.watcher.Error())
		case <-reconcile:
		case <-t.Chan():
		case <-forceReconcile.Chan():
		}

	}
}

// Close closes the reconciler.
func (r *IneligibleStatusReconciler) Close() error {
	return r.watcher.Close()
}

func (r *IneligibleStatusReconciler) reconciliationLoop(ctx context.Context, now time.Time) (nextExpirationTime time.Duration, err error) {
	r.logger.DebugContext(ctx, "Reconciling memberships")
	defer func() {
		r.logger.DebugContext(ctx, "AccessList reconciliation complete", "next_expiration_time", nextExpirationTime, "error", err)
	}()
	// get all users
	users, err := getAllUsers(ctx, r.cache, 0 /* use the default page size */)
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

	nextExpirationTime, err = r.reconcileMemberships(ctx, now, accessLists, usersMap)
	return nextExpirationTime, trace.Wrap(err, "unable to reconcile memberships")
}

func getAllAccessLists(ctx context.Context, cache Cache) ([]*accesslist.AccessList, error) {
	var accessLists []*accesslist.AccessList
	startToken := ""
	for {
		batch, nextToken, err := cache.ListAccessLists(ctx, 0 /* default pageSize */, startToken)
		if err != nil {
			return nil, trace.Wrap(err, "unable to get access lists")
		}
		accessLists = append(accessLists, batch...)
		if nextToken == "" {
			break
		}
		startToken = nextToken
	}
	return accessLists, nil
}

func (r *IneligibleStatusReconciler) reconcileAccessListOwnership(ctx context.Context, accessLists []*accesslist.AccessList, usersMap map[string]types.User) error {
	var updateAccessLists []*accesslist.AccessList
	for _, accessList := range accessLists {
		var toUpdate bool
		for i, owner := range accessList.Spec.Owners {
			if owner.MembershipKind == accesslist.MembershipKindList {
				// we don't need to check the ineligibility status of owner lists
				continue
			}
			ineligibleStatus := checkUserIsStillEligible(StillEligibleFields{
				userLookup: usersMap,
				username:   owner.Name,
				expires:    time.Time{}, // owners don't have expiry's
				clock:      r.clock,
				requires:   accessList.GetOwnershipRequires(),
			})
			oldIneligibleStatus := owner.IneligibleStatus
			owner.IneligibleStatus = accesslistv1.IneligibleStatus_name[int32(ineligibleStatus)]
			if oldIneligibleStatus != owner.IneligibleStatus {
				r.logger.DebugContext(ctx, "Updating access list owner ineligibility status",
					"access_list", accessList.GetName(),
					"username", owner.Name,
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
	accessListsMap := sliceToMap(accessLists)

	startKey := ""
	nextExpiration := neverDuration
	for {
		accessListsMembers, nextKey, err := r.cache.ListAllAccessListMembers(ctx, 0 /* default pageSize */, startKey)
		if err != nil {
			return 0, trace.Wrap(err, "unable to get access list members")
		}

		var toUpdate []*accesslist.AccessListMember
		for _, member := range accessListsMembers {
			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				// we don't need to check the ineligibility status of member lists
				continue
			}
			accessList, ok := accessListsMap[member.Spec.AccessList]
			if !ok {
				r.logger.WarnContext(ctx, "Access list not found", "access_list", member.Spec.AccessList)
				continue
			}
			ineligibleStatus := checkUserIsStillEligible(StillEligibleFields{
				userLookup: usersMap,
				username:   member.Spec.Name,
				expires:    member.Spec.Expires,
				clock:      r.clock,
				requires:   accessList.GetMembershipRequires(),
			})
			oldIneligibleStatus := member.Spec.IneligibleStatus
			member.Spec.IneligibleStatus = ineligibleStatus.String()
			if oldIneligibleStatus != member.Spec.IneligibleStatus {
				r.logger.DebugContext(ctx, "Updating access list member ineligibility status",
					"access_list", member.Spec.AccessList,
					"username", member.Spec.Name,
					"old_status", oldIneligibleStatus,
					"new_status", member.Spec.IneligibleStatus,
				)
				toUpdate = append(toUpdate, member)
			}
		}
		for _, member := range toUpdate {
			if err := r.updateMember(ctx, member); err != nil {
				return 0, trace.Wrap(err, "unable to update access list member")
			}
		}

		batchNextExpirationTime := nextExpirationTime(now, accessListsMembers)
		nextExpiration = min(nextExpiration, batchNextExpirationTime)
		if nextKey == "" {
			break
		}
		startKey = nextKey
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

// StillEligibleFields holds the fields required to check if a user is still eligible.
func sliceToMap[T interface{ GetName() string }](slice []T) map[string]T {
	m := make(map[string]T, len(slice))
	for _, item := range slice {
		m[item.GetName()] = item
	}
	return m
}

// nextExpirationTime returns the time when the next access list member expires across all access lists.
// If no access list members expire, it returns a duration of 100 years.
func nextExpirationTime(now time.Time, accessListMembers []*accesslist.AccessListMember) time.Duration {
	expires := now.Add(neverDuration)
	for _, member := range accessListMembers {
		if !member.Spec.Expires.IsZero() &&
			member.Spec.Expires.After(now) &&
			member.Spec.Expires.Before(expires) {
			expires = member.Spec.Expires
		}
	}
	return expires.Sub(now)
}

func (r *IneligibleStatusReconciler) informReconciliationMustHappen(reconcile chan<- struct{}) {
	for {
		select {
		case <-r.watcher.Events():
			// if we receive an event from the watcher, we should reconcile.
			// Often, when we reconcile several resources, we may not have
			// the reconciliation loop is still running, so we might batch
			// the reconciliation requests to avoid unnecessary work.
			// This is a signal to the reconciliation loop that it should
			// reconcile.
		innerLoop:
			for {
				select {
				// if we receive an event from the watcher, but the reconcile isn't ready,
				// we should wait for the reconcile to be ready and discard the events
				// that we received in the meantime if we ensure that we will send the signal
				// to reconcile once the reconcile is ready.
				case <-r.watcher.Events():
				case reconcile <- struct{}{}:
					break innerLoop
				case <-r.watcher.Done():
					return
				}
			}
		case <-r.watcher.Done():
			return
		}
	}
}
