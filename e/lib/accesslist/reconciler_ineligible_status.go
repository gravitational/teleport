package accesslist

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

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
	// Log is the logger.
	Log logrus.FieldLogger
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
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error)
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
		log:     cfg.Log,
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
	log     logrus.FieldLogger
}

const (
	// 100 years is a good enough approximation for "never"
	neverDuration = time.Hour * 24 * 365 * 100
)

// Run runs the reconciliation loop.
func (r *IneligibleStatusReconciler) Run(ctx context.Context) error {
	t := r.clock.NewTimer(neverDuration)
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
			r.log.WithError(err).Warn("Failed to reconcile memberships")
		}

		if nextExpirationTime <= 0 {
			nextExpirationTime = 1 * time.Second
		}
		if !t.Stop() {
			<-t.Chan()
		}
		t.Reset(nextExpirationTime)
		select {
		case <-ctx.Done():
			return nil
		case <-r.watcher.Done():
			return trace.Wrap(r.watcher.Error())
		case <-reconcile:
		case <-t.Chan():
		}

	}

}

// Close closes the reconciler.
func (r *IneligibleStatusReconciler) Close() error {
	return r.watcher.Close()
}

func (r *IneligibleStatusReconciler) reconciliationLoop(ctx context.Context, now time.Time) (time.Duration, error) {
	// get all access lists
	accessLists, err := r.cache.GetAccessLists(ctx)
	if err != nil {
		return 0, trace.Wrap(err, "unable to get access lists")
	}

	// get all users
	users, err := getAllUsers(ctx, r.cache, 0 /* use the default page size */)
	if err != nil {
		return 0, trace.Wrap(err, "unable to get users")
	}
	usersMap := sliceToMap(users)

	// reconcile access list ownership
	if err := r.reconcileAccessListOwnership(ctx, accessLists, usersMap); err != nil {
		return 0, trace.Wrap(err, "unable to reconcile access list ownership")
	}

	accessListsMap := sliceToMap(accessLists)
	nextExpirationTime, err := r.reconcileMemberships(ctx, now, accessListsMap, usersMap)
	return nextExpirationTime, trace.Wrap(err, "unable to reconcile memberships")
}

func (r *IneligibleStatusReconciler) reconcileAccessListOwnership(ctx context.Context, accessLists []*accesslist.AccessList, usersMap map[string]types.User) error {
	var updateAccessLists []*accesslist.AccessList
	for _, accessList := range accessLists {
		var toUpdate bool
		for i, owner := range accessList.Spec.Owners {
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

func (r *IneligibleStatusReconciler) reconcileMemberships(ctx context.Context, now time.Time, accessListsMap map[string]*accesslist.AccessList, usersMap map[string]types.User) (time.Duration, error) {

	accessListsMembers, err := listAllAccessListsMembers(ctx, r.cache)
	if err != nil {
		return 0, trace.Wrap(err, "unable to get access list members")
	}

	var toUpdate []*accesslist.AccessListMember
	for _, member := range accessListsMembers {
		accessList, ok := accessListsMap[member.Spec.AccessList]
		if !ok {
			r.log.WithField("access_list", member.Spec.AccessList).Warn("Access list not found")
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
		member.Spec.IneligibleStatus = accesslistv1.IneligibleStatus_name[int32(ineligibleStatus)]
		if oldIneligibleStatus != member.Spec.IneligibleStatus {
			toUpdate = append(toUpdate, member)
		}
	}
	for _, member := range toUpdate {
		if err := r.updateMember(ctx, member); err != nil {
			return 0, trace.Wrap(err, "unable to update access list member")
		}
	}

	return nextExpirationTime(now, accessListsMembers), nil
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

func listAllAccessListsMembers(ctx context.Context, accessListsClient Cache) ([]*accesslist.AccessListMember, error) {
	var startKey = ""
	var allMembers []*accesslist.AccessListMember
	for {
		members, nextKey, err := accessListsClient.ListAllAccessListMembers(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err, "unable to get access list members")
		}
		allMembers = append(allMembers, members...)
		if nextKey == "" {
			break
		}
		startKey = nextKey
	}
	return allMembers, nil
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
