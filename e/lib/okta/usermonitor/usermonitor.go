package usermonitor

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/api/utils/retryutils"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	userMonitorRetryPeriod = 5 * time.Second

	// userMonitorReconcile is when to reconcile all user login states in Teleport.
	userMonitorReconcile = 10 * time.Minute
	// userMonitorLockTTL is the TTL for the user monitor lock.
	userMonitorLockTTL = 3 * time.Minute
)

// Config is the configuration for the user monitor.
type Config struct {
	// Logger is the logger for the user monitor.
	Logger *slog.Logger

	// Clock is the click used for the user monitor.
	Clock clockwork.Clock

	// AuthServer is the parent auth server for the user monitor.
	AuthServer *auth.Server

	// Events is the event monitor. This will allow us to monitor for access list membership
	// and user definition changes. Events can create watchers from the cache
	// so it is safe to fetch resources both from cache or backend when
	// reacting to its events.
	Events types.Events
	// Backend is the backend used for locking to ensure only one user monitor
	Backend backend.Backend
	// ReconcileInterval is the interval for the periodic reconciliation of user states.
	ReconcileInterval time.Duration
	// LockTTL is the TTL for the user monitor lock.
	LockTTL time.Duration
}

func (u *Config) CheckAndSetDefaults() error {
	if u.Logger == nil {
		u.Logger = slog.With(teleport.ComponentKey, eteleport.ComponentUserMonitor)
	}

	if u.Clock == nil {
		u.Clock = clockwork.NewRealClock()
	}

	if u.AuthServer == nil {
		return trace.BadParameter("auth server is missing")
	}

	if u.Events == nil {
		return trace.BadParameter("events is missing")
	}

	if u.Backend == nil {
		return trace.BadParameter("backend is missing")
	}
	if u.ReconcileInterval == 0 {
		u.ReconcileInterval = userMonitorReconcile
	}
	if u.LockTTL == 0 {
		u.LockTTL = userMonitorLockTTL
	}

	return nil
}

// UserMonitor is a service that must run on the auth service that monitors for
// user changes:
// - User changes
// - Role changes
// - Access list changes
// - Access list membership changes.
// - User locks and lock deletions.
// The user login hooks attached to the auth service will be re-run, which will allow
// for dynamic changing of things like Okta assignments. This monitor must be run on
// the auth server.
type UserMonitor struct {
	logger     *slog.Logger
	clock      clockwork.Clock
	authServer *auth.Server
	events     types.Events

	// locksToTarget keeps tracks of which locks map to which targets. This allows us to
	// react quickly when a lock deletion occurs, as we may otherwise have no context
	// as to what the lock was actually doing.
	lockToTargetMu sync.Mutex
	lockToTarget   map[string]types.LockTarget
	backend        backend.Backend

	reconcileInterval time.Duration
	lockTTL           time.Duration
}

func New(cfg Config) (*UserMonitor, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	u := &UserMonitor{
		logger:            cfg.Logger,
		clock:             cfg.Clock,
		authServer:        cfg.AuthServer,
		events:            cfg.Events,
		lockToTarget:      map[string]types.LockTarget{},
		backend:           cfg.Backend,
		reconcileInterval: cfg.ReconcileInterval,
		lockTTL:           cfg.LockTTL,
	}

	return u, nil
}

// Start will start the user monitor.
func (u *UserMonitor) Start(ctx context.Context) {
	go u.run(ctx)
}
func (u *UserMonitor) run(ctx context.Context) {
	// The interval is half of the TTL capped to 1m.
	lockInterval := min(u.lockTTL/2, time.Minute)

	runWhileLockedConfig := backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            u.backend,
			LockNameComponents: []string{"auth", "user-monitor"},
			TTL:                u.lockTTL,
			RetryInterval:      lockInterval,
		},
		RefreshLockInterval: lockInterval,
	}

	waitWithJitter := retryutils.SeventhJitter(lockInterval)

	for {
		err := backend.RunWhileLocked(ctx, runWhileLockedConfig, func(ctx context.Context) error {
			var wg sync.WaitGroup
			wg.Go(func() {
				u.runWatcher(ctx)
			})
			u.reconciler(ctx)
			wg.Wait()
			return nil
		})
		if err != nil {
			if ctx.Err() != nil {
				// Just return, context is canceled.
				return
			}
			u.logger.DebugContext(ctx,
				"User Monitor syncer encountered an error, it will restart after backoff",
				"error", err,
				"restart_after", waitWithJitter,
			)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(waitWithJitter):
		}
	}
}

// reconciler will periodically reconcile user states.
func (u *UserMonitor) reconciler(ctx context.Context) {
	interval := interval.New(interval.Config{
		Duration:      u.reconcileInterval,
		FirstDuration: time.Second,
		Clock:         u.clock,
		Jitter:        retryutils.SeventhJitter,
	})
	defer interval.Stop()

	for {
		u.logger.InfoContext(ctx, "Reconciling users")

		if err := u.reconcile(ctx); err != nil {
			u.logger.DebugContext(ctx, "Error during reconciliation", "error", err)
		}

		select {
		case <-interval.Next():
		case <-ctx.Done():
			return
		}
	}
}

func (u *UserMonitor) reconcile(ctx context.Context) error {
	// Get all locks and rebuild the lock to target map.
	locks, err := u.authServer.GetLocks(ctx, true)
	if err != nil {
		return trace.Wrap(err)
	}

	u.lockToTargetMu.Lock()
	u.lockToTarget = map[string]types.LockTarget{}
	for _, lock := range locks {
		if !isLockSupported(lock.Target()) {
			continue
		}
		u.lockToTarget[lock.GetName()] = lock.Target()
	}
	u.lockToTargetMu.Unlock()

	users, err := u.authServer.GetUsers(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	maxUsers := len(users)

	states, err := u.authServer.UserLoginStates.GetUserLoginStates(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	numStates := len(states)
	if numStates > maxUsers {
		maxUsers = numStates
	}

	// We'll assume that the maximum length between get users and get user login states
	// is the maximum number of users we'll see.
	usersToProcess := make(map[string]types.User, maxUsers)
	for _, user := range users {
		if user.IsBot() {
			// Do not attempt to reconcile bots, they are always local users and
			// cannot self-heal invalid user_login_state entries that might get
			// created.
			continue
		}

		usersToProcess[user.GetName()] = user
	}

	// For each state, see if there's already a user present. If so, skip it. Otherwise, attempt
	// to rebuild the user state.
	for _, uls := range states {
		if _, ok := usersToProcess[uls.GetName()]; ok {
			continue
		}
		if _, isBot := uls.GetLabel(types.BotLabel); isBot {
			// As above, do not attempt to reconcile bots, even if they have an
			// existing (invalid) ULS.
			continue
		}

		rebuilt, err := rebuildUserFromUserLoginState(uls)
		if err != nil {
			u.logger.DebugContext(ctx, "Unable to rebuild user", "user", uls.GetName(), "error", err)
			continue
		}
		usersToProcess[uls.GetName()] = rebuilt
	}

	var processErrs []error
	for _, user := range usersToProcess {
		if err := u.processUserChange(ctx, user); err != nil {
			processErrs = append(processErrs, err)
		}
	}

	return trace.NewAggregate(processErrs...)
}

// runWatcher will watch events and retry on failure until the context is canceled.
func (u *UserMonitor) runWatcher(ctx context.Context) {
	for {
		err := u.watchEvents(ctx)
		if ctx.Err() != nil {
			return
		}

		u.logger.WarnContext(ctx, "Watcher closed, backing off before creating another watcher", "error", err, "backoff_interval", userMonitorRetryPeriod)

		select {
		case <-u.clock.After(userMonitorRetryPeriod):
		case <-ctx.Done():
			return
		}
	}
}

// newWatcher will create a new watcher.
func (u *UserMonitor) newWatcher(ctx context.Context) (types.Watcher, error) {
	watcher, err := u.events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindUser},
			{Kind: types.KindRole},
			{Kind: types.KindAccessListMember},
			{Kind: types.KindAccessList},
			{Kind: types.KindLock},
		},
	})
	return watcher, trace.Wrap(err)
}

// watchEvents will watch events from a newly created watcher.
func (u *UserMonitor) watchEvents(ctx context.Context) error {
	watcher, err := u.newWatcher(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// We don't worry about the init event here, as we won't process it.
	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init operation on start, got %s", event.Type.String())
		}
	case <-watcher.Done():
		return watcher.Error()
	}

	for {
		select {
		case event := <-watcher.Events():
			if err := u.processResource(ctx, event.Resource, event.Type); err != nil {
				u.logger.DebugContext(ctx, "Error while processing events", "error", err)
			}
		case <-watcher.Done():
			return watcher.Error()
		}
	}
}

func (u *UserMonitor) processResource(ctx context.Context, resource types.Resource, op types.OpType) error {
	// in processResource and all the other UserMonitor functions, we can
	// safely read resources like users, ULS, roles and accesslists from the
	// cache because the UserMonitor reacts to events from the cache.
	if resource == nil {
		return trace.BadParameter("resource is empty")
	}

	if op != types.OpDelete && op != types.OpPut {
		return trace.BadParameter("only modification operations are supported")
	}

	switch resource.GetKind() {
	case types.KindUser:
		if op == types.OpPut {
			user, ok := resource.(types.User)
			if !ok {
				return trace.BadParameter("got resource %T, expected User", resource)
			}
			return trace.Wrap(u.processUserChange(ctx, user))
		} else {
			return trace.Wrap(u.rebuildAndProcessUser(ctx, resource.GetName()))
		}
	case types.KindRole:
		return trace.Wrap(u.processRoleChange(ctx, resource.GetName()))
	case types.KindAccessListMember:
		if err := u.rebuildAndProcessUser(ctx, resource.GetName()); err != nil {
			if trace.IsNotFound(err) && isAWSIdentityCenterOriginated(resource.GetMetadata().Labels) {
				return nil
			}
			return trace.Wrap(err)
		}
	case types.KindAccessList:
		// Delete isn't needed because the individual removals of the access list members will be handled by the monitor
		// individually.
		if op == types.OpDelete {
			return nil
		}

		accessList, ok := resource.(*accesslist.AccessList)
		if !ok {
			return trace.BadParameter("got resource %T, expected AccessList", resource)
		}

		return trace.Wrap(u.processAccessListChange(ctx, accessList))
	case types.KindLock:
		return trace.Wrap(u.processLock(ctx, resource, op))
	}

	return nil
}

// processUserChange will re-process a user by re-calling the login hooks for the user.
func (u *UserMonitor) processUserChange(ctx context.Context, user types.User) error {
	if user.IsBot() {
		// Do not attempt to process changes for bot users. This can result in
		// creation of invalid user_login_state resources which (unlike local
		// users) are never repaired, since bots never authenticate in a way
		// that trigger the ULS generator.
		return nil
	}

	if err := u.authServer.CallLoginHooks(ctx, user); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// processRoleChange will re-process any users affected by a role change.
func (u *UserMonitor) processRoleChange(ctx context.Context, roleName string) error {
	users, err := u.authServer.GetUsers(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	// Process all users that have this role defined.
	var allErrs []error
	for _, user := range users {
		if slices.Contains(user.GetRoles(), roleName) {
			if err := u.processUserChange(ctx, user); err != nil {
				allErrs = append(allErrs, err)
			}
		}
	}

	// Process all access lists which grant the changed role.
	var nextToken string
	for {
		var accessLists []*accesslist.AccessList
		var err error
		accessLists, nextToken, err = u.authServer.AccessListsInternal.ListAccessLists(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			allErrs = append(allErrs, err)
			break
		}

		for _, accessList := range accessLists {
			if slices.Contains(accessList.Spec.Grants.Roles, roleName) {
				if err := u.processAccessListChange(ctx, accessList); err != nil {
					allErrs = append(allErrs, err)
				}
			}
		}

		if nextToken == "" {
			break
		}
	}

	return trace.NewAggregate(allErrs...)
}

// rebuildAndProcessUser will process a user by rebuilding the user if necessary and processing it.
func (u *UserMonitor) rebuildAndProcessUser(ctx context.Context, name string) error {
	uls, err := u.authServer.GetUserLoginState(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, isBot := uls.GetLabel(types.BotLabel); isBot {
		// Do not attempt to rebuild users for bots. Note that if a corrupt ULS
		// entry for a bot user already exists in the backend, its bot label
		// will be missing, so IsBot() will always return false and this check
		// will not skip it. In this case, GetUserLoginState() is responsible
		// for discarding existing broken ULS entries on read.
		return nil
	}

	user, err := rebuildUserFromUserLoginState(uls)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(u.processUserChange(ctx, user))
}

// rebuildUserFromUserLoginState will attempt to create a user object from the user login state if the user login state
// represents an SSO user. Otherwise this will return an empty user.
func rebuildUserFromUserLoginState(uls *userloginstate.UserLoginState) (types.User, error) {
	user, err := types.NewUser(uls.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if uls.GetUserType() == types.UserTypeSSO {
		user.SetRoles(uls.GetOriginalRoles())
		user.SetTraits(uls.GetOriginalTraits())

		// This will ensure that user login state generated from this user will
		// also be set to type SSO.
		user.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{},
		})
	}

	return user, nil
}

// processAccessListChange will process all users affected by a change to an access list.
func (u *UserMonitor) processAccessListChange(ctx context.Context, accessList *accesslist.AccessList) error {
	// Iterate through all of the access list members.
	var nextToken string
	for {
		var members []*accesslist.AccessListMember
		var err error
		members, nextToken, err = u.authServer.AccessListsInternal.ListAccessListMembers(ctx, accessList.GetName(), 0 /* default page size */, nextToken)
		if err != nil {
			return trace.BadParameter("error while getting access list members for access list %s", accessList.GetName())
		}

		for _, member := range members {
			if err := u.rebuildAndProcessUser(ctx, member.GetName()); err != nil {
				if trace.IsNotFound(err) && isAWSIdentityCenterOriginated(member.GetMetadata().Labels) {
					continue
				} else {
					return trace.Wrap(err)
				}
			}
		}

		if nextToken == "" {
			break
		}
	}

	return nil
}

func (u *UserMonitor) processLock(ctx context.Context, resource types.Resource, op types.OpType) error {
	target, valid, err := u.getLockTarget(resource, op)
	if err != nil {
		return trace.Wrap(err)
	}

	if !valid {
		return nil
	}

	return trace.Wrap(u.processLockTarget(ctx, target))
}

// getLockTarget will return a lock target either from the given event or from the lock to target mapping. It will
// return a target, whether the target is valid, and an error. This function will also manage the lock to target
// mapping, which will add to the mapping when op is OpPut and delete from the mapping when op is OpDelete.
func (u *UserMonitor) getLockTarget(resource types.Resource, op types.OpType) (types.LockTarget, bool, error) {
	if op == types.OpPut {
		lock, ok := resource.(types.Lock)
		if !ok {
			return types.LockTarget{}, false, trace.BadParameter("got resource %T, expected Lock", resource)
		}

		target := lock.Target()

		if !isLockSupported(target) {
			return types.LockTarget{}, false, nil
		}

		// Record which user the lock is targeting.
		u.lockToTargetMu.Lock()
		u.lockToTarget[lock.GetName()] = lock.Target()
		u.lockToTargetMu.Unlock()

		return target, true, nil
	}

	// Delete the lock mapping.
	u.lockToTargetMu.Lock()
	target, ok := u.lockToTarget[resource.GetName()]
	delete(u.lockToTarget, resource.GetName())
	u.lockToTargetMu.Unlock()

	// No supported lock target was found here.
	if !ok {
		return types.LockTarget{}, false, nil
	}

	return target, true, nil
}

func (u *UserMonitor) processLockTarget(ctx context.Context, target types.LockTarget) error {
	lockProcessed := false
	var errs []error
	if target.User != "" {
		lockProcessed = true
		if err := u.rebuildAndProcessUser(ctx, target.User); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}
	if target.Role != "" {
		lockProcessed = true
		if err := u.processRoleChange(ctx, target.Role); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	if target.AccessRequest != "" {
		lockProcessed = true
		accessRequest, err := services.GetAccessRequest(ctx, u.authServer.DynamicAccessExt, target.AccessRequest)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
		} else {
			if err := u.rebuildAndProcessUser(ctx, accessRequest.GetUser()); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if !lockProcessed {
		return trace.BadParameter("unsupported lock targets")
	}

	return trace.NewAggregate(errs...)
}

// isLockSupported will return true if the lock type is supported.
func isLockSupported(target types.LockTarget) bool {
	switch {
	case target.User != "":
		return true
	case target.Role != "":
		return true
	case target.AccessRequest != "":
		return true
	}

	return false
}

// isAWSIdentityCenterOriginated checks for label value containing OriginAWSIdentityCenter.
func isAWSIdentityCenterOriginated(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	if originLabelValue, ok := labels[types.OriginLabel]; ok {
		return originLabelValue == common.OriginAWSIdentityCenter
	}

	return false
}
