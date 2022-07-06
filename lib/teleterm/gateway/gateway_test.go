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

package gateway

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/stretchr/testify/require"
)

type mockCLICommandProvider struct{}

func (m mockCLICommandProvider) GetCommand(gateway *Gateway) (string, error) {
	command := fmt.Sprintf("%s/%s", gateway.TargetName, gateway.TargetSubresourceName)
	return command, nil
}

func TestCLICommandUsesCLICommandProvider(t *testing.T) {
	gateway := Gateway{
		Config: Config{
			TargetName:            "foo",
			TargetSubresourceName: "bar",
			Protocol:              defaults.ProtocolPostgres,
		},
		cliCommandProvider: mockCLICommandProvider{},
	}

	command, err := gateway.CLICommand()
	require.NoError(t, err)

	require.Equal(t, "foo/bar", command)
}

func TestGatewayStart(t *testing.T) {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})

	gateway, err := New(
		Config{
			TargetName:   "foo",
			TargetURI:    uri.NewClusterURI("bar").AppendDB("foo").String(),
			TargetUser:   "alice",
			Protocol:     defaults.ProtocolPostgres,
			CertPath:     "../../../fixtures/certs/proxy1.pem",
			KeyPath:      "../../../fixtures/certs/proxy1-key.pem",
			Insecure:     true,
			WebProxyAddr: hs.Listener.Addr().String(),
		},
		mockCLICommandProvider{},
	)
	require.NoError(t, err)

	openErr := make(chan error)

	go func() {
		err := gateway.Open()
		openErr <- err
	}()

	// Dial to make sure gateway is open.
	gatewayAddress := net.JoinHostPort(gateway.LocalAddress, gateway.LocalPort)
	_, err = net.DialTimeout("tcp", gatewayAddress, time.Second*1)
	require.NoError(t, err)

	err = gateway.Close()
	require.NoError(t, err)
	require.NoError(t, <-openErr)
}
