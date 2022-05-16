package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// TestClientWithExpiredCredentialsAndDetailedErrorMessage creates and connects to the Auth service
// using an expired user identity
// We should receive an error message which contains the real cause (ssh: handshake)
func TestClientWithExpiredCredentialsAndDetailedErrorMessage(t *testing.T) {
	rc := NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		log:         utils.NewLoggerForTests(),
		Ports:       singleProxyPortSetup(),
	})

	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = true
	rcConf.Version = "v2"

	username := mustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	err := rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)
	err = rc.Start()
	require.NoError(t, err)
	defer rc.StopAll()

	// Create an expired identity file: ttl is 1 second in the past
	identityFilePath := mustCreateUserIdentityFile(t, rc, username, -time.Second)

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	defer cancelFunc()
	_, err = client.New(ctx, client.Config{
		Addrs:       []string{rc.GetAuthAddr()},
		Credentials: []client.Credentials{client.LoadIdentityFile(identityFilePath)},
		DialOpts: []grpc.DialOption{
			// ask for underlying errors
			grpc.WithReturnConnectionError(),
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ssh: handshake failed")
}
