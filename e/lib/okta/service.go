package okta

import (
	"context"
	"crypto"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
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
	// 4 requests per second is the absolute maximum okta will allow due to End User Rate Limits.
	// https://developer.okta.com/docs/reference/rl-additional-limits/#end-user-rate-limits
	APICallsPerSecond     = 4
	RequestTimeoutSeconds = 300 // Okta request timeout is 5 minutes.

	// Default to running synchronizations every half hour.
	oktaDefaultTimeBetweenSyncs = 30 * time.Minute
	oktaTransportIdleTimeout    = 30 * time.Second
	oktaConnectionTimeout       = 30 * time.Second

	semaphoreKind              = "okta-service"
	semaphoreExpiration        = 10 * time.Minute
	semaphoreRenewal           = 2 * time.Minute
	semaphoreRenewalMaxRetries = 5
)

// ProxyGetter is an interface for retrieving proxy IDs.
type ProxyGetter interface {
	GetProxyIDs() []string
}

// Config is the configuration for the Okta service.
type Config struct {
	// Log is the logger for the Okta config.
	Log *logrus.Entry

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

	// OktaAPIToken is the API token.
	OktaAPIToken string

	// TimeBetweenSyncs is the amount of time between synchronization calls.
	TimeBetweenSyncs time.Duration

	// BackendTasksPerSecond is the number of backend modifying tasks that can be run per second.
	BackendTasksPerSecond int

	// PluginStatusSink is an optional status sink for reporting the plugin status.
	PluginStatusSink common.StatusSink

	// UserSyncEnabled indicates that the Okta service will try to sync user
	// records from the upstream Okta organization
	UserSyncEnabled bool

	// SSOConnectorID specifies which SSO connector users will be joining from
	SSOConnectorID string

	// AccessListSyncEnabled indicates that the Okta service will try to sync
	// Okta permissions via access lists.
	AccessListSyncEnabled bool

	// DefaultOwners is the list of default owners for imported access lists.
	DefaultOwners []string

	// AccessListSyncAppFilters is the list of app filters for use by the access list sync.
	AccessListSyncAppFilters []string
	accessListSyncAppFilters []*regexp.Regexp

	// AccessListSyncGroupFilters is the list of group filters for use by the access list sync.
	AccessListSyncGroupFilters []string
	accessListSyncGroupFilters []*regexp.Regexp

	// OktaSAMLAppID is the Okta-assigned ID for the SAML app that the service
	// uses as a gateway for syncing users. If empty, the service will revert to
	// the legacy method of polling the whole Okta organization.
	OktaSAMLAppID string

	// ConnectorService is the SAML connector service.
	ConnectorService SAMLConnectorService

	// DisableAppGroupSync allows to disable Okta application and group sync.
	DisableAppGroupSync bool
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, eteleport.ComponentOkta)
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
	if c.OktaAPIToken == "" {
		return trace.BadParameter("Okta API token is missing")
	}
	if c.ConnectorService == nil {
		return trace.BadParameter("ConnectorService service is missing")
	}
	if c.TimeBetweenSyncs == 0 {
		c.TimeBetweenSyncs = oktaDefaultTimeBetweenSyncs
	}
	if c.BackendTasksPerSecond == 0 {
		// Default to running 5 backend tasks per second.
		c.BackendTasksPerSecond = 5
	}
	if c.UserSyncEnabled && c.SSOConnectorID == "" {
		return trace.BadParameter("Okta SSO Connector ID must be set if user sync is enabled")
	}

	if c.AccessListSyncEnabled {
		if c.Access == nil {
			return trace.BadParameter("access is missing")
		}
		if c.AccessLists == nil {
			return trace.BadParameter("access lists is missing")
		}
		if len(c.DefaultOwners) == 0 {
			return trace.BadParameter("default owners is missing")
		}

		for _, filter := range c.AccessListSyncAppFilters {
			compiledFilter, err := utils.CompileExpression(filter)
			if err != nil {
				return trace.Wrap(err)
			}
			c.accessListSyncAppFilters = append(c.accessListSyncAppFilters, compiledFilter)
		}

		for _, filter := range c.AccessListSyncGroupFilters {
			compiledFilter, err := utils.CompileExpression(filter)
			if err != nil {
				return trace.Wrap(err)
			}
			c.accessListSyncGroupFilters = append(c.accessListSyncGroupFilters, compiledFilter)
		}
	}

	return nil
}

// errStopIteration is a sentinel value that iterator functions can use to
// signals oktaClient iterate* methods to stop iterating without it
// being passed up the call stack.
var errStopIteration = errors.New("stop iterating")

// appAssignmentrepresents an individual assignmenet to an application.
type appAssignment struct {
	userID string
	scope  appAssignmentScope
}

// appAssignmentScope is the assignment scope of the application.
// components/schemas/AppUserProfile Okta API users assigment scope
// A description of this field can be found in the Okta API docs:
// https://developer.okta.com/docs/reference/api/apps/#application-user-object
type appAssignmentScope string

const (
	userScope  appAssignmentScope = "USER"
	groupScope appAssignmentScope = "GROUP"
)

// OktaClient is an Okta client interface that can be mocked for testing.
type OktaClient interface {
	// getCurrentUser will fetch the profile of the user currently logged into
	// Okta.
	getCurrentUser(context.Context) (*okta.User, error)

	// iterateUsers will iterate over the list of all Okta users. The supplied
	// iterator callback may return errStopIteration to signal that it does not want
	// to continue receiving users. All other non-nil return values are
	// considered an error and will be propagated to the caller.
	iterateUsers(context.Context, func(*okta.User) error, ...query.ParamOptions) error

	// listUserGroups will return the list of groups a user belongs to.
	listUserGroups(ctx context.Context, userID string) ([]UserGroup, error)

	// iterateAppUsers will iterate over the list of all Okta users assigned to
	// a given app. The supplied iterator callback may return stopIteration to
	// signal that it does not want to continue receiving users. All other
	// non-nil return values are considered an error and will be propagated to
	// the caller.
	iterateAppUsers(context.Context, oktaAppID, func(*okta.AppUser) error) error

	// iterateGroups will iterate over the list of all Okta groups. The supplied
	// iterator callback may return errStopIteration to signal that it does not want
	// to continue receiving groups. All other non-nil return values are
	// considered an error and will be propagated to the caller.
	iterateGroups(context.Context, func(*okta.Group) error) error

	// iterateApps will iterate over the list of all Okta applications. The
	// supplied iterator callback may return errStopIteration to signal that it
	// does not want to continue receiving apps. All other non-nil return values
	// are considered an error and will be propagated to the caller.
	iterateApps(context.Context, func(okta.App) error) error

	// getGroupAssignments will return the list of users assigned to a group.
	getGroupAssignments(ctx context.Context, groupID oktaGroupID) ([]oktaUserID, error)

	// getAppAssignments will return the list of users assigned to an app.
	getAppAssignments(ctx context.Context, appID oktaAppID) ([]appAssignment, error)

	// getAppGroups will return the list of groups an application belongs to.
	getAppGroups(ctx context.Context, appID oktaAppID) ([]oktaGroupID, error)

	// listUsers will return a mapping of usernames to user IDs from Okta.
	listUsers(ctx context.Context, paramOpts ...query.ParamOptions) (map[userName]oktaUserID, error)

	// assignUserToGroup will assign the given user to the group.
	assignUserToGroup(ctx context.Context, userID oktaUserID, groupId oktaGroupID) error

	// unassignUserFromGroup will unassign the given user from the group.
	unassignUserFromGroup(ctx context.Context, userID oktaUserID, groupId oktaGroupID) error

	// assignUserToApplication will assign the given user to the application.
	assignUserToApplication(ctx context.Context, userID oktaUserID, applicationId oktaAppID) error

	// assignGroupToApplication assigns the given group to the application.
	assignGroupToApplication(ctx context.Context, groupID oktaGroupID, applicationID oktaAppID) error

	// unassignUserFromApplication will unassign the given user from the application.
	unassignUserFromApplication(ctx context.Context, userID oktaUserID, applicationId oktaAppID) error

	// createApplication attempts to create a new Okta application from the
	// supplied application request.
	createApplication(ctx context.Context, application okta.App) (okta.App, error)

	// getApplication fetches the data for single application.
	getApplication(ctx context.Context, appID oktaAppID, appType okta.App) (okta.App, error)

	// getOrgURL will return the org URL for the client.
	orgURL() string

	// orgName returns the configured
	orgName(context.Context) (string, error)

	// doHttp executes a HTTP request on the supplied URL using the same
	// credentials and headers used by underlying Okta client
	doHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error)
}

// Service is the core data for the Okta integration service. The running
// service synchronizes data with an upstream Okta IdP.
type Service struct {
	log        *logrus.Entry
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
	accessPoint  authclient.OktaAccessPoint
	onHeartbeat  func(error)
	client       OktaClient
	emitter      apievents.Emitter
	orgURL       string
	orgURLBase64 string

	// rateLimiter will rate limit backend interactions.
	rateLimiter *rate.Limiter

	// hash function for getting unique names from app IDs/app link names.
	hash crypto.Hash

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

	pluginStatusSink common.StatusSink

	// synchronizerSuccess will be set to true if the synchronizer has completed at least
	// once successfully.
	synchronizerSuccess atomic.Bool

	synchronizingMu        sync.RWMutex
	leadershipAcquired     atomic.Bool
	leadershipRenewRetries atomic.Int32

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

	// connectorService is the SAML connector service.
	connectorService SAMLConnectorService

	// disableOktaAppGroupSync allows to disable Okta application and group sync.
	// when only SCIM or user sync integration is needed.
	disableOktaAppGroupSync bool
}

// rateLimitingHTTPTransport will only perform HTTP requests after waiting the
// for the rate limiter.
type rateLimitingHTTPTransport struct {
	delegate    *http.Transport
	rateLimiter *rate.Limiter
}

func (r *rateLimitingHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Before issuing any HTTP request, wait to ensure we only issue the number of
	// requests the rate limiter allows.
	if err := r.rateLimiter.Wait(req.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	return r.delegate.RoundTrip(req)
}

func (r *rateLimitingHTTPTransport) CloseIdleConnections() {
	r.delegate.CloseIdleConnections()
}

// ClientConfig holds the various parameters for creating an Okta client.
type ClientConfig struct {
	// HTTPClient is an optional HTTP client that can be used to override the
	// default client for testing. Do not set in production.
	HTTPClient *http.Client

	// Endpoint is an URL indicating the root endpoint of the Okta API service
	Endpoint string

	// Token is an Okta-supplied user API access token
	Token string

	// Log receives any log info
	Log *logrus.Entry

	// StatusSink receives status update information from the OktaClient.
	// May be nil, in which case status updates will be dropped.
	StatusSink common.StatusSink
}

// Check validates the state of the ClientConfig, returning a non-nil error
// if the config is invalid.
func (cfg *ClientConfig) Check() error {
	if cfg.Endpoint == "" {
		return trace.BadParameter("missing Okta Client parameter EndPoint")
	}
	if cfg.Token == "" {
		return trace.BadParameter("missing Okta Client parameter Token")
	}
	if cfg.Log == nil {
		return trace.BadParameter("missing OktaCLient parameter Log")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Transport: &rateLimitingHTTPTransport{
				// This transport was taken from the Okta client.
				delegate: &http.Transport{
					IdleConnTimeout: oktaTransportIdleTimeout,
				},
				rateLimiter: rate.NewLimiter(
					rate.Every(time.Second/time.Duration(APICallsPerSecond)), 1),
			},
			Timeout: oktaConnectionTimeout,
		}
	}
	return nil
}

// NewClient creates and initializes a new okta client
func NewClient(ctx context.Context, cfg ClientConfig) (OktaClient, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	_, client, err := okta.NewClient(ctx,
		okta.WithCache(false), // We don't want a cache as we need up to date info.
		okta.WithOrgUrl(cfg.Endpoint),
		okta.WithToken(cfg.Token),
		okta.WithHttpClientPtr(cfg.HTTPClient),

		// This will retry until the request timeout has passed, doing a backoff
		// of up to 30 seconds.
		okta.WithRequestTimeout(RequestTimeoutSeconds),
		okta.WithRateLimitMaxRetries(math.MaxInt32),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &wrappedClient{
		log:              cfg.Log,
		client:           client,
		oktaOrgURL:       cfg.Endpoint,
		pluginStatusSink: cfg.StatusSink,
	}, nil
}

// oktaClientFn is a function interface for creating Okta client.
type oktaClientFn func(context.Context, ClientConfig) (OktaClient, error)

// createNewOktaClient will create a new Okta client.
func createNewOktaClient(ctx context.Context, config ClientConfig) (OktaClient, error) {
	return NewClient(ctx, config)
}

// New will create a new Okta service.
func New(ctx context.Context, config Config) (*Service, error) {
	return newWithClientCreator(ctx, config, createNewOktaClient)
}

// newWithClientCreator will create a new Okta service with the given oktaClient.
func newWithClientCreator(ctx context.Context, config Config, creator oktaClientFn) (*Service, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		reportPluginStatus(ctx, config.Log, config.PluginStatusSink, types.PluginStatusCode_OTHER_ERROR)
		return nil, trace.Wrap(err)
	}

	client, err := creator(ctx, ClientConfig{
		Endpoint:   config.OktaAPIEndpoint,
		Token:      config.OktaAPIToken,
		Log:        config.Log,
		StatusSink: config.PluginStatusSink,
	})
	if err != nil {
		reportPluginStatus(ctx, config.Log, config.PluginStatusSink, types.PluginStatusCode_OTHER_ERROR)
		return nil, trace.Wrap(err)
	}

	orgURL := strings.TrimSuffix(client.orgURL(), "/")
	orgURLBase64 := base64.RawURLEncoding.EncodeToString([]byte(orgURL))

	var reconciler *userReconciler
	if config.UserSyncEnabled {
		config.Log.Info("User sync is enabled. Configuring reconciler.")
		reconciler, err = newUserReconciler(userReconcilerConfig{
			clusterName: config.ClusterName,
			teleportAP:  config.AccessPoint,
			log:         config.Log,
			userOrgURL:  config.OktaAPIEndpoint,
			emitter:     config.Emitter,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		config.Log.Info("User synchronization is disabled.")
	}

	s := &Service{
		connectorService:        config.ConnectorService,
		log:                     config.Log,
		clock:                   config.Clock,
		authorizer:              config.Authorizer,
		clusterName:             config.ClusterName,
		hostname:                config.Hostname,
		hostID:                  config.HostID,
		rotationGetter:          config.RotationGetter,
		proxyGetter:             config.ProxyGetter,
		accessPoint:             config.AccessPoint,
		onHeartbeat:             config.OnHeartbeat,
		client:                  client,
		emitter:                 config.Emitter,
		orgURL:                  orgURL,
		orgURLBase64:            orgURLBase64,
		rateLimiter:             rate.NewLimiter(rate.Every(time.Second/time.Duration(config.BackendTasksPerSecond)), 1),
		hash:                    crypto.SHA256,
		heartbeats:              map[string]*srv.Heartbeat{},
		groupIRMapping:          map[string]prioritizedLabels{},
		applicationIRMapping:    map[string]prioritizedLabels{},
		groupNameRegexes:        []regexAndPriorityLabels{},
		appNameRegexes:          []regexAndPriorityLabels{},
		timeBetweenSyncs:        config.TimeBetweenSyncs,
		syncStoppedCh:           make(chan struct{}, 1),
		stopCh:                  make(chan struct{}, 1),
		pluginStatusSink:        config.PluginStatusSink,
		userReconciler:          reconciler,
		ssoConnectorID:          config.SSOConnectorID,
		oktaSAMLAppID:           config.OktaSAMLAppID,
		disableOktaAppGroupSync: config.DisableAppGroupSync,
	}
	s.tlsConfig = app.CopyAndConfigureTLS(config.Log, s.accessPoint, config.TLSConfig)

	clusterName, err := s.accessPoint.GetClusterName()
	if err != nil {
		reportPluginStatus(ctx, config.Log, config.PluginStatusSink, types.PluginStatusCode_OTHER_ERROR)
		return nil, trace.Wrap(err)
	}

	if !s.disableOktaAppGroupSync {
		s.assignmentReconciler = newAssignmentReconciler(ctx, clusterName.GetClusterName(), s)
	}

	if config.AccessListSyncEnabled {
		config.Log.Info("Access list synchronization is enabled. Configuring synchronizer.")
		alSync, err := newAccessListSync(accessListSyncConfig{
			Log:          s.log,
			Clock:        s.clock,
			ClusterName:  s.clusterName,
			Client:       s.client,
			Emitter:      config.Emitter,
			Access:       config.Access,
			AccessLists:  config.AccessLists,
			OrgURL:       s.orgURL,
			Owners:       config.DefaultOwners,
			AppsGetter:   s.apps.Clone,
			GroupsGetter: s.groups.Clone,
			AppFilters:   config.accessListSyncAppFilters,
			GroupFilters: config.accessListSyncGroupFilters,

			SynchronizerSuccess: &s.synchronizerSuccess,
			SynchronizingMu:     &s.synchronizingMu,
			StopChannel:         s.stopCh,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		s.accessListSync = alSync
	} else {
		config.Log.Info("Access list synchronization is disabled.")
	}

	reportPluginStatus(ctx, config.Log, config.PluginStatusSink, types.PluginStatusCode_RUNNING)

	return s, nil
}

// Start will start the Okta service.
func (s *Service) Start(ctx context.Context) error {
	// becomeLeader will ensure that the Okta service is the leader before processing anything. This service
	// will not make any calls the Okta API while it is not the leader.
	go s.becomeLeader(ctx)

	if err := s.startSynchronizerReconcilers(ctx); err != nil {
		return trace.Wrap(err)
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
			s.log.Errorf("Unable to close heartbeat: %v", err)
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

// IsLeader will return true if this service is currently the leader.
func (s *Service) IsLeader() bool {
	return s.leadershipAcquired.Load()
}

// SetLeader will set leadership acquired bool to the given value. Used for testing.
func (s *Service) SetLeader(isLeader bool) {
	s.leadershipAcquired.Store(isLeader)
}

// errStopLeadership will be returned by the leadership acquisition functions when
// the Okta service has been stopped.
var errStopLeadership = errors.New("stop leadership acquisition")

// becomeLeader will repeatedly try to acquire and renew a semaphore scoped to the Okta organization
// managed by this service. Once this semaphore is acquired, this Okta service will be allowed to issue
// API calls and process assignments. This will prevent multiple Okta services from managing the same Okta
// organization, as the Okta API rate limits are pretty severe.
func (s *Service) becomeLeader(ctx context.Context) {
	for {
		lease, err := s.acquireSemaphore(ctx)
		if errors.Is(err, errStopLeadership) {
			return
		} else if err != nil {
			s.log.WithError(err).Debug("Error acquiring semaphore")
			continue
		}

		s.leadershipAcquired.Store(true)

		s.log.Debug("Semaphore acquired, going into renew loop")

		err = s.renewSemaphoreLease(ctx, lease)
		s.leadershipAcquired.Store(false)

		if errors.Is(err, errStopLeadership) {
			return
		} else if !trace.IsLimitExceeded(err) {
			s.log.WithError(err).Debug("Error renewing lease")
		}
	}
}

// acquireSemaphore will acquire the semaphore for this Okta service and org.
func (s *Service) acquireSemaphore(ctx context.Context) (*types.SemaphoreLease, error) {
	ticker := s.clock.NewTicker(semaphoreRenewal)
	defer ticker.Stop()
	for {
		s.log.Debug("Attempting to acquire semaphore before starting.")
		lease, err := s.accessPoint.AcquireSemaphore(ctx, types.AcquireSemaphoreRequest{
			SemaphoreKind: semaphoreKind,
			SemaphoreName: s.orgURLBase64,
			MaxLeases:     1,
			Expires:       s.clock.Now().Add(semaphoreExpiration),
			Holder:        s.hostID,
		})
		if err == nil {
			return lease, nil
		}

		s.log.Debugf("Unable to get semaphore (%s), seeing if this host already has a lease", err.Error())
		semaphores, err := s.accessPoint.GetSemaphores(ctx, types.SemaphoreFilter{
			SemaphoreKind: semaphoreKind,
			SemaphoreName: s.orgURLBase64,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Look through the existing leases to see if the holder for the existing lease is the same as
		// the current host. We're expected to only have 1 lease per semaphore, so if the holder of that
		// lease is the same as this host, we can be certain that the lease actually belongs to this host.
		for _, semaphore := range semaphores {
			leases := semaphore.LeaseRefs()
			for _, lease := range leases {
				if lease.Holder != s.hostID {
					continue
				}

				s.log.Debug("Lease found for this host")
				// This lease belongs to this host, so we'll return this lease.
				return &types.SemaphoreLease{
					SemaphoreKind: semaphoreKind,
					SemaphoreName: s.orgURLBase64,
					LeaseID:       lease.LeaseID,
					Expires:       lease.Expires,
				}, nil
			}
		}

		s.log.Debugf("Unable to acquire semaphore, retrying in %s", semaphoreRenewal.String())

		select {
		case <-s.stopCh:
			return nil, errStopLeadership
		case <-ctx.Done():
			return nil, errStopLeadership
		case <-ticker.Chan():
		}
	}
}

// renewSemaphoreLease will repeatedly attempt to renew the semaphore lease.
func (s *Service) renewSemaphoreLease(ctx context.Context, lease *types.SemaphoreLease) error {
	// Set up a function to renew the lease regularly.
	ticker := s.clock.NewTicker(semaphoreRenewal)
	defer ticker.Stop()

	// Reset renew retries.
	s.leadershipRenewRetries.Store(0)

	for {
		select {
		case <-s.stopCh:
			return errStopLeadership
		case <-ctx.Done():
			return errStopLeadership
		case <-ticker.Chan():
		}

		lease.Expires = s.clock.Now().Add(semaphoreExpiration)
		if err := s.accessPoint.KeepAliveSemaphoreLease(ctx, *lease); err != nil {
			retryCount := s.leadershipRenewRetries.Add(1)
			if retryCount >= semaphoreRenewalMaxRetries {
				return trace.LimitExceeded("max semaphore renew attempts reached, service will stop processing")
			} else {
				s.log.WithError(err).WithField("retry_count", retryCount).Warnf("Error renewing semaphore lease, will retry in %s", semaphoreRenewal.String())
			}
		} else {
			s.leadershipRenewRetries.Store(0)
		}
	}
}

// reportPluginStatus will report the plugin status to the given status sink if it exists.
func reportPluginStatus(ctx context.Context, log *logrus.Entry, pluginStatusSink common.StatusSink, code types.PluginStatusCode) {
	if pluginStatusSink == nil {
		return
	}

	err := pluginStatusSink.Emit(ctx, &types.PluginStatusV1{
		Code: code,
	})
	if err != nil {
		log.Errorf("Error emitting plugin status: %v", err)
	}
}

// SelectSCIMToken searches the supplied list of credentials for a SCIM bearer
// token. Returns a NotFound error if no such credential exists.
func SelectSCIMToken(staticCredentials []types.PluginStaticCredentials) (types.PluginStaticCredentials, error) {
	creds, err := selectCredsByPurposeLabel(staticCredentials, CredPurposeSCIMToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

// SelectAPIToken searches the supplied list of credentials for a credential
// containing an Okta API token.  Returns a NotFound error if no such credential
// exists.
func SelectAPIToken(staticCredentials []types.PluginStaticCredentials) (types.PluginStaticCredentials, error) {
	// CredPurposeOktaAPITokenWithSCIMOnlyIntegration is set only when Okta app sync is disabled.
	// For backward compatibility, when Teleport is downgraded to a version that doesn't support
	// stopping app group sync via the feature flag (AppGroupSyncDisabled), we will rely on the behavior
	// of preventing starting the Okta Plugin due to the missing credential.
	if v, err := selectCredsByPurposeLabel(staticCredentials, CredPurposeOktaAPITokenWithSCIMOnlyIntegration); err == nil {
		return v, nil
	}
	// For now, we'll just choose the first eligible static credential until we
	// have a need for rotation or other complexity.
	for _, cred := range staticCredentials {
		// Older Okta API credentials are not labeled with a purpose, so a cred
		// is considered eligible if it has no purpose label, or a purpose label
		// set to okta.CredPurposeOktaAuth.
		purpose, present := cred.GetLabel(CredPurposeLabel)
		if !present || purpose == CredPurposeOktaAuth {
			return cred, nil
		}
	}
	return nil, trace.NotFound("Okta API token")
}

func selectCredsByPurposeLabel(staticCredentials []types.PluginStaticCredentials, purposeLabel string) (types.PluginStaticCredentials, error) {
	for _, cred := range staticCredentials {
		purpose, present := cred.GetLabel(CredPurposeLabel)
		if present && purpose == purposeLabel {
			return cred, nil
		}
	}
	return nil, trace.NotFound("credential")
}
