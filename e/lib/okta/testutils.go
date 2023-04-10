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
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
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
	services.Access
	services.Apps
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
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	streamer := events.NewDiscardEmitter()

	// TODO(mdwn): Remove the app service once the Okta cache changes have been
	// checked in.
	apps := local.NewAppService(backend)
	ca := local.NewCAService(backend)
	clusterConfiguration, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	connectionsDiagnostic := local.NewConnectionsDiagnosticService(backend)
	databaseServices := local.NewDatabaseServicesService(backend)
	identity := local.NewIdentityService(backend)
	okta, err := local.NewOktaService(backend)
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

	client := &testAccessPoint{
		Streamer:              streamer,
		Closer:                io.NopCloser(nil),
		Apps:                  apps,
		ClusterConfiguration:  clusterConfiguration,
		ConnectionsDiagnostic: connectionsDiagnostic,
		DatabaseServices:      databaseServices,
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
	svc, err := newWithClientCreator(ctx, Config{
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
