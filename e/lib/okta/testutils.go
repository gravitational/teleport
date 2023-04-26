/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package okta

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
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
	testOrgURL      = "https://test-url.com"
	testHostname    = "test-host"
	testHostID      = "test-host-id"
	testClusterName = "test-cluster-name"
)

var (
	testProxyIDs = []string{"proxy-ids"}
)

// testAccessPoint is a test access point for the Okta service.
type testAccessPoint struct {
	events.Streamer
	io.Closer
	*local.DynamicAccessService
	services.Access
	services.ClusterConfiguration
	services.ConnectionsDiagnostic
	services.DatabaseServices
	services.Identity
	services.Okta
	services.Presence
	services.Trust
	services.UserGroups
	services.WindowsDesktops
	types.Events
}

var _ auth.OktaAccessPoint = (*testAccessPoint)(nil)

func (*testAccessPoint) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) { return nil, nil }

func (*testAccessPoint) GenerateCertAuthorityCRL(context.Context, types.CertAuthType) ([]byte, error) {
	return nil, nil
}

// newTestAccessPoint will create a memory backed test access point for the Okta service.
func newTestAccessPoint(t *testing.T, clock clockwork.Clock) *testAccessPoint {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	streamer := events.NewDiscardEmitter()

	access := local.NewAccessService(backend)
	ca := local.NewCAService(backend)
	clusterConfiguration, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	connectionsDiagnostic := local.NewConnectionsDiagnosticService(backend)
	databaseServices := local.NewDatabaseServicesService(backend)
	dynamicAccess := local.NewDynamicAccessService(backend)
	identity := local.NewIdentityService(backend)
	okta, err := local.NewOktaService(backend, clock)
	require.NoError(t, err)
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

	require.NoError(t, clusterConfiguration.SetAuthPreference(ctx, types.DefaultAuthPreference()))

	client := &testAccessPoint{
		Streamer:              streamer,
		Closer:                io.NopCloser(nil),
		Access:                access,
		ClusterConfiguration:  clusterConfiguration,
		ConnectionsDiagnostic: connectionsDiagnostic,
		DatabaseServices:      databaseServices,
		DynamicAccessService:  dynamicAccess,
		Identity:              identity,
		Okta:                  okta,
		Presence:              presence,
		Trust:                 ca,
		UserGroups:            userGroups,
		WindowsDesktops:       windowsDesktops,
		Events:                events,
	}

	return client
}

type testProxyGetter struct{}

func (t *testProxyGetter) GetProxyIDs() []string {
	return testProxyIDs
}

// newTestService creates a new test Okta service.
func newTestService(t *testing.T, ap auth.OktaAccessPoint) (*Service, *testOktaClient) {
	ctx := context.Background()

	emitter := eventstest.NewCountingEmitter()
	client := &testOktaClient{
		oktaOrgURL: testOrgURL,
	}
	services.NewLockWatcher(ctx, services.LockWatcherConfig{})
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentOkta,
			Client:    ap,
		},
	})
	require.NoError(t, err)
	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: testClusterName,
		AccessPoint: ap,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)
	svc, err := newWithClientCreator(ctx, Config{
		TLSConfig:       generateTestTLSConfig(t, testHostID, nil),
		Authorizer:      authorizer,
		ClusterName:     testClusterName,
		Hostname:        testHostname,
		HostID:          testHostID,
		RotationGetter:  func(role types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
		ProxyGetter:     &testProxyGetter{},
		AccessPoint:     ap,
		OnHeartbeat:     func(err error) {},
		Emitter:         emitter,
		OktaAPIEndpoint: "dummy",
		OktaAPIToken:    "dummy",
	}, func(_ context.Context, _ Config) (oktaClient, error) {
		return client, nil
	})
	require.NoError(t, err)

	// Skip client cert verification for tests.
	svc.tlsConfig.ClientAuth = tls.RequireAnyClientCert

	proxyServer, err := types.NewServer("proxy", types.KindProxy, types.ServerSpecV2{})
	require.NoError(t, err)
	require.NoError(t, ap.UpsertProxy(proxyServer))

	return svc, client
}

// testOktaClient is a testing Okta client that is backed by fixed values.
type testOktaClient struct {
	oktaGroups []*okta.Group
	oktaApps   []okta.App
	oktaOrgURL string
}

// iterateGroups will iterate over the list of all Okta groups.
func (t *testOktaClient) iterateGroups(_ context.Context, fn func(*okta.Group) error) error {
	for _, oktaGroup := range t.oktaGroups {
		if err := fn(oktaGroup); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// iterateApps will iterate over the list of all Okta applications.
func (t *testOktaClient) iterateApps(_ context.Context, fn func(okta.App) error) error {
	for _, oktaApp := range t.oktaApps {
		if err := fn(oktaApp); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (t *testOktaClient) orgURL() string {
	return t.oktaOrgURL
}

// generateTestTLSConfig will generate a TLS config for testing.
func generateTestTLSConfig(t *testing.T, name string, roles []string, extensions ...pkix.AttributeTypeAndValue) *tls.Config {
	keyPEM, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{
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
	select {
	case val := <-ch:
		require.Equal(t, expected, val)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out")
	}
}

func mustAppName(t *testing.T, hash crypto.Hash, name, appLinkName string) string {
	appName, err := appName(hash, name, appLinkName)
	require.NoError(t, err)
	return appName
}

func newApp(t *testing.T, metadata types.Metadata, appSpec types.AppSpecV3) *types.AppV3 {
	app, err := types.NewAppV3(metadata, appSpec)
	require.NoError(t, err)

	return app
}

func application(t *testing.T, hash crypto.Hash, name, appLinkName, origin string) types.AppServer {
	metadata := types.Metadata{
		Name: mustAppName(t, hash, name, appLinkName),
		Labels: map[string]string{
			types.OriginLabel: origin,
		},
	}

	app := newApp(t, metadata, types.AppSpecV3{
		URI:        "https://www.link1.com",
		PublicAddr: "public-addr",
	})
	appServer, err := types.NewAppServerV3(metadata, types.AppServerSpecV3{
		Hostname: testHostname,
		HostID:   testHostID,
		App:      app,
	})
	require.NoError(t, err)
	return appServer
}

func group(t *testing.T, name, origin string) types.UserGroup {
	userGroup, err := types.NewUserGroup(types.Metadata{
		Name: name,
		Labels: map[string]string{
			types.OriginLabel: origin,
		},
	})
	require.NoError(t, err)
	return userGroup
}

func target(targetType types.OktaAssignmentTargetV1_OktaAssignmentTargetType, id string) *types.OktaAssignmentTargetV1 {
	return &types.OktaAssignmentTargetV1{Type: targetType, Id: id}
}

func assignment(t *testing.T, accessRequestName, user string, cleanupTime time.Time, status string, lastTransition time.Time, targets ...*types.OktaAssignmentTargetV1) types.OktaAssignment {
	assignment, err := types.NewOktaAssignment(types.Metadata{
		Name: accessRequestName,
		Labels: map[string]string{
			assignmentSourceLabel: fmt.Sprintf(accessRequestFormat, accessRequestName),
		},
	},
		types.OktaAssignmentSpecV1{
			User:           user,
			Targets:        targets,
			CleanupTime:    cleanupTime,
			LastTransition: lastTransition,
		},
	)
	require.NoError(t, err)

	require.NoError(t, assignment.SetStatus(status))
	return assignment
}
