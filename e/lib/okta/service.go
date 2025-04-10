package okta

import (
	"context"
	"crypto/tls"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/app"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// APICallsPerSecond 4 requests per second is the absolute maximum okta will allow due to End User Rate Limits.
	// https://developer.okta.com/docs/reference/rl-additional-limits/#end-user-rate-limits
	APICallsPerSecond     = 4
	RequestTimeoutSeconds = 300 // Okta request timeout is 5 minutes.

	// OktaServiceSemaphoreKind is the name of the semaphore to acquire.
	OktaServiceSemaphoreKind = "okta-service"
)

var (
	// OktaDefaultTimeBetweenSyncs to running synchronizations every half hour.
	OktaDefaultTimeBetweenSyncs = 30 * time.Minute
)

// ProxyGetter is an interface for retrieving proxy IDs.
type ProxyGetter interface {
	GetProxyIDs() []string
}

// Config is the configuration for the Okta service.
type Config struct {
	Leader isLeaderGetter
	// Log is the logger for the Okta config.
	Logger *slog.Logger

	// Clock is the clock to use for this service.
	Clock clockwork.Clock

	// TLSConfig is the TLS configuration for the Okta service which handles
	// HTTP redirects.
	TLSConfig *tls.Config

	// Authorizer is the authorizer for the Okta service.
	Authorizer authz.Authorizer

	// ClusterName is the name of the cluster.
	ClusterName string

	// Hostname is the hostname.
	Hostname string

	// HostID is the ID of this host.
	HostID string

	// RotationGetter gets the rotation for this server.
	RotationGetter services.RotationGetter

	// ProxyGetter returns a list of proxies.
	ProxyGetter ProxyGetter

	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter

	// AccessPoint is the caching access point used by the Okta service to
	// manipulate the Teleport cluster.
	AccessPoint authclient.OktaAccessPoint

	// Access is the service for interacting with roles.
	Access services.Access

	// AccessLists is the service for interacting with access lists.
	AccessLists services.AccessLists

	// OnHeartbeat is called after every heartbeat. Used to update process state.
	OnHeartbeat func(error)

	// OktaAPIEndpoint is the API endpoint to use for interacting with Okta.
	OktaAPIEndpoint string

	// TimeBetweenSyncs is the amount of time between synchronization calls.
	TimeBetweenSyncs time.Duration

	// BackendTasksPerSecond is the number of backend modifying tasks that can be run per second.
	BackendTasksPerSecond int

	// PluginStatusSink is an optional status sink for reporting the plugin status.
	PluginStatusSink common.StatusSink

	// accessListSyncAppFilters is the list of app filters for use by the access list sync.
	accessListSyncAppFilters []*regexp.Regexp

	// accessListSyncGroupFilters is the list of group filters for use by the access list sync.
	accessListSyncGroupFilters []*regexp.Regexp

	// ConnectorService is the SAML connector service.
	ConnectorService sso.SAMLConnectorService

	// SyncSettings are Okta plugin sync settings.
	SyncSettings types.PluginOktaSyncSettings

	// SCIMEnabled indicates that SCIM support is enabled for this instance
	SCIMEnabled bool

	// AuthProvider is the auth provider for the Okta service.
	AuthProvider oktaapi.AuthProvider
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, eteleport.ComponentOkta)
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.TLSConfig == nil {
		return trace.BadParameter("TLS config is missing")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("cluster name is missing")
	}
	if c.Hostname == "" {
		return trace.BadParameter("hostname is missing")
	}
	if c.HostID == "" {
		return trace.BadParameter("host ID is missing")
	}
	if c.RotationGetter == nil {
		return trace.BadParameter("rotation getter is missing")
	}
	if c.ProxyGetter == nil {
		c.ProxyGetter = reversetunnel.NewConnectedProxyGetter()
	}
	if c.Emitter == nil {
		return trace.BadParameter("emitter is missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point is missing")
	}
	if c.OnHeartbeat == nil {
		return trace.BadParameter("OnHeartbeat is missing")
	}
	if c.OktaAPIEndpoint == "" {
		return trace.BadParameter("Okta API endpoint is missing")
	}
	if c.ConnectorService == nil {
		return trace.BadParameter("ConnectorService service is missing")
	}
	if c.TimeBetweenSyncs == 0 {
		c.TimeBetweenSyncs = OktaDefaultTimeBetweenSyncs
	}
	if c.BackendTasksPerSecond == 0 {
		// Default to running 5 backend tasks per second.
		c.BackendTasksPerSecond = 5
	}
	if c.SyncSettings.SyncUsers {
		if c.SyncSettings.SsoConnectorId == "" {
			return trace.BadParameter("user sync is enabled, but Okta SSO Connector is missing")
		}
		if c.SyncSettings.AppId == "" && c.SyncSettings.GetUserSyncSource() == types.OktaUserSyncSourceSamlApp {
			return trace.BadParameter("user sync with Okta SAML app as a source is enabled, but app ID is missing")
		}
	}

	if c.SyncSettings.SyncAccessLists {
		if c.Access == nil {
			return trace.BadParameter("access is missing")
		}
		if c.AccessLists == nil {
			return trace.BadParameter("access lists is missing")
		}
		if len(c.SyncSettings.DefaultOwners) == 0 {
			return trace.BadParameter("default owners is missing")
		}

		for _, filter := range c.SyncSettings.AppFilters {
			compiledFilter, err := utils.CompileExpression(filter)
			if err != nil {
				return trace.Wrap(err)
			}
			c.accessListSyncAppFilters = append(c.accessListSyncAppFilters, compiledFilter)
		}

		for _, filter := range c.SyncSettings.GroupFilters {
			compiledFilter, err := utils.CompileExpression(filter)
			if err != nil {
				return trace.Wrap(err)
			}
			c.accessListSyncGroupFilters = append(c.accessListSyncGroupFilters, compiledFilter)
		}
	}
	if c.AuthProvider == nil {
		return trace.BadParameter("auth provider is missing")
	}

	return nil
}

// Service is the core data for the Okta integration service. The running
// service synchronizes data with an upstream Okta IdP.
type Service struct {
	leader     isLeaderGetter
	logger     *slog.Logger
	clock      clockwork.Clock
	tlsConfig  *tls.Config
	authorizer authz.Authorizer

	clusterName    string
	hostname       string
	hostID         string
	rotationGetter services.RotationGetter
	proxyGetter    ProxyGetter

	// accessPoint is a caching AccessPoint with Okta Extensions, used by this
	// service to interact with the Teleport cluster.
	accessPoint authclient.OktaAccessPoint
	onHeartbeat func(error)
	client      oktaapi.Interface
	emitter     apievents.Emitter
	orgURL      string

	// rateLimiter will rate limit backend interactions.
	rateLimiter *rate.Limiter

	// Heartbeat monitoring
	heartbeatsMu sync.Mutex
	heartbeats   map[string]*srv.Heartbeat

	// groupsReconciler will reconcile groups discovered in Okta.
	groupsReconciler *services.Reconciler[types.UserGroup]

	// groups is the current mapping of groups.
	groups utils.SyncMap[string, types.UserGroup]

	// newGroups is the mapping of groups discovered by Okta, not yet synchronized
	// to the group reconciler.
	newGroups utils.SyncMap[string, types.UserGroup]

	// group stats for the audit even for a particular reconcile.
	groupsAdded   []*apievents.OktaResource
	groupsUpdated []*apievents.OktaResource
	groupsDeleted []*apievents.OktaResource

	// appsReconciler will reconcile applications discovered in Okta.
	appsReconciler *services.Reconciler[types.Application]

	// apps is the current mapping of { appName => app }.
	apps utils.SyncMap[string, types.Application]

	// newApps is the mapping of { appName => app } for apps discovered
	// by Okta, not yet synchronzied to the apps map by the reconciler.
	newApps utils.SyncMap[string, types.Application]

	// app stats for the audit even for a particular reconcile.
	appsAdded   []*apievents.OktaResource
	appsUpdated []*apievents.OktaResource
	appsDeleted []*apievents.OktaResource

	// labelsMu protects access to the *IRMapping and *NameRegexes.
	labelMu sync.RWMutex

	// Import Rule mapping and regexes for the Okta objects.
	groupIRMapping       map[string]prioritizedLabels
	applicationIRMapping map[string]prioritizedLabels
	groupNameRegexes     []regexAndPriorityLabels
	appNameRegexes       []regexAndPriorityLabels

	timeBetweenSyncs time.Duration

	assignmentReconciler *assignmentReconciler

	syncStoppedChCloser sync.Once
	syncStoppedCh       chan struct{}

	stopChCloser sync.Once
	stopCh       chan struct{}

	shutdownCalled atomic.Bool
	closeCalled    atomic.Bool

	// synchronizerSuccess will be set to true if the synchronizer has completed at least
	// once successfully.
	synchronizerSuccess atomic.Bool

	synchronizingMu sync.RWMutex

	// userReconciler is used to reconcile the Teleport user DB with an upstream
	// Okta organization. If this value is `nil` it means that user syncing is
	// disabled via config.
	userReconciler *userReconciler

	// ssoConnectorID is the ID of the SSO connector managing the login for
	// users associated with this integration. May be empty if user sync is
	// disabled (i.e. if `userReconciler` is `nil`)
	ssoConnectorID string

	// accessListSync is used to synchronize and import user permissions from
	// Okta into Teleport, using access lists and roles to represent them. If
	// this value is `nil`, it means that access list sync is disabled via
	// config.
	accessListSync *accessListSync

	// oktaSAMLAppID is the Okta-assigned ID for the SAML app that the service
	// uses as a gateway for syncing users. If empty, the service will revert to
	// the legacy method of polling the whole Okta organization.
	oktaSAMLAppID string

	// userSyncSource indicates the source of truth of the Okta users. It can be either
	// connector SAML app or the whole Okta organization.
	userSyncSource types.OktaUserSyncSource

	// connectorService is the SAML connector service.
	connectorService sso.SAMLConnectorService

	// disableOktaAppGroupSync allows to disable Okta application and group sync.
	// when only SCIM or user sync integration is needed.
	disableOktaAppGroupSync bool

	// serviceStatus holds the serviceStatus information for the service and
	// broadcasts changes as necessary.
	serviceStatus serviceStatus
}

// New will create a new Okta service.
func New(ctx context.Context, config Config) (*Service, error) {
	return newWithClientCreator(ctx, config, oktaapi.New)
}

// newWithClientCreator will create a new Okta service with the given oktaClient.
func newWithClientCreator(ctx context.Context, config Config, creator oktaapi.OktaClientFn) (service *Service, err error) {
	oktaStatus := NewPluginOktaStatus(PluginOktaStatusParams{
		SyncSettings: config.SyncSettings,
		ScimEnabled:  config.SCIMEnabled,
	})

	defer func() {
		if err != nil {
			ReportPluginStatus(ctx, config.Logger, config.PluginStatusSink,
				types.PluginStatusCode_OTHER_ERROR,
				oktaStatus)

		} else {
			ReportPluginStatus(ctx, config.Logger, config.PluginStatusSink,
				types.PluginStatusCode_RUNNING,
				oktaStatus)
		}
	}()

	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var (
		userSyncEnabled          = config.SyncSettings.SyncUsers
		appGroupSyncEnabled      = !config.SyncSettings.DisableSyncAppGroups
		accessListSyncEnabled    = config.SyncSettings.SyncAccessLists
		bidirectionalSyncEnabled = !config.SyncSettings.DisableBidirectionalSync
	)

	if !userSyncEnabled {
		if accessListSyncEnabled {
			config.Logger.WarnContext(ctx, "Access List sync enabled but User sync disabled, proceeding without enabling Access List sync")
		}
		appGroupSyncEnabled = false
		accessListSyncEnabled = false
		bidirectionalSyncEnabled = false
	}

	if !appGroupSyncEnabled {
		if accessListSyncEnabled {
			config.Logger.WarnContext(ctx, "Access List sync enabled but App and Group sync disabled, proceeding without enabling Access List sync")
		}
		accessListSyncEnabled = false
		bidirectionalSyncEnabled = false
	}

	if !accessListSyncEnabled {
		bidirectionalSyncEnabled = false
	}

	s := &Service{
		leader:                  config.Leader,
		connectorService:        config.ConnectorService,
		logger:                  config.Logger,
		clock:                   config.Clock,
		authorizer:              config.Authorizer,
		clusterName:             config.ClusterName,
		hostname:                config.Hostname,
		hostID:                  config.HostID,
		rotationGetter:          config.RotationGetter,
		proxyGetter:             config.ProxyGetter,
		accessPoint:             config.AccessPoint,
		onHeartbeat:             config.OnHeartbeat,
		emitter:                 config.Emitter,
		rateLimiter:             rate.NewLimiter(rate.Every(time.Second/time.Duration(config.BackendTasksPerSecond)), 1),
		heartbeats:              map[string]*srv.Heartbeat{},
		groupIRMapping:          map[string]prioritizedLabels{},
		applicationIRMapping:    map[string]prioritizedLabels{},
		groupNameRegexes:        []regexAndPriorityLabels{},
		appNameRegexes:          []regexAndPriorityLabels{},
		timeBetweenSyncs:        config.TimeBetweenSyncs,
		syncStoppedCh:           make(chan struct{}, 1),
		stopCh:                  make(chan struct{}, 1),
		ssoConnectorID:          config.SyncSettings.SsoConnectorId,
		oktaSAMLAppID:           config.SyncSettings.AppId,
		userSyncSource:          config.SyncSettings.GetUserSyncSource(),
		disableOktaAppGroupSync: config.SyncSettings.DisableSyncAppGroups,
		serviceStatus: serviceStatus{
			sink:    config.PluginStatusSink,
			code:    types.PluginStatusCode_UNKNOWN,
			logger:  config.Logger,
			details: oktaStatus,
		},
	}
	// TODO(tross) pass in config.Logger once this supports slog. Until then
	// it will create a logger if the passed in logger is nil.
	s.tlsConfig = app.CopyAndConfigureTLS(nil, s.accessPoint, config.TLSConfig)

	scopes := []string{
		oktaapi.ScopeUserRead,
		oktaapi.ScopeUserManage,
		oktaapi.ScopeAppsRead,
		oktaapi.ScopeGroupsRead,
	}

	if accessListSyncEnabled && bidirectionalSyncEnabled {
		// If app and group sync is enabled, add the necessary scopes.
		// to manage apps and groups assignments in Okta.
		scopes = append(scopes, []string{
			oktaapi.ScopeAppsManage,
			oktaapi.ScopeGroupsManage,
		}...)
	}

	client, err := creator(ctx, oktaapi.Config{
		OrgUrl:       config.OktaAPIEndpoint,
		AuthProvider: config.AuthProvider,
		Log:          config.Logger.With("okta", "client"),
		Scopes:       scopes,
	})
	if err != nil {
		if userSyncEnabled {
			s.serviceStatus.UpdateUserSync(ctx, config.Clock.Now(), 0, err)
		}
		if appGroupSyncEnabled {
			s.serviceStatus.UpdateAppGroupSync(ctx, config.Clock.Now(), 0, 0, err)
		}
		if accessListSyncEnabled {
			s.serviceStatus.UpdateAccessListSync(ctx, config.Clock.Now(), 0, 0, err)
		}
		return nil, trace.Wrap(err)
	}
	// Assign the client to the service.
	s.client = client
	s.orgURL = strings.TrimSuffix(client.GetOrgUrl(), "/")

	if userSyncEnabled {
		config.Logger.InfoContext(ctx, "User sync is enabled")

		var err error
		s.userReconciler, err = newUserReconciler(userReconcilerConfig{
			clusterName: config.ClusterName,
			teleportAP:  config.AccessPoint,
			logger:      config.Logger,
			userOrgURL:  config.OktaAPIEndpoint,
			emitter:     config.Emitter,
		})
		if err != nil {
			s.serviceStatus.UpdateUserSync(ctx, config.Clock.Now(), 0, err)
			return nil, trace.Wrap(err)
		}
	} else {
		config.Logger.InfoContext(ctx, "User sync is disabled")
	}

	if appGroupSyncEnabled {
		config.Logger.InfoContext(ctx, "App and Group sync is enabled")

		s.appsReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.Application]{
			Matcher:             s.appsMatcher,
			GetCurrentResources: s.apps.Clone,
			GetNewResources:     s.newApps.Clone,
			OnCreate:            s.onCreateApp,
			OnUpdate:            s.onUpdateApp,
			OnDelete:            s.onDeleteApp,
			Logger:              s.logger.With("kind", types.KindAppServer),
		})
		if err != nil {
			s.serviceStatus.UpdateAppGroupSync(ctx, config.Clock.Now(), 0, 0, err)
			return nil, trace.Wrap(err)
		}

		s.groupsReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.UserGroup]{
			Matcher:             s.groupMatcher,
			GetCurrentResources: s.groups.Clone,
			GetNewResources:     s.newGroups.Clone,
			OnCreate:            s.onCreateGroup,
			OnUpdate:            s.onUpdateGroup,
			OnDelete:            s.onDeleteGroup,
			Logger:              s.logger.With("kind", types.KindUserGroup),
		})
		if err != nil {
			s.serviceStatus.UpdateAppGroupSync(ctx, config.Clock.Now(), 0, 0, err)
			return nil, trace.Wrap(err)
		}
	} else {
		config.Logger.InfoContext(ctx, "App and Group sync is enabled", "bidirectional", bidirectionalSyncEnabled)
	}

	if accessListSyncEnabled {
		config.Logger.InfoContext(ctx, "Access List sync is enabled", "bidirectional", bidirectionalSyncEnabled)
		s.accessListSync, err = newAccessListSync(accessListSyncConfig{
			Logger:              s.logger,
			Clock:               s.clock,
			ClusterName:         s.clusterName,
			Client:              s.client,
			Emitter:             config.Emitter,
			Access:              config.Access,
			AccessLists:         config.AccessLists,
			OrgURL:              s.orgURL,
			Owners:              config.SyncSettings.DefaultOwners,
			AppsGetter:          s.apps.Clone,
			GroupsGetter:        s.groups.Clone,
			AppFilters:          config.accessListSyncAppFilters,
			GroupFilters:        config.accessListSyncGroupFilters,
			ServiceStatus:       &s.serviceStatus,
			SynchronizerSuccess: &s.synchronizerSuccess,
			SynchronizingMu:     &s.synchronizingMu,
			StopChannel:         s.stopCh,
		})
		if err != nil {
			s.serviceStatus.UpdateAppGroupSync(ctx, config.Clock.Now(), 0, 0, err)
			return nil, trace.Wrap(err)
		}

		if bidirectionalSyncEnabled {
			clusterName, err := s.accessPoint.GetClusterName(ctx)
			if err != nil {
				s.serviceStatus.UpdateAppGroupSync(ctx, config.Clock.Now(), 0, 0, err)
				return nil, trace.Wrap(err)
			}
			s.assignmentReconciler = newAssignmentReconciler(ctx, clusterName.GetClusterName(), s)
		}
	} else {
		config.Logger.InfoContext(ctx, "Access List sync is disabled")
	}

	return s, nil
}

// Start will start the Okta service. This service will not make any calls the Okta API while it is
// not the leader.
func (s *Service) Start(ctx context.Context) error {
	if s.groupsReconciler != nil {
		if err := s.seedGroupReconciler(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	go s.synchronizeLoop(ctx)

	if s.assignmentReconciler != nil {
		if err := s.assignmentReconciler.start(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	if s.accessListSync != nil {
		go s.accessListSync.startSync(ctx)
	}

	return nil
}

// Wait will wait for the Okta service to complete.
func (s *Service) Wait(ctx context.Context) {
	select {
	case <-s.syncStoppedCh:
	case <-ctx.Done():
		return
	}

	if s.assignmentReconciler != nil {
		s.assignmentReconciler.wait(ctx)
	}
}

// Shutdown will stop any processes that are currently running.
func (s *Service) Shutdown() error {
	// If we've already called shutdown, return.
	if !s.shutdownCalled.CompareAndSwap(false, true) {
		return nil
	}

	s.stopChCloser.Do(func() { close(s.stopCh) })
	s.syncStoppedChCloser.Do(func() { close(s.syncStoppedCh) })

	s.heartbeatsMu.Lock()
	defer s.heartbeatsMu.Unlock()

	if s.assignmentReconciler != nil {
		s.assignmentReconciler.stop()
	}

	var errs []error
	for _, heartbeat := range s.heartbeats {
		if err := heartbeat.Close(); err != nil {
			s.logger.ErrorContext(context.Background(), "Unable to close heartbeat", "error", err)
		}
	}

	return trace.NewAggregate(errs...)
}

// Close cleans up any lingering resources.
func (s *Service) Close(ctx context.Context) error {
	// If we've already called close, return.
	if !s.closeCalled.CompareAndSwap(false, true) {
		return nil
	}

	var errs []error
	if services.ShouldDeleteServerHeartbeatsOnShutdown(ctx) {
		for appName := range s.heartbeats {
			if err := s.accessPoint.DeleteApplicationServer(ctx, defaults.Namespace, s.hostID, appName); err != nil {
				if !trace.IsNotFound(err) {
					errs = append(errs, err)
				}
			}
		}
	}

	return trace.NewAggregate(errs...)
}
