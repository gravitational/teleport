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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
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

	gateway, err := gateway.New(gateway.Config{
		LocalPort:             params.LocalPort,
		TargetURI:             params.TargetURI,
		TargetUser:            params.TargetUser,
		TargetName:            params.TargetURI,
		TargetSubresourceName: params.TargetSubresourceName,
		Protocol:              defaults.ProtocolPostgres,
		CertPath:              "../../../fixtures/certs/proxy1.pem",
		KeyPath:               "../../../fixtures/certs/proxy1-key.pem",
		Insecure:              true,
		WebProxyAddr:          hs.Listener.Addr().String(),
		CLICommandProvider:    params.CLICommandProvider,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m.t.Cleanup(func() {
		gateway.Close()
	})

	return gateway, nil
}

func TestGatewayCRUD(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		gatewayNamesToCreate []string
		testFunc             func(*testing.T, map[string]*gateway.Gateway, *mockGatewayCreator, *Service)
	}{
		{
			name:                 "create then find",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(
				t *testing.T, nameToGateway map[string]*gateway.Gateway, mockGatewayCreator *mockGatewayCreator, daemon *Service,
			) {
				createdGateway := nameToGateway["gateway"]
				foundGateway, err := daemon.findGateway(createdGateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, createdGateway, foundGateway)
			},
		},
		{
			name:                 "ListGateways",
			gatewayNamesToCreate: []string{"gateway1", "gateway2"},
			testFunc: func(
				t *testing.T, nameToGateway map[string]*gateway.Gateway, mockGatewayCreator *mockGatewayCreator, daemon *Service,
			) {
				gateways := daemon.ListGateways()
				gatewayURIs := map[uri.ResourceURI]struct{}{}

				for _, gateway := range gateways {
					gatewayURIs[gateway.URI()] = struct{}{}
				}

				require.Equal(t, 2, len(gateways))
				require.Contains(t, gatewayURIs, nameToGateway["gateway1"].URI())
				require.Contains(t, gatewayURIs, nameToGateway["gateway2"].URI())
			},
		},
		{
			name:                 "RemoveGateway",
			gatewayNamesToCreate: []string{"gatewayToRemove", "gatewayToKeep"},
			testFunc: func(
				t *testing.T, nameToGateway map[string]*gateway.Gateway, mockGatewayCreator *mockGatewayCreator, daemon *Service,
			) {
				gatewayToRemove := nameToGateway["gatewayToRemove"]
				gatewayToKeep := nameToGateway["gatewayToKeep"]
				err := daemon.RemoveGateway(gatewayToRemove.URI().String())
				require.NoError(t, err)

				_, err = daemon.findGateway(gatewayToRemove.URI().String())
				require.True(t, trace.IsNotFound(err), "gatewayToRemove wasn't removed")

				_, err = daemon.findGateway(gatewayToKeep.URI().String())
				require.NoError(t, err)
			},
		},
		{
			name:                 "RestartGateway",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(
				t *testing.T, nameToGateway map[string]*gateway.Gateway, mockGatewayCreator *mockGatewayCreator, daemon *Service,
			) {
				gateway := nameToGateway["gateway"]
				require.Equal(t, 1, mockGatewayCreator.callCount)

				err := daemon.RestartGateway(context.Background(), gateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, 2, mockGatewayCreator.callCount)
				require.Equal(t, 1, len(daemon.gateways))

				// Check if the restarted gateway is still available under the same URI.
				_, err = daemon.findGateway(gateway.URI().String())
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			homeDir := t.TempDir()
			mockGatewayCreator := &mockGatewayCreator{t: t}

			storage, err := clusters.NewStorage(clusters.Config{
				Dir:                homeDir,
				InsecureSkipVerify: true,
			})
			require.NoError(t, err)

			daemon, err := New(Config{
				Storage:        storage,
				GatewayCreator: mockGatewayCreator,
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

			tt.testFunc(t, nameToGateway, mockGatewayCreator, daemon)
		})
	}
}
