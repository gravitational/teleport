// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"crypto"
	"crypto/tls"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/vnet"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

func TestEmbeddedVNet(t *testing.T) {
	// This is a high-level integration test for the EmbeddedVNet API. It asserts
	// that running the admin and user process parts of VNet in a single process,
	// embedded in another application (i.e. tbot), works broadly; but relies on
	// the existing VNet tests to cover the finer details.
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	// Start a fake ALPN-aware server to mimic the Teleport proxy.
	tlsCA := newSelfSignedCA(t)
	proxyDialOpts := mustStartFakeWebProxy(ctx, t, fakeWebProxyConfig{
		tlsCA: tlsCA,
		clock: clockwork.NewRealClock(),
	})
	proxyDialOpts.InsecureSkipVerify = true

	clientCert, err := newClientCert(ctx,
		tlsCA,
		"testclient",
		time.Now().Add(appCertLifetime),
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		cryptosuites.UserTLS,
	)
	require.NoError(t, err)

	// Create a fake host network.
	hostNetwork, err := NewFakeHostNetwork()
	require.NoError(t, err)
	t.Cleanup(hostNetwork.Close)

	// Start the EmbeddedVNet.
	vnet, err := NewEmbeddedVNet(EmbeddedVNetConfig{
		Device:        hostNetwork.TUNDevice(),
		ConfigureHost: hostNetwork.Configure,
		ApplicationService: &testEmbeddedApplicationService{
			appCert:  &clientCert,
			dialOpts: proxyDialOpts,
		},
	})
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- vnet.Run(ctx) }()

	// Wait for host configuration to complete.
	select {
	case <-hostNetwork.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for host configuration")
	}

	// Dial the echo server.
	conn, err := hostNetwork.ResolveAndDial(t.Context(), "tcp", "echo-server:1234")
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Write hello world to the echo server, and expect to get it back.
	_, err = conn.Write([]byte("hello world"))
	require.NoError(t, err)

	rsp := make([]byte, 11)
	_, err = conn.Read(rsp)
	require.NoError(t, err)
	require.Equal(t, "hello world", string(rsp))

	// Cancel the context and expect VNet to exit cleanly.
	cancel()
	require.NoError(t, <-errCh)
}

type testEmbeddedApplicationService struct {
	EmbeddedApplicationService

	appCert  *tls.Certificate
	dialOpts *vnetv1.DialOptions
}

func (s *testEmbeddedApplicationService) ResolveFQDN(ctx context.Context, fqdn string) (*vnetv1.ResolveFQDNResponse, error) {
	return &vnetv1.ResolveFQDNResponse{
		Match: &vnetv1.ResolveFQDNResponse_MatchedTcpApp{
			MatchedTcpApp: &vnetv1.MatchedTCPApp{
				AppInfo: &vnetv1.AppInfo{
					AppKey: &vnetv1.AppKey{
						Name: "echo-server",
					},
					App: &types.AppV3{
						Spec: types.AppSpecV3{
							URI: "tcp://this-address-does-not-matter:1234",
						},
					},
					Ipv4CidrRange: vnet.DefaultIPv4CIDRRange,
					DialOptions:   s.dialOpts,
				},
			},
		},
	}, nil
}

func (s *testEmbeddedApplicationService) GetTargetOSConfiguration(ctx context.Context) (*vnetv1.TargetOSConfiguration, error) {
	return &vnetv1.TargetOSConfiguration{
		DnsZones:       []string{"dunder-mifflin.com"},
		Ipv4CidrRanges: []string{vnet.DefaultIPv4CIDRRange},
	}, nil
}

func (s *testEmbeddedApplicationService) GetAppCert(context.Context, *vnetv1.AppInfo, uint16) (*tls.Certificate, error) {
	return s.appCert, nil
}

func (s *testEmbeddedApplicationService) GetAppSigner(context.Context, *vnetv1.AppKey, uint16) (crypto.Signer, error) {
	return s.appCert.PrivateKey.(crypto.Signer), nil
}
