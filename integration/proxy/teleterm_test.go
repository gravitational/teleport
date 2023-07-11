// Copyright 2022 Gravitational, Inc
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

package proxy

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	dbhelpers "github.com/gravitational/teleport/integration/db"
	"github.com/gravitational/teleport/integration/helpers"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
)

// testTeletermGatewaysCertRenewal is run from within TestALPNSNIProxyDatabaseAccess to amortize the
// cost of setting up clusters in tests.
func testTeletermGatewaysCertRenewal(t *testing.T, pack *dbhelpers.DatabasePack) {
	rootClusterName, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
	require.NoError(t, err)

	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  pack.Root.Cluster.Process,
		Username: pack.Root.User.GetName(),
	})
	require.NoError(t, err)

	t.Run("root cluster", func(t *testing.T) {
		databaseURI := uri.NewClusterURI(rootClusterName).
			AppendDB(pack.Root.MysqlService.Name)

		testGatewayCertRenewal(t, pack, "", creds, databaseURI)
	})
	t.Run("leaf cluster", func(t *testing.T) {
		leafClusterName := pack.Leaf.Cluster.Secrets.SiteName
		databaseURI := uri.NewClusterURI(rootClusterName).
			AppendLeafCluster(leafClusterName).
			AppendDB(pack.Leaf.MysqlService.Name)

		testGatewayCertRenewal(t, pack, "", creds, databaseURI)
	})
	t.Run("ALPN connection upgrade", func(t *testing.T) {
		// Make a mock ALB which points to the Teleport Proxy Service. Then
		// ALPN local proxies will point to this ALB instead.
		albProxy := helpers.MustStartMockALBProxy(t, pack.Root.Cluster.Web)

		databaseURI := uri.NewClusterURI(rootClusterName).
			AppendDB(pack.Root.MysqlService.Name)

		testGatewayCertRenewal(t, pack, albProxy.Addr().String(), creds, databaseURI)
	})
}
func testGatewayCertRenewal(t *testing.T, pack *dbhelpers.DatabasePack, albAddr string, creds *helpers.UserCreds, databaseURI uri.ResourceURI) {
	tc, err := pack.Root.Cluster.NewClientWithCreds(helpers.ClientConfig{
		Login:   pack.Root.User.GetName(),
		Cluster: pack.Root.Cluster.Secrets.SiteName,
		ALBAddr: albAddr,
	}, *creds)
	require.NoError(t, err)
	// Save the profile yaml file to disk as NewClientWithCreds doesn't do that by itself.
	tc.SaveProfile(false /* makeCurrent */)

	fakeClock := clockwork.NewFakeClockAt(time.Now())

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		// Inject a fake clock into clusters.Storage so we can control when the middleware thinks the
		// db cert has expired.
		Clock: fakeClock,
	})
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

	// Create a mock tshd events service server and have the daemon connect to it,
	// like it would during normal initialization of the app.

	tshdEventsService, tshEventsServerAddr := newMockTSHDEventsServiceServer(t, tc, pack)
	err = daemonService.UpdateAndDialTshdEventsServerAddress(tshEventsServerAddr)
	require.NoError(t, err)

	// Here the test setup ends and actual test code starts.

	gateway, err := daemonService.CreateGateway(context.Background(), daemon.CreateGatewayParams{
		TargetURI:  databaseURI.String(),
		TargetUser: "root",
	})
	require.NoError(t, err, trace.DebugReport(err))

	// Open a new connection.
	client, err := mysql.MakeTestClientWithoutTLS(
		net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort()),
		gateway.RouteToDatabase())
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Execute("select 1")
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)

	// Disconnect.
	require.NoError(t, client.Close())

	// Advance the fake clock to simulate the db cert expiry inside the middleware.
	fakeClock.Advance(time.Hour * 48)
	// Overwrite user certs with expired ones to simulate the user cert expiry.
	expiredCreds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  pack.Root.Cluster.Process,
		Username: pack.Root.User.GetName(),
		TTL:      -time.Hour,
	})
	require.NoError(t, err)
	err = helpers.SetupUserCreds(tc, pack.Root.Cluster.Config.Proxy.SSHAddr.Addr, *expiredCreds)
	require.NoError(t, err)

	// Open a new connection.
	// This should trigger the relogin flow. The middleware will notice that the db cert has expired
	// and then it will attempt to reissue the db cert using an expired user cert.
	// The mocked tshdEventsClient will issue a valid user cert, save it to disk, and the middleware
	// will let the connection through.
	client, err = mysql.MakeTestClientWithoutTLS(
		net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort()),
		gateway.RouteToDatabase())
	require.NoError(t, err)

	// Execute a query.
	result, err = client.Execute("select 1")
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)

	// Disconnect.
	require.NoError(t, client.Close())

	require.Equal(t, 1, tshdEventsService.callCounts["Relogin"],
		"Unexpected number of calls to TSHDEventsClient.Relogin")
	require.Equal(t, 0, tshdEventsService.callCounts["SendNotification"],
		"Unexpected number of calls to TSHDEventsClient.SendNotification")
}

type mockTSHDEventsService struct {
	*api.UnimplementedTshdEventsServiceServer

	tc         *libclient.TeleportClient
	pack       *dbhelpers.DatabasePack
	callCounts map[string]int
}

func newMockTSHDEventsServiceServer(t *testing.T, tc *libclient.TeleportClient, pack *dbhelpers.DatabasePack) (service *mockTSHDEventsService, addr string) {
	t.Helper()

	tshdEventsService := &mockTSHDEventsService{
		tc:         tc,
		pack:       pack,
		callCounts: make(map[string]int),
	}

	ls, err := net.Listen("tcp", "127.0.0.1:0")
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

// Relogin simulates the act of the user logging in again in the Electron app by replacing the user
// cert on disk with a valid one.
func (c *mockTSHDEventsService) Relogin(context.Context, *api.ReloginRequest) (*api.ReloginResponse, error) {
	c.callCounts["Relogin"]++
	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  c.pack.Root.Cluster.Process,
		Username: c.pack.Root.User.GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = helpers.SetupUserCreds(c.tc, c.pack.Root.Cluster.Config.Proxy.SSHAddr.Addr, *creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.ReloginResponse{}, nil
}

func (c *mockTSHDEventsService) SendNotification(context.Context, *api.SendNotificationRequest) (*api.SendNotificationResponse, error) {
	c.callCounts["SendNotification"]++
	return &api.SendNotificationResponse{}, nil
}
