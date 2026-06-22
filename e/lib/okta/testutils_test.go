package okta

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ossteleport "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	testOrgURL      = "https://test.okta.example.com"
	testHostname    = "test-host"
	testHostID      = "test-host-id"
	testClusterName = "test-cluster-name"
)

// testAccessPoint is a test access point for the Okta service.
type testAccessPoint struct {
	events.Streamer
	io.Closer
	*local.DynamicAccessService
	*local.AccessService
	services.AccessLists
	services.ClusterConfiguration
	services.ConnectionsDiagnostic
	services.DatabaseServices
	services.LinuxDesktopGetter
	services.Identity
	services.Okta
	services.Plugins
	services.PresenceInternal
	services.Trust
	services.UserGroups
	services.WindowsDesktops
	services.SAMLIdpServiceProviderGetter
	services.IdentityCenterAccountGetter
	services.IdentityCenterAccountAssignmentGetter
	services.GitServerGetter
	types.Events

	clock         clockwork.Clock
	serviceCounts map[types.SystemRole]uint64
	mu            sync.Mutex
}

var _ authclient.OktaAccessPoint = (*testAccessPoint)(nil)

func (t *testAccessPoint) Clock() clockwork.Clock {
	return t.clock
}

func (*testAccessPoint) NewKeepAliver(context.Context) (types.KeepAliver, error) { return nil, nil }

func (*testAccessPoint) GenerateCertAuthorityCRL(context.Context, types.CertAuthType) ([]byte, error) {
	return nil, nil
}

func (t *testAccessPoint) UpsertProxyServerWithoutReturn(ctx context.Context, s types.Server) error {
	return trace.NotImplemented("UpsertProxyServerWithoutReturn is not implemented in testAccessPoint")
}

// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
func (t *testAccessPoint) GetInventoryConnectedServiceCount(service types.SystemRole) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.serviceCounts[service]
}

func (t *testAccessPoint) setServiceCounts(m map[types.SystemRole]uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.serviceCounts = m
}

// newTestAccessPoint will create a memory backed test access point for the Okta service.
func newTestAccessPoint(t testing.TB, clock clockwork.Clock) *testAccessPoint {
	t.Helper()

	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })

	streamer := events.NewDiscardStreamer()

	access := local.NewAccessService(backend)
	accessLists, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: backend,
		Modules: modulestest.EnterpriseModules(),
	})
	require.NoError(t, err)
	ca := local.NewCAService(backend)
	clusterConfiguration, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	connectionsDiagnostic := local.NewConnectionsDiagnosticService(backend)
	databaseServices := local.NewDatabaseServicesService(backend)
	dynamicAccess := local.NewDynamicAccessService(backend)
	identity, err := local.NewIdentityService(backend)
	require.NoError(t, err)
	okta, err := local.NewOktaService(backend, clock)
	require.NoError(t, err)
	plugins := local.NewPluginsService(backend)
	presence := local.NewPresenceService(backend)
	userGroups, err := local.NewUserGroupService(backend)
	require.NoError(t, err)
	windowsDesktops := local.NewWindowsDesktopService(backend)
	events := local.NewEventsService(backend)

	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterID:   uuid.NewString(),
		ClusterName: testClusterName,
	})
	require.NoError(t, err)
	require.NoError(t, clusterConfiguration.SetClusterName(clusterName))
	_, err = clusterConfiguration.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)

	git, err := local.NewGitServerService(backend)
	require.NoError(t, err)

	linuxDesktops, err := local.NewLinuxDesktopService(backend)
	require.NoError(t, err)

	ic, err := local.NewIdentityCenterService(local.IdentityCenterServiceConfig{Backend: backend})
	require.NoError(t, err)

	idp, err := local.NewSAMLIdPServiceProviderService(backend)
	require.NoError(t, err)

	client := &testAccessPoint{
		clock:                                 clock,
		Streamer:                              streamer,
		Closer:                                io.NopCloser(nil),
		AccessService:                         access,
		AccessLists:                           accessLists,
		ClusterConfiguration:                  clusterConfiguration,
		ConnectionsDiagnostic:                 connectionsDiagnostic,
		DatabaseServices:                      databaseServices,
		DynamicAccessService:                  dynamicAccess,
		Identity:                              identity,
		Okta:                                  okta,
		Plugins:                               plugins,
		PresenceInternal:                      presence,
		Trust:                                 ca,
		UserGroups:                            userGroups,
		WindowsDesktops:                       windowsDesktops,
		Events:                                events,
		SAMLIdpServiceProviderGetter:          idp,
		IdentityCenterAccountGetter:           ic,
		IdentityCenterAccountAssignmentGetter: ic,
		LinuxDesktopGetter:                    linuxDesktops,
		GitServerGetter:                       git,
	}
	client.serviceCounts = map[types.SystemRole]uint64{
		types.RoleOkta: 1,
	}

	return client
}

func createStubSAMLConnector(ctx context.Context, t *testing.T, name string, ap *testAccessPoint) (conn types.SAMLConnector, err error) {
	connType, err := types.NewSAMLConnector(name, types.SAMLConnectorSpecV2{
		SSO:                      "test",
		AssertionConsumerService: "test",
		EntityDescriptor: `<?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="test">
      <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
        <md:KeyDescriptor use="signing">
          <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
            <ds:X509Data>
              <ds:X509Certificate></ds:X509Certificate>
            </ds:X509Data>
          </ds:KeyInfo>
        </md:KeyDescriptor>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://example.com" />
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://example.com" />
      </md:IDPSSODescriptor>
    </md:EntityDescriptor>`,
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "test",
				Roles: []string{"test"},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return ap.CreateSAMLConnector(ctx, connType)
}

type testServiceOpt func(*Config)

func withUserSyncEnabled(syncSource types.OktaUserSyncSource) testServiceOpt {
	return func(cfg *Config) {
		cfg.SyncSettings.SyncUsers = true
		cfg.SyncSettings.UserSyncSource = string(syncSource)
	}
}

func withOktaAppID(appID string) testServiceOpt {
	return func(cfg *Config) {
		cfg.SyncSettings.AppId = appID
	}
}

func withClock(clock clockwork.Clock) testServiceOpt {
	return func(cfg *Config) {
		cfg.Clock = clock
	}
}

func withSSOConnector(c string) testServiceOpt {
	return func(cfg *Config) {
		cfg.SyncSettings.SsoConnectorId = c
	}
}

func withBackendTasksPerSecond(backendTasksPerSecond int) testServiceOpt {
	return func(cfg *Config) {
		cfg.BackendTasksPerSecond = backendTasksPerSecond
	}
}

func newTestConfig(t *testing.T, ap *testAccessPoint, options ...testServiceOpt) (Config, *eventstest.ChannelEmitter) {
	t.Helper()

	emitter := eventstest.NewChannelEmitter(4)
	lockWatcher := newLockWatcher(t, ap)
	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: testClusterName,
		AccessPoint: ap,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	config := Config{
		Logger:                            slog.Default(),
		TLSConfig:                         generateTestTLSConfig(t, testHostID, nil),
		Authorizer:                        authorizer,
		ClusterName:                       testClusterName,
		Hostname:                          testHostname,
		HostID:                            testHostID,
		AccessPoint:                       ap,
		Access:                            ap,
		AccessLists:                       ap,
		Emitter:                           emitter,
		OktaAPIEndpoint:                   "dummy",
		TimeBetweenImports:                oktaplugin.DefaultTimeBetweenImports,
		TimeBetweenAssignmentProcessLoops: oktaplugin.DefaultTimeBetweenAssignmentProcessLoops,
		ConnectorService:                  ap,
		AuthProvider:                      oktaapi.NewSSWSAuthProvider("dummy"),
		SyncSettings: types.PluginOktaSyncSettings{
			SsoConnectorId:       "dummy-connector-id",
			SyncUsers:            true,
			DisableSyncAppGroups: false,
			SyncAccessLists:      true,
			DefaultOwners:        []string{"the-owner"},
		},
		Backend: ap,
	}
	for _, opt := range options {
		opt(&config)
	}

	return config, emitter
}

// newTestService creates a new test Okta service.
func newTestService(t *testing.T, ap *testAccessPoint, oktaClient oktaapi.Interface, opts ...testServiceOpt) (*Service, *eventstest.ChannelEmitter) {
	t.Helper()

	ctx := context.Background()

	config, emitter := newTestConfig(t, ap, opts...)

	if config.SyncSettings.SsoConnectorId != "" {
		if conn, err := ap.GetSAMLConnector(ctx, config.SyncSettings.SsoConnectorId, false); err != nil || conn == nil {
			_, err := createStubSAMLConnector(ctx, t, config.SyncSettings.SsoConnectorId, ap)
			require.NoError(t, err)
		}
	}

	oktaClientFn := func(_ context.Context, _ oktaapi.Config) (oktaapi.Interface, error) {
		return oktaClient, nil
	}

	svc, err := newWithClientCreator(ctx, config, oktaClientFn)
	require.NoError(t, err)

	// Skip client cert verification for tests.
	svc.tlsConfig.ClientAuth = tls.RequireAnyClientCert

	proxyServer, err := types.NewServer("proxy", types.KindProxy, types.ServerSpecV2{})
	require.NoError(t, err)
	_, err = ap.UpsertProxyServer(ctx, proxyServer)
	require.NoError(t, err)

	return svc, emitter
}

func newLockWatcher(t *testing.T, ap *testAccessPoint) *services.LockWatcher {
	t.Helper()

	lockWatcher, err := services.NewLockWatcher(context.Background(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentOkta,
			Client:    ap,
		},
	})
	require.NoError(t, err)
	t.Cleanup(lockWatcher.Close)

	return lockWatcher
}

// generateTestTLSConfig will generate a TLS config for testing.
func generateTestTLSConfig(t *testing.T, name string, roles []string, extensions ...pkix.AttributeTypeAndValue) *tls.Config {
	t.Helper()

	keyPEM, certPEM, err := utils.GenerateRSASelfSignedSigningCert(pkix.Name{
		Organization: roles,
		CommonName:   name,
		ExtraNames: append([]pkix.AttributeTypeAndValue{
			{
				Type:  tlsca.TeleportClusterASN1ExtensionOID,
				Value: testClusterName,
			},
		}, extensions...),
	}, nil, 10*365*24*time.Hour)
	require.NoError(t, err)

	key, err := utils.ParsePrivateKeyPEM(keyPEM)
	require.NoError(t, err)
	cert, err := tlsutils.ParseCertificatePEM(certPEM)
	require.NoError(t, err)

	tlsCert := tls.Certificate{
		PrivateKey:  key,
		Certificate: [][]byte{cert.Raw},
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		ServerName:         name,
		InsecureSkipVerify: true,
	}
}

// waitForResult will wait for a value on a channel and see if the value matches the expected value.
func waitForResult[T any](t *testing.T, ch chan T, expected T, numTimes int) {
	t.Helper()

	for range numTimes {
		select {
		case val := <-ch:
			require.Equal(t, expected, val)
		case <-time.After(5 * time.Second):
			require.Fail(t, "timed out")
		}
	}
}

func mustAppName(t require.TestingT, name, appLinkName string) string {
	testCallHelper(t)
	appName, err := AppName(name, appLinkName)
	require.NoError(t, err)
	return appName
}

type newAppServerOptions struct {
	hostID string
}

type newAppServerOpt func(*newAppServerOptions)

func withHostID(hostID string) newAppServerOpt {
	return func(o *newAppServerOptions) {
		o.hostID = hostID
	}
}

func newAppServer(t testing.TB, metadata types.Metadata, appSpec types.AppSpecV3, opts ...newAppServerOpt) *types.AppServerV3 {
	t.Helper()

	opt := newAppServerOptions{
		hostID: oktaAppServerHostID,
	}
	for _, o := range opts {
		o(&opt)
	}

	app, err := types.NewAppV3(metadata, appSpec)
	require.NoError(t, err)

	appServer, err := types.NewAppServerV3(
		types.Metadata{
			Name:        app.GetName(),
			Description: app.GetDescription(),
			Labels:      app.GetStaticLabels(),
		},
		types.AppServerSpecV3{
			Version:  ossteleport.Version,
			Hostname: testHostname,
			HostID:   opt.hostID,
			App:      app,
		},
	)
	require.NoError(t, err)

	return appServer
}

func upsertAppServer(t testing.TB, ap services.Presence, appServer types.AppServer) {
	t.Helper()
	ctx := t.Context()

	_, err := ap.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)
}

func application(t testing.TB, name, appLinkName, origin, orgURL string, opts ...newAppServerOpt) types.AppServer {
	t.Helper()

	labels := map[string]string{
		types.OriginLabel:       origin,
		teleport.OktaAppIDLabel: name,
	}
	if orgURL != "" {
		labels[teleport.OktaOrgURLLabel] = orgURL
	}
	metadata := types.Metadata{
		Name:   mustAppName(t, name, appLinkName),
		Labels: labels,
	}

	appServer := newAppServer(t, metadata, types.AppSpecV3{
		URI:        "https://www.link1.com",
		PublicAddr: "public-addr",
	}, opts...)
	return appServer
}

func group(t *testing.T, name, origin, orgURL string) types.UserGroup {
	t.Helper()

	userGroup, err := types.NewUserGroup(types.Metadata{
		Name: name,
		Labels: map[string]string{
			types.OriginLabel:        origin,
			teleport.OktaOrgURLLabel: orgURL,
		},
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	return userGroup
}

type targetOpt func(*types.OktaAssignmentTargetV1)

func withStatus(status *types.OktaAssignmentTargetStatus) targetOpt {
	return func(t *types.OktaAssignmentTargetV1) {
		t.Status = status
	}
}

func target(targetType types.OktaAssignmentTargetV1_OktaAssignmentTargetType, id string, opts ...targetOpt) *types.OktaAssignmentTargetV1 {
	target := &types.OktaAssignmentTargetV1{Type: targetType, Id: id}
	for _, opt := range opts {
		opt(target)
	}
	return target
}

func status(op constants.OktaAssignmentTargetOp, outcome constants.OktaAssignmentTargetOutcome, lastProcessed time.Time, failureCount int) *types.OktaAssignmentTargetStatus {
	return &types.OktaAssignmentTargetStatus{
		Op:            string(op),
		Outcome:       string(outcome),
		LastProcessed: lastProcessed,
		FailureCount:  int32(failureCount),
	}
}

func assignment(t *testing.T, accessRequestName string, user userName, cleanupTime time.Time, status string, lastTransition time.Time,
	finalized bool, targets ...*types.OktaAssignmentTargetV1,
) types.OktaAssignment {
	t.Helper()

	assignment, err := types.NewOktaAssignment(types.Metadata{
		Name: accessRequestName,
		Labels: map[string]string{
			teleport.OktaAssignmentSourceLabel: fmt.Sprintf(accessRequestFormat, accessRequestName),
		},
	},
		types.OktaAssignmentSpecV1{
			User:           string(user),
			Targets:        targets,
			CleanupTime:    cleanupTime,
			LastTransition: lastTransition,
			Finalized:      finalized,
		},
	)
	require.NoError(t, err)

	require.NoError(t, assignment.SetStatus(status))
	return assignment
}

func assignmentLess(a1, a2 types.OktaAssignment) bool {
	return a1.GetName() < a2.GetName()
}

func collectAllEvents[T any](t *testing.T, emitter *eventstest.ChannelEmitter, result *[]T) {
	t.Helper()

	var res []T
	for {
		select {
		case event := <-emitter.C():
			e, ok := event.(T)
			require.True(t, ok, "expected type %T, got %T", e, event)
			res = append(res, e)
		default:
			*result = res
			return
		}
	}
}

func requireAuditEvent[T any](t *testing.T, emitter *eventstest.ChannelEmitter) T {
	t.Helper()

	select {
	case event := <-emitter.C():
		auditEvent, ok := event.(T)
		require.True(t, ok)
		return auditEvent
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for event")
	}

	var empty T
	return empty
}

func expectAuditEvent[T any](t *testing.T, emitter *eventstest.ChannelEmitter, fn func(T)) {
	t.Helper()
	fn(requireAuditEvent[T](t, emitter))
}

func requireOktaSideApplicationAssignments(t require.TestingT, oktaClient oktaapi.Interface, oktaApplicationID string, oktaUserIDs []string) {
	testCallHelper(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	applicationAssignments, err := oktaClient.GetAppAssignments(ctx, oktaapi.OktaAppID(oktaApplicationID))
	require.NoError(t, err)
	var assignedUsers []string
	for _, a := range applicationAssignments {
		assignedUsers = append(assignedUsers, a.UserID)
	}
	require.ElementsMatch(t, oktaUserIDs, assignedUsers)
}

func requireOktaSideGroupAssignments(t require.TestingT, oktaClient oktaapi.Interface, oktaGroupID string, oktaUserIDs []string) {
	testCallHelper(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	groupAssignments, err := oktaClient.GetGroupAssignments(ctx, oktaapi.OktaGroupID(oktaGroupID))
	require.NoError(t, err)
	var assignedUsers []string
	for _, a := range groupAssignments {
		assignedUsers = append(assignedUsers, string(a))
	}
	require.ElementsMatch(t, oktaUserIDs, assignedUsers)
}

type testApplicationServerGetter interface {
	GetApplicationServers(context.Context, string) ([]types.AppServer, error)
}

func testGetAppServer(t *testing.T, ap testApplicationServerGetter, name string) types.AppServer {
	t.Helper()
	ctx := t.Context()

	appServers, err := ap.GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	for _, as := range appServers {
		if as.GetName() == name {
			return as
		}
	}
	t.Fatalf("app_server %q not found", name)
	return nil // should never get there because of the t.Fatalf call above
}

func requireOktaAppServers(t require.TestingT, ap testApplicationServerGetter, names []string) {
	testCallHelper(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	appServers, err := ap.GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)

	// This loop will build the slice of currentNames with Okta app ID added as a suffix if
	// found. This suffix is also added to the original names list so the comparison doesn't
	// fail. This makes it easier to debug when you see "3hfhsud9otxdnc:app2" instead of
	// "3hfhsud9otxdnc".
	var currentNames []string
	enrichedNames := slices.Clone(names)
	for _, as := range appServers {
		oktaAppID, hasOktaAppID := as.GetLabel(types.OktaAppIDLabel)
		if hasOktaAppID {
			currentNames = append(currentNames, as.GetName()+":"+oktaAppID)
			if i := slices.Index(enrichedNames, as.GetName()); i >= 0 {
				enrichedNames[i] += ":" + oktaAppID
			}
		} else {
			currentNames = append(currentNames, as.GetName())
		}
	}
	require.ElementsMatch(t, enrichedNames, currentNames)
}

func requireAppServerExists(t *testing.T, ap testApplicationServerGetter, hostID, name string) {
	t.Helper()
	ctx := t.Context()

	appServers, err := ap.GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)

	found := slices.ContainsFunc(appServers, func(appServer types.AppServer) bool {
		return appServer.GetHostID() == hostID && appServer.GetName() == name
	})
	require.True(t, found)
}

func testCallHelper(t assert.TestingT) {
	h, ok := t.(interface{ Helper() })
	if ok {
		h.Helper()
	}
}
