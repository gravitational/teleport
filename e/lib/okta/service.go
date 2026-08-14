package okta

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
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
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/app"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// APICallsPerSecond 4 requests per second is the absolute maximum okta will allow due to End User Rate Limits.
	// https://developer.okta.com/docs/reference/rl-additional-limits/#end-user-rate-limits
	APICallsPerSecond     = 4
	RequestTimeoutSeconds = 300 // Okta request timeout is 5 minutes.

	// OktaServiceSemaphoreKind is the kind of the semaphore used to elect a
	// single Okta plugin instance across all Auth servers.
	OktaServiceSemaphoreKind = "okta-service"

	// oktaAppServerHostID is a fake fixed host ID used for app servers created by Okta
	// integration. Since the integration moved from being agent-based to hosted we don't use
	// heartbeats anymore and using a fixed value for host ID simplifies the implementation.
	oktaAppServerHostID = "okta-integration"
)

// ProxyGetter is an interface for retrieving proxy IDs.
type ProxyGetter interface {
	GetProxyIDs() []string
}

// Config is the configuration for the Okta service.
type Config struct {
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

	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter

	// AccessPoint is the caching access point used by the Okta service to
	// manipulate the Teleport cluster.
	AccessPoint authclient.OktaAccessPoint

	// Access is the service for interacting with roles.
	Access services.Access

	// AccessLists is the service for interacting with access lists.
	AccessLists services.AccessLists

	// OktaAPIEndpoint is the API endpoint to use for interacting with Okta.
	OktaAPIEndpoint string

	// TimeBetweenImports is the amount of time between Okta to Teleport periodic syncs. This
	// setting can be configured with the okta.sync_settings.time_between_imports Okta plugin
	// value.
	TimeBetweenImports time.Duration

	// TimeBetweenAssignmentProcessLoops is the amount of time that has to pass between running
	// the assignments process loop. It also determines how often the cached Okta assignments
	// client is invalidated. This setting can be configured with the
	// okta.sync_settings.time_between_assignment_process_loops Okta plugin value.
	TimeBetweenAssignmentProcessLoops time.Duration

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

	// Backend is used to directly interact with the backend instead of fetching thought cache
	Backend Backend

	// TestHTTPClient is an optional HTTP client that can be used to override the
	// default client for testing. Do not set in production.
	TestHTTPClient *http.Client

	// Plugin is the Plugin object.
	Plugin types.Plugin

	// TargetProcessingBackoffStep is the step value for linear backoff when processing
	// Okta assignment targets.
	TargetProcessingBackoffStep time.Duration

	// TargetProcessingBackoffMax is the max duration for linear backoff when processing
	// Okta assignment targets.
	TargetProcessingBackoffMax time.Duration
}

// Backend groups the service interfaces backed by auth.Server.Services
// direct backend reads
type Backend interface {
	oktacommon.OktaAssignmentService
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
	if c.Emitter == nil {
		return trace.BadParameter("emitter is missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point is missing")
	}
	if c.OktaAPIEndpoint == "" {
		return trace.BadParameter("Okta API endpoint is missing")
	}
	if c.ConnectorService == nil {
		return trace.BadParameter("ConnectorService service is missing")
	}
	if c.TimeBetweenImports == 0 {
		return trace.BadParameter("TimeBetweenImports not set")
	}
	if c.TimeBetweenAssignmentProcessLoops == 0 {
		return trace.BadParameter("TimeBetweenAssignmentProcessLoops not set")
	}
	if c.BackendTasksPerSecond == 0 {
		// Default to running 5 backend tasks per second.
		c.BackendTasksPerSecond = 5
	}
	if c.Backend == nil {
		return trace.BadParameter("OktaAssignmentService is missing")
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

	if c.TargetProcessingBackoffStep == 0 {
		return trace.BadParameter("TargetProcessingBackoffStep not set")
	}
	if c.TargetProcessingBackoffMax == 0 {
		return trace.BadParameter("TargetProcessingBackoffMax not set")
	}

	return nil
}

// Service is the core data for the Okta integration service. The running
// service synchronizes data with an upstream Okta IdP.
type Service struct {
	logger     *slog.Logger
	clock      clockwork.Clock
	tlsConfig  *tls.Config
	authorizer authz.Authorizer

	clusterName string
	hostname    string
	hostID      string

	// accessPoint is a caching AccessPoint with Okta Extensions, used by this
	// service to interact with the Teleport cluster.
	accessPoint authclient.OktaAccessPoint
	accessLists services.AccessLists
	client      oktaapi.Interface
	emitter     apievents.Emitter
	orgURL      string

	// rateLimiter will rate limit backend interactions.
	rateLimiter *rate.Limiter

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

	// TODO(kopiczko) Extract apps reconciler code and get rid of the sync maps because they
	// are almost certainly not needed.

	// appsReconciler will reconcile applications discovered in Okta.
	appsReconciler *services.Reconciler[types.AppServer]

	// appServers is the current mapping of { appName => app }.
	appServers utils.SyncMap[string, types.AppServer]

	// newAppServers is the mapping of { appName => app } for apps discovered
	// by Okta, not yet synchronzied to the apps map by the reconciler.
	newAppServers utils.SyncMap[string, types.AppServer]

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

	// timeBetweenImports is the amount of time between Okta to Teleport periodic syncs. This
	// setting can be configured with the okta.sync_settings.time_between_imports Okta plugin
	// value.
	timeBetweenImports time.Duration

	// timeBetweenAssignmentProcessLoops is the amount of time that has to pass between running
	// the assignments process loop. It also determines how often the cached Okta assignments
	// client is invalidated. This setting can be configured with the
	// okta.sync_settings.time_between_assignment_process_loops Okta plugin value.
	timeBetweenAssignmentProcessLoops time.Duration

	assignmentReconciler *assignmentReconciler

	syncStoppedChCloser sync.Once
	syncStoppedCh       chan struct{}

	stopChCloser sync.Once
	stopCh       chan struct{}

	shutdownCalled atomic.Bool

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

	// targetProcessingBackoffStep is the step value for linear backoff when processing
	// Okta assignment targets.
	targetProcessingBackoffStep time.Duration
	// targetProcessingBackoffMax is the max duration for linear backoff when processing
	// Okta assignment targets.
	targetProcessingBackoffMax time.Duration
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
		OrgUrl:         config.OktaAPIEndpoint,
		AuthProvider:   config.AuthProvider,
		Log:            config.Logger,
		Scopes:         oktacommon.GetOAuthScopesForSyncSettings(&config.SyncSettings),
		TestHTTPClient: config.TestHTTPClient,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating Okta client")
	}

	s := &Service{
		connectorService:                  config.ConnectorService,
		logger:                            config.Logger,
		clock:                             config.Clock,
		authorizer:                        config.Authorizer,
		clusterName:                       config.ClusterName,
		hostname:                          config.Hostname,
		hostID:                            config.HostID,
		accessPoint:                       config.AccessPoint,
		accessLists:                       config.AccessLists,
		client:                            oktaClient,
		orgURL:                            strings.TrimSuffix(oktaClient.GetOrgUrl(), "/"),
		emitter:                           config.Emitter,
		rateLimiter:                       rate.NewLimiter(rate.Every(time.Second/time.Duration(config.BackendTasksPerSecond)), 1),
		groupIRMapping:                    map[string]prioritizedLabels{},
		applicationIRMapping:              map[string]prioritizedLabels{},
		groupNameRegexes:                  []regexAndPriorityLabels{},
		appNameRegexes:                    []regexAndPriorityLabels{},
		timeBetweenImports:                config.TimeBetweenImports,
		timeBetweenAssignmentProcessLoops: config.TimeBetweenAssignmentProcessLoops,
		syncStoppedCh:                     make(chan struct{}, 1),
		stopCh:                            make(chan struct{}, 1),
		ssoConnectorID:                    config.SyncSettings.SsoConnectorId,
		oktaSAMLAppID:                     config.SyncSettings.AppId,
		userSyncSource:                    config.SyncSettings.GetUserSyncSource(),
		assignDefaultRoles:                config.SyncSettings.GetAssignDefaultRoles(),
		disableOktaAppGroupSync:           config.SyncSettings.DisableSyncAppGroups,
		serviceStatus:                     serviceStatus,
		targetProcessingBackoffStep:       config.TargetProcessingBackoffStep,
		targetProcessingBackoffMax:        config.TargetProcessingBackoffMax,
	}
	s.tlsConfig = app.CopyAndConfigureTLSForCluster(s.logger, s.accessPoint, config.ClusterName, config.TLSConfig)

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

		s.appsReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.AppServer]{
			Matcher:             s.appsMatcher,
			CompareResources:    func(a, b types.AppServer) int { return services.CompareServers(a, b) },
			GetCurrentResources: s.appServers.Clone,
			GetNewResources:     s.newAppServers.Clone,
			OnCreate:            s.onCreateAppServer,
			OnUpdate:            s.onUpdateAppServer,
			OnDelete:            s.onDeleteAppServer,
			Logger:              s.logger.With("kind", types.KindAppServer),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		s.groupsReconciler, err = services.NewReconciler(services.ReconcilerConfig[types.UserGroup]{
			CompareResources:    func(ug1, ug2 types.UserGroup) int { return services.EqualFromBool(ug1.IsEqual(ug2)) },
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
			s.assignmentReconciler = newAssignmentReconciler(s)
		}
	} else {
		config.Logger.InfoContext(ctx, "App and Group sync is disabled")
	}

	if accessListSyncEnabled {
		config.Logger.InfoContext(ctx, "Access List sync is enabled", "bidirectional", bidirectionalSyncEnabled)
		s.accessListSync, err = newAccessListSync(accessListSyncConfig{
			Logger:                   s.logger,
			Clock:                    s.clock,
			ClusterName:              s.clusterName,
			Client:                   s.client,
			Emitter:                  config.Emitter,
			Access:                   config.Access,
			AccessLists:              config.AccessLists,
			SyncInterval:             config.TimeBetweenImports,
			OrgURL:                   s.orgURL,
			Owners:                   config.SyncSettings.DefaultOwners,
			AppsGetter:               s.appServers.Clone,
			GroupsGetter:             s.groups.Clone,
			AppFilters:               config.accessListSyncAppFilters,
			GroupFilters:             config.accessListSyncGroupFilters,
			ServiceStatus:            s.serviceStatus,
			StopChannel:              s.stopCh,
			Backend:                  config.Backend,
			BidirectionalSyncEnabled: bidirectionalSyncEnabled,
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
	if s.appsReconciler != nil {
		if err := s.seedAppsReconciler(ctx); err != nil {
			return trace.Wrap(err)
		}
	}
	if s.groupsReconciler != nil {
		if err := s.seedGroupsReconciler(ctx); err != nil {
			return trace.Wrap(err)
		}
	}
	if s.accessListSync != nil {
		s.accessListSync.init(ctx)
	}

	go s.synchronizeLoop(ctx)

	if s.assignmentReconciler != nil {
		if err := s.assignmentReconciler.start(ctx); err != nil {
			return trace.Wrap(err)
		}
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

	if s.assignmentReconciler != nil {
		s.assignmentReconciler.stop()
	}

	return nil
}
