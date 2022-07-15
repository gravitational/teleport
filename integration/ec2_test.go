/*
Copyright 2021 Gravitational, Inc.

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

package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/breaker"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/labels/ec2"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func newSilentLogger() utils.Logger {
	logger := utils.NewLoggerForTests()
	logger.SetLevel(logrus.PanicLevel)
	logger.SetOutput(io.Discard)
	return logger
}

func newNodeConfig(t *testing.T, authAddr utils.NetAddr, tokenName string, joinMethod types.JoinMethod) *service.Config {
	config := service.MakeDefaultConfig()
	config.Token = tokenName
	config.JoinMethod = joinMethod
	config.SSH.Enabled = true
	config.SSH.Addr.Addr = net.JoinHostPort(Host, ports.Pop())
	config.Auth.Enabled = false
	config.Proxy.Enabled = false
	config.DataDir = t.TempDir()
	config.AuthServers = append(config.AuthServers, authAddr)
	config.Log = newSilentLogger()
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	return config
}

func newProxyConfig(t *testing.T, authAddr utils.NetAddr, tokenName string, joinMethod types.JoinMethod) *service.Config {
	config := service.MakeDefaultConfig()
	config.Version = defaults.TeleportConfigVersionV2
	config.Token = tokenName
	config.JoinMethod = joinMethod
	config.SSH.Enabled = false
	config.Auth.Enabled = false

	proxyAddr := net.JoinHostPort(Host, ports.Pop())
	config.Proxy.Enabled = true
	config.Proxy.DisableWebInterface = true
	config.Proxy.WebAddr.Addr = proxyAddr
	config.Proxy.EnableProxyProtocol = true

	config.DataDir = t.TempDir()
	config.AuthServers = append(config.AuthServers, authAddr)
	config.Log = newSilentLogger()
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	return config
}

func newAuthConfig(t *testing.T, clock clockwork.Clock) *service.Config {
	var err error
	storageConfig := backend.Config{
		Type: lite.GetName(),
		Params: backend.Params{
			"path":               t.TempDir(),
			"poll_stream_period": 50 * time.Millisecond,
		},
	}

	config := service.MakeDefaultConfig()
	config.DataDir = t.TempDir()
	config.Auth.SSHAddr.Addr = net.JoinHostPort(Host, ports.Pop())
	config.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "testcluster",
	})
	require.NoError(t, err)
	config.AuthServers = append(config.AuthServers, config.Auth.SSHAddr)
	config.Auth.StorageConfig = storageConfig
	config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	config.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	require.NoError(t, err)
	config.Proxy.Enabled = false
	config.SSH.Enabled = false
	config.Clock = clock
	config.Log = newSilentLogger()
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	return config
}

func getIID(t *testing.T) imds.InstanceIdentityDocument {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	require.NoError(t, err)
	imdsClient := imds.NewFromConfig(cfg)
	output, err := imdsClient.GetInstanceIdentityDocument(context.TODO(), nil)
	require.NoError(t, err)
	return output.InstanceIdentityDocument
}

func getCallerIdentity(t *testing.T) *sts.GetCallerIdentityOutput {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	require.NoError(t, err)
	stsService := sts.New(sess)
	output, err := stsService.GetCallerIdentity(nil /*input*/)
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

	// fetch the IID to create a token which will match this instance
	iid := getIID(t)

	tokenName := "test_token"
	token, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(time.Hour),
		types.ProvisionTokenSpecV2{
			Roles: []types.SystemRole{types.RoleNode},
			Allow: []*types.TokenRule{
				&types.TokenRule{
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

	err = authServer.UpsertToken(context.Background(), token)
	require.NoError(t, err)

	// sanity check there are no nodes to start with
	nodes, err := authServer.GetNodes(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, nodes)

	// create and start the node
	nodeConfig := newNodeConfig(t, authConfig.Auth.SSHAddr, tokenName, types.JoinMethodEC2)
	nodeSvc, err := service.NewTeleport(nodeConfig)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Start())
	t.Cleanup(func() { require.NoError(t, nodeSvc.Close()) })

	eventCh := make(chan service.Event, 1)
	nodeSvc.WaitForEvent(nodeSvc.ExitContext(), service.TeleportReadyEvent, eventCh)
	select {
	case <-eventCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for node readiness")
	}

	// the node should eventually join the cluster and heartbeat
	require.Eventually(t, func() bool {
		nodes, err := authServer.GetNodes(context.Background(), apidefaults.Namespace)
		require.NoError(t, err)
		return len(nodes) > 0
	}, time.Minute, time.Second, "waiting for node to join cluster")
}

// TestIAMNodeJoin is an integration test which asserts that the IAM method for
// Simplified Node Joining works when run on a real EC2 instance with an
// attached IAM role.  This is a very basic test, unit testing with mocked AWS
// endpoints is in lib/auth/join_iam_test.go
func TestIAMNodeJoin(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_EC2") == "" {
		t.Skipf("Skipping TestIAMNodeJoin because TELEPORT_TEST_EC2 is not set")
	}

	// create and start the auth server
	authConfig := newAuthConfig(t, nil /*clock*/)
	authSvc, err := service.NewTeleport(authConfig)
	require.NoError(t, err)
	require.NoError(t, authSvc.Start())
	t.Cleanup(func() { require.NoError(t, authSvc.Close()) })

	authServer := authSvc.GetAuthServer()

	// fetch the caller identity to find the AWS account and create the token
	id := getCallerIdentity(t)

	tokenName := "test_token"
	token, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(time.Hour),
		types.ProvisionTokenSpecV2{
			Roles: []types.SystemRole{types.RoleNode, types.RoleProxy},
			Allow: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: *id.Account,
				},
			},
			JoinMethod: types.JoinMethodIAM,
		})
	require.NoError(t, err)

	err = authServer.UpsertToken(context.Background(), token)
	require.NoError(t, err)

	// sanity check there are no proxies to start with
	proxies, err := authServer.GetProxies()
	require.NoError(t, err)
	require.Empty(t, proxies)

	// create and start the proxy, will use the IAM method to join by connecting
	// directly to the auth server
	proxyConfig := newProxyConfig(t, authConfig.Auth.SSHAddr, tokenName, types.JoinMethodIAM)
	proxySvc, err := service.NewTeleport(proxyConfig)
	require.NoError(t, err)
	require.NoError(t, proxySvc.Start())
	t.Cleanup(func() { require.NoError(t, proxySvc.Close()) })

	// the proxy should eventually join the cluster and heartbeat
	require.Eventually(t, func() bool {
		proxies, err := authServer.GetProxies()
		require.NoError(t, err)
		return len(proxies) > 0
	}, time.Minute, time.Second, "waiting for proxy to join cluster")

	// InsecureDevMode needed for node to trust proxy
	wasInsecureDevMode := lib.IsInsecureDevMode()
	t.Cleanup(func() { lib.SetInsecureDevMode(wasInsecureDevMode) })
	lib.SetInsecureDevMode(true)

	// sanity check there are no nodes to start with
	nodes, err := authServer.GetNodes(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, nodes)

	// create and start a node, with use the IAM method to join in IoT mode by
	// connecting to the proxy
	nodeConfig := newNodeConfig(t, proxyConfig.Proxy.WebAddr, tokenName, types.JoinMethodIAM)
	nodeSvc, err := service.NewTeleport(nodeConfig)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Start())
	t.Cleanup(func() { require.NoError(t, nodeSvc.Close()) })

	// the node should eventually join the cluster and heartbeat
	require.Eventually(t, func() bool {
		nodes, err := authServer.GetNodes(context.Background(), apidefaults.Namespace)
		require.NoError(t, err)
		return len(nodes) > 0
	}, time.Minute, time.Second, "waiting for node to join cluster")
}

type mockIMDSClient struct {
	tags map[string]string
}

func (m *mockIMDSClient) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *mockIMDSClient) GetTagKeys(ctx context.Context) ([]string, error) {
	keys := make([]string, 0, len(m.tags))
	for k := range m.tags {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockIMDSClient) GetTagValue(ctx context.Context, key string) (string, error) {
	if value, ok := m.tags[key]; ok {
		return value, nil
	}
	return "", trace.NotFound("Tag %q not found", key)
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
	tconf := service.MakeDefaultConfig()
	tconf.Log = newSilentLogger()
	tconf.DataDir = t.TempDir()
	tconf.Auth.Enabled = true
	tconf.Proxy.Enabled = true
	tconf.Proxy.DisableWebInterface = true
	tconf.Auth.StorageConfig = storageConfig
	tconf.Auth.SSHAddr.Addr = net.JoinHostPort(Host, ports.Pop())
	tconf.AuthServers = append(tconf.AuthServers, tconf.Auth.SSHAddr)

	tconf.SSH.Enabled = true
	tconf.SSH.Addr.Addr = net.JoinHostPort(Host, ports.Pop())

	appConf := service.App{
		Name: "test-app",
		URI:  "app.example.com",
	}

	tconf.Apps.Enabled = true
	tconf.Apps.Apps = []service.App{appConf}

	dbConfig := service.Database{
		Name:     "test-db",
		Protocol: "postgres",
		URI:      "postgres://somewhere.example.com",
	}
	tconf.Databases.Enabled = true
	tconf.Databases.Databases = []service.Database{dbConfig}

	enableKubernetesService(t, tconf)

	imClient := &mockIMDSClient{
		tags: map[string]string{
			"Name": "my-instance",
		},
	}

	proc, err := service.NewTeleport(tconf, service.WithIMDSClient(imClient))
	require.NoError(t, err)
	require.NoError(t, proc.Start())
	t.Cleanup(func() { require.NoError(t, proc.Close()) })

	ctx := context.Background()
	authServer := proc.GetAuthServer()

	var nodes []types.Server
	var apps []types.AppServer
	var databases []types.DatabaseServer
	var kubes []types.Server

	// Wait for everything to come online.
	require.Eventually(t, func() bool {
		var err error
		nodes, err = authServer.GetNodes(ctx, tconf.SSH.Namespace)
		require.NoError(t, err)
		apps, err = authServer.GetApplicationServers(ctx, tconf.SSH.Namespace)
		require.NoError(t, err)
		databases, err = authServer.GetDatabaseServers(ctx, tconf.SSH.Namespace)
		require.NoError(t, err)
		kubes, err = authServer.GetKubeServices(ctx)
		require.NoError(t, err)
		return len(nodes) == 1 && len(apps) == 1 && len(databases) == 1 && len(kubes) == 1
	}, 10*time.Second, time.Second)

	tagName := fmt.Sprintf("%s/Name", ec2.AWSNamespace)

	// Check that EC2 labels were applied.
	require.Eventually(t, func() bool {
		node, err := authServer.GetNode(ctx, tconf.SSH.Namespace, nodes[0].GetName())
		require.NoError(t, err)
		_, nodeHasLabel := node.GetAllLabels()[tagName]
		apps, err := authServer.GetApplicationServers(ctx, tconf.SSH.Namespace)
		require.NoError(t, err)
		require.Len(t, apps, 1)
		app := apps[0].GetApp()
		_, appHasLabel := app.GetAllLabels()[tagName]

		databases, err := authServer.GetDatabaseServers(ctx, tconf.SSH.Namespace)
		require.NoError(t, err)
		require.Len(t, databases, 1)
		database := databases[0].GetDatabase()
		_, dbHasLabel := database.GetAllLabels()[tagName]

		kubeClusters := getKubeClusters(t, authServer)
		require.Len(t, kubeClusters, 1)
		kube := kubeClusters[0]
		_, kubeHasLabel := kube.StaticLabels[tagName]
		return nodeHasLabel && appHasLabel && dbHasLabel && kubeHasLabel
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
	tconf := service.MakeDefaultConfig()
	tconf.Log = newSilentLogger()
	tconf.DataDir = t.TempDir()
	tconf.Auth.Enabled = true
	tconf.Proxy.Enabled = true
	tconf.Proxy.DisableWebInterface = true
	tconf.Auth.StorageConfig = storageConfig
	tconf.Auth.SSHAddr.Addr = net.JoinHostPort(Host, ports.Pop())
	tconf.AuthServers = append(tconf.AuthServers, tconf.Auth.SSHAddr)

	tconf.SSH.Enabled = true
	tconf.SSH.Addr.Addr = net.JoinHostPort(Host, ports.Pop())

	imClient := &mockIMDSClient{
		tags: map[string]string{
			types.EC2HostnameTag: teleportHostname,
		},
	}

	proc, err := service.NewTeleport(tconf, service.WithIMDSClient(imClient))
	require.NoError(t, err)
	require.NoError(t, proc.Start())
	t.Cleanup(func() { require.NoError(t, proc.Close()) })

	ctx := context.Background()
	authServer := proc.GetAuthServer()
	var node types.Server
	require.Eventually(t, func() bool {
		nodes, err := authServer.GetNodes(ctx, tconf.SSH.Namespace)
		require.NoError(t, err)
		if len(nodes) == 1 {
			node = nodes[0]
			return true
		}
		return false
	}, 10*time.Second, time.Second)

	require.Equal(t, teleportHostname, node.GetHostname())
}
