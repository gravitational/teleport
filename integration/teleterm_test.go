// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	dbhelpers "github.com/gravitational/teleport/integration/db"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/apiserver/handler"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
)

func TestTeleterm(t *testing.T) {
	pack := dbhelpers.SetupDatabaseTest(t,
		dbhelpers.WithListenerSetupDatabaseTest(helpers.SingleProxyPortSetup),
		dbhelpers.WithLeafConfig(func(config *servicecfg.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
		dbhelpers.WithRootConfig(func(config *servicecfg.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
	)
	pack.WaitForLeaf(t)

	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  pack.Root.Cluster.Process,
		Username: pack.Root.User.GetName(),
	})
	require.NoError(t, err)

	t.Run("adding root cluster", func(t *testing.T) {
		t.Parallel()

		testAddingRootCluster(t, pack, creds)
	})

	t.Run("ListRootClusters returns logged in user", func(t *testing.T) {
		t.Parallel()

		testListRootClustersReturnsLoggedInUser(t, pack, creds)
	})
	t.Run("GetCluster returns properties from auth server", func(t *testing.T) {
		t.Parallel()

		testGetClusterReturnsPropertiesFromAuthServer(t, pack)
	})

	t.Run("Test headless watcher", func(t *testing.T) {
		t.Parallel()

		testHeadlessWatcher(t, pack, creds)
	})
}

func testAddingRootCluster(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	t.Helper()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                t.TempDir(),
		InsecureSkipVerify: true,
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	addedCluster, err := daemonService.AddCluster(context.Background(), pack.Root.Cluster.Web)
	require.NoError(t, err)

	clusters, err := daemonService.ListRootClusters(context.Background())
	require.NoError(t, err)

	clusterURIs := make([]uri.ResourceURI, 0, len(clusters))
	for _, cluster := range clusters {
		clusterURIs = append(clusterURIs, cluster.URI)
	}
	require.ElementsMatch(t, clusterURIs, []uri.ResourceURI{addedCluster.URI})
}

func testListRootClustersReturnsLoggedInUser(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	tc := mustLogin(t, pack.Root.User.GetName(), pack, creds)

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	handler, err := handler.New(
		handler.Config{
			DaemonService: daemonService,
		},
	)
	require.NoError(t, err)

	response, err := handler.ListRootClusters(context.Background(), &api.ListClustersRequest{})
	require.NoError(t, err)

	require.Equal(t, 1, len(response.Clusters))
	require.Equal(t, pack.Root.User.GetName(), response.Clusters[0].LoggedInUser.Name)
}

func testGetClusterReturnsPropertiesFromAuthServer(t *testing.T, pack *dbhelpers.DatabasePack) {
	authServer := pack.Root.Cluster.Process.GetAuthServer()

	// Use random names to not collide with other tests.
	uuid := uuid.NewString()
	suggestedReviewer := "suggested-reviewer"
	requestableRoleName := fmt.Sprintf("%s-%s", "requested-role", uuid)
	userName := fmt.Sprintf("%s-%s", "user", uuid)
	roleName := fmt.Sprintf("%s-%s", "get-cluster-role", uuid)

	requestableRole, err := types.NewRole(requestableRoleName, types.RoleSpecV6{})
	require.NoError(t, err)

	// Create user role with ability to request role
	userRole, err := types.NewRole(roleName, types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow: types.RoleConditions{
			Logins: []string{
				userName,
			},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Request: &types.AccessRequestConditions{
				Roles:              []string{requestableRoleName},
				SuggestedReviewers: []string{suggestedReviewer},
			},
		},
	})
	require.NoError(t, err)

	// add role that user can request
	err = authServer.UpsertRole(context.Background(), requestableRole)
	require.NoError(t, err)

	// add role that allows to request "requestableRole"
	err = authServer.UpsertRole(context.Background(), userRole)
	require.NoError(t, err)

	user, err := types.NewUser(userName)
	user.AddRole(userRole.GetName())
	require.NoError(t, err)

	err = authServer.UpsertUser(user)
	require.NoError(t, err)

	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  pack.Root.Cluster.Process,
		Username: userName,
	})
	require.NoError(t, err)

	tc := mustLogin(t, userName, pack, creds)

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	handler, err := handler.New(
		handler.Config{
			DaemonService: daemonService,
		},
	)
	require.NoError(t, err)

	rootClusterName, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
	require.NoError(t, err)

	response, err := handler.GetCluster(context.Background(), &api.GetClusterRequest{
		ClusterUri: uri.NewClusterURI(rootClusterName).String(),
	})
	require.NoError(t, err)

	require.Equal(t, userName, response.LoggedInUser.Name)
	require.ElementsMatch(t, []string{requestableRoleName}, response.LoggedInUser.RequestableRoles)
	require.ElementsMatch(t, []string{suggestedReviewer}, response.LoggedInUser.SuggestedReviewers)
}

func testHeadlessWatcher(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	t.Helper()
	ctx := context.Background()

	tc := mustLogin(t, pack.Root.User.GetName(), pack, creds)

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
	})
	require.NoError(t, err)

	cluster, err := storage.Add(ctx, tc.WebProxyAddr)
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
		CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
			return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	// Start the tshd event service and connect the daemon to it. This should also
	// start a headless watcher for the connected cluster.

	tshdEventsService, addr := newMockTSHDEventsServiceServer(t)
	err = daemonService.UpdateAndDialTshdEventsServerAddress(addr)
	require.NoError(t, err)

	// Ensure the watcher catches events and sends them to the Electron App.

	expires := pack.Root.Cluster.Config.Clock.Now().Add(time.Minute)
	ha, err := types.NewHeadlessAuthentication(pack.Root.User.GetName(), "uuid", expires)
	require.NoError(t, err)
	ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING

	eventuallyCatchAndSendHeadlessAuthn := func(t require.TestingT) {
		tshdEventsService.sendPendingHeadlessAuthenticationCount.Store(0)

		err = pack.Root.Cluster.Process.GetAuthServer().UpsertHeadlessAuthentication(ctx, ha)
		require.NoError(t, err)

		assert.Eventually(t, func() bool {
			return tshdEventsService.sendPendingHeadlessAuthenticationCount.Load() == 1
		}, 100*time.Millisecond, 20*time.Millisecond, "Expected tshdEventService to receive a SendPendingHeadlessAuthentication message")
	}

	// The watcher takes some amount of time to set up, so if we immediately upsert a headless
	// authentication, it may not be caught by the watcher.
	// require.Eventually(t, upsertAndWaitForEvent, time.Second, 100*time.Millisecond, "Expected tshdEventService to receive a SendPendingHeadlessAuthentication message")

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		eventuallyCatchAndSendHeadlessAuthn(collect)
	}, time.Second, 100*time.Millisecond)

	// Stop and restart the watcher twice to simulate logout + login + relogin. Ensure the watcher catches events.

	err = daemonService.StopHeadlessWatcher(cluster.URI.String())
	require.NoError(t, err)
	err = daemonService.StartHeadlessWatcher(cluster.URI.String())
	require.NoError(t, err)
	err = daemonService.StartHeadlessWatcher(cluster.URI.String())
	require.NoError(t, err)

	// Ensure the watcher catches events and sends them to the Electron App.

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		eventuallyCatchAndSendHeadlessAuthn(collect)
	}, time.Second, 100*time.Millisecond)
}

func mustLogin(t *testing.T, userName string, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) *client.TeleportClient {
	tc, err := pack.Root.Cluster.NewClientWithCreds(helpers.ClientConfig{
		Login:   userName,
		Cluster: pack.Root.Cluster.Secrets.SiteName,
	}, *creds)
	require.NoError(t, err)
	// Save the profile yaml file to disk as NewClientWithCreds doesn't do that by itself.
	tc.SaveProfile(false /* makeCurrent */)
	return tc
}

type mockTSHDEventsService struct {
	*api.UnimplementedTshdEventsServiceServer
	sendPendingHeadlessAuthenticationCount atomic.Uint32
}

func newMockTSHDEventsServiceServer(t *testing.T) (service *mockTSHDEventsService, addr string) {
	tshdEventsService := &mockTSHDEventsService{}

	ls, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	api.RegisterTshdEventsServiceServer(grpcServer, tshdEventsService)
	t.Cleanup(grpcServer.GracefulStop)

	go func() {
		err := grpcServer.Serve(ls)
		assert.NoError(t, err)
	}()

	return tshdEventsService, ls.Addr().String()
}

func (c *mockTSHDEventsService) SendPendingHeadlessAuthentication(context.Context, *api.SendPendingHeadlessAuthenticationRequest) (*api.SendPendingHeadlessAuthenticationResponse, error) {
	c.sendPendingHeadlessAuthenticationCount.Add(1)
	return &api.SendPendingHeadlessAuthenticationResponse{}, nil
}
