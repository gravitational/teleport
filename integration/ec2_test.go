/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package integration

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	cloudimds "github.com/gravitational/teleport/lib/cloud/imds"
	cloudaws "github.com/gravitational/teleport/lib/cloud/imds/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

func newNodeConfig(t *testing.T, tokenName string, joinMethod types.JoinMethod) *servicecfg.Config {
	config := servicecfg.MakeDefaultConfig()
	config.Version = defaults.TeleportConfigVersionV3
	config.SetToken(tokenName)
	config.JoinMethod = joinMethod
	config.SSH.Enabled = true
	config.SSH.Addr.Addr = helpers.NewListener(t, service.ListenerNodeSSH, &config.FileDescriptors)
	config.Auth.Enabled = false
	config.Proxy.Enabled = false
	config.DataDir = t.TempDir()
	config.Logger = slog.New(slog.DiscardHandler)
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	config.InstanceMetadataClient = cloudimds.NewDisabledIMDSClient()
	return config
}

func newProxyConfig(t *testing.T, authAddr utils.NetAddr, tokenName string, joinMethod types.JoinMethod) *servicecfg.Config {
	config := servicecfg.MakeDefaultConfig()
	config.Version = defaults.TeleportConfigVersionV3
	config.SetToken(tokenName)
	config.JoinMethod = joinMethod
	config.SSH.Enabled = false
	config.Auth.Enabled = false

	proxyAddr := helpers.NewListener(t, service.ListenerProxyWeb, &config.FileDescriptors)
	config.Proxy.Enabled = true
	config.Proxy.DisableWebInterface = true
	config.Proxy.WebAddr.Addr = proxyAddr

	config.DataDir = t.TempDir()
	config.SetAuthServerAddress(authAddr)
	config.Logger = slog.New(slog.DiscardHandler)
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	config.InstanceMetadataClient = cloudimds.NewDisabledIMDSClient()
	return config
}

func newAuthConfig(t *testing.T, clock clockwork.Clock) *servicecfg.Config {
	var err error
	storageConfig := backend.Config{
		Type: lite.GetName(),
		Params: backend.Params{
			"path":               t.TempDir(),
			"poll_stream_period": 50 * time.Millisecond,
		},
	}

	config := servicecfg.MakeDefaultConfig()
	config.Version = defaults.TeleportConfigVersionV3
	config.DataDir = t.TempDir()
	config.Auth.ListenAddr.Addr = helpers.NewListener(t, service.ListenerAuth, &config.FileDescriptors)
	config.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "testcluster",
	})
	require.NoError(t, err)
	config.SetAuthServerAddress(config.Auth.ListenAddr)
	config.Auth.StorageConfig = storageConfig
	config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	config.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	require.NoError(t, err)
	config.Proxy.Enabled = false
	config.SSH.Enabled = false
	config.Clock = clock
	config.Logger = slog.New(slog.DiscardHandler)
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	config.InstanceMetadataClient = cloudimds.NewDisabledIMDSClient()
	return config
}

func getIID(ctx context.Context, t *testing.T) imds.InstanceIdentityDocument {
	cfg, err := config.LoadDefaultConfig(ctx)
	require.NoError(t, err)
	imdsClient := imds.NewFromConfig(cfg)
	output, err := imdsClient.GetInstanceIdentityDocument(ctx, nil)
	require.NoError(t, err)
	return output.InstanceIdentityDocument
}

func getCallerIdentity(ctx context.Context, t *testing.T) *sts.GetCallerIdentityOutput {
	cfg, err := config.LoadDefaultConfig(ctx)
	require.NoError(t, err)
	if cfg.Region == "" {
		imdsClient, err := cloudaws.NewInstanceMetadataClient(ctx)
		require.NoError(t, err)
		cfg.Region, err = imdsClient.GetRegion(ctx)
		require.NoError(t, err, "trying to get local region from IMDSv2")
	}
	stsClient := stsutils.NewFromConfig(cfg)
	output, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	require.NoError(t, err)
	return output
}

// TestEC2NodeJoin is an integration test which asserts that the EC2 method for
// Simplified Node Joining works when run on a real EC2 instance with access to
// the IMDS and the ec2:DesribeInstances API. This is a very basic test, unit
// testing with mocked AWS endpoints is in lib/auth/join_ec2_test.go
func TestEC2NodeJoin(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_EC2") == "" {
		t.Skipf("Skipping TestEC2NodeJoin because TELEPORT_TEST_EC2 is not set")
	}
	ctx := context.Background()

	// fetch the IID to create a token which will match this instance
	iid := getIID(ctx, t)

	tokenName := "test_token"
	token, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(time.Hour),
		types.ProvisionTokenSpecV2{
			Roles: []types.SystemRole{types.RoleNode},
			Allow: []*types.TokenRule{
				{
					AWSAccount: iid.AccountID,
					AWSRegions: []string{iid.Region},
				},
			},
		})
	require.NoError(t, err)

	// mock the current time so that the IID will pass the TTL check
	clock := clockwork.NewFakeClockAt(iid.PendingTime.Add(time.Second))

	// create and start the auth server
	authConfig := newAuthConfig(t, clock)
	authSvc, err := service.NewTeleport(authConfig)
	require.NoError(t, err)
	require.NoError(t, authSvc.Start())
	t.Cleanup(func() { require.NoError(t, authSvc.Close()) })

	authServer := authSvc.GetAuthServer()
	authServer.SetClock(clock)

	err = authServer.UpsertToken(ctx, token)
	require.NoError(t, err)

	// sanity check there are no nodes to start with
	nodes, err := authServer.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, nodes)

	// create and start the node
	nodeConfig := newNodeConfig(t, tokenName, types.JoinMethodEC2)
	nodeConfig.SetAuthServerAddress(authConfig.Auth.ListenAddr)
	nodeSvc, err := service.NewTeleport(nodeConfig)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Start())
	t.Cleanup(func() { require.NoError(t, nodeSvc.Close()) })

	_, err = nodeSvc.WaitForEventTimeout(10*time.Second, service.TeleportReadyEvent)
	require.NoError(t, err, "timeout waiting for node readiness")

	// the node should eventually join the cluster and heartbeat
	require.Eventually(t, func() bool {
		nodes, _ := authServer.GetNodes(ctx, apidefaults.Namespace)
		return len(nodes) > 0
	}, 10*time.Second, 50*time.Millisecond, "waiting for node to join cluster")
}

// TestIAMNodeJoin is an integration test which asserts that the IAM method for
// Simplified Node Joining works when run on a real EC2 instance with an
// attached IAM role.  This is a very basic test, unit testing with mocked AWS
// endpoints is in lib/auth/join_iam_test.go
func TestIAMNodeJoin(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_EC2") == "" {
		t.Skipf("Skipping TestIAMNodeJoin because TELEPORT_TEST_EC2 is not set")
	}
	ctx := context.Background()

	// create and start the auth server
	authConfig := newAuthConfig(t, nil /*clock*/)
	authSvc, err := service.NewTeleport(authConfig)
	require.NoError(t, err)
	require.NoError(t, authSvc.Start())
	t.Cleanup(func() { require.NoError(t, authSvc.Close()) })

	authServer := authSvc.GetAuthServer()

	// fetch the caller identity to find the AWS account and create the token
	id := getCallerIdentity(ctx, t)

	tokenName := "test_token"
	token, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(time.Hour),
		types.ProvisionTokenSpecV2{
			Roles: []types.SystemRole{types.RoleNode, types.RoleProxy},
			Allow: []*types.TokenRule{
				{
					AWSAccount: *id.Account,
				},
			},
			JoinMethod: types.JoinMethodIAM,
		})
	require.NoError(t, err)

	err = authServer.UpsertToken(ctx, token)
	require.NoError(t, err)

	// sanity check there are no proxies to start with
	proxies, err := authServer.GetProxies()
	require.NoError(t, err)
	require.Empty(t, proxies)

	// create and start the proxy, will use the IAM method to join by connecting
	// directly to the auth server
	proxyConfig := newProxyConfig(t, authConfig.Auth.ListenAddr, tokenName, types.JoinMethodIAM)
	proxySvc, err := service.NewTeleport(proxyConfig)
	require.NoError(t, err)
	require.NoError(t, proxySvc.Start())
	t.Cleanup(func() { require.NoError(t, proxySvc.Close()) })

	// the proxy should eventually join the cluster and heartbeat
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		proxies, err := authServer.GetProxies()
		assert.NoError(t, err)
		assert.NotEmpty(t, proxies)
	}, 10*time.Second, 50*time.Millisecond, "waiting for proxy to join cluster")
	// InsecureDevMode needed for node to trust proxy
	wasInsecureDevMode := lib.IsInsecureDevMode()
	t.Cleanup(func() { lib.SetInsecureDevMode(wasInsecureDevMode) })
	lib.SetInsecureDevMode(true)

	// sanity check there are no nodes to start with
	nodes, err := authServer.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, nodes)

	// create and start a node, will use the IAM method to join in IoT mode by
	// connecting to the proxy
	nodeConfig := newNodeConfig(t, tokenName, types.JoinMethodIAM)
	nodeConfig.ProxyServer = proxyConfig.Proxy.WebAddr
	nodeSvc, err := service.NewTeleport(nodeConfig)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Start())
	t.Cleanup(func() { require.NoError(t, nodeSvc.Close()) })

	// the node should eventually join the cluster and heartbeat
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		nodes, err := authServer.GetNodes(ctx, apidefaults.Namespace)
		assert.NoError(t, err)
		assert.NotEmpty(t, nodes)
	}, 10*time.Second, 50*time.Millisecond, "waiting for node to join cluster")
}

type mockIMDSClient struct {
	tags map[string]string
}

func (m *mockIMDSClient) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *mockIMDSClient) GetTags(ctx context.Context) (map[string]string, error) {
	return m.tags, nil
}

func (m *mockIMDSClient) GetHostname(ctx context.Context) (string, error) {
	value, ok := m.tags[types.CloudHostnameTag]
	if !ok {
		return "", trace.NotFound("cloud hostname key not found")
	}
	return value, nil
}

func (m *mockIMDSClient) GetType() types.InstanceMetadataType {
	return types.InstanceMetadataTypeEC2
}

func (m *mockIMDSClient) GetID(ctx context.Context) (string, error) {
	return "", nil
}

// TestEC2Labels is an integration test which asserts that Teleport correctly picks up
// EC2 tags when running on an EC2 instance.
func TestEC2Labels(t *testing.T) {
	storageConfig := backend.Config{
		Type: lite.GetName(),
		Params: backend.Params{
			"path":               t.TempDir(),
			"poll_stream_period": 50 * time.Millisecond,
		},
	}
	tconf := servicecfg.MakeDefaultConfig()
	tconf.Logger = slog.New(slog.DiscardHandler)
	tconf.DataDir = t.TempDir()
	tconf.Auth.Enabled = true
	tconf.Proxy.Enabled = true
	tconf.Proxy.SSHAddr.Addr = helpers.NewListener(t, service.ListenerProxySSH, &tconf.FileDescriptors)
	tconf.Proxy.WebAddr.Addr = helpers.NewListener(t, service.ListenerProxyWeb, &tconf.FileDescriptors)
	tconf.Proxy.ReverseTunnelListenAddr.Addr = helpers.NewListener(t, service.ListenerProxyTunnel, &tconf.FileDescriptors)
	tconf.Proxy.DisableWebInterface = true
	tconf.Auth.StorageConfig = storageConfig
	tconf.Auth.ListenAddr.Addr = helpers.NewListener(t, service.ListenerAuth, &tconf.FileDescriptors)
	tconf.SetAuthServerAddress(tconf.Auth.ListenAddr)

	tconf.SSH.Enabled = true
	tconf.SSH.Addr.Addr = helpers.NewListener(t, service.ListenerNodeSSH, &tconf.FileDescriptors)

	appConf := servicecfg.App{
		Name: "test-app",
		URI:  "app.example.com",
	}

	tconf.Apps.Enabled = true
	tconf.Apps.Apps = []servicecfg.App{appConf}

	dbConfig := servicecfg.Database{
		Name:     "test-db",
		Protocol: "postgres",
		URI:      "postgres://somewhere.example.com",
	}
	tconf.Databases.Enabled = true
	tconf.Databases.Databases = []servicecfg.Database{dbConfig}

	helpers.EnableKubernetesService(t, tconf)

	tconf.InstanceMetadataClient = &mockIMDSClient{
		tags: map[string]string{
			"Name": "my-instance",
		},
	}

	proc, err := service.NewTeleport(tconf)
	require.NoError(t, err)
	require.NoError(t, proc.Start())
	t.Cleanup(func() { require.NoError(t, proc.Close()) })

	ctx := context.Background()
	authServer := proc.GetAuthServer()

	var nodes []types.Server
	var apps []types.AppServer
	var databases []types.DatabaseServer
	var kubes []types.KubeServer

	// Wait for everything to come online.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var err error
		nodes, err = authServer.GetNodes(ctx, tconf.SSH.Namespace)
		assert.NoError(t, err)
		apps, err = authServer.GetApplicationServers(ctx, tconf.SSH.Namespace)
		assert.NoError(t, err)
		databases, err = authServer.GetDatabaseServers(ctx, tconf.SSH.Namespace)
		assert.NoError(t, err)
		kubes, err = authServer.GetKubernetesServers(ctx)
		assert.NoError(t, err)

		assert.Len(t, nodes, 1)
		assert.Len(t, apps, 1)
		assert.Len(t, databases, 1)
		assert.Len(t, kubes, 1)
	}, 10*time.Second, time.Second)

	tagName := fmt.Sprintf("%s/Name", labels.AWSLabelNamespace)

	// Check that EC2 labels were applied.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		node, err := authServer.GetNode(ctx, tconf.SSH.Namespace, nodes[0].GetName())
		assert.NoError(t, err)

		_, nodeHasLabel := node.GetAllLabels()[tagName]
		assert.True(t, nodeHasLabel)

		apps, err := authServer.GetApplicationServers(ctx, tconf.SSH.Namespace)
		assert.NoError(t, err)
		assert.Len(t, apps, 1)

		app := apps[0].GetApp()
		_, appHasLabel := app.GetAllLabels()[tagName]
		assert.True(t, appHasLabel)

		databases, err := authServer.GetDatabaseServers(ctx, tconf.SSH.Namespace)
		assert.NoError(t, err)
		assert.Len(t, databases, 1)

		database := databases[0].GetDatabase()
		_, dbHasLabel := database.GetAllLabels()[tagName]
		assert.True(t, dbHasLabel)

		kubeResources, err := apiclient.GetResourcesWithFilters(
			context.Background(), authServer,
			proto.ListResourcesRequest{ResourceType: types.KindKubeServer},
		)
		assert.NoError(t, err)
		assert.Len(t, kubeResources, 1)

		kubeServers, err := types.ResourcesWithLabels(kubeResources).AsKubeServers()
		assert.NoError(t, err)

		kube := kubeServers[0].GetCluster()
		_, kubeHasLabel := kube.GetStaticLabels()[tagName]
		assert.True(t, kubeHasLabel)
	}, 10*time.Second, time.Second)
}

// TestEC2Hostname is an integration test which asserts that Teleport sets its
// hostname if the EC2 tag `TeleportHostname` is available.
func TestEC2Hostname(t *testing.T) {
	teleportHostname := "fakehost.example.com"

	storageConfig := backend.Config{
		Type: lite.GetName(),
		Params: backend.Params{
			"path":               t.TempDir(),
			"poll_stream_period": 50 * time.Millisecond,
		},
	}
	tconf := servicecfg.MakeDefaultConfig()
	tconf.Logger = slog.New(slog.DiscardHandler)
	tconf.DataDir = t.TempDir()
	tconf.Auth.Enabled = true
	tconf.Proxy.Enabled = true
	tconf.Proxy.DisableWebInterface = true
	tconf.Proxy.SSHAddr.Addr = helpers.NewListener(t, service.ListenerProxySSH, &tconf.FileDescriptors)
	tconf.Proxy.WebAddr.Addr = helpers.NewListener(t, service.ListenerProxyWeb, &tconf.FileDescriptors)
	tconf.Auth.StorageConfig = storageConfig
	tconf.Auth.ListenAddr.Addr = helpers.NewListener(t, service.ListenerAuth, &tconf.FileDescriptors)
	tconf.SetAuthServerAddress(tconf.Auth.ListenAddr)

	tconf.SSH.Enabled = true
	tconf.SSH.Addr.Addr = helpers.NewListener(t, service.ListenerNodeSSH, &tconf.FileDescriptors)

	tconf.InstanceMetadataClient = &mockIMDSClient{
		tags: map[string]string{
			types.CloudHostnameTag: teleportHostname,
		},
	}

	proc, err := service.NewTeleport(tconf)
	require.NoError(t, err)
	require.NoError(t, proc.Start())
	t.Cleanup(func() {
		require.NoError(t, proc.Close())
		require.NoError(t, proc.Wait())
	})
	require.Equal(t, teleportHostname, proc.Config.Hostname)
}
