package okta

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	testOrgURL        = "https://test.okta.example.com"
	testHostname      = "test-host"
	testHostID        = "test-host-id"
	testConnectorName = "okta-test"
	testClusterName   = "test-cluster-name"
	testClusterURL    = "https://test-cluster.example.com"
)

var testProxyIDs = []string{"proxy-ids"}

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
	services.Identity
	services.Okta
	services.Plugins
	services.Presence
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

	streamer := events.NewDiscardStreamer()

	access := local.NewAccessService(backend)
	accessLists, err := local.NewAccessListService(backend, clock)
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
		Presence:                              presence,
		Trust:                                 ca,
		UserGroups:                            userGroups,
		WindowsDesktops:                       windowsDesktops,
		Events:                                events,
		SAMLIdpServiceProviderGetter:          idp,
		IdentityCenterAccountGetter:           ic,
		IdentityCenterAccountAssignmentGetter: ic,
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

type testProxyGetter struct{}

func (t *testProxyGetter) GetProxyIDs() []string {
	return testProxyIDs
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

func newTestConfig(t *testing.T, ap *testAccessPoint, options ...testServiceOpt) (Config, *eventstest.ChannelEmitter) {
	t.Helper()

	emitter := eventstest.NewChannelEmitter(2)
	lockWatcher := newLockWatcher(t, ap)
	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: testClusterName,
		AccessPoint: ap,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	config := Config{
		Leader:           &mockIsLeader{true},
		TLSConfig:        generateTestTLSConfig(t, testHostID, nil),
		Authorizer:       authorizer,
		ClusterName:      testClusterName,
		Hostname:         testHostname,
		HostID:           testHostID,
		RotationGetter:   func(role types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
		ProxyGetter:      &testProxyGetter{},
		AccessPoint:      ap,
		Access:           ap,
		AccessLists:      ap,
		OnHeartbeat:      func(err error) {},
		Emitter:          emitter,
		OktaAPIEndpoint:  "dummy",
		ConnectorService: ap,
		AuthProvider:     oktaapi.NewSSWSAuthProvider("dummy"),
		SyncSettings: types.PluginOktaSyncSettings{
			SsoConnectorId:       "dummy-connector-id",
			SyncUsers:            true,
			DisableSyncAppGroups: false,
			SyncAccessLists:      true,
			DefaultOwners:        []string{"the-owner"},
		},
		AssignmentsService: ap,
	}
	for _, opt := range options {
		opt(&config)
	}

	return config, emitter
}

// newTestService creates a new test Okta service.
func newTestService(t *testing.T, ap *testAccessPoint, options ...testServiceOpt) (*Service, *testOktaClient, *eventstest.ChannelEmitter) {
	t.Helper()

	ctx := context.Background()

	client := newTestClient()
	config, emitter := newTestConfig(t, ap, options...)

	if config.SyncSettings.SsoConnectorId != "" {
		if conn, err := ap.GetSAMLConnector(ctx, config.SyncSettings.SsoConnectorId, false); err != nil || conn == nil {
			_, err := createStubSAMLConnector(ctx, t, config.SyncSettings.SsoConnectorId, ap)
			require.NoError(t, err)
		}
	}

	svc, err := newWithClientCreator(ctx, config, oktaapi.CreatorFromTestClient(client))
	require.NoError(t, err)

	// Skip client cert verification for tests.
	svc.tlsConfig.ClientAuth = tls.RequireAnyClientCert

	proxyServer, err := types.NewServer("proxy", types.KindProxy, types.ServerSpecV2{})
	require.NoError(t, err)
	require.NoError(t, ap.UpsertProxy(ctx, proxyServer))

	return svc, client, emitter
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

func mustAppName(t testing.TB, name, appLinkName string) string {
	t.Helper()
	appName, err := AppName(name, appLinkName)
	require.NoError(t, err)
	return appName
}

func newApp(t testing.TB, metadata types.Metadata, appSpec types.AppSpecV3) *types.AppV3 {
	t.Helper()

	app, err := types.NewAppV3(metadata, appSpec)
	require.NoError(t, err)

	return app
}

func application(t testing.TB, hash crypto.Hash, name, appLinkName, origin, orgURL, hostID string) types.AppServer {
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

	app := newApp(t, metadata, types.AppSpecV3{
		URI:        "https://www.link1.com",
		PublicAddr: "public-addr",
	})
	appServer, err := types.NewAppServerV3(metadata, types.AppServerSpecV3{
		Hostname: testHostname,
		HostID:   hostID,
		App:      app,
	})
	require.NoError(t, err)
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

func target(targetType types.OktaAssignmentTargetV1_OktaAssignmentTargetType, id string) *types.OktaAssignmentTargetV1 {
	return &types.OktaAssignmentTargetV1{Type: targetType, Id: id}
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
