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
	"net/url"
	"sync"
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
	testOrgURL        = "https://test-url.com"
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
	services.Access
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
	types.Events

	serviceCounts map[types.SystemRole]uint64
	mu            sync.Mutex
}

var _ auth.OktaAccessPoint = (*testAccessPoint)(nil)

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
func newTestAccessPoint(t *testing.T, clock clockwork.Clock) *testAccessPoint {
	t.Helper()

	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	streamer := events.NewDiscardStreamer()

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
		Plugins:               plugins,
		Presence:              presence,
		Trust:                 ca,
		UserGroups:            userGroups,
		WindowsDesktops:       windowsDesktops,
		Events:                events,
	}
	client.serviceCounts = map[types.SystemRole]uint64{
		types.RoleOkta: 1,
	}

	return client
}

type testProxyGetter struct{}

func (t *testProxyGetter) GetProxyIDs() []string {
	return testProxyIDs
}

// newTestService creates a new test Okta service.
func newTestService(t *testing.T, ap *testAccessPoint) (*Service, *testOktaClient, *eventstest.ChannelEmitter) {
	t.Helper()

	ctx := context.Background()

	emitter := eventstest.NewChannelEmitter(2)
	client := newTestClient()
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
	require.NoError(t, ap.UpsertProxy(ctx, proxyServer))

	return svc, client, emitter
}

// testOktaClient is a testing Okta client that is backed by fixed values.
type testOktaClient struct {
	oktaUsers  []*okta.User
	oktaGroups []*okta.Group
	oktaApps   []okta.App
	oktaOrgURL string

	usernamesToUserIDsMu sync.Mutex
	usernamesToUserIDs   map[string]string

	groupsToUsersMu sync.Mutex
	// groupsToUsers is a mapping of group IDs to users that have been assigned to them.
	groupsToUsers map[string]map[string]bool

	appsToUsersMu sync.Mutex
	// appsToUsers is a mapping of application IDs to users that have been assigned to them.
	appsToUsers map[string]map[string]bool

	appsToGroups map[string][]string

	unassignGroupErr map[string]error
	unassignAppErr   map[string]error

	// monkeyPatch allows individual tests to override the default
	// testOktaClient behavior in cases where it is difficult to rig the
	// internal state in the way necessary for a test.
	monkeyPatch struct {
		createApp                    func(ctx context.Context, app okta.App) (okta.App, error)
		assignGroupToApplicationByID func(ctx context.Context, groupId, appId string) error
		doHttp                       func(context.Context, string, *url.URL, []string) ([]byte, error)
		orgName                      func(context.Context) (string, error)
	}
}

func newTestClient() *testOktaClient {
	return &testOktaClient{
		usernamesToUserIDs: map[string]string{},
		groupsToUsers:      map[string]map[string]bool{},
		appsToUsers:        map[string]map[string]bool{},
		appsToGroups:       map[string][]string{},
		unassignGroupErr:   map[string]error{},
		unassignAppErr:     map[string]error{},
		oktaOrgURL:         testOrgURL,
	}
}

// iterateUsers will iterate over the list of all Okta users.
func (t *testOktaClient) iterateUsers(_ context.Context, fn func(*okta.User) error) error {
	for _, oktaUser := range t.oktaUsers {
		if err := fn(oktaUser); err != nil {
			if err == stopIteration {
				break
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

// iterateGroups will iterate over the list of all Okta groups.
func (t *testOktaClient) iterateGroups(_ context.Context, fn func(*okta.Group) error) error {
	for _, oktaGroup := range t.oktaGroups {
		if err := fn(oktaGroup); err != nil {
			if err == stopIteration {
				break
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

// iterateApps will iterate over the list of all Okta applications.
func (t *testOktaClient) iterateApps(_ context.Context, fn func(okta.App) error) error {
	for _, oktaApp := range t.oktaApps {
		if err := fn(oktaApp); err != nil {
			if err == stopIteration {
				break
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

// getGroupAssignments will return the list of users assigned to a group.
func (t *testOktaClient) getGroupAssignments(_ context.Context, groupID string) ([]string, error) {
	t.groupsToUsersMu.Lock()
	defer t.groupsToUsersMu.Unlock()

	userMap, ok := t.groupsToUsers[groupID]
	if !ok {
		return nil, trace.NotFound("assignments for group %s not found", groupID)
	}

	var users []string
	for user := range userMap {
		users = append(users, user)
	}

	return users, nil
}

// getAppAssignments will return the list of users assigned to an app.
func (t *testOktaClient) getAppAssignments(_ context.Context, appID string) ([]string, error) {
	t.appsToUsersMu.Lock()
	defer t.appsToUsersMu.Unlock()

	userMap, ok := t.appsToUsers[appID]
	if !ok {
		return nil, trace.NotFound("assignments for app %s not found", appID)
	}

	var users []string
	for user := range userMap {
		users = append(users, user)
	}

	return users, nil
}

// getAppGroups will return the list of groups an application belongs to.
func (t *testOktaClient) getAppGroups(_ context.Context, appID string) ([]string, error) {
	return t.appsToGroups[appID], nil
}

// listUsers will return a mapping of usernames to user IDs from Okta.
func (t *testOktaClient) listUsers(_ context.Context) (map[string]string, error) {
	t.usernamesToUserIDsMu.Lock()
	defer t.usernamesToUserIDsMu.Unlock()

	usernamesToUserIDs := map[string]string{}
	for k, v := range t.usernamesToUserIDs {
		usernamesToUserIDs[k] = v
	}

	return usernamesToUserIDs, nil
}

// addUserID will add a mapping from the username to the user ID.
func (t *testOktaClient) addUserID(username, userID string) {
	t.usernamesToUserIDsMu.Lock()
	defer t.usernamesToUserIDsMu.Unlock()

	t.usernamesToUserIDs[username] = userID
}

// addGroupToMapping will add the given group to the group to user mapping in the test client.
func (t *testOktaClient) addGroupToMapping(groupId string) {
	t.groupsToUsersMu.Lock()
	defer t.groupsToUsersMu.Unlock()

	t.oktaGroups = append(t.oktaGroups, &okta.Group{
		Id: groupId,
	})
	t.groupsToUsers[groupId] = map[string]bool{}
}

// assignUserToGroup will assign the given user to the group.
func (t *testOktaClient) assignUserToGroup(_ context.Context, username, groupId string) error {
	t.groupsToUsersMu.Lock()
	defer t.groupsToUsersMu.Unlock()

	if _, ok := t.groupsToUsers[groupId]; !ok {
		return trace.NotFound("provision: unable to find group %s", groupId)
	}
	t.groupsToUsers[groupId][username] = true

	return nil
}

// unassignUserFromGroup will unassign the given user from the group.
func (t *testOktaClient) unassignUserFromGroup(_ context.Context, username, groupId string) error {
	t.groupsToUsersMu.Lock()
	defer t.groupsToUsersMu.Unlock()

	if err, ok := t.unassignGroupErr[groupId]; ok {
		return err
	}

	if _, ok := t.groupsToUsers[groupId]; !ok {
		return trace.NotFound("cleanup: unable to find group %s", groupId)
	}
	delete(t.groupsToUsers[groupId], username)

	return nil
}

// addApplicationToMapping will add the given application to the application to user mapping in the test client.
func (t *testOktaClient) addApplicationToMapping(applicationId string) {
	t.appsToUsersMu.Lock()
	defer t.appsToUsersMu.Unlock()

	t.oktaApps = append(t.oktaApps, &okta.Application{
		Id: applicationId,
	})
	t.appsToUsers[applicationId] = map[string]bool{}
}

// assignUserToApplication will assign the given user to the application.
func (t *testOktaClient) assignUserToApplication(_ context.Context, username, applicationId string) error {
	t.appsToUsersMu.Lock()
	defer t.appsToUsersMu.Unlock()

	if _, ok := t.appsToUsers[applicationId]; !ok {
		return trace.NotFound("provision: unable to find application %s", applicationId)
	}
	t.appsToUsers[applicationId][username] = true

	return nil
}

// unassignUserFromApplication will unassign the given user from the application.
func (t *testOktaClient) unassignUserFromApplication(_ context.Context, username, applicationId string) error {
	t.appsToUsersMu.Lock()
	defer t.appsToUsersMu.Unlock()

	if err, ok := t.unassignAppErr[applicationId]; ok {
		return err
	}

	if _, ok := t.appsToUsers[applicationId]; !ok {
		return trace.NotFound("cleanup: unable to find application %s", applicationId)
	}
	delete(t.appsToUsers[applicationId], username)

	return nil
}

func (t *testOktaClient) assignGroupToApplicationByID(ctx context.Context, groupId, appId string) error {
	if t.monkeyPatch.assignGroupToApplicationByID != nil {
		return t.monkeyPatch.assignGroupToApplicationByID(ctx, groupId, appId)
	}
	return trace.NotImplemented("assignGroupToApplicationByID")
}

func (t *testOktaClient) createApplication(ctx context.Context, app okta.App) (okta.App, error) {
	if t.monkeyPatch.createApp != nil {
		return t.monkeyPatch.createApp(ctx, app)
	}
	return nil, trace.NotImplemented("createApp")
}

// getOrgURL will return the org URL for the client.
func (t *testOktaClient) orgURL() string {
	return t.oktaOrgURL
}

func (t *testOktaClient) orgName(ctx context.Context) (string, error) {
	if t.monkeyPatch.orgName != nil {
		return t.monkeyPatch.orgName(ctx)
	}
	return "", nil
}

func (t *testOktaClient) doHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error) {
	if t.monkeyPatch.doHttp != nil {
		return t.monkeyPatch.doHttp(ctx, method, url, accept)
	}
	return nil, trace.NotImplemented("doHttp")
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
	for i := 0; i < numTimes; i++ {
		select {
		case val := <-ch:
			require.Equal(t, expected, val)
		case <-time.After(5 * time.Second):
			require.Fail(t, "timed out")
		}
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

func application(t *testing.T, hash crypto.Hash, name, appLinkName, origin, orgURL string) types.AppServer {
	labels := map[string]string{
		types.OriginLabel:       origin,
		teleport.OktaAppIDLabel: name,
	}
	if orgURL != "" {
		labels[teleport.OktaOrgURLLabel] = orgURL
	}
	metadata := types.Metadata{
		Name:   mustAppName(t, hash, name, appLinkName),
		Labels: labels,
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

func group(t *testing.T, name, origin, orgURL string) types.UserGroup {
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

func assignment(t *testing.T, accessRequestName, user string, cleanupTime time.Time, status string, lastTransition time.Time,
	finalized bool, targets ...*types.OktaAssignmentTargetV1,
) types.OktaAssignment {
	assignment, err := types.NewOktaAssignment(types.Metadata{
		Name: accessRequestName,
		Labels: map[string]string{
			teleport.OktaAssignmentSourceLabel: fmt.Sprintf(accessRequestFormat, accessRequestName),
		},
	},
		types.OktaAssignmentSpecV1{
			User:           user,
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

func expectAuditEvent[T any](t *testing.T, emitter *eventstest.ChannelEmitter, fn func(T)) {
	select {
	case event := <-emitter.C():
		auditEvent, ok := event.(T)
		require.True(t, ok)
		fn(auditEvent)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for event")
	}
}
