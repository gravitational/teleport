package okta

import (
	"context"
	"crypto/tls"
	"errors"
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
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaauditlogs "github.com/gravitational/teleport/e/lib/okta/audit_logs"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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

	// AssignmentsService is the service for managing Okta assignments.
	AssignmentsService oktacommon.OktaAssignmentService
}

var (
	ErrMissingAppId          = trace.BadParameter("user sync with Okta SAML app as a source is enabled, but app ID is missing")
	ErrMissingSsoConnectorId = trace.BadParameter("user sync is enabled, but Okta SSO Connector is missing")
	ErrMissingDefaultOwners  = trace.BadParameter("default owners are missing")
)

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
	if c.AssignmentsService == nil {
		return trace.BadParameter("OktaAssignmentService is missing")
	}
	if _, ok := c.AssignmentsService.(*cache.Cache); ok {
		return trace.BadParameter("AssignmentsService must not be a cache; A non-cached service is required to fetch up-to-date assignments state")
	}

	if c.SyncSettings.SyncUsers {
		if c.SyncSettings.SsoConnectorId == "" {
			return ErrMissingSsoConnectorId
		}
		if c.SyncSettings.AppId == "" && c.SyncSettings.GetUserSyncSource() == types.OktaUserSyncSourceSamlApp {
			return ErrMissingAppId
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
			return ErrMissingDefaultOwners
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
	accessLists services.AccessLists
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
	// assignDefaultRoles controls whether the builtin "okta-requester" role should be
	// assigned to the synchronized users.
	assignDefaultRoles bool

	// connectorService is the SAML connector service.
	connectorService sso.SAMLConnectorService

	// disableOktaAppGroupSync allows to disable Okta application and group sync.
	// when only SCIM or user sync integration is needed.
	disableOktaAppGroupSync bool

	// serviceStatus holds the serviceStatus information for the service and
	// broadcasts changes as necessary.
	serviceStatus *serviceStatus
}

// New will create a new Okta service.
func New(ctx context.Context, config Config) (*Service, error) {
	return newWithClientCreator(ctx, config, oktaapi.New)
}

// newWithClientCreator will create a new Okta service with the given oktaClient.
func newWithClientCreator(ctx context.Context, config Config, creator oktaapi.OktaClientFn) (service *Service, err error) {
	var connectorInfo types.SAMLConnector

	// Fetch related SAML connector for status
	if config.SyncSettings.SsoConnectorId != "" {
		var err error
		if connectorInfo, err = config.ConnectorService.GetSAMLConnector(ctx, config.SyncSettings.SsoConnectorId, false); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	oktaStatus := NewPluginOktaStatus(PluginOktaStatusParams{
		SsoConnector: connectorInfo,
		SyncSettings: config.SyncSettings,
		ScimEnabled:  config.SCIMEnabled,
	})
	serviceStatus := &serviceStatus{
		sink:    config.PluginStatusSink,
		code:    types.PluginStatusCode_UNKNOWN,
		logger:  config.Logger,
		details: oktaStatus,
	}

	var (
		userSyncEnabled          = config.SyncSettings.GetEnableUserSync()
		appGroupSyncEnabled      = config.SyncSettings.GetEnableAppGroupSync()
		accessListSyncEnabled    = config.SyncSettings.GetEnableAccessListSync()
		bidirectionalSyncEnabled = config.SyncSettings.GetEnableBidirectionalSync()
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

	// NOTE: Since we handle plugin status here, it's important no errors are returned before
	// the defer call below.

	defer func() {
		if err != nil {
			errorCode, startErr := getPluginStartError(err)
			if userSyncEnabled {
				serviceStatus.UpdateUserSync(ctx, config.Clock.Now(), 0, startErr)
			}
			if appGroupSyncEnabled {
				serviceStatus.UpdateAppGroupSync(ctx, config.Clock.Now(), 0, 0, startErr)
			}
			if accessListSyncEnabled {
				serviceStatus.UpdateAccessListSync(ctx, config.Clock.Now(), 0, 0, startErr)
			}

			ReportPluginStatusError(ctx, config.Logger, config.PluginStatusSink, errorCode, oktaStatus, startErr.Error())
		} else {
			ReportPluginStatus(ctx, config.Logger, config.PluginStatusSink, types.PluginStatusCode_RUNNING, oktaStatus)
		}
	}()

	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	oktaClient, err := creator(ctx, oktaapi.Config{
		OrgUrl:       config.OktaAPIEndpoint,
		AuthProvider: config.AuthProvider,
		Log:          config.Logger,
		Scopes:       oktacommon.GetOAuthScopesForSyncSettings(&config.SyncSettings),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating Okta client")
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
		accessLists:             config.AccessLists,
		onHeartbeat:             config.OnHeartbeat,
		client:                  oktaClient,
		orgURL:                  strings.TrimSuffix(oktaClient.GetOrgUrl(), "/"),
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
		assignDefaultRoles:      config.SyncSettings.GetAssignDefaultRoles(),
		disableOktaAppGroupSync: config.SyncSettings.DisableSyncAppGroups,
		serviceStatus:           serviceStatus,
	}
	s.tlsConfig = app.CopyAndConfigureTLS(s.logger, s.accessPoint, config.TLSConfig)

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
			return nil, trace.Wrap(err)
		}
	} else {
		config.Logger.InfoContext(ctx, "User sync is disabled")
	}

	if appGroupSyncEnabled {
		config.Logger.InfoContext(ctx, "App and Group sync is enabled", "bidirectional", bidirectionalSyncEnabled)

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
			return nil, trace.Wrap(err)
		}

		if bidirectionalSyncEnabled {
			clusterName, err := s.accessPoint.GetClusterName(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			s.assignmentReconciler = newAssignmentReconciler(clusterName.GetClusterName(), s)
		}
	} else {
		config.Logger.InfoContext(ctx, "App and Group sync is disabled")
	}

	if accessListSyncEnabled {
		config.Logger.InfoContext(ctx, "Access List sync is enabled", "bidirectional", bidirectionalSyncEnabled)
		s.accessListSync, err = newAccessListSync(accessListSyncConfig{
			Logger:                s.logger,
			Clock:                 s.clock,
			ClusterName:           s.clusterName,
			Client:                s.client,
			Emitter:               config.Emitter,
			Access:                config.Access,
			AccessLists:           config.AccessLists,
			OrgURL:                s.orgURL,
			Owners:                config.SyncSettings.DefaultOwners,
			AppsGetter:            s.apps.Clone,
			GroupsGetter:          s.groups.Clone,
			AppFilters:            config.accessListSyncAppFilters,
			GroupFilters:          config.accessListSyncGroupFilters,
			ServiceStatus:         s.serviceStatus,
			SynchronizerSuccess:   &s.synchronizerSuccess,
			SynchronizingMu:       &s.synchronizingMu,
			StopChannel:           s.stopCh,
			OktaAssignmentService: config.AssignmentsService,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		config.Logger.InfoContext(ctx, "Access List sync is disabled")
	}

	return s, nil
}

func getPluginStartError(err error) (types.PluginStatusCode, error) {
	msg := ""
	switch {
	case errors.Is(err, ErrMissingAppId):
		msg = "Okta SAML app ID is missing: Verify your API Services application in Okta can access your SAML application as part of the defined resource set and has all necessary scopes granted, or try setting up User Sync again."
	case errors.Is(err, ErrMissingSsoConnectorId):
		msg = "SSO Connector ID is missing: Verify your SSO Connector is configured correctly, or try setting up the integration again."
	case errors.Is(err, ErrMissingDefaultOwners):
		msg = "Default Owners are missing: Verify you have provided Default Access List Owners, or try setting up App and Group Sync again."
	}

	if msg != "" {
		return types.PluginStatusCode_OKTA_CONFIG_ERROR, errors.New(msg)
	}
	return types.PluginStatusCode_OTHER_ERROR, err
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

type StartIntegrationOpts struct {
	AccessGraphConfig  servicecfg.AccessGraphConfig
	BootstrapStartDate time.Time
	ClusterFeatures    func() proto.Features
	GetCreds           accessgraph.ClientCredentialsGetter
}

func (s *Service) StartSystemLogExporter(ctx context.Context, cfg StartIntegrationOpts) {
	svc, err := oktaauditlogs.New(
		ctx,
		oktaauditlogs.Config{
			Client:             s.client,
			Clock:              s.clock,
			Logger:             s.logger,
			AccessGraphConfig:  cfg.AccessGraphConfig,
			HostID:             s.hostID,
			AccessPoint:        s.accessPoint,
			GetCreds:           cfg.GetCreds,
			ClusterFeatures:    cfg.ClusterFeatures,
			BootstrapStartDate: cfg.BootstrapStartDate,
			ReportStatus:       s.serviceStatus.UpdateSystemLogExporter,
		},
	)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to start Okta SIEM integration", "error", err)
		return
	}
	go func() {
		err := svc.Run(ctx)
		if err != nil {
			s.logger.ErrorContext(ctx, "Okta SIEM integration failed", "error", err)
			return
		}
		s.logger.InfoContext(ctx, "Okta SIEM integration stopped")
	}()
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
