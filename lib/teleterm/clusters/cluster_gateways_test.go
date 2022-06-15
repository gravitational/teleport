package clusters

import (
	"fmt"
	"testing"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"

	"github.com/stretchr/testify/require"
)

type mockCLICommandProvider struct{}

func (m mockCLICommandProvider) GetCommand(cluster *Cluster, gateway *gateway.Gateway) (*string, error) {
	command := fmt.Sprintf("%s/%s", gateway.TargetName, gateway.TargetSubresourceName)
	return &command, nil
}

func TestSetGatewayTargetSubresourceName(t *testing.T) {
	clusterClient := client.TeleportClient{Config: client.Config{}}

	cluster := Cluster{
		URI:                uri.NewClusterURI("test"),
		Name:               "test",
		clusterClient:      &clusterClient,
		cliCommandProvider: &mockCLICommandProvider{},
	}

	gateway := gateway.Gateway{
		CLICommand: "",
		Config: gateway.Config{
			TargetName: "foo",
			Protocol:   defaults.ProtocolPostgres,
		},
	}

	err := cluster.SetGatewayTargetSubresourceName(&gateway, "bar")
	require.NoError(t, err)

	require.Equal(t, "bar", gateway.TargetSubresourceName)
	require.Equal(t, "foo/bar", gateway.CLICommand)
}
