package okta

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
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
	services.UserGetter

	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)

	// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
	GetInventoryConnectedServiceCount(service types.SystemRole) uint64
}

// AccessRequestReconcilerConfig is the configuration for the AccessRequestReconciler.
type AccessRequestReconcilerConfig struct {
	// Logger is the logger for the AccessRequestReconciler.
	Logger *slog.Logger

	// Clock is the clock to use for the reconciler.
	Clock clockwork.Clock

	// ClusterName is the name of the cluster.
	ClusterName string

	// LocKWatcher is the lock watcher for the reconciler.
	LockWatcher *services.LockWatcher

	// AccessPoint is the access point for the access request reconciler.
	AccessPoint AccessRequestReconcilerAccessPoint

	// OktaConnected is a utility that will detect if an Okta service is connected.
	OktaConnected *connected.OktaConnected

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
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, eteleport.ComponentOktaAccessRequestReconciler)
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

	if c.OktaConnected == nil {
		return trace.BadParameter("okta connected is missing")
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
	logger      *slog.Logger
	clock       clockwork.Clock
	clusterName string
	lockWatcher *services.LockWatcher

	accessPoint AccessRequestReconcilerAccessPoint
	connected   *connected.OktaConnected
	oktaClient  services.OktaAssignments
	onReconcile func(types.AccessRequests)

	watcherMu sync.Mutex
	watcher   *services.AccessRequestWatcher

	reconcileCh chan struct{}
	stopCh      chan struct{}

	accessRequests    *accessRequestSyncMap
	newAccessRequests *accessRequestSyncMap

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
		logger:                  config.Logger,
		clock:                   config.Clock,
		clusterName:             config.ClusterName,
		lockWatcher:             config.LockWatcher,
		accessPoint:             config.AccessPoint,
		onReconcile:             config.OnReconcile,
		connected:               config.OktaConnected,
		oktaClient:              config.OktaClient,
		reconcileCh:             make(chan struct{}, 1),
		stopCh:                  make(chan struct{}),
		accessRequests:          newAccessRequestSyncMap(),
		newAccessRequests:       newAccessRequestSyncMap(),
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

	for {
		newOktaServiceConnected := a.connected.IsConnected(ctx)
		if newOktaServiceConnected {
			serviceConnectionFailures = 0

			if !serviceStarted {
				a.logger.InfoContext(ctx, "Okta service connected to the auth server, starting the Okta access request reconciler")

				var err error
				cancel, resourcesCleaned, err = a.start(ctx)
				if err != nil {
					a.logger.ErrorContext(ctx, "error starting access request reconciler", "error", err)
					continue
				}
				a.logger.InfoContext(ctx, "Okta access request reconciler started")
				serviceStarted = true
			}
		} else if !newOktaServiceConnected && serviceStarted {
			serviceConnectionFailures++
			if serviceConnectionFailures >= maxOktaServiceConnectionFailures {
				a.logger.InfoContext(ctx, "Okta service has disconnected, stopping the access request reconciler")
				cancel()
				// wait for the resources to be cleaned up otherwise we can end up with
				// multiple reconcilers running at the same time for short periods of time
				// which cause tests to fail when both invoke retryer.Reset() at the same time.
				<-resourcesCleaned
				serviceStarted = false
				a.logger.InfoContext(ctx, "Okta access request reconciler has stopped")
			} else {
				a.logger.WarnContext(ctx, "No Okta service connected", "connection_failures", serviceConnectionFailures, "max_connection_failures", maxOktaServiceConnectionFailures)
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
		GetCurrentResources: a.accessRequests.CopyAsMap,
		GetNewResources:     a.newAccessRequests.CopyAsMap,
		OnCreate:            a.onCreate,
		OnUpdate:            a.onUpdate,
		OnDelete:            a.onDelete,
		Logger:              a.logger,
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
		case <-a.reconcileCh:
			if err := reconciler.Reconcile(ctx); err != nil {
				a.retryer.Inc()
				a.logger.ErrorContext(ctx, "Failed to reconcile", "backoff", a.retryer.Duration(), "error", err)

				// On error, we'll need to retry so that we re-attempt reconciliation. Otherwise,
				// reconciliation will only happen again when the next access request event comes in,
				// which could be indefinitely, and the associated OktaAssignment may never actually
				// be created.
				a.retryer.Inc()
				a.clock.AfterFunc(a.retryer.Duration(), a.queueReconcile)
			} else if a.onReconcile != nil {
				a.onReconcile(a.accessRequests.CopyAsSlice())
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

// queueReconcile will make sure reconciliation notification channel has a message in it. It is non-blocking.
func (a *AccessRequestReconciler) queueReconcile() {
	// reconcileCh is buffered so simply put a struct there or do nothing if it's full.
	select {
	case a.reconcileCh <- struct{}{}:
	default:
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

// startResourceWatcher starts watching changes to access request resources and
// create OktaAssignment resources.
func (a *AccessRequestReconciler) startResourceWatcher(ctx context.Context) (*services.AccessRequestWatcher, error) {
	a.logger.DebugContext(ctx, "Initializing access request resource watcher")
	watcher, err := services.NewAccessRequestWatcher(ctx, services.AccessRequestWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: eteleport.ComponentOktaAccessRequestReconciler,
			Logger:    a.logger,
			Client:    a.accessPoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer a.logger.DebugContext(ctx, "Access request resource watcher finished")

		for {
			select {
			case newAccessRequests := <-watcher.AccessRequestsC:
				a.newAccessRequests.ReplaceWith(newAccessRequests)

				a.queueReconcile()
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
		isUserLocal, err := a.isUserLocal(ctx, newAccessRequest.GetUser())
		if err != nil {
			return trace.Wrap(err)
		}
		if isUserLocal {
			a.logger.DebugContext(ctx, "Okta assignment cannot be created for non-SSO user", "user", newAccessRequest.GetUser())
			return nil
		}

		assignment, err := a.accessRequestToOktaAssignment(ctx, newAccessRequest, constants.OktaAssignmentStatusPending)
		if err != nil {
			if trace.IsNotFound(err) {
				a.logger.DebugContext(ctx, "access request cannot be processed", "error", err)
				return nil
			}
			return trace.Wrap(err)
		}

		// If the assignment already exists, register it locally and move on.
		_, err = a.oktaClient.CreateOktaAssignment(ctx, assignment)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		a.accessRequests.Insert(newAccessRequest)
	}

	return nil
}

// onUpdate will cleanup Okta assignments from access requests.
func (a *AccessRequestReconciler) onUpdate(ctx context.Context, updatedAccessRequest, _ types.AccessRequest) error {
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

		a.accessRequests.Insert(updatedAccessRequest)
	}

	return nil
}

// onUpdate will cleanup Okta assignments from access requests.
func (a *AccessRequestReconciler) onDelete(ctx context.Context, request types.AccessRequest) error {
	// No need to look at access request state, we should clean up the associated Okta assignments.
	assignment, err := a.oktaClient.GetOktaAssignment(ctx, request.GetName())
	if trace.IsNotFound(err) {
		// Nothing to reconcile. The Okta assignment is already gone.
	} else if err != nil {
		return trace.Wrap(err)
	} else {
		// Set the cleanup time to now to trigger a cleanup.
		assignment.SetCleanupTime(a.clock.Now())
		_, err := a.oktaClient.UpdateOktaAssignment(ctx, assignment)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "error marking assignment for cleanup")
		}
	}

	a.accessRequests.Delete(request)
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
				a.logger.DebugContext(ctx, "Error getting user group", "error", err)
			}
		case types.KindApp:
			resource, err = a.getAppServer(ctx, resourceID.Name)
			if err != nil {
				a.logger.DebugContext(ctx, "Error getting application", "error", err)
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
			eteleport.OktaAssignmentSourceLabel: fmt.Sprintf(accessRequestFormat, accessRequest.GetName()),
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
	// If no Okta service is connected, return immediately. Anything that is missed will be caught
	// once the Okta service connects and the usermonitor re-runs.
	if !a.connected.IsConnected(ctx) {
		return nil
	}

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
	accessRequests := a.accessRequests.CopyAsMap()
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
			a.logger.DebugContext(ctx, "Assignment updated", "assignment_name", assignment.GetName())
			_, err = a.oktaClient.UpdateOktaAssignment(ctx, assignment)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// isUserLocal will return true if the given user is a local user. If the user cannot be determined,
// it will return false, which means it will indicate that the user is an SSO user.
func (a *AccessRequestReconciler) isUserLocal(ctx context.Context, username string) (bool, error) {
	user, err := a.accessPoint.GetUser(ctx, username, false /* withSecrets */)
	if err != nil {
		// If we can't find the user, just return false, as we can't make a determination.
		if trace.IsNotFound(err) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	return user.GetUserType() == types.UserTypeLocal, nil
}

type accessRequestSyncMap struct {
	mu             sync.RWMutex
	accessRequests map[string]types.AccessRequest
}

func newAccessRequestSyncMap() *accessRequestSyncMap {
	return &accessRequestSyncMap{
		accessRequests: make(map[string]types.AccessRequest),
	}
}

func (m *accessRequestSyncMap) Insert(v types.AccessRequest) {
	m.mu.Lock()
	m.accessRequests[v.GetName()] = v
	m.mu.Unlock()
}

func (m *accessRequestSyncMap) Delete(v types.AccessRequest) {
	m.mu.Lock()
	delete(m.accessRequests, v.GetName())
	m.mu.Unlock()
}

func (m *accessRequestSyncMap) ReplaceWith(values types.AccessRequests) {
	newData := make(map[string]types.AccessRequest)
	for _, v := range values {
		newData[v.GetName()] = v
	}

	m.mu.Lock()
	m.accessRequests = newData
	m.mu.Unlock()
}

func (m *accessRequestSyncMap) CopyAsMap() map[string]types.AccessRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make(map[string]types.AccessRequest, len(m.accessRequests))
	for k, v := range m.accessRequests {
		res[k] = v.Copy()
	}

	return res
}

func (m *accessRequestSyncMap) CopyAsSlice() types.AccessRequests {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]types.AccessRequest, len(m.accessRequests))
	i := 0
	for _, v := range m.accessRequests {
		res[i] = v.Copy()
		i++
	}

	return res
}
