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

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/retryutils"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// DefaultStateRefreshInterval is the default time between full User and Access List
// state resynchronisations.
var DefaultStateRefreshInterval = 2 * time.Minute

const (
	provisioningComponent          = "PROV"
	defaultUpdateAttempts          = 5
	defaultEventBufferSize         = 128
	defaultProvisioningConcurrency = 8
)

// stateMap defines a mapping of provisioning state IDs to the states they
// represent. Used for brevity.
type stateMap map[services.ProvisioningStateID]*provisioningv1.PrincipalState

type Service struct {
	downstreamID        services.DownstreamID
	stateSvc            services.DownstreamProvisioningStates
	stateSvcCache       services.DownstreamProvisioningStateGetter
	usersSvcCache       UsersService
	userPredicate       identitycentercommon.UserFilterFunc
	accessListsSvcCache AccessListsService
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

	// onExternalIDUpdated passes external update events on to the
	// world outside the provisioning system
	onExternalIDUpdated EventHandler
}

func NewService(cfg ServiceConfig) (svc *Service, err error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	svc = &Service{
		downstreamID:         cfg.DownstreamID,
		stateSvc:             cfg.StateSvc,
		stateSvcCache:        cfg.StateSvcCache,
		usersSvcCache:        cfg.UsersCache,
		userPredicate:        cfg.UserPredicate,
		accessListsSvcCache:  cfg.AccessListsCache,
		accessListPredicate:  cfg.AccessListPredicate,
		eventsClient:         cfg.EventsClient,
		eventsSvc:            cfg.EventsClient,
		log:                  cfg.Logger,
		clock:                cfg.Clock,
		stateRefreshInterval: cfg.StateRefreshInterval,
		eventsChan:           make(chan *provisioningEvent, cfg.EventBufferSize),
		fullRefreshSignal:    make(chan struct{}, 1),
		onExternalIDUpdated:  cfg.OnExternalIDUpdated,
	}

	svc.provisioner, err = newProvisioner(provisionerConfig{
		scimClient:                        cfg.SCIMClient,
		log:                               cfg.Logger,
		stateSvc:                          cfg.StateSvc,
		usersSvc:                          cfg.UsersCache,
		accessListsSvc:                    cfg.AccessListsCache,
		locksSvc:                          cfg.Locks,
		maxConcurrency:                    cfg.ProvisioningConcurrency,
		externalIDGetter:                  svc,
		userProvisioningMode:              cfg.UserProvisioningMode,
		onPrincipalProvisioning:           cfg.OnPrincipalProvisioning,
		onPrincipalProvisioned:            cfg.OnPrincipalProvisioned,
		onPrincipalDeprovisioning:         cfg.OnPrincipalDeprovisioning,
		onPrincipalRequiresReprovisioning: svc.onPrincipalNeedsProvisioning,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating downstream provisioner")
	}

	return svc, nil
}

// Run the provisioning service, blocking until it exits
func (svc *Service) Run(ctx context.Context) (err error) {
	svc.log.DebugContext(ctx, "Entering provisioning service")
	defer svc.log.DebugContext(ctx, "Exiting provisioning service")

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	// An ExternalID update happens when a principal is first provisioned
	// downstream via SCIM and we learn what ID the downstream system has given
	// it. We need to record this External ID value so that we can refer to
	// principals in a way that the downstream system understands, for example
	// when constructing SCIM group member lists.
	//
	// Note that we add the event handler to update here, rather than in the
	// service constructor, because this is the first time we know which context
	// to use to cancel any in-flight re-provisioning on exit.
	svc.provisioner.onExternalIDUpdated =
		func(_ context.Context, principalState *provisioningv1.PrincipalState) {
			svc.onProvisionerExternalIDUpdated(ctx, principalState)
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

// SetUserStateLabel sets a label on the user's Provisioning State record.
func (svc *Service) SetUserStateLabel(ctx context.Context, username string, key, value string) error {
	return trace.Wrap(svc.SetProvisioningStateLabel(ctx, GetIDForUserName(username), key, value))
}

// SetAccessListStateLabel sets a label on the Access List's Provisioning State
// record.
func (svc *Service) SetAccessListStateLabel(ctx context.Context, aclName string, key, value string) error {
	return trace.Wrap(svc.SetProvisioningStateLabel(ctx, getIDForAccessListName(aclName), key, value))
}

// SetProvisioningStateLabel sets a label on the specified Provisioning State
// record.
func (svc *Service) SetProvisioningStateLabel(ctx context.Context, id services.ProvisioningStateID, key, value string) error {
	originalState, err := svc.stateSvc.GetProvisioningState(ctx, svc.downstreamID, id)
	if err != nil {
		return trace.Wrap(err)
	}

	setLabel := func(state *provisioningv1.PrincipalState) error {
		if state.Metadata.Labels == nil {
			state.Metadata.Labels = make(map[string]string)
		}
		state.Metadata.Labels[key] = value
		return nil
	}

	_, err = updateProvisioningState(ctx, svc.stateSvc, originalState, setLabel)
	if err != nil {
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

	jitter := retryutils.SeventhJitter
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
			log.DebugContext(ctx, "User needs reprovisioning",
				"lock_state_changed", lockStateChanged, "user_revision_changed", revisionChanged)

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
	includeACL, err := svc.accessListPredicate(ctx, acl)
	if err != nil {
		return trace.Wrap(err)
	}
	if !includeACL {
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

	err = svc.enqueuePrincipalEvent(ctx,
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

	for acl, err := range allAccessLists(ctx, svc.accessListsSvcCache) {
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

// GetExternalID looks up the External ID of the supplied principal, first trying
// the cache and then proceeding to the primary data service if that fails.
//
// The reason for this fallback is that we sometimes need to pull the External ID
// immediately after it gets discovered and is written to the principal's provisioning
// state record, for example when an enclosing Identity Center integration wants to
// create AWS Account Assignments for the recently-provisioned principal. This read is often
// too early for the cache service to have been updated, so we get a lot of false
// negatives that delay operations that depend on knowing a principal's external ID.
//
// To counter this, we fall back to the primary data service if the cache read fails
// to produce an External ID.
func (svc *Service) GetExternalID(ctx context.Context, principalID services.ProvisioningStateID) (ExternalID, error) {
	// first, try the cache. If the cache doesn't know the ExternalID, then fall
	// back to the main data services
	principalState, err := svc.stateSvcCache.GetProvisioningState(ctx, svc.downstreamID, principalID)
	if trace.IsNotFound(err) {
		// fall through to looking up in the primary data service
	} else if err != nil {
		return "", trace.Wrap(err, "looking up external ID for principal %q", principalID)
	}

	// Note that the buf-generated field getters automatically handle the case
	// where principalState is nil because GetProvisioningState() returned Not Found.
	extID := ExternalID(principalState.GetStatus().GetExternalId())
	if extID != "" {
		return extID, nil
	}

	// if we get to here, no External ID was recorded in the cache - try the promary data service instead.
	principalState, err = svc.stateSvc.GetProvisioningState(ctx, svc.downstreamID, principalID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return ExternalID(principalState.GetStatus().GetExternalId()), nil
}

// GetUserExternalID tries to look up a Teleport user's external ID in the
// downstream system
func (svc *Service) GetUserExternalID(ctx context.Context, username string) (ExternalID, error) {
	return svc.GetExternalID(ctx, GetIDForUserName(username))
}

// GetUserExternalID tries to look up a Teleport Access Lists's external group ID
// in the downstream system
func (svc *Service) GetAccessListExternalID(ctx context.Context, aclName string) (ExternalID, error) {
	return svc.GetExternalID(ctx, getIDForAccessListName(aclName))
}

func (svc *Service) reprovisionUserAccessLists(ctx context.Context, principalState *provisioningv1.PrincipalState) error {
	if principalState.GetSpec().GetPrincipalType() != provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER {
		return trace.BadParameter("state must represent a Teleport user")
	}

	username := principalState.GetSpec().GetPrincipalId()

	for acl, err := range allAccessLists(ctx, svc.accessListsSvcCache) {
		if err != nil {
			return trace.Wrap(err, "re-provisioning user access lists")
		}

		// Is this Access List exported by the provisioning system?
		includeACL, err := svc.accessListPredicate(ctx, acl)
		if err != nil {
			return trace.Wrap(err)
		}
		if !includeACL {
			continue
		}

		// Is the user of interest a member of this Access List?
		_, err = svc.accessListsSvcCache.GetAccessListMember(ctx, acl.GetName(), username)
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

// ErrDoNotProvision can be returned by event handlers to suppress provisioning
// or de-provisioning of a principal into the downstream system.
var ErrDoNotProvision = errors.New("provisioning suppressed by event handler")

// errNoFurtherAction is an error returned by a resource event handler when
// the provided resource does is not under the provisioners control and no
// action needs to be taken.
var errNoFurtherAction = errors.New("resource not for provisioning")

// handleResourcePut validates that the requested principal matches the appropriate
// resource predicate and, if so, marks the resource provisioning state as stale
// in preparation for re-provisioning
func (svc *Service) handleResourcePut(ctx context.Context, principalName string, principalType provisioningv1.PrincipalType) (*provisioningv1.PrincipalState, error) {
	principalMatchesPredicate := false
	var provisioningStateID services.ProvisioningStateID

	switch principalType {
	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER:
		u, err := svc.usersSvcCache.GetUser(ctx, principalName, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		principalMatchesPredicate = svc.userPredicate(u)
		provisioningStateID = GetIDForUserName(principalName)

	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST:
		acl, err := svc.accessListsSvcCache.GetAccessList(ctx, principalName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		principalMatchesPredicate, err = svc.accessListPredicate(ctx, acl)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		provisioningStateID = getIDForAccessListName(principalName)

	default:
		return nil, trace.BadParameter("Invalid principal type: %v", principalType)
	}

	if !principalMatchesPredicate {
		if err := svc.handleExcludedResource(ctx, provisioningStateID); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	svc.log.DebugContext(ctx, "Marking state as stale",
		"provisioning_state_id", provisioningStateID,
		"principal_name", principalName,
		"principal_type", principalType)

	state, err := svc.setPrincipalProvisioningState(
		ctx, provisioningStateID, principalName, principalType,
		provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE,
		createIfMissing)
	if err != nil {
		return nil, trace.Wrap(err, "creating/marking state record as stale")
	}
	return state, nil
}

// handleExcludedResource handles a resource that fails a predicate match,
// with detection for cases where the resource has transitioned out of the
// set of resources for provisioning.
func (svc *Service) handleExcludedResource(ctx context.Context, provisioningStateId services.ProvisioningStateID) error {
	provisioningState, err := svc.stateSvc.GetProvisioningState(ctx, svc.downstreamID, provisioningStateId)
	switch {
	case trace.IsNotFound(err):
		// All good; the principal should NOT be provisioned downstream and
		// does NOT have a provisioning state record. Everything is as it
		// should be.
		return trace.Wrap(errNoFurtherAction)

	case err == nil:
		// The principal should NOT be provisioned downstream but has an
		// existing provisioning state record. This can happen when a
		// principal is updated and transitions from matching the predicate
		// to not matching it. The principal needs to be deleted from the
		// downstream system.
		_, err := updateProvisioningState(ctx, svc.stateSvc, provisioningState,
			setProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_DELETED))
		if err != nil {
			return trace.Wrap(err)
		}
		return nil

	default:
		// Failure while trying to read the provisioning state.
		return trace.Wrap(err)
	}
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
				if err != nil && !errors.Is(err, errNoFurtherAction) {
					svc.log.ErrorContext(ctx, "handling resource put", "error", err)
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

// onProvisionerExternalIDUpdated is invoked by the provisioner whenever a
// principal's ExternalID changes, for example when a principal is first
// provisioned downstream via SCIM and we learn what ID the downstream system
// has given it.
func (svc *Service) onProvisionerExternalIDUpdated(ctx context.Context, principalState *provisioningv1.PrincipalState) {
	// Pass the event on up to the outside world
	svc.onExternalIDUpdated(ctx, principalState)

	// If we have detected that a user's External ID has changed then we will
	// need to make sure that any Access Lists containing the target user are
	// re-provisioned to include them.
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

func (svc *Service) onPrincipalNeedsProvisioning(ctx context.Context, state *provisioningv1.PrincipalState) {
	err := svc.enqueuePrincipalEvent(ctx,
		provisioningOpCreate,
		state.GetSpec().GetPrincipalType(),
		state.GetSpec().GetPrincipalId())
	if err != nil {
		svc.log.ErrorContext(ctx, "Failed queueing resource event for update",
			principalStateAttr(state))
	}
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
	log := svc.log.With("principal_type", principalType, "principal_name", principalName)

	state, err := svc.stateSvc.GetProvisioningState(ctx, svc.downstreamID, id)
	if trace.IsNotFound(err) {
		if createMode == doNotCreateIfMissing {
			return nil, nil
		}

		// Create the initial state record and offer the outside world a chance
		// to customize it before creating the formal record in the system
		// backend
		initialState := newPrincipalState(svc.downstreamID, principalType, id, principalName, newState)
		createdState, err := svc.stateSvc.CreateProvisioningState(ctx, initialState)
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
		switch status.GetProvisioningState() {
		case provisioningv1.ProvisioningState_PROVISIONING_STATE_DELETED:
			log.Log(ctx, logutils.TraceLevel, "Principal already deleted. Abandoning update.")
			return errNoChangeRequired
		case newState:
			log.Log(ctx, logutils.TraceLevel, "Principal already has designated state. Abandoning update.")
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
		doNotCreateIfMissing,
	)
	return updated, trace.Wrap(err, "marking provisioning state as deleted")
}
