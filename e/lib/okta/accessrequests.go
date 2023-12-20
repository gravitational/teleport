package okta

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	accessRequestFormat = "access-request/%s"

	maxAccessRequestRetryWait = time.Minute

	checkInventoryWait = time.Minute

	// maxOktaServiceConnectionFailures is the number of connection failures to allow before
	// considering the Okta service disconnected.
	maxOktaServiceConnectionFailures = 5
)

// AccessRequestReconcilerAccessPoint is a client that consists of only the interfaces
// needed for the AccessRequestReconciler.
type AccessRequestReconcilerAccessPoint interface {
	services.UserGroups
	types.Events

	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)

	// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
	GetInventoryConnectedServiceCount(service types.SystemRole) uint64
}

// AccessRequestReconcilerConfig is the configuration for the AccessRequestReconciler.
type AccessRequestReconcilerConfig struct {
	// Log is the logger for the AccessRequestReconciler.
	Log *logrus.Entry

	// Clock is the clock to use for the reconciler.
	Clock clockwork.Clock

	// ClusterName is the name of the cluster.
	ClusterName string

	// LocKWatcher is the lock watcher for the reconciler.
	LockWatcher *services.LockWatcher

	// AccessPoint is the access point for the access request reconciler.
	AccessPoint AccessRequestReconcilerAccessPoint

	// OktaClient is the Okta client for creating Okta assignment objects.
	OktaClient services.OktaAssignments

	// OnReconcile is called after each access request resource reconciliation.
	OnReconcile func(types.AccessRequests)

	// Plugins is an optional plugins service that will allow the access request reconciler to query
	// plugins. This may be nil.
	Plugins services.Plugins

	// onServiceDisconnectedCh is a channel that will be signaled to when the service disconnects.
	// This is to be used for testing.
	onServiceDisconnectedCh chan struct{}

	// onReconcileCh is a channel that will be signaled to when reconciliation completes.
	// This is to be used for testing.
	onReconcileCh chan struct{}
}

func (c *AccessRequestReconcilerConfig) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, teleport.ComponentOktaAccessRequestReconciler)
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.ClusterName == "" {
		return trace.BadParameter("cluster name is missing")
	}

	if c.LockWatcher == nil {
		return trace.BadParameter("lock watcher is missing")
	}

	if c.AccessPoint == nil {
		return trace.BadParameter("access point is missing")
	}

	if c.OktaClient == nil {
		return trace.BadParameter("okta client is missing")
	}

	return nil
}

// AccessRequestReconciler is a process that monitors access requests and creates PENDING
// OktaAssignment objects from them. This reconciler will live on the auth server.
//
// TODO(mdwn): This must be extended to support leaf clusters.
type AccessRequestReconciler struct {
	log         logrus.FieldLogger
	clock       clockwork.Clock
	clusterName string
	lockWatcher *services.LockWatcher

	accessPoint AccessRequestReconcilerAccessPoint
	plugins     services.Plugins
	oktaClient  services.OktaAssignments
	onReconcile func(types.AccessRequests)

	watcherMu sync.Mutex
	watcher   *services.AccessRequestWatcher

	reconcileCh chan struct{}
	stopCh      chan struct{}

	accessRequestsMu sync.RWMutex
	accessRequests   map[string]types.AccessRequest

	newAccessRequestsMu sync.RWMutex
	newAccessRequests   map[string]types.AccessRequest

	retryer retryutils.Retry

	// these channels are used for testing.
	onServiceDisconnectedCh chan struct{}
	onReconcileCh           chan struct{}
}

// NewAccessRequestReconciler creates a new AccessRequestReconciler.
func NewAccessRequestReconciler(ctx context.Context, config *AccessRequestReconcilerConfig) (*AccessRequestReconciler, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retryer, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Clock:  config.Clock,
		Driver: retryutils.NewExponentialDriver(time.Second),
		Max:    maxAccessRequestRetryWait,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a := &AccessRequestReconciler{
		log:                     config.Log,
		clock:                   config.Clock,
		clusterName:             config.ClusterName,
		lockWatcher:             config.LockWatcher,
		accessPoint:             config.AccessPoint,
		onReconcile:             config.OnReconcile,
		plugins:                 config.Plugins,
		oktaClient:              config.OktaClient,
		reconcileCh:             make(chan struct{}),
		stopCh:                  make(chan struct{}, 1),
		accessRequests:          map[string]types.AccessRequest{},
		newAccessRequests:       map[string]types.AccessRequest{},
		retryer:                 retryer,
		onServiceDisconnectedCh: config.onServiceDisconnectedCh,
		onReconcileCh:           config.onReconcileCh,
	}

	return a, nil
}

// Start will start the reconciler.
func (a *AccessRequestReconciler) Start(ctx context.Context) error {
	go a.manageReconcilerStartStop(ctx)

	return nil
}

func (a *AccessRequestReconciler) manageReconcilerStartStop(ctx context.Context) {
	ticker := a.clock.NewTicker(checkInventoryWait)
	var (
		cancel           context.CancelFunc
		resourcesCleaned chan struct{}
	)
	defer ticker.Stop()
	serviceStarted := false
	var serviceConnectionFailures int

	if a.plugins == nil {
		a.log.Debug("This auth server does not support plugins, so the Okta access request reconciler will not check for Okta plugins.")
	} else {
		a.log.Debug("This auth server supports plugins, so the Okta access request reconciler will check for Okta plugins.")
	}

	for {
		newOktaServiceConnected := isOktaServiceConnected(ctx, a.log, a.accessPoint, a.plugins)
		if newOktaServiceConnected {
			serviceConnectionFailures = 0

			if !serviceStarted {
				a.log.Infof("Okta service connected to the auth server, starting the Okta access request reconciler.")

				var err error
				cancel, resourcesCleaned, err = a.start(ctx)
				if err != nil {
					a.log.Errorf("Error starting access request reconciler: %v", err)
					continue
				}
				a.log.Infof("Okta access request reconciler started.")
				serviceStarted = true
			}
		} else if !newOktaServiceConnected && serviceStarted {
			serviceConnectionFailures++
			if serviceConnectionFailures >= maxOktaServiceConnectionFailures {
				a.log.Infof("Okta service has disconnected, stopping the access request reconciler.")
				cancel()
				// wait for the resources to be cleaned up otherwise we can end up with
				// multiple reconcilers running at the same time for short periods of time
				// which cause tests to fail when both invoke retryer.Reset() at the same time.
				<-resourcesCleaned
				serviceStarted = false
				a.log.Infof("Okta access request reconciler has stopped.")
			} else {
				a.log.Warnf("No Okta service connected (check %d/%d)", serviceConnectionFailures, maxOktaServiceConnectionFailures)
			}

			if a.onServiceDisconnectedCh != nil {
				a.onServiceDisconnectedCh <- struct{}{}
			}
		}

		select {
		case <-ticker.Chan():
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// start the reconciler.
func (a *AccessRequestReconciler) start(ctx context.Context) (context.CancelFunc, chan struct{}, error) {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig[types.AccessRequest]{
		Matcher: func(resource types.AccessRequest) bool {
			return a.matcher(ctx, resource)
		},
		GetCurrentResources: a.getAccessRequests,
		GetNewResources:     a.getNewAccessRequests,
		OnCreate:            a.onCreate,
		OnUpdate:            a.onUpdate,
		OnDelete:            a.onDelete,
		Log:                 a.log,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	watcher, err := a.startResourceWatcher(ctx)
	if err != nil {
		cancel()
		return nil, nil, trace.Wrap(err)
	}
	a.watcherMu.Lock()
	a.watcher = watcher
	a.watcherMu.Unlock()
	resourcesCleaned := make(chan struct{})

	go func() {
		defer close(resourcesCleaned)
		a.reconcile(ctx, reconciler)
	}()

	return cancel, resourcesCleaned, nil
}

// reconciler will reconcile access requests and transform them into OktaAssignments.
func (a *AccessRequestReconciler) reconcile(ctx context.Context, reconciler *services.Reconciler[types.AccessRequest]) {
	for {
		select {
		case _, ok := <-a.reconcileCh:
			if !ok {
				return
			}

			if err := reconciler.Reconcile(ctx); err != nil {
				a.retryer.Inc()
				a.log.WithError(err).Errorf("Failed to reconcile. Will retry in %s.", a.retryer.Duration())

				// On error, we'll need to retry so that we re-attempt reconciliation. Otherwise,
				// reconciliation will only happen again when the next access request event comes in,
				// which could be indefinitely, and the associated OktaAssignment may never actually
				// be created.
				a.retryer.Inc() // Increment the retry attempt so that the next reconciler attempt waits.
				a.clock.AfterFunc(a.retryer.Duration(), func() {
					select {
					case <-a.stopCh:
					case <-ctx.Done():
					case a.reconcileCh <- struct{}{}:
					}
				})
			} else if a.onReconcile != nil {
				a.accessRequestsMu.RLock()
				a.onReconcile(copyAccessRequestMapToAccessRequests(a.accessRequests))
				a.accessRequestsMu.RUnlock()
			}
			if a.onReconcileCh != nil {
				a.onReconcileCh <- struct{}{}
			}
			a.retryer.Reset()
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Wait will wait for the reconciler to complete.
func (a *AccessRequestReconciler) Wait(ctx context.Context) {
	select {
	case <-a.stopCh:
	case <-ctx.Done():
	}
}

// Stop will stop and close any lingering resources in the AccessRequestReconciler.
func (a *AccessRequestReconciler) Stop() {
	a.watcherMu.Lock()
	if a.watcher != nil {
		a.watcher.Close()
	}
	a.watcherMu.Unlock()

	close(a.stopCh)
}

// getAccessRequests returns the list of access requests currently known to the reconciler.
func (a *AccessRequestReconciler) getAccessRequests() map[string]types.AccessRequest {
	a.accessRequestsMu.RLock()
	defer a.accessRequestsMu.RUnlock()

	return utils.FromSlice(copyAccessRequestMapToAccessRequests(a.accessRequests), types.AccessRequest.GetName)
}

// getNewAccessRequests returns the list of new access requests that the reconciler has yet to act on.
func (a *AccessRequestReconciler) getNewAccessRequests() map[string]types.AccessRequest {
	a.newAccessRequestsMu.RLock()
	defer a.newAccessRequestsMu.RUnlock()

	return utils.FromSlice(copyAccessRequestMapToAccessRequests(a.newAccessRequests), types.AccessRequest.GetName)
}

// startResourceWatcher starts watching changes to access request resources and
// create OktaAssignment resources.
func (a *AccessRequestReconciler) startResourceWatcher(ctx context.Context) (*services.AccessRequestWatcher, error) {
	a.log.Debug("Initializing access request resource watcher.")
	watcher, err := services.NewAccessRequestWatcher(ctx, services.AccessRequestWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentOktaAccessRequestReconciler,
			Log:       a.log,
			Client:    a.accessPoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer func() {
			a.log.Debug("Access request resource watcher finished.")
		}()

		for {
			select {
			case newAccessRequests := <-watcher.AccessRequestsC:
				a.newAccessRequestsMu.Lock()
				a.newAccessRequests = map[string]types.AccessRequest{}
				for _, newAccessRequest := range newAccessRequests {
					a.newAccessRequests[newAccessRequest.GetName()] = newAccessRequest
				}
				a.newAccessRequestsMu.Unlock()

				select {
				case a.reconcileCh <- struct{}{}:
				case <-a.stopCh:
					return
				case <-ctx.Done():
					return
				}
			case <-a.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return watcher, nil
}

// onCreate will create Okta assignments from access requests.
func (a *AccessRequestReconciler) onCreate(ctx context.Context, newAccessRequest types.AccessRequest) error {
	// Only create an assignment if the access state is approved.
	if newAccessRequest.GetState() == types.RequestState_APPROVED {
		assignment, err := a.accessRequestToOktaAssignment(ctx, newAccessRequest, constants.OktaAssignmentStatusPending)
		if err != nil {
			if trace.IsNotFound(err) {
				a.log.Debugf("access request cannot be processed: %v", err)
				return nil
			}
			return trace.Wrap(err)
		}

		// If the assignment already exists, register it locally and move on.
		_, err = a.oktaClient.CreateOktaAssignment(ctx, assignment)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		a.accessRequestsMu.Lock()
		a.accessRequests[newAccessRequest.GetName()] = newAccessRequest
		a.accessRequestsMu.Unlock()
	}

	return nil
}

// onUpdate will cleanup Okta assignments from access requests.
func (a *AccessRequestReconciler) onUpdate(ctx context.Context, updatedAccessRequest types.AccessRequest) error {
	// Only update an Okta assignment if the request state is denied.
	if updatedAccessRequest.GetState() == types.RequestState_DENIED {
		assignment, err := a.oktaClient.GetOktaAssignment(ctx, updatedAccessRequest.GetName())
		if err != nil {
			return trace.Wrap(err)
		}

		// Set the cleanup time to now to trigger a cleanup.
		assignment.SetCleanupTime(a.clock.Now())
		if _, err := a.oktaClient.UpdateOktaAssignment(ctx, assignment); err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "error marking assignment for cleanup")
		}

		a.accessRequestsMu.Lock()
		a.accessRequests[updatedAccessRequest.GetName()] = updatedAccessRequest
		a.accessRequestsMu.Unlock()
	}

	return nil
}

// onUpdate will cleanup Okta assignments from access requests.
func (a *AccessRequestReconciler) onDelete(ctx context.Context, request types.AccessRequest) error {
	// No need to look at access request state, we should clean up the associated Okta assignments.
	assignment, err := a.oktaClient.GetOktaAssignment(ctx, request.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	// Set the cleanup time to now to trigger a cleanup.
	assignment.SetCleanupTime(a.clock.Now())
	if _, err := a.oktaClient.UpdateOktaAssignment(ctx, assignment); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err, "error marking assignment for cleanup")
	}

	a.accessRequestsMu.Lock()
	delete(a.accessRequests, request.GetName())
	a.accessRequestsMu.Unlock()

	return nil
}

// matcher will match user groups or applications that are sourced from Okta.
func (a *AccessRequestReconciler) matcher(ctx context.Context, accessRequest types.AccessRequest) bool {
	// Look for all requested resource IDs for any user groups or apps that have an
	// origin label of types.OriginOkta
	matches := false

	for _, resourceID := range accessRequest.GetRequestedResourceIDs() {
		var resource types.ResourceWithLabels
		var err error

		// Skip resources that don't belong to this cluster.
		if resourceID.ClusterName != a.clusterName {
			continue
		}

		switch resourceID.Kind {
		case types.KindUserGroup:
			resource, err = a.accessPoint.GetUserGroup(ctx, resourceID.Name)
			if err != nil {
				a.log.Debugf("Error getting user group: %v", err)
			}
		case types.KindApp:
			resource, err = a.getAppServer(ctx, resourceID.Name)
			if err != nil {
				a.log.Debugf("Error getting application: %v", err)
			}
		}

		if resource != nil && resource.Origin() == types.OriginOkta {
			matches = true
			break
		}
	}

	state := accessRequest.GetState()
	return matches && (state == types.RequestState_APPROVED || state == types.RequestState_DENIED)
}

// accessRequestToOktaAssignment will take an access request and convert it into an Okta assignment.
func (a *AccessRequestReconciler) accessRequestToOktaAssignment(ctx context.Context, accessRequest types.AccessRequest, assignmentStatus string) (types.OktaAssignment, error) {
	targets := []*types.OktaAssignmentTargetV1{}

	// Look for the requested targets in the access request.
	for _, resourceID := range accessRequest.GetRequestedResourceIDs() {
		var targetType types.OktaAssignmentTargetV1_OktaAssignmentTargetType
		var id string

		// Skip resources that don't belong to this cluster.
		if resourceID.ClusterName != a.clusterName {
			continue
		}

		switch resourceID.Kind {
		case types.KindUserGroup:
			userGroup, err := a.accessPoint.GetUserGroup(ctx, resourceID.Name)
			if err != nil {
				// If the user group no longer exists, we'll try the next resource.
				if trace.IsNotFound(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}

			if userGroup.Origin() == types.OriginOkta {
				targetType = types.OktaAssignmentTargetV1_GROUP
				id = userGroup.GetName()
			}
		case types.KindApp:
			appServer, err := a.getAppServer(ctx, resourceID.Name)
			if err != nil {
				// If the application no longer exists, we'll try the next resource.
				if trace.IsNotFound(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}

			if appServer.Origin() == types.OriginOkta {
				targetType = types.OktaAssignmentTargetV1_APPLICATION
				id = appServer.GetName()
			}
		}

		// If an ID has been set, we'll add this target.
		if id != "" {
			target := &types.OktaAssignmentTargetV1{
				Type: targetType,
				Id:   id,
			}
			targets = append(targets, target)
		}
	}

	if len(targets) == 0 {
		return nil, trace.NotFound("no Okta targets found in access request")
	}

	cleanupTime := accessRequest.GetAccessExpiry()

	assignment, err := types.NewOktaAssignment(types.Metadata{
		Name: accessRequest.GetName(),
		Labels: map[string]string{
			teleport.OktaAssignmentSourceLabel: fmt.Sprintf(accessRequestFormat, accessRequest.GetName()),
		},
	}, types.OktaAssignmentSpecV1{
		User:        accessRequest.GetUser(),
		Targets:     targets,
		CleanupTime: cleanupTime,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := assignment.SetStatus(assignmentStatus); err != nil {
		return nil, trace.Wrap(err)
	}
	assignment.SetLastTransition(a.clock.Now())

	return assignment, nil
}

// getAppServer will get the app server corresponding to the given name.
func (a *AccessRequestReconciler) getAppServer(ctx context.Context, name string) (types.AppServer, error) {
	req := proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		Limit:               1,
		PredicateExpression: fmt.Sprintf(`name == %q`, name),
	}

	resp, err := a.accessPoint.ListResources(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(resp.Resources) != 1 {
		return nil, trace.NotFound("found %d resources when getting application %s", len(resp.Resources), name)
	}

	appServer, ok := resp.Resources[0].(types.AppServer)
	if !ok {
		return nil, trace.BadParameter("expected types.ApplicationServer, found %T", resp.Resources[0])
	}
	return appServer, nil
}

// OnLogin's job is to mark assignments cleaned up when a lock is encountered or to restore assignments when a
// lock is removed.
func (a *AccessRequestReconciler) OnLogin(ctx context.Context, user types.User) error {
	locks := a.lockWatcher.GetCurrent()
	userLocked := false
	accessRequestsLocked := map[string]struct{}{}
	for _, lock := range locks {
		if lock.Target().User == user.GetName() {
			userLocked = true
			break
		}
		if lock.Target().AccessRequest != "" {
			accessRequestsLocked[lock.Target().AccessRequest] = struct{}{}
		}
	}

	// Cycle through all access requests, looking for access requests that belong to the
	// given user.
	accessRequests := a.getAccessRequests()
	for accessRequestName, accessRequest := range accessRequests {
		// This access request is already expired, so no need to process it. Its
		// corresponding Okta assignment should also be expired.
		if a.clock.Now().After(accessRequest.Expiry()) {
			continue
		}

		// This access request doesn't target this user.
		if accessRequest.GetUser() != user.GetName() {
			continue
		}

		assignment, err := a.oktaClient.GetOktaAssignment(ctx, accessRequestName)
		// Access requests for non-Okta resource are expected to have no corresponding
		// Okta assignment.
		if trace.IsNotFound(err) {
			continue
		} else if err != nil {
			return trace.Wrap(err)
		}

		assignmentNeedsUpdate := false
		_, accessRequestLocked := accessRequestsLocked[accessRequest.GetName()]

		needsLock := userLocked || accessRequestLocked

		if needsLock && accessRequest.Expiry() == assignment.GetCleanupTime() {
			// If the user or access request is locked and the cleanup time matches the access request expiry time,
			// update the assignment so that it gets cleaned up immediately.
			assignmentNeedsUpdate = true
			assignment.SetFinalized(false)
			assignment.SetCleanupTime(a.clock.Now())
		} else if !needsLock && accessRequest.Expiry() != assignment.GetCleanupTime() {
			// If the user or access request is not locked and the cleanup time does not match the access request
			// expiry time, update the assignment so that the assignment gets re-processed.
			assignmentNeedsUpdate = true
			assignment.SetCleanupTime(accessRequest.Expiry())
		}

		if assignmentNeedsUpdate {
			a.log.Debugf("Assignment %s updated", assignment.GetName())
			_, err = a.oktaClient.UpdateOktaAssignment(ctx, assignment)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

func copyAccessRequestMapToAccessRequests(accessRequests map[string]types.AccessRequest) types.AccessRequests {
	accessRequestsCopy := types.AccessRequests(make([]types.AccessRequest, 0, len(accessRequests)))
	for _, accessRequest := range accessRequests {
		accessRequestsCopy = append(accessRequestsCopy, accessRequest.Copy())
	}

	return accessRequestsCopy
}
