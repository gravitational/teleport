package provisioning

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"maps"
	"math"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/retryutils"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	provisioningComponent          = "PROV"
	defaultPageSize                = 100
	defaultUserPageSize            = defaultPageSize
	defaultUpdateAttempts          = 5
	defaultProvisioningInterval    = 5 * time.Minute
	defaultStateSyncInterval       = 2 * time.Minute
	defaultEventBufferSize         = 128
	defaultProvisioningConcurrency = 8
)

// stateMap defines a mapping of provisioning state IDs to the states they
// represent. Used for brevity.
type stateMap map[services.ProvisioningStateID]*provisioningv1.PrincipalState

// UsersService defines the subset of services.UsersService that the
// provisioning system actually uses.
type UsersService interface {
	ListUsers(ctx context.Context, req *usersv1.ListUsersRequest) (*usersv1.ListUsersResponse, error)
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
}

// AccessListsService defines the subset of services.AccessListsService that the
// provisioning system actually uses.
type AccessListsService interface {
	ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error)
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	ListAccessListMembers(ctx context.Context, accessList string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}

// UserPredicate is a filter function for identifying users to provision
// downstream.
type UserPredicate func(types.User) bool

// AccessListPredicate is a filter function for identifying Access Lists to
// provision downstream
type AccessListPredicate func(*accesslist.AccessList) bool

type ServiceConfig struct {
	// SCIMClient is the SCIM client implementation the provisioning system will
	// use to interact with the downstream server.
	SCIMClient scimsdk.Client

	// UsersCache is the users service used by the provisioning service. The
	// provisioning service only reads from this service, so a cached service
	// is appropriate.
	UsersCache UsersService

	// AccessListsCache is the AccessLists service read by the provisioning
	// service. The provisioning service only reads from this service, so a
	// cached service is appropriate.
	AccessListsCache AccessListsService

	// Locks provides read-only access to the system locks service. Used to
	// verify users are in good standing before provisioning them downstream.
	Locks services.LockGetter

	// DownstreamID selects which Provisioning Principal State records to read.
	DownstreamID services.DownstreamID

	// StateSvc is a reference to the Provisioning Principal State CRUD service
	// used by the provisioning service. The provisioning service needs read/write
	// access, so this should be a primary CRUD service
	StateSvc services.DownstreamProvisioningStates

	// StateSvcCache is a reference to a Provisioning Principal State CRUD service
	// that can be used as like a cache, when quick reads are important
	StateSvcCache services.DownstreamProvisioningStateGetter

	// UserPredicate is a function used to select which users are provisioned
	// downstream. Returns `true` if the user should be provisioned downstream.
	// Defaults to including ALL non-system Users.
	UserPredicate UserPredicate

	// AccessListPredicate is a function used to select which access lists are
	// provisioned downstream. Returns `true` if the given access list should be
	// provisioned downstream. Defaults to including ALL access lists.
	AccessListPredicate AccessListPredicate

	// EventsClient is used to hook into the eventing system to create resource
	// watchers.
	EventsClient types.Events

	// Logger is the slog logger instance to receive log output from the
	// provisioning service
	Logger *slog.Logger

	// Clock is the service time source. Defaults to the system clock if not
	// specified.
	Clock clockwork.Clock

	// ProvisioningConcurrency sets the upper bound on how many principals can
	// be provisioned at once. Defaults to `defaultProvisioningConcurrency` if
	// not set
	ProvisioningConcurrency int

	// StateRefreshInterval sets the interval between full state refreshes, which
	// scans the User and AccessList services for changes that require
	// provisioning. Defaults to defaultStateRefreshInterval if not set.
	StateRefreshInterval time.Duration

	// EventBufferSize is the number of provisioning events to buffer between
	// the resource monitors and the provisioner. Defaults to
	// `defaultEventBufferSize` if unset.
	EventBufferSize int
}

func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if cfg.SCIMClient == nil {
		return trace.BadParameter("must supply a configured SCIM client")
	}

	if cfg.DownstreamID == services.DownstreamID("") {
		return trace.BadParameter("must supply downstream state service")
	}

	if cfg.StateSvc == nil {
		return trace.BadParameter("must supply provisioning state service")
	}

	if cfg.StateSvcCache == nil {
		return trace.BadParameter("must supply provisioning state cache service")
	}

	if cfg.UsersCache == nil {
		return trace.BadParameter("must supply user listing service")
	}

	if cfg.AccessListsCache == nil {
		return trace.BadParameter("must supply access lists service")
	}

	if cfg.Locks == nil {
		return trace.BadParameter("must supply locks service")
	}

	if cfg.EventsClient == nil {
		return trace.BadParameter("must supply events")
	}

	if cfg.UserPredicate == nil {
		cfg.UserPredicate = func(u types.User) bool {
			return !types.IsSystemResource(u)
		}
	}

	if cfg.AccessListPredicate == nil {
		cfg.AccessListPredicate = func(*accesslist.AccessList) bool { return true }
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, provisioningComponent)
	}

	if cfg.EventBufferSize == 0 {
		cfg.EventBufferSize = defaultEventBufferSize
	}

	if cfg.StateRefreshInterval == 0 {
		cfg.StateRefreshInterval = defaultStateSyncInterval
	}

	if cfg.ProvisioningConcurrency == 0 {
		cfg.ProvisioningConcurrency = defaultProvisioningConcurrency
	}

	return nil
}

type Service struct {
	downstreamID        services.DownstreamID
	stateSvc            services.DownstreamProvisioningStates
	stateSvcCache       services.DownstreamProvisioningStateGetter
	usersSvcCache       UsersService
	userPredicate       UserPredicate
	assessListsSvcCache AccessListsService
	accessListPredicate AccessListPredicate
	eventsClient        types.Events
	log                 *slog.Logger
	clock               clockwork.Clock
	eventsSvc           types.Events

	provisioner *provisioner

	// eventsChan receives event notifications telling the provisioner what to
	// provision
	eventsChan chan *provisioningEvent

	// maintains a map of locks to their target users
	lockCache utils.SyncMap[string, services.ProvisioningStateID]

	// interval between full user and access list scans
	stateRefreshInterval time.Duration

	// fullRefreshSignal is used to signal that the service should do a full
	// refresh. The fullRefreshSignal channel has a 1-item buffer, so if you
	// can't write to it without blocking then an update is already queued
	// and the refresh routine hasn't picked it up yet.
	fullRefreshSignal chan struct{}
}

func NewService(cfg ServiceConfig) (svc *Service, err error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provisioner, err := newProvisioner(provisionerConfig{
		scimClient:     cfg.SCIMClient,
		log:            cfg.Logger,
		stateSvc:       cfg.StateSvc,
		usersSvc:       cfg.UsersCache,
		accessListsSvc: cfg.AccessListsCache,
		locksSvc:       cfg.Locks,
		maxConcurrency: cfg.ProvisioningConcurrency,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating downstream provisioner")
	}

	svc = &Service{
		downstreamID:         cfg.DownstreamID,
		stateSvc:             cfg.StateSvc,
		stateSvcCache:        cfg.StateSvcCache,
		usersSvcCache:        cfg.UsersCache,
		userPredicate:        cfg.UserPredicate,
		assessListsSvcCache:  cfg.AccessListsCache,
		accessListPredicate:  cfg.AccessListPredicate,
		eventsClient:         cfg.EventsClient,
		eventsSvc:            cfg.EventsClient,
		log:                  cfg.Logger,
		provisioner:          provisioner,
		clock:                cfg.Clock,
		stateRefreshInterval: cfg.StateRefreshInterval,
		eventsChan:           make(chan *provisioningEvent, cfg.EventBufferSize),
		fullRefreshSignal:    make(chan struct{}, 1),
	}

	provisioner.externalIDCache = svc

	return svc, nil
}

// Runs the provisioning service, blocking until it exits
func (svc *Service) Run(ctx context.Context) (err error) {
	svc.log.DebugContext(ctx, "Entering provisioning service")
	defer svc.log.DebugContext(ctx, "Exiting provisioning service")

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	// An ExternalID update happens when a new downstream resource is provisioned
	// via SCIM. We need to know about this for users so that any Access Lists
	// they appear in will need need to be re-provisioned
	//
	// Note that we add the event handler to update here, rather than in the
	// service constructor, because this is the first time we know which context
	// to use to cancel any in-flight re-provisioning on exit.
	svc.provisioner.onExternalIDUpdated =
		func(_ context.Context, principalState *provisioningv1.PrincipalState) {
			if principalState.GetSpec().GetPrincipalType() != provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER {
				return
			}
			go func() {
				if err := svc.reprovisionUserAccessLists(ctx, principalState); err != nil {
					svc.log.ErrorContext(ctx, "error reprovisioning access lists",
						"error", err)
				}
			}()
		}

	monitor, err := newResourceMonitor(svc)
	if err != nil {
		return trace.Wrap(err)
	}
	go monitor.watch(ctx)

	if err := svc.init(ctx); err != nil {
		svc.log.ErrorContext(ctx, "Failed event handler init")
		cancel()
		return trace.Wrap(err)
	}

	// kick off an independent refresh loop, so that we can detect situations
	// like the inclusion predicate changing.
	go svc.syncStateRefreshLoop(ctx)

	// Start handling provisioning system events, like updated state records
	// deleted users, and so on
	if err := svc.handleResourceEvents(ctx); err != nil {
		svc.log.ErrorContext(ctx, "Service loop exited with error", "error", err)
		return trace.Wrap(err)
	}

	return nil
}

func (svc *Service) init(ctx context.Context) error {
	svc.log.DebugContext(ctx, "Building lock cache")
	for user, err := range allUsers(ctx, svc.usersSvcCache) {
		if err != nil {
			return trace.Wrap(err, "building lock cache")
		}

		locks, err := svc.provisioner.locksSvc.GetLocks(ctx, true,
			types.LockTarget{User: user.GetName()})
		if err != nil {
			return trace.Wrap(err)
		}

		for _, lock := range locks {
			svc.lockCache.Store(lock.GetName(), getIDForUser(user))
		}
	}

	return nil
}

func (svc *Service) syncStateRefreshLoop(ctx context.Context) {
	svc.log.DebugContext(ctx, "Entering Provisioning Service state refresh loop")
	defer svc.log.DebugContext(ctx, "Exiting Provisioning Service state refresh loop")

	jitter := retryutils.NewSeventhJitter()
	timer := svc.clock.NewTimer(jitter(svc.stateRefreshInterval))
	defer timer.Stop()

	for {
		if err := svc.refreshProvisioningStates(ctx); err != nil {
			svc.log.ErrorContext(ctx, "failed refreshing provisioning states",
				"error", err)
		}

		select {
		case <-ctx.Done():
			svc.log.DebugContext(ctx, "Exit signal detected")
			return

		case <-svc.fullRefreshSignal:
			// No sense in doing a full refresh straight after a triggered one.
			// reset the time so the the next update honors the interval
			timer.Reset(jitter(svc.stateRefreshInterval))
			continue

		case <-timer.Chan():
			timer.Reset(jitter(svc.stateRefreshInterval))
			continue
		}
	}
}

func (svc *Service) userLockStateChanged(
	ctx context.Context,
	user *types.UserV2,
	state *provisioningv1.PrincipalState,
) (bool, error) {
	locks, err := svc.provisioner.locksSvc.GetLocks(ctx, true, types.LockTarget{User: user.GetName()})
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the user has transitioned from *some* locks to *no* locks...
	if len(state.GetStatus().ActiveLocks) > 0 && len(locks) == 0 {
		return true, nil
	}

	// If the user has transitioned from *no* locks to *some* locks...
	if len(state.GetStatus().ActiveLocks) == 0 && len(locks) > 0 {
		return true, nil
	}

	return false, nil
}

func (svc *Service) refreshUser(ctx context.Context, user *types.UserV2, allStates stateMap) error {
	if !svc.userPredicate(user) {
		return nil
	}

	key := getIDForUser(user)
	log := svc.log.With(
		"principal_type", "user",
		"principal_name", user.GetName())

	// If the state already exists, determine if has changed enough to require
	// re-provisioning
	if existingState, present := allStates[key]; present {
		// remove the known state from the state map so we can spot deleted
		// principals.
		delete(allStates, key)

		// if the user has been locked or un-locked since we last touched them
		// we will need to let the downstream system know
		lockStateChanged, err := svc.userLockStateChanged(ctx, user, existingState)
		if err != nil {
			return trace.Wrap(err)
		}

		// Check to see if the user has been updated without us noticing.
		revisionChanged := user.GetRevision() != existingState.GetStatus().GetProvisionedPrincipalRevision()

		if lockStateChanged || revisionChanged {
			log.DebugContext(ctx, "User lock state changed")

			err = svc.enqueuePrincipalEvent(ctx,
				provisioningOpStale,
				provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
				user.GetName())
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ctx.Err()
			} else if err != nil {
				log.ErrorContext(ctx, "Failed enqueuing state update", "error", err)
				return nil
			}
		}
		return nil
	}

	// If we get to here, this is a new state record that needs provisioning
	err := svc.enqueuePrincipalEvent(ctx,
		provisioningOpCreate,
		provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
		user.GetName())
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ctx.Err()
	} else if err != nil {
		log.ErrorContext(ctx, "failed enqueuing state update", "error", err)
		return nil
	}

	return nil
}

func (svc *Service) refreshAccessList(ctx context.Context, acl *accesslist.AccessList, allStates stateMap) error {
	if !svc.accessListPredicate(acl) {
		return nil
	}

	key := getIDForAccessList(acl)
	log := svc.log.With(
		"provisioning_state_id", key,
		"principal_type", "access_list",
		"principal_name", acl.GetName())

	if existingState, present := allStates[key]; present {
		// remove the known state from the state map so we can spot deleted
		// principals.
		delete(allStates, key)

		// Check to see if the Access List has been updated without us noticing.
		revisionChanged := acl.GetRevision() != existingState.GetStatus().GetProvisionedPrincipalRevision()

		if revisionChanged {
			err := svc.enqueuePrincipalEvent(ctx,
				provisioningOpCreate,
				provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST,
				acl.GetName())
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ctx.Err()
			} else if err != nil {
				log.ErrorContext(ctx, "Failed enqueuing state update", "error", err)
				return nil
			}
		}

		return nil
	}

	err := svc.enqueuePrincipalEvent(ctx,
		provisioningOpCreate,
		provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST,
		acl.GetName())
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ctx.Err()
	} else if err != nil {
		log.ErrorContext(ctx, "failed enqueuing state update", "error", err)
		return nil
	}

	return nil
}

func (svc *Service) refreshProvisioningStates(ctx context.Context) error {
	svc.log.DebugContext(ctx, "Entering Provisioning Service state refresh")
	defer svc.log.DebugContext(ctx, "Exiting Provisioning Service state refresh")

	allStates, err := svc.loadStateMap(ctx)
	if err != nil {
		return trace.Wrap(err, "populating initial existing user state set")
	}

	// Create provisioning states for any users and access lists that don't
	// already have them. This will probably only be a big deal on first startup
	svc.log.DebugContext(ctx, "Refreshing user provisioning state list")
	for user, err := range allUsers(ctx, svc.usersSvcCache) {
		if err != nil {
			svc.log.ErrorContext(ctx, "error refreshing user provisioning states",
				"error", err)
			break
		}

		if err := svc.refreshUser(ctx, user, allStates); err != nil {
			svc.log.ErrorContext(ctx, "refreshing user provisioning state",
				"username", user.GetName(),
				"error", err)
		}
	}

	for acl, err := range allAccessLists(ctx, svc.assessListsSvcCache) {
		if err != nil {
			svc.log.ErrorContext(ctx, "error refreshing access list provisioning states",
				"error", err)
			break
		}

		if err := svc.refreshAccessList(ctx, acl, allStates); err != nil {
			svc.log.ErrorContext(ctx, "refreshing access list provisioning states",
				"access)list", acl.GetName(),
				"error", err)
		}
	}

	for _, state := range allStates {
		log := svc.log.With("principal", principalStateValuer{state})

		err := svc.enqueuePrincipalEvent(ctx,
			provisioningOpDelete,
			state.GetSpec().PrincipalType,
			state.GetSpec().PrincipalId)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ctx.Err()
		} else if err != nil {
			log.ErrorContext(ctx, "failed enqueuing state update", "error", err)
			return nil
		}
	}
	return nil
}

func (svc *Service) GetExternalID(ctx context.Context, principalID services.ProvisioningStateID) (ExternalID, error) {
	principalState, err := svc.stateSvcCache.GetProvisioningState(ctx, svc.downstreamID, principalID)
	if err != nil {
		return "", trace.Wrap(err, "looking up external ID for principal %q", principalID)
	}

	return ExternalID(principalState.GetStatus().GetExternalId()), nil
}

func (svc *Service) GetUserExternalID(ctx context.Context, username string) (ExternalID, error) {
	return svc.GetExternalID(ctx, getIDForUserName(username))
}

func (svc *Service) GetAccessListExternalID(ctx context.Context, aclName string) (ExternalID, error) {
	return svc.GetExternalID(ctx, getIDForAccessListName(aclName))
}

func (svc *Service) reprovisionUserAccessLists(ctx context.Context, principalState *provisioningv1.PrincipalState) error {
	if principalState.GetSpec().GetPrincipalType() != provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER {
		return trace.BadParameter("state must represent a Teleport user")
	}

	username := principalState.GetSpec().GetPrincipalId()

	for acl, err := range allAccessLists(ctx, svc.assessListsSvcCache) {
		if err != nil {
			return trace.Wrap(err, "re-provisioning user access lists")
		}

		if !svc.accessListPredicate(acl) {
			continue
		}

		_, err := svc.assessListsSvcCache.GetAccessListMember(ctx, acl.GetName(), username)
		if err != nil {
			continue
		}

		svc.enqueuePrincipalEvent(ctx,
			provisioningOpStale,
			provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST,
			acl.GetName())
	}

	return nil
}

type provisioningOperation int

const (
	provisioningOpCreate provisioningOperation = 0
	provisioningOpStale  provisioningOperation = 1
	provisioningOpDelete provisioningOperation = 2
)

type provisioningEvent struct {
	operation     provisioningOperation
	principalType provisioningv1.PrincipalType
	principalName string
}

func (svc *Service) enqueuePrincipalEvent(ctx context.Context, op provisioningOperation, principalType provisioningv1.PrincipalType, principalID string) error {
	event := &provisioningEvent{
		operation:     op,
		principalType: principalType,
		principalName: principalID,
	}

	select {
	case svc.eventsChan <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// signalFullRefresh signals the refresh process to start now.
func (svc *Service) signalFullRefresh() {
	// fullRefreshSignal is a 1-item buffered channel, so if writing would block
	// then the signal is already set, and the refresh process just hasn't picked
	// it up yet.
	select {
	case svc.fullRefreshSignal <- struct{}{}:
	default:
	}
}

func (svc *Service) handleLockCreation(ctx context.Context, lock types.Lock) error {
	log := svc.log.With("lock", lock.GetName())
	log.DebugContext(ctx, "Handling lock creation")

	target := lock.Target().User
	if target == "" {
		log.DebugContext(ctx, "Non-user target. Ignoring.")
		return nil
	}

	log = log.With("user", target)

	user, err := svc.usersSvcCache.GetUser(ctx, target, false)
	if err != nil {
		log.ErrorContext(ctx, "Failed user lookup", "error", err)
		return nil
	}

	if !svc.userPredicate(user) {
		log.DebugContext(ctx, "Irrelevant user lock target. Ignoring.")
		return nil
	}

	svc.lockCache.Store(lock.GetName(), getIDForUser(user))

	log.DebugContext(ctx, "Triggering re-provisioning")
	err = svc.enqueuePrincipalEvent(ctx,
		provisioningOpStale,
		provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
		target)
	return trace.Wrap(err)
}

func (svc *Service) handleLockDeletion(ctx context.Context, lockID string) error {
	log := svc.log.With("lock", lockID)
	log.DebugContext(ctx, "Handling lock deletion")

	targetID, present := svc.lockCache.LoadAndDelete(lockID)
	if !present {
		svc.log.DebugContext(ctx, "Lock not recorded. Ignoring.")
		return nil
	}

	targetState, err := svc.stateSvc.GetProvisioningState(ctx, svc.downstreamID, targetID)
	if err != nil {
		svc.log.ErrorContext(ctx, "Error fetching provisioning state", "error", err)
		return nil
	}

	svc.log.DebugContext(ctx, "Triggering re-provisioning")
	err = svc.enqueuePrincipalEvent(ctx,
		provisioningOpStale,
		provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
		targetState.GetSpec().PrincipalId)
	return trace.Wrap(err)
}

// handleResourcePut validates that the requested principal matches the appropriate
// resource predicate and, if so, marks the resource provisioning state as stale
// in preparation for re-provisioning
func (svc *Service) handleResourcePut(ctx context.Context, principalName string, principalType provisioningv1.PrincipalType) (*provisioningv1.PrincipalState, error) {
	var provisioningStateId services.ProvisioningStateID
	switch principalType {
	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER:
		u, err := svc.usersSvcCache.GetUser(ctx, principalName, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !svc.userPredicate(u) {
			return nil, nil
		}
		provisioningStateId = getIDForUserName(principalName)

	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST:
		acl, err := svc.assessListsSvcCache.GetAccessList(ctx, principalName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !svc.accessListPredicate(acl) {
			return nil, nil
		}
		provisioningStateId = getIDForAccessListName(principalName)

	default:
		return nil, trace.BadParameter("Invalid principal type: %v", principalType)
	}

	svc.log.DebugContext(ctx, "Marking state as stale",
		"provisioning_state_id", provisioningStateId,
		"principal_name", principalName,
		"principal_type", principalType)

	state, err := svc.setPrincipalProvisioningState(
		ctx, provisioningStateId, principalName, principalType,
		provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE,
		createIfMissing)
	if err != nil {
		return nil, trace.Wrap(err, "creating/marking state record as stale")
	}
	return state, nil
}

func (svc *Service) handleResourceEvents(ctx context.Context) error {
	svc.log.DebugContext(ctx, "Entering provisioner event handling loop")
	defer svc.log.DebugContext(ctx, "Exiting provisioner event handling loop")

	keepRunning := true
	for keepRunning {
		// In general, we want to ensure that user updates are provisioned *first*
		// so that access lists have valid users to operate on when it comes time to
		// provision them. We store them in maps in order to de-duplicate any updates
		// as they come through, with the latest update winning.
		pendingUsers := map[services.ProvisioningStateID]*provisioningv1.PrincipalState{}
		pendingAccessLists := map[services.ProvisioningStateID]*provisioningv1.PrincipalState{}
		for e, ok := range drainChannel(ctx, svc.eventsChan, math.MaxInt16) {
			if !ok {
				keepRunning = false
				svc.log.InfoContext(ctx, "Provisioner event handling loop exit requested")
				break
			}

			log := svc.log.With("principal_name", e.principalName, "principal_type", e.principalType, "event", e.operation)

			principalStateID, err := getIDForPrincipal(e.principalName, e.principalType)
			if err != nil {
				svc.log.ErrorContext(ctx, "making principal ID", "error", err)
				continue
			}

			var state *provisioningv1.PrincipalState
			switch e.operation {
			case provisioningOpCreate, provisioningOpStale:
				state, err = svc.handleResourcePut(ctx, e.principalName, e.principalType)
				if err != nil {
					svc.log.ErrorContext(ctx, "handling resource pur", "error", err)
				}

			case provisioningOpDelete:
				log.DebugContext(ctx, "Marking state as deleted")
				state, err = svc.markProvisioningStateAsDeleted(ctx, principalStateID, e.principalName, e.principalType)
				if err != nil {
					svc.log.ErrorContext(ctx, "making state record for deletion", "error", err)
				}
			}

			if state == nil {
				log.DebugContext(ctx, "No state to provision. Abandoning update.")
				continue
			}

			switch e.principalType {
			case provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER:
				pendingUsers[principalStateID] = state

			case provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST:
				pendingAccessLists[principalStateID] = state
			}
		}

		svc.log.DebugContext(ctx, "Provisioning pending users", "count", len(pendingUsers))
		if err := svc.provisionUpdates(ctx, maps.Values(pendingUsers)); err != nil {
			svc.log.ErrorContext(ctx, "User provisioning failed",
				"error", err)
		}

		svc.log.DebugContext(ctx, "Provisioning pending access lists", "count", len(pendingAccessLists))
		if err := svc.provisionUpdates(ctx, maps.Values(pendingAccessLists)); err != nil {
			svc.log.ErrorContext(ctx, "Access list provisioning failed",
				"error", err)
		}
	}

	return nil
}

func (svc *Service) provisionUpdates(ctx context.Context, states iter.Seq[*provisioningv1.PrincipalState]) error {
	if err := svc.provisioner.ProvisionAll(ctx, states); err != nil {
		return trace.Wrap(err, "provisioning multiple updates")
	}
	return nil
}

func (svc *Service) loadStateMap(ctx context.Context) (stateMap, error) {
	result := stateMap{}
	for state, err := range allProvisioningStates(ctx, svc.stateSvc, svc.downstreamID) {
		if err != nil {
			return nil, trace.Wrap(err, "loading all provisioning states")
		}
		result[getID(state)] = state
	}

	return result, nil
}

type createMode bool

const (
	createIfMissing      createMode = true
	doNotCreateIfMissing createMode = false
)

// setPrincipalProvisioningState updates the supplied record with
func (svc *Service) setPrincipalProvisioningState(
	ctx context.Context,
	id services.ProvisioningStateID,
	principalName string,
	principalType provisioningv1.PrincipalType,
	newState provisioningv1.ProvisioningState,
	createMode createMode,
) (*provisioningv1.PrincipalState, error) {
	state, err := svc.stateSvc.GetProvisioningState(ctx, svc.downstreamID, id)
	if trace.IsNotFound(err) {
		if createMode == doNotCreateIfMissing {
			return nil, nil
		}

		createdState, err := svc.stateSvc.CreateProvisioningState(ctx,
			newPrincipalState(svc.downstreamID, principalType, id, principalName, newState))
		if err != nil {
			return nil, trace.Wrap(err, "creating new provisioning state")
		}
		return createdState, nil
	}
	if err != nil {
		return nil, trace.Wrap(err, "creating new provisioning state")
	}

	setProvisioningState := func(s *provisioningv1.PrincipalState) error {
		status := s.GetStatus()
		if status.ProvisioningState == provisioningv1.ProvisioningState_PROVISIONING_STATE_DELETED {
			return errNoChangeRequired
		}
		status.ProvisioningState = newState
		return nil
	}
	updatedState, err := updateProvisioningState(ctx, svc.stateSvc, state, setProvisioningState)
	if err != nil {
		return nil, trace.Wrap(err, "updating provisioning state")
	}

	return updatedState, nil
}

func (svc *Service) markProvisioningStateAsDeleted(
	ctx context.Context,
	id services.ProvisioningStateID,
	principalName string,
	principalType provisioningv1.PrincipalType,
) (*provisioningv1.PrincipalState, error) {
	updated, err := svc.setPrincipalProvisioningState(
		ctx, id, principalName, principalType,
		provisioningv1.ProvisioningState_PROVISIONING_STATE_DELETED,
		doNotCreateIfMissing)
	return updated, trace.Wrap(err, "marking provisioning state as deleted")
}
