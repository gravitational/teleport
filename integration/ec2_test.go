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
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func newNodeConfig(t *testing.T, authAddr utils.NetAddr, awsTokenName string) *service.Config {
	config := service.MakeDefaultConfig()
	config.Token = awsTokenName
	config.JoinMethod = types.JoinMethodEC2
	config.SSH.Enabled = true
	config.SSH.Addr.Addr = net.JoinHostPort(Host, ports.Pop())
	config.Auth.Enabled = false
	config.Proxy.Enabled = false
	config.DataDir = t.TempDir()
	config.AuthServers = append(config.AuthServers, authAddr)
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
	config.Auth.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        Host,
		},
	}
	config.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "testcluster",
	})
	require.NoError(t, err)
	config.AuthServers = append(config.AuthServers, config.Auth.SSHAddr)
	config.Auth.StorageConfig = storageConfig
	config.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	require.NoError(t, err)
	config.Proxy.Enabled = false
	config.SSH.Enabled = false
	config.Clock = clock
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

// TestSimplifiedNodeJoin is an integration test which asserts that Simplified
// Node Joining works when run on a real EC2 instance with access to the IMDS
// and the ec2:DesribeInstances API.
// This is a very basic test, unit testing with mocked AWS endopoints is in
// lib/auth/ec2_join_test.go
func TestSimplifiedNodeJoin(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_EC2") == "" {
		t.Skipf("Skipping TestSimplifiedNodeJoin because TELEPORT_TEST_EC2 is not set")
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
	nodes, err := authServer.GetNodes(context.Background(), defaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, nodes)

	// create and start the node
	nodeConfig := newNodeConfig(t, authConfig.Auth.SSHAddr, tokenName)
	nodeSvc, err := service.NewTeleport(nodeConfig)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Start())
	t.Cleanup(func() { require.NoError(t, nodeSvc.Close()) })

	// the node should eventually join the cluster and heartbeat
	require.Eventually(t, func() bool {
		nodes, err := authServer.GetNodes(context.Background(), defaults.Namespace)
		require.NoError(t, err)
		return len(nodes) > 0
	}, time.Minute, time.Second, "waiting for node to join cluster")
}
