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

package daemon

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/teleterm/gatewaytest"
	"github.com/gravitational/teleport/lib/tlsca"
)

type mockGatewayCreator struct {
	t         *testing.T
	callCount int
}

func (m *mockGatewayCreator) CreateGateway(ctx context.Context, params clusters.CreateGatewayParams) (*gateway.Gateway, error) {
	m.callCount++

	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	m.t.Cleanup(func() {
		hs.Close()
	})

	resourceURI := uri.New(params.TargetURI)

	keyPairPaths := gatewaytest.MustGenAndSaveCert(m.t, tlsca.Identity{
		Username: params.TargetUser,
		Groups:   []string{"test-group"},
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: resourceURI.GetDbName(),
			Protocol:    defaults.ProtocolPostgres,
			Username:    params.TargetUser,
		},
	})

	gateway, err := gateway.New(gateway.Config{
		LocalPort:             params.LocalPort,
		TargetURI:             params.TargetURI,
		TargetUser:            params.TargetUser,
		TargetName:            params.TargetURI,
		TargetSubresourceName: params.TargetSubresourceName,
		Protocol:              defaults.ProtocolPostgres,
		CertPath:              keyPairPaths.CertPath,
		KeyPath:               keyPairPaths.KeyPath,
		Insecure:              true,
		WebProxyAddr:          hs.Listener.Addr().String(),
		CLICommandProvider:    params.CLICommandProvider,
		TCPPortAllocator:      params.TCPPortAllocator,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m.t.Cleanup(func() {
		if err := gateway.Close(); err != nil {
			m.t.Logf("Ignoring error from gateway.Close() during cleanup, it appears the gateway was already closed. The error was: %s", err)
		}
	})

	return gateway, nil
}

type gatewayCRUDTestContext struct {
	nameToGateway        map[string]*gateway.Gateway
	mockGatewayCreator   *mockGatewayCreator
	mockTCPPortAllocator *gatewaytest.MockTCPPortAllocator
}

func TestGatewayCRUD(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		gatewayNamesToCreate []string
		// tcpPortAllocator is an optional field which lets us provide a custom
		// gatewaytest.MockTCPPortAllocator with some ports already in use.
		tcpPortAllocator *gatewaytest.MockTCPPortAllocator
		testFunc         func(*testing.T, *gatewayCRUDTestContext, *Service)
	}{
		{
			name:                 "create then find",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				createdGateway := c.nameToGateway["gateway"]
				foundGateway, err := daemon.findGateway(createdGateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, createdGateway, foundGateway)
			},
		},
		{
			name:                 "ListGateways",
			gatewayNamesToCreate: []string{"gateway1", "gateway2"},
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				gateways := daemon.ListGateways()
				gatewayURIs := map[uri.ResourceURI]struct{}{}

				for _, gateway := range gateways {
					gatewayURIs[gateway.URI()] = struct{}{}
				}

				require.Equal(t, 2, len(gateways))
				require.Contains(t, gatewayURIs, c.nameToGateway["gateway1"].URI())
				require.Contains(t, gatewayURIs, c.nameToGateway["gateway2"].URI())
			},
		},
		{
			name:                 "RemoveGateway",
			gatewayNamesToCreate: []string{"gatewayToRemove", "gatewayToKeep"},
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				gatewayToRemove := c.nameToGateway["gatewayToRemove"]
				gatewayToKeep := c.nameToGateway["gatewayToKeep"]
				err := daemon.RemoveGateway(gatewayToRemove.URI().String())
				require.NoError(t, err)

				_, err = daemon.findGateway(gatewayToRemove.URI().String())
				require.True(t, trace.IsNotFound(err), "gatewayToRemove wasn't removed")

				_, err = daemon.findGateway(gatewayToKeep.URI().String())
				require.NoError(t, err)
			},
		},
		{
			name:                 "SetGatewayLocalPort closes previous gateway if new port is free",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				oldGateway := c.nameToGateway["gateway"]
				oldListener := c.mockTCPPortAllocator.RecentListener()

				require.Equal(t, 0, oldListener.CloseCallCount)

				updatedGateway, err := daemon.SetGatewayLocalPort(oldGateway.URI().String(), "12345")
				require.NoError(t, err)
				require.Equal(t, "12345", updatedGateway.LocalPort())
				updatedGatewayAddress := c.mockTCPPortAllocator.RecentListener().RealAddr().String()

				// Check if the restarted gateway is still available under the same URI.
				foundGateway, err := daemon.findGateway(oldGateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, oldGateway.URI(), foundGateway.URI())

				// Verify that the gateway accepts connections on the new address.
				gatewaytest.BlockUntilGatewayAcceptsConnections(t, updatedGatewayAddress)

				// Verify that the old listener was closed.
				require.Equal(t, 1, oldListener.CloseCallCount)
			},
		},
		{
			name:                 "SetGatewayLocalPort doesn't close or modify previous gateway if new port is occupied",
			gatewayNamesToCreate: []string{"gateway"},
			tcpPortAllocator:     &gatewaytest.MockTCPPortAllocator{PortsInUse: []string{"12345"}},
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				gateway := c.nameToGateway["gateway"]
				gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())
				listener := c.mockTCPPortAllocator.RecentListener()

				require.Equal(t, 0, listener.CloseCallCount)

				_, err := daemon.SetGatewayLocalPort(gateway.URI().String(), "12345")
				require.ErrorContains(t, err, "address already in use")

				// Verify that the gateway still accepts connections on the old address.
				require.Equal(t, 0, listener.CloseCallCount)
				gatewaytest.BlockUntilGatewayAcceptsConnections(t, gatewayAddress)
			},
		},
		{
			name:                 "SetGatewayLocalPort is a noop if new port is equal to old port",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				gateway := c.nameToGateway["gateway"]
				localPort := gateway.LocalPort()
				require.Equal(t, 1, c.mockTCPPortAllocator.CallCount)

				_, err := daemon.SetGatewayLocalPort(gateway.URI().String(), localPort)
				require.NoError(t, err)

				require.Equal(t, 1, c.mockTCPPortAllocator.CallCount)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.tcpPortAllocator == nil {
				tt.tcpPortAllocator = &gatewaytest.MockTCPPortAllocator{}
			}

			homeDir := t.TempDir()
			mockGatewayCreator := &mockGatewayCreator{t: t}

			storage, err := clusters.NewStorage(clusters.Config{
				Dir:                homeDir,
				InsecureSkipVerify: true,
			})
			require.NoError(t, err)

			daemon, err := New(Config{
				Storage:          storage,
				GatewayCreator:   mockGatewayCreator,
				TCPPortAllocator: tt.tcpPortAllocator,
			})
			require.NoError(t, err)

			nameToGateway := make(map[string]*gateway.Gateway, len(tt.gatewayNamesToCreate))

			for _, gatewayName := range tt.gatewayNamesToCreate {
				gatewayName := gatewayName
				gateway, err := daemon.CreateGateway(context.Background(), CreateGatewayParams{
					TargetURI:             uri.NewClusterURI("foo").AppendDB(gatewayName).String(),
					TargetUser:            "alice",
					TargetSubresourceName: "",
					LocalPort:             "",
				})
				require.NoError(t, err)

				nameToGateway[gatewayName] = gateway
			}

			tt.testFunc(t, &gatewayCRUDTestContext{
				nameToGateway:        nameToGateway,
				mockGatewayCreator:   mockGatewayCreator,
				mockTCPPortAllocator: tt.tcpPortAllocator,
			}, daemon)
		})
	}
}

func TestUpdateTshdEventsServerAddress(t *testing.T) {
	homeDir := t.TempDir()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                homeDir,
		InsecureSkipVerify: true,
	})
	require.NoError(t, err)

	createTshdEventsClientCredsFuncCallCount := 0
	createTshdEventsClientCredsFunc := func() (grpc.DialOption, error) {
		createTshdEventsClientCredsFuncCallCount++
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	}

	gatewayCertReissuer := GatewayCertReissuer{Log: storage.Log}

	daemon, err := New(Config{
		Storage:                         storage,
		CreateTshdEventsClientCredsFunc: createTshdEventsClientCredsFunc,
		GatewayCertReissuer:             &gatewayCertReissuer,
	})
	require.NoError(t, err)

	ls, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { ls.Close() })

	err = daemon.UpdateAndDialTshdEventsServerAddress(ls.Addr().String())
	require.NoError(t, err)
	require.Equal(t, 1, createTshdEventsClientCredsFuncCallCount,
		"Expected createTshdEventsClientCredsFunc to be called exactly once")
}

func TestUpdateTshdEventsServerAddress_CredsErr(t *testing.T) {
	homeDir := t.TempDir()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                homeDir,
		InsecureSkipVerify: true,
	})
	require.NoError(t, err)

	createTshdEventsClientCredsFunc := func() (grpc.DialOption, error) {
		return nil, trace.Errorf("Error while creating creds")
	}

	daemon, err := New(Config{
		Storage:                         storage,
		CreateTshdEventsClientCredsFunc: createTshdEventsClientCredsFunc,
	})
	require.NoError(t, err)

	err = daemon.UpdateAndDialTshdEventsServerAddress("foo")
	require.ErrorContains(t, err, "Error while creating creds")
}
